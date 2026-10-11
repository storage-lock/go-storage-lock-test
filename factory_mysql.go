package storage_lock_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	mysql_storage "github.com/storage-lock/go-mysql-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// MysqlStorageFactory 为 MysqlStorage 实现 StorageLockTestFactory
// mysql 是 SQL 家族（sql-based 基类派生 13 仓）的实证代表，走真实服务端验证。
// 如果 STORAGE_LOCK_MYSQL_DSN 环境变量为空则返回 nil（调用方应 t.Skip）。
type MysqlStorageFactory struct {
	storage *mysql_storage.MysqlStorage
	db      *sql.DB
	table   string
}

func NewMysqlStorageFactory(t *testing.T) *MysqlStorageFactory {
	dsn := os.Getenv("STORAGE_LOCK_MYSQL_DSN")
	if dsn == "" {
		return nil
	}
	ctx := context.Background()
	table := fmt.Sprintf("semantics_test_%d", time.Now().UnixNano())
	cm := mysql_storage.NewMysqlConnectionManagerFromDsn(dsn)
	s, err := mysql_storage.NewMysqlStorage(ctx, mysql_storage.NewMysqlStorageOptions().
		SetConnectionManager(cm).SetTableName(table))
	if err != nil {
		t.Fatalf("failed to create MysqlStorage: %v", err)
	}
	db, err := cm.Take(ctx)
	if err != nil {
		t.Fatalf("failed to take mysql conn: %v", err)
	}
	return &MysqlStorageFactory{storage: s, db: db, table: table}
}

func (f *MysqlStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *MysqlStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with MysqlStorage: %v", err)
	}
	return lock
}

func (f *MysqlStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
}

func (f *MysqlStorageFactory) Cleanup() {
	if f.db != nil {
		_, _ = f.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", f.table))
		_ = f.db.Close()
	}
}
