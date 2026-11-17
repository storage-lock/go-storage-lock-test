//go:build storage_lock_sybase

package backends

import (
	"context"

	sybase_storage "github.com/storage-lock/go-sybase-storage"
	sql_based_storage "github.com/storage-lock/go-sql-based-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("sybase", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_SYBASE_DSN")
		if err != nil {
			return nil, err
		}
		options := sql_based_storage.NewSqlBasedStorageOptions().
			SetConnectionManager(storage.NewDsnConnectionManager("sybase", dsn))
		return sybase_storage.NewSybaseStorage(ctx, options)
	})
}
