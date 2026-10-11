package storage_lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/storage-lock/go-memory-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"github.com/stretchr/testify/assert"
)

// noAtomicDeleteStorage 包装 MemoryStorage，模拟"不支持原子条件删除"的存储介质
// （如对象存储 S3/OSS：只有条件 PUT If-Match ETag，没有条件 DELETE）。
// 它去掉 CapabilityAtomicDelete，并让 DeleteWithVersion 始终失败，
// 强制 unlockRelease 走墓碑降级路径（UpdateWithVersion 写 LockCount=0）。
type noAtomicDeleteStorage struct {
	storage.Storage
}

func (s *noAtomicDeleteStorage) Capabilities() []storage.StorageCapability {
	// 仅声明 CAS + 可靠时间，不声明原子删除
	return []storage.StorageCapability{
		storage.CapabilityCAS,
		storage.CapabilityReliableTime,
	}
}

func (s *noAtomicDeleteStorage) DeleteWithVersion(ctx context.Context, lockId string, exceptedVersion storage.Version, lockInformation *storage.LockInformation) error {
	// 模拟对象存储：不支持原子条件删除，调用即失败
	return storage_lock.ErrVersionMiss
}

// testTombstoneReleasePath 验证漏洞 B 的修复：
// 当存储不支持原子条件删除时，释放锁应降级为写墓碑（LockCount=0），
// 记录保留但逻辑已释放，下次 Lock 能正确识别墓碑并抢占，互斥性不被破坏。
//
// 这正是对象存储（S3/OSS）、纯 HTTP 存储等介质接入分布式锁的关键路径。
func testTombstoneReleasePath(t *testing.T) {
	ctx := context.Background()

	inner := memory_storage.NewMemoryStorage()
	s := &noAtomicDeleteStorage{Storage: inner}

	lockId := "test-TombstoneReleasePath"
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(s, options)
	assert.Nil(t, err, "应能创建锁：CAS 满足、时间满足，不要求 AtomicDelete")

	owner := "owner-tombstone"

	// 1. 加锁成功
	err = lock.Lock(ctx, owner)
	assert.Nil(t, err, "加锁应成功")

	// 2. 解锁：因不支持原子删除，应走墓碑路径，不报错
	err = lock.UnLock(ctx, owner)
	assert.Nil(t, err, "墓碑路径解锁应成功")

	// 3. 验证记录仍存在，且 LockCount==0（墓碑）
	jsonStr, err := inner.Get(ctx, lockId)
	assert.Nil(t, err, "墓碑路径下记录应保留")
	info, err := storage.LockInformationFromJsonString(jsonStr)
	assert.Nil(t, err)
	assert.Equal(t, 0, info.LockCount, "墓碑应 LockCount=0")

	// 4. 另一个 owner 应能再次加锁（识别墓碑走抢占路径）
	owner2 := "owner-tombstone-2"
	err = lock.Lock(ctx, owner2)
	assert.Nil(t, err, "应能抢占墓碑记录再次加锁")

	// 5. 验证互斥性：owner2 持有期间，owner3 不能加锁（ErrLockBusy 会重试，故用短超时 ctx）
	owner3 := "owner-tombstone-3"
	busyCtx, cancel := context.WithTimeout(ctx, time.Millisecond*200)
	defer cancel()
	err = lock.Lock(busyCtx, owner3)
	assert.NotNil(t, err, "墓碑抢占后互斥性应保持，owner3 不应获取到锁")

	// 清理
	_ = lock.UnLock(ctx, owner2)
}

// testTombstoneRLockReclaim 验证 RLock 对墓碑记录的抢占：
// 曾经的 BUG：tryRLock 遇到过期/墓碑（LockCount==0）记录时统一调用 rlockCreate，
// 而 rlockCreate 内部用 CreateWithVersion（只适用于记录真正不存在的场景）。对于不支持
// 原子删除、走墓碑释放的存储，记录一直都在，CreateWithVersion 只会不断返回
// ErrLockAlreadyExists，rlockCreate 对该错误的处理又是回退到 tryRLock 重新判断，
// 形成死循环，RLock 永远卡在 sleep-retry（context.Background() 不会超时，直接卡死）。
// 修复后 tryRLock 遇到过期/墓碑记录改为 rlockReclaim（UpdateWithVersion 抢占），与写锁
// lockExpired 的处理方式一致。
func testTombstoneRLockReclaim(t *testing.T) {
	ctx := context.Background()

	inner := memory_storage.NewMemoryStorage()
	s := &noAtomicDeleteStorage{Storage: inner}

	lockId := "test-TombstoneRLockReclaim"
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(s, options)
	assert.Nil(t, err)

	// 写锁加锁、解锁，留下墓碑记录（LockCount=0）
	writer := "owner-write"
	assert.Nil(t, lock.WLock(ctx, writer))
	assert.Nil(t, lock.WUnLock(ctx, writer))

	// 读锁应能立即抢占墓碑记录，而不是卡死在 sleep-retry 死循环里
	reader := "owner-read"
	done := make(chan error, 1)
	go func() {
		done <- lock.RLock(ctx, reader)
	}()
	select {
	case err := <-done:
		assert.Nil(t, err, "读锁应能抢占墓碑记录成功")
	case <-time.After(2 * time.Second):
		t.Fatal("RLock 抢占墓碑记录超时未返回 -- 死循环回归")
	}

	// 读锁释放后再走一次墓碑，验证写锁也能正常抢占读锁留下的墓碑
	assert.Nil(t, lock.RUnLock(ctx, reader))

	writer2 := "owner-write-2"
	done2 := make(chan error, 1)
	go func() {
		done2 <- lock.WLock(ctx, writer2)
	}()
	select {
	case err := <-done2:
		assert.Nil(t, err, "写锁应能抢占读锁留下的墓碑记录成功")
	case <-time.After(2 * time.Second):
		t.Fatal("WLock 抢占墓碑记录超时未返回")
	}
	_ = lock.WUnLock(ctx, writer2)
}
