//go:build storage_lock_access

package backends

import (
	"context"

	access_storage "github.com/storage-lock/go-access-storage"
	sql_based_storage "github.com/storage-lock/go-sql-based-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("access", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_ACCESS_DSN")
		if err != nil {
			return nil, err
		}
		options := sql_based_storage.NewSqlBasedStorageOptions().
			SetConnectionManager(storage.NewDsnConnectionManager("access", dsn))
		return access_storage.NewAccessStorage(ctx, options)
	})
}
