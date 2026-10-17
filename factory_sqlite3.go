package storage_lock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	sqlite3_storage "github.com/storage-lock/go-sqlite3-storage"
)

// Sqlite3StorageFactory 为 Sqlite3Storage 实现 StorageLockTestFactory
// sqlite3 是 SQL 家族第一个接入完整语义套件的代表（文件型，cgo mattn/go-sqlite3）。
// 接入时附带修复：UpdateWithVersion 的 WHERE 条件曾多带 owner_id，导致核心层
// 抢占过期/墓碑记录（传入新持有者信息）永远匹配不上，已对齐 sql-based 基类去掉。
type Sqlite3StorageFactory struct {
	storage *sqlite3_storage.Sqlite3Storage
	dbPath  string
}

func NewSqlite3StorageFactory(t *testing.T) *Sqlite3StorageFactory {
	dbPath := filepath.Join(t.TempDir(), "semantics-test.db3")
	connectionManager, err := sqlite3_storage.NewSqlite3ConnectionManager(dbPath)
	if err != nil {
		t.Fatalf("failed to create Sqlite3ConnectionManager: %v", err)
	}
	s, err := sqlite3_storage.NewSqlite3Storage(context.Background(), sqlite3_storage.NewSqlite3StorageOptions().SetConnectionManager(connectionManager))
	if err != nil {
		t.Fatalf("failed to create Sqlite3Storage: %v", err)
	}
	return &Sqlite3StorageFactory{storage: s, dbPath: dbPath}
}

func (f *Sqlite3StorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *Sqlite3StorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with Sqlite3Storage: %v", err)
	}
	return lock
}

func (f *Sqlite3StorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
}

func (f *Sqlite3StorageFactory) Cleanup() {
	_ = os.Remove(f.dbPath)
}
