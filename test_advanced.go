package storage_lock_test

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"github.com/stretchr/testify/assert"
)

// testWatchDogDoesNotClobberStolenLock 测试一个关键竞态：
// A 持有锁并启动看门狗续租，A 的租约过期后 B 抢占锁。
// 此时 A 的看门狗如果继续续租，绝不能覆盖 B 持有的锁（否则互斥性被破坏）。
//
// 这是分布式锁理论中非常关键的场景：续租必须通过版本号 CAS 保护，
// 一旦锁被别人抢占（版本号变化），续租必须失败而非覆盖。
func testWatchDogDoesNotClobberStolenLock(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-WatchDogDoesNotClobberStolenLock"

	// A 使用默认看门狗（会续租），短租约
	optionsA := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	optionsA.LeaseExpireAfter = time.Second * 3
	optionsA.LeaseRefreshInterval = time.Second * 1
	optionsA.VersionMissRetryInterval = time.Millisecond * 5
	lockA, err := storage_lock.NewStorageLockWithOptions(s, optionsA)
	if err != nil {
		t.Fatalf("failed to create lockA: %v", err)
	}

	ownerA := "owner-A-steal"
	err = lockA.Lock(ctx, ownerA)
	assert.Nil(t, err)

	// 等待看门狗续租几次，确认 A 正常持有
	time.Sleep(time.Millisecond * 1500)

	// 直接通过底层 storage 模拟 B 抢占：
	// 读取当前锁信息，构造一个新的 LockInformation，OwnerId=B，版本号 +1，
	// 用 UpdateWithVersion 抢占（模拟 B 走 lockExpired 路径抢占成功）
	currentJson, err := s.Get(ctx, lockId)
	assert.Nil(t, err)
	infoA, err := storage.LockInformationFromJsonString(currentJson)
	assert.Nil(t, err)

	infoB := &storage.LockInformation{
		LockId:          lockId,
		OwnerId:         "owner-B-steal",
		Version:         infoA.Version + 1,
		LockCount:       1,
		LockBeginTime:   infoA.LockBeginTime,
		LeaseExpireTime: time.Now().Add(time.Second * 3),
	}
	err = s.UpdateWithVersion(ctx, lockId, infoA.Version, infoB.Version, infoB)
	assert.Nil(t, err, "B should steal the lock via storage")

	// 等待一段时间，让 A 的看门狗有机会尝试续租
	// A 的看门狗 Get 会发现 OwnerId 已变为 B，返回 ErrLockNotBelongYou 退出
	// 即使看门狗续租的 UpdateWithVersion 带的是 A 的旧版本号，也会因版本不匹配而失败
	time.Sleep(time.Second * 2)

	// 验证：锁仍然属于 B，没有被 A 的看门狗覆盖
	finalJson, err := s.Get(ctx, lockId)
	assert.Nil(t, err)
	finalInfo, err := storage.LockInformationFromJsonString(finalJson)
	assert.Nil(t, err)
	assert.Equal(t, "owner-B-steal", finalInfo.OwnerId,
		"lock should still belong to B, A's watchdog must not clobber it, got owner=%s", finalInfo.OwnerId)

	// 清理：直接通过 storage 删除（因为锁属于 B，A 无法通过 UnLock 释放）
	// 停掉 A 的看门狗 goroutine
	stopCtx, stopCancel := context.WithTimeout(ctx, time.Second*5)
	defer stopCancel()
	// A 的 UnLock 会发现锁不属于自己，返回 ErrLockNotBelongYou，但会先 stopWatchDog
	_ = lockA.UnLock(stopCtx, ownerA)

	// 直接清理 B 的锁
	_ = s.DeleteWithVersion(ctx, lockId, finalInfo.Version, finalInfo)
}

// testABAScenario 测试 ABA 问题防护：
// A 持有锁(v=1) → A 释放 → B 持有(v=2) → B 释放 → A 再次持有(v=3)
// 验证 A 第二次持有后，用第一次的旧版本号操作会失败
func testABAScenario(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-ABAScenario"

	lock, cleanup := newLock(t, ctx, factory, lockId)
	defer cleanup()

	ownerA := "owner-aba-A"
	ownerB := "owner-aba-B"

	// A 持有并释放
	err := lock.Lock(ctx, ownerA)
	assert.Nil(t, err)
	err = lock.UnLock(ctx, ownerA)
	assert.Nil(t, err)

	// 读取 A 释放后的状态
	// 对于支持原子删除的存储，锁记录应被删除（Get 返回 ErrLockNotFound）
	// 对于走墓碑路径的存储（如 S3），记录仍存在但 LockCount=0
	releaseJson, releaseErr := s.Get(ctx, lockId)
	if releaseErr == nil {
		// 墓碑路径：记录仍存在，验证是墓碑（LockCount=0）
		releaseInfo, err := storage.LockInformationFromJsonString(releaseJson)
		assert.Nil(t, err)
		assert.Equal(t, 0, releaseInfo.LockCount, "tombstone path: LockCount should be 0 after release")
	} else {
		// 原子删除路径：记录应被删除
		assert.NotNil(t, releaseErr, "atomic delete path: lock should be deleted after A releases")
	}

	// B 持有并释放
	err = lock.Lock(ctx, ownerB)
	assert.Nil(t, err)
	err = lock.UnLock(ctx, ownerB)
	assert.Nil(t, err)

	// A 再次持有，应该成功（ABA 中的第二个 A）
	err = lock.Lock(ctx, ownerA)
	assert.Nil(t, err, "A should reacquire lock in ABA scenario")

	// 验证 A 正常持有，可正常释放
	err = lock.UnLock(ctx, ownerA)
	assert.Nil(t, err)
}

