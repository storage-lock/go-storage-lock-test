package integration

import (
	"context"
	"os"
	"testing"
	"time"

	rocksdb_storage "github.com/storage-lock/go-rocksdb-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// RocksDBStorageFactory 为 RocksDBStorage 实现 StorageLockTestFactory
// RocksDB 是本轮修复 UpdateWithVersion 误报（NotFound→VersionMiss）的仓库之一，
// 有真实 grocksdb cgo 绑定（沙箱 librocksdb 可用），接入完整语义套件做实证验证。
type RocksDBStorageFactory struct {
	storage *rocksdb_storage.RocksDBStorage
	tmpDir  string
}

func NewRocksDBStorageFactory(t *testing.T) *RocksDBStorageFactory {
	tmpDir, err := os.MkdirTemp("", "storage-lock-rocksdb-semantics-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	ctx := context.Background()
	cm := rocksdb_storage.NewRocksDBConnectionManager(tmpDir)
	s, err := rocksdb_storage.NewRocksDBStorage(ctx, rocksdb_storage.NewRocksDBStorageOptions().SetConnectionManager(cm))
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create RocksDBStorage: %v", err)
	}
	return &RocksDBStorageFactory{storage: s, tmpDir: tmpDir}
}

func (f *RocksDBStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *RocksDBStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with RocksDBStorage: %v", err)
	}
	return lock
}

func (f *RocksDBStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
}

func (f *RocksDBStorageFactory) Cleanup() {
	ctx := context.Background()
	_ = f.storage.Close(ctx)
	os.RemoveAll(f.tmpDir)
}
