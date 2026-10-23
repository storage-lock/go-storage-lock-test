package storage_lock

import (
	"context"
	"testing"
	"time"

	memory_storage "github.com/storage-lock/go-memory-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// MemoryStorageFactory 为公开 memory backend 提供语义测试实例。
type MemoryStorageFactory struct {
	storage *memory_storage.MemoryStorage
}

func NewMemoryStorageFactory() *MemoryStorageFactory {
	return &MemoryStorageFactory{storage: memory_storage.NewMemoryStorage()}
}

func (f *MemoryStorageFactory) CreateStorage(context.Context, *testing.T) storage.Storage {
	return f.storage
}

func (f *MemoryStorageFactory) NewStorageLock(_ context.Context, t *testing.T, lockID string) *storage_lock.StorageLock {
	options := testLockOptions(lockID)
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("create memory StorageLock: %v", err)
	}
	return lock
}

func (*MemoryStorageFactory) CloseStorageLock(context.Context, *testing.T, *storage_lock.StorageLock) {
}

func testLockOptions(lockID string) *storage_lock.StorageLockOptions {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockID)
	options.LeaseExpireAfter = 3 * time.Second
	options.LeaseRefreshInterval = time.Second
	options.VersionMissRetryInterval = 5 * time.Millisecond
	return options
}
