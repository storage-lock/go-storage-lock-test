package storage_lock

import (
	"context"
	"testing"
	"time"

	memory_storage "github.com/storage-lock/go-memory-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// MemoryStorageFactory 为 MemoryStorage 实现 StorageLockTestFactory
type MemoryStorageFactory struct {
	storage *memory_storage.MemoryStorage
}

// NewMemoryStorageFactory 创建一个基于 MemoryStorage 的测试 factory
func NewMemoryStorageFactory() *MemoryStorageFactory {
	return &MemoryStorageFactory{
		storage: memory_storage.NewMemoryStorage(),
	}
}

func (f *MemoryStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *MemoryStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	// 使用较短的租约和刷新间隔，便于测试过期场景
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with MemoryStorage: %v", err)
	}
	return lock
}

func (f *MemoryStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// MemoryStorage 无需特殊清理，但应确保锁被释放
	// 这里尝试用一个不存在的 owner 释放，主要是为了停止可能残留的看门狗
	// 真正的清理在测试用例内部完成
}
