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
