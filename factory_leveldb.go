package storage_lock_test

import (
	"context"
	"os"
	"testing"
	"time"

	leveldb_storage "github.com/storage-lock/go-leveldb-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// LevelDBStorageFactory 为 LevelDBStorage 实现 StorageLockTestFactory
type LevelDBStorageFactory struct {
	tmpDir  string
	storage *leveldb_storage.LevelDBStorage
}

// NewLevelDBStorageFactory 创建一个基于 LevelDBStorage 的测试 factory
func NewLevelDBStorageFactory(t *testing.T) *LevelDBStorageFactory {
	tmpDir, err := os.MkdirTemp("", "storage-lock-leveldb-semantics-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ctx := context.Background()
	connectionManager := leveldb_storage.NewLevelDBConnectionManagerFromPath(tmpDir)
	s, err := leveldb_storage.NewLevelDBStorage(ctx, leveldb_storage.NewLevelDBStorageOptions().SetConnectionManager(connectionManager))
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create LevelDBStorage: %v", err)
	}

	return &LevelDBStorageFactory{
		tmpDir:  tmpDir,
		storage: s,
	}
}

func (f *LevelDBStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *LevelDBStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with LevelDBStorage: %v", err)
	}
	return lock
}

func (f *LevelDBStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// LevelDB 无需特殊清理
}

// Cleanup 清理临时目录，应在测试完成后调用
func (f *LevelDBStorageFactory) Cleanup() {
	_ = os.RemoveAll(f.tmpDir)
}
