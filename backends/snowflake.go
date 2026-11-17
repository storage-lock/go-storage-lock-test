//go:build storage_lock_snowflake

package backends

import (
	"context"

	snowflake_storage "github.com/storage-lock/go-snowflake-storage"
	sql_based_storage "github.com/storage-lock/go-sql-based-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("snowflake", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_SNOWFLAKE_DSN")
		if err != nil {
			return nil, err
		}
		options := sql_based_storage.NewSqlBasedStorageOptions().
			SetConnectionManager(storage.NewDsnConnectionManager("snowflake", dsn))
		return snowflake_storage.NewSnowflakeStorage(ctx, options)
	})
}
