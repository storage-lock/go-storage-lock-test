package storage_lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"github.com/stretchr/testify/assert"
)

// testLeaseExpiration 测试租约过期：持有者"宕机"（使用 NoopWatchDog 不续租）后，
// 锁在租约到期后应该能被其他人抢占
func testLeaseExpiration(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)

	// 创建一个使用 NoopWatchDog 的锁实例，模拟持有者宕机后不续租
	// owner1 加锁后不续租，锁会在 LeaseExpireAfter 后自然过期
	lockId := "test-LeaseExpiration"
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3 // 短租约，3秒过期
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 50
	options.WatchDogFactory = &NoopWatchDogFactory{} // 不续租，模拟宕机

	owner1Lock, err := storage_lock.NewStorageLockWithOptions(s, options)
	if err != nil {
		t.Fatalf("failed to create owner1 lock: %v", err)
	}

	owner1 := "owner-lease-1"
	err = owner1Lock.Lock(ctx, owner1)
	assert.Nil(t, err, "owner1 should acquire lock")

	// owner1 "宕机"：不调用 UnLock，NoopWatchDog 也不续租
	// 等待租约自然过期（3秒 + 一点余量）
	time.Sleep(time.Second*3 + time.Millisecond*500)

	// owner2 现在应该能抢占过期的锁（走 lockExpired 路径）
	owner2 := "owner-lease-2"
	owner2Lock, _, err := createLockWithStorage(t, ctx, factory, lockId)
	if err != nil {
		t.Fatalf("failed to create owner2 lock: %v", err)
	}

	acquireCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	err = owner2Lock.Lock(acquireCtx, owner2)
	assert.Nil(t, err, "owner2 should acquire expired lock after owner1 crash")

	// 清理
	err = owner2Lock.UnLock(ctx, owner2)
	assert.Nil(t, err)
}

// testWatchDogRenewal 测试看门狗续租：持有锁期间，看门狗应该持续续租，
// 使得即使持有时间超过 LeaseExpireAfter，锁也不会过期，其他人无法抢占
func testWatchDogRenewal(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-WatchDogRenewal"

	lock, cleanup := newLock(t, ctx, factory, lockId)
	defer cleanup()

	owner := "owner-watchdog"

	// 加锁（使用默认看门狗，会续租）
	err := lock.Lock(ctx, owner)
	assert.Nil(t, err)

	// 等待时间超过 LeaseExpireAfter（3秒），如果看门狗正常工作，锁不会过期
	// 等待 4.5 秒，期间看门狗应该续租 4 次左右
	waitDuration := time.Millisecond * 4500
	time.Sleep(waitDuration)

	// 检查存储中的锁，租约过期时间应该仍然在未来（被看门狗续租了）
	currentJson, err := s.Get(ctx, lockId)
	assert.Nil(t, err)
	info, err := storage.LockInformationFromJsonString(currentJson)
	assert.Nil(t, err)

	// 获取存储时间用于判断
	storageTime, err := s.GetTime(ctx)
	assert.Nil(t, err)

	// 租约过期时间应该在存储当前时间之后（说明看门狗续租了，锁未过期）
	assert.True(t, info.LeaseExpireTime.After(storageTime),
		"lease should be renewed by watchdog, expireTime=%v, storageTime=%v",
		info.LeaseExpireTime, storageTime)

	// 在此期间，其他人仍然无法获取锁（因为锁未过期）
	other := "owner-watchdog-other"
	shortCtx, cancel2 := context.WithTimeout(ctx, time.Millisecond*300)
	defer cancel2()
	err = lock.Lock(shortCtx, other)
	assert.NotNil(t, err, "other should not acquire lock while watchdog keeps it alive")

	// 正常释放
	err = lock.UnLock(ctx, owner)
	assert.Nil(t, err)
}

// createLockWithStorage 用底层 storage 创建一个带默认看门狗的锁
func createLockWithStorage(t *testing.T, ctx context.Context, factory StorageLockTestFactory, lockId string) (*storage_lock.StorageLock, storage.Storage, error) {
	s := factory.CreateStorage(ctx, t)
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 50
	lock, err := storage_lock.NewStorageLockWithOptions(s, options)
	return lock, s, err
}
