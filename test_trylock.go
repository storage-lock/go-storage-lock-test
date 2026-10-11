package storage_lock_test

import (
	"context"
	"errors"
	"testing"

	storage_lock "github.com/storage-lock/go-storage-lock"
)

// testTryLockSuccess 测试 TryLock 首次获取成功
func testTryLockSuccess(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "trylock-success")
	defer cleanup()

	ownerId := "owner-trylock-success"

	// TryLock 应该成功
	err := lock.TryLock(ctx, ownerId)
	if err != nil {
		t.Fatalf("TryLock should succeed on first attempt, got error: %v", err)
	}

	// 验证锁确实被持有（通过 TTL）
	ttl, err := lock.TTL(ctx, ownerId)
	if err != nil || ttl <= 0 {
		t.Fatalf("TTL should be positive after TryLock success, ttl=%v, err=%v", ttl, err)
	}

	// 清理
	_ = lock.UnLock(ctx, ownerId)
}

// testTryLockFailWhenLocked 测试当锁被他人持有时 TryLock 立即返回 ErrLockBusy
func testTryLockFailWhenLocked(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock1, cleanup1 := newLock(t, ctx, factory, "trylock-busy")
	defer cleanup1()

	lock2, cleanup2 := newLock(t, ctx, factory, "trylock-busy")
	defer cleanup2()

	owner1 := "owner-1"
	owner2 := "owner-2"

	// owner1 先获取锁
	err := lock1.Lock(ctx, owner1)
	if err != nil {
		t.Fatalf("lock1.Lock should succeed, got error: %v", err)
	}
	defer func() { _ = lock1.UnLock(ctx, owner1) }()

	// owner2 的 TryLock 应该立即失败，返回 ErrLockBusy
	err = lock2.TryLock(ctx, owner2)
	if !errors.Is(err, storage_lock.ErrLockBusy) {
		t.Fatalf("TryLock should return ErrLockBusy when lock is held by another owner, got: %v", err)
	}
}
