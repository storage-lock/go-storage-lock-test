//go:build storage_lock_sqlserver

package backends

import (
	"context"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
	sqlserver_storage "github.com/storage-lock/go-sqlserver-storage"
)

func init() {
	framework.RegisterBackend("sqlserver", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_SQLSERVER_DSN")
		if err != nil {
			return nil, err
		}
		manager := sqlserver_storage.NewSqlServerConnectionManagerFromDsn(dsn)
		return sqlserver_storage.NewSqlServerStorage(ctx, sqlserver_storage.NewSqlServerStorageOptions().SetConnectionManager(manager))
	})
}
