//go:build storage_lock_mysql

package backends

import (
	"context"

	mysql_storage "github.com/storage-lock/go-mysql-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("mysql", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_MYSQL_DSN")
		if err != nil {
			return nil, err
		}
		manager := mysql_storage.NewMysqlConnectionManagerFromDsn(dsn)
		return mysql_storage.NewMysqlStorage(ctx, mysql_storage.NewMysqlStorageOptions().SetConnectionManager(manager))
	})
}