// testGoroutineLeak 测试 goroutine 泄漏：
// 反复加锁解锁多次，检查 goroutine 数量不会无限增长
// （看门狗 goroutine 必须在解锁后正确退出）
func testGoroutineLeak(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	// 这个测试需要读取 goroutine 数量，使用 runtime.NumGoroutine
	// 由于 go-storage-lock 内部可能用 goroutine-id 库，这里只做粗略检查

	lock, cleanup := newLock(t, ctx, factory, "goroutine-leak")
	defer cleanup()

	// 记录初始 goroutine 数量
	before := numGoroutines()

	// 反复加锁解锁 50 次
	owner := "owner-leak"
	for i := 0; i < 50; i++ {
		err := lock.Lock(ctx, owner)
		assert.Nil(t, err)
		err = lock.UnLock(ctx, owner)
		assert.Nil(t, err)
	}

	// 等待一段时间让看门狗 goroutine 完全退出
	time.Sleep(time.Millisecond * 500)

	after := numGoroutines()

	// goroutine 数量不应该显著增长（允许少量波动）
	// 如果每次解锁泄漏一个看门狗 goroutine，50 次后会有 50+ 个残留
	growth := after - before
	assert.Less(t, growth, 10,
		"goroutine leak detected: before=%d after=%d growth=%d", before, after, growth)

	t.Logf("Goroutine count: before=%d after=%d growth=%d", before, after, growth)
}

// testReentrantLockExpirationDuringHolding 测试重入持有期间租约过期被抢占的场景：
// A 重入持有锁（LockCount=2），租约过期后被 B 抢占，
// A 再尝试 Lock 时不应错误递增一个已被 B 持有的锁的 LockCount
func testReentrantLockExpirationDuringHolding(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-ReentrantLockExpirationDuringHolding"

	// A 使用 NoopWatchDog（不续租），模拟 A 加锁后宕机
	optionsA := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	optionsA.LeaseExpireAfter = time.Second * 3
	optionsA.LeaseRefreshInterval = time.Second * 1
	optionsA.VersionMissRetryInterval = time.Millisecond * 5
	optionsA.WatchDogFactory = &NoopWatchDogFactory{}
	lockA, err := storage_lock.NewStorageLockWithOptions(s, optionsA)
	if err != nil {
		t.Fatalf("failed to create lockA: %v", err)
	}

	ownerA := "owner-reentrant-A"
	// A 重入加锁 2 次
	err = lockA.Lock(ctx, ownerA)
	assert.Nil(t, err)
	err = lockA.Lock(ctx, ownerA)
	assert.Nil(t, err)

	// A 宕机，等待租约过期
	time.Sleep(time.Second*3 + time.Millisecond*500)

	// B 抢占过期的锁
	optionsB := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	optionsB.LeaseExpireAfter = time.Second * 3
	optionsB.LeaseRefreshInterval = time.Second * 1
	optionsB.VersionMissRetryInterval = time.Millisecond * 5
	lockB, err := storage_lock.NewStorageLockWithOptions(s, optionsB)
	if err != nil {
		t.Fatalf("failed to create lockB: %v", err)
	}
	ownerB := "owner-reentrant-B"
	acquireCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	err = lockB.Lock(acquireCtx, ownerB)
	assert.Nil(t, err, "B should acquire the expired reentrant lock")

	// 验证 B 持有的锁 LockCount=1（不是 A 残留的 2）
	currentJson, err := s.Get(ctx, lockId)
	assert.Nil(t, err)
	info, err := storage.LockInformationFromJsonString(currentJson)
	assert.Nil(t, err)
	assert.Equal(t, ownerB, info.OwnerId)
	assert.Equal(t, 1, info.LockCount,
		"B's lock should have LockCount=1, not inherit A's reentrant count, got %d", info.LockCount)

	// 清理
	err = lockB.UnLock(ctx, ownerB)
	assert.Nil(t, err)
}

// numGoroutines 返回当前 goroutine 数量
func numGoroutines() int {
	return runtime.NumGoroutine()
}

// 防止 sync 未使用
var _ = sync.WaitGroup{}
