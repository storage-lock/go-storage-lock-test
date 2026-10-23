package storage_lock

import (
	"context"
	"path/filepath"
	"testing"

	sqlite3_storage "github.com/storage-lock/go-sqlite3-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// Sqlite3StorageFactory 为公开 sqlite3 backend 提供语义测试实例。
type Sqlite3StorageFactory struct {
	storage *sqlite3_storage.Sqlite3Storage
}

func NewSqlite3StorageFactory(t *testing.T) *Sqlite3StorageFactory {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "semantics-test.db3")
	manager, err := sqlite3_storage.NewSqlite3ConnectionManager(dbPath)
	if err != nil {
		t.Fatalf("create sqlite3 connection manager: %v", err)
	}
	store, err := sqlite3_storage.NewSqlite3Storage(context.Background(), sqlite3_storage.NewSqlite3StorageOptions().SetConnectionManager(manager))
	if err != nil {
		t.Fatalf("create sqlite3 storage: %v", err)
	}
	return &Sqlite3StorageFactory{storage: store}
}

func (f *Sqlite3StorageFactory) CreateStorage(context.Context, *testing.T) storage.Storage {
	return f.storage
}

func (f *Sqlite3StorageFactory) NewStorageLock(_ context.Context, t *testing.T, lockID string) *storage_lock.StorageLock {
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, testLockOptions(lockID))
	if err != nil {
		t.Fatalf("create sqlite3 StorageLock: %v", err)
	}
	return lock
}

func (*Sqlite3StorageFactory) CloseStorageLock(context.Context, *testing.T, *storage_lock.StorageLock) {
}
