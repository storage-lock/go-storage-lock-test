//go:build storage_lock_teradata

package backends

import (
	"context"

	teradata_storage "github.com/storage-lock/go-teradata-storage"
	sql_based_storage "github.com/storage-lock/go-sql-based-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("teradata", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_TERADATA_DSN")
		if err != nil {
			return nil, err
		}
		options := sql_based_storage.NewSqlBasedStorageOptions().
			SetConnectionManager(storage.NewDsnConnectionManager("teradata", dsn))
		return teradata_storage.NewTeradataStorage(ctx, options)
	})
}
