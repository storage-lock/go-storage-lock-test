package storage_lock_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	postgresql_storage "github.com/storage-lock/go-postgresql-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// PostgresqlStorageFactory 为 PostgresqlStorage 实现 StorageLockTestFactory
// postgres 有独立 SqlProvider（$ 占位符方言，非 Sql92），单独实证。
// 如果 STORAGE_LOCK_POSTGRESQL_DSN 环境变量为空则返回 nil（调用方应 t.Skip）。
type PostgresqlStorageFactory struct {
	storage *postgresql_storage.PostgresqlStorage
	db      *sql.DB
	table   string
}

func NewPostgresqlStorageFactory(t *testing.T) *PostgresqlStorageFactory {
	dsn := os.Getenv("STORAGE_LOCK_POSTGRESQL_DSN")
	if dsn == "" {
		return nil
	}
	ctx := context.Background()
	table := fmt.Sprintf("semantics_test_%d", time.Now().UnixNano())
	cm := postgresql_storage.NewPostgresqlConnectionGetterFromDsn(dsn)
	s, err := postgresql_storage.NewPostgresqlStorage(ctx, postgresql_storage.NewPostgresqlStorageOptions().
		SetConnectionManager(cm).SetTableName(table))
	if err != nil {
		t.Fatalf("failed to create PostgresqlStorage: %v", err)
	}
	db, err := cm.Take(ctx)
	if err != nil {
		t.Fatalf("failed to take postgresql conn: %v", err)
	}
	return &PostgresqlStorageFactory{storage: s, db: db, table: table}
}

func (f *PostgresqlStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *PostgresqlStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with PostgresqlStorage: %v", err)
	}
	return lock
}

func (f *PostgresqlStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
}

func (f *PostgresqlStorageFactory) Cleanup() {
	if f.db != nil {
		_, _ = f.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", f.table))
		_ = f.db.Close()
	}
}
