//go:build storage_lock_firebird

package backends

import (
	"context"

	firebird_storage "github.com/storage-lock/go-firebird-storage"
	sql_based_storage "github.com/storage-lock/go-sql-based-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("firebird", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_FIREBIRD_DSN")
		if err != nil {
			return nil, err
		}
		options := sql_based_storage.NewSqlBasedStorageOptions().
			SetConnectionManager(storage.NewDsnConnectionManager("firebird", dsn))
		return firebird_storage.NewFirebirdStorage(ctx, options)
	})
}
