package storage_lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testBasicLockUnlock 测试最基本的加锁和解锁流程
func testBasicLockUnlock(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "basic")
	defer cleanup()

	ownerId := "owner-basic"

	// 加锁应该成功
	err := lock.Lock(ctx, ownerId)
	assert.Nil(t, err, "Lock should succeed")

	// 解锁应该成功
	err = lock.UnLock(ctx, ownerId)
	assert.Nil(t, err, "UnLock should succeed")
}

// testReacquireAfterRelease 测试锁释放后可以被人重新获取
func testReacquireAfterRelease(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "reacquire")
	defer cleanup()

	owner1 := "owner-1"

	// owner1 加锁
	err := lock.Lock(ctx, owner1)
	assert.Nil(t, err)

	// owner1 解锁
	err = lock.UnLock(ctx, owner1)
	assert.Nil(t, err)

	// owner2 应该能立即获取到锁
	owner2 := "owner-2"
	err = lock.Lock(ctx, owner2)
	assert.Nil(t, err, "Lock should be acquirable after release")

	err = lock.UnLock(ctx, owner2)
	assert.Nil(t, err)
}

// testMutexExclusion 测试互斥性：一个 owner 持有锁时，另一个 owner 无法获取
func testMutexExclusion(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "mutex")
	defer cleanup()

	owner1 := "owner-mutex-1"

	// owner1 持有锁
	err := lock.Lock(ctx, owner1)
	assert.Nil(t, err)

	// owner2 尝试获取锁，使用短超时应该失败
	owner2 := "owner-mutex-2"
	shortCtx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()
	err = lock.Lock(shortCtx, owner2)
	assert.NotNil(t, err, "owner2 should not acquire lock while owner1 holds it")

	// owner1 释放后，owner2 应该能获取
	err = lock.UnLock(ctx, owner1)
	assert.Nil(t, err)

	err = lock.Lock(ctx, owner2)
	assert.Nil(t, err, "owner2 should acquire lock after owner1 releases")
	err = lock.UnLock(ctx, owner2)
	assert.Nil(t, err)
}

// testNonOwnerUnlock 测试非持有者无法解锁
func testNonOwnerUnlock(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "nonowner")
	defer cleanup()

	owner1 := "owner-nonowner-1"
	owner2 := "owner-nonowner-2"

	err := lock.Lock(ctx, owner1)
	assert.Nil(t, err)

	// owner2 尝试解锁应该失败
	err = lock.UnLock(ctx, owner2)
	assert.NotNil(t, err, "non-owner should not unlock")

	// owner1 仍然可以正常解锁
	err = lock.UnLock(ctx, owner1)
	assert.Nil(t, err)
}

// testReentrantLock 测试可重入：同一 owner 多次 Lock，LockCount 递增，
// 需要多次 UnLock 才真正释放
func testReentrantLock(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "reentrant")
	defer cleanup()

	owner := "owner-reentrant"

	// 第一次加锁
	err := lock.Lock(ctx, owner)
	assert.Nil(t, err)

	// 第二次加锁（可重入）
	err = lock.Lock(ctx, owner)
	assert.Nil(t, err, "reentrant Lock should succeed")

	// 第三次加锁
	err = lock.Lock(ctx, owner)
	assert.Nil(t, err)

	// 此时其他 owner 仍然无法获取
	other := "owner-other"
	shortCtx, cancel := context.WithTimeout(ctx, time.Millisecond*300)
	defer cancel()
	err = lock.Lock(shortCtx, other)
	assert.NotNil(t, err, "other owner should not acquire while reentrant holder holds")

	// 第一次解锁，不应该真正释放（LockCount 还有 2）
	err = lock.UnLock(ctx, owner)
	assert.Nil(t, err)

	// 其他 owner 仍然无法获取
	shortCtx2, cancel2 := context.WithTimeout(ctx, time.Millisecond*300)
	defer cancel2()
	err = lock.Lock(shortCtx2, other)
	assert.NotNil(t, err, "other owner should not acquire after first unlock (LockCount still > 0)")

	// 第二次解锁
	err = lock.UnLock(ctx, owner)
	assert.Nil(t, err)

	// 第三次解锁，此时应该真正释放
	err = lock.UnLock(ctx, owner)
	assert.Nil(t, err)

	// 现在其他 owner 可以获取了
	err = lock.Lock(ctx, other)
	assert.Nil(t, err, "other owner should acquire after all reentrant unlocks")
	err = lock.UnLock(ctx, other)
	assert.Nil(t, err)
}
