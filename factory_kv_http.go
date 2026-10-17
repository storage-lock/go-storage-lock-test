package storage_lock

import (
	"context"
	"testing"
	"time"

	http_storage "github.com/storage-lock/go-http-storage"
	kv_storage "github.com/storage-lock/go-kv-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// KVStorageFactory 为 KVStorage 实现 StorageLockTestFactory
// KV 是本轮修复 UpdateWithVersion 误报（NotFound→VersionMiss）的仓库之一，
// 接入完整语义套件做实证验证。
type KVStorageFactory struct {
	storage *kv_storage.KVStorage
}

func NewKVStorageFactory(t *testing.T) *KVStorageFactory {
	s, err := kv_storage.NewKVStorage(context.Background())
	if err != nil {
		t.Fatalf("failed to create KVStorage: %v", err)
	}
	return &KVStorageFactory{storage: s}
}

func (f *KVStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *KVStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with KVStorage: %v", err)
	}
	return lock
}

func (f *KVStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
}

// HTTPStorageFactory 为 HTTPStorage 实现 StorageLockTestFactory
// HTTP 是本轮修复 UpdateWithVersion 误报的另一仓库，同样接入完整语义套件。
type HTTPStorageFactory struct {
	storage *http_storage.HTTPStorage
}

func NewHTTPStorageFactory(t *testing.T) *HTTPStorageFactory {
	s, err := http_storage.NewHTTPStorage(context.Background())
	if err != nil {
		t.Fatalf("failed to create HTTPStorage: %v", err)
	}
	return &HTTPStorageFactory{storage: s}
}

func (f *HTTPStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *HTTPStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with HTTPStorage: %v", err)
	}
	return lock
}

func (f *HTTPStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
}
