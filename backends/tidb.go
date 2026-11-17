//go:build storage_lock_tidb

package backends

import (
	"context"

	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
	tidb_storage "github.com/storage-lock/go-tidb-storage"
)

func init() {
	framework.RegisterBackend("tidb", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_TIDB_DSN")
		if err != nil {
			return nil, err
		}
		manager := tidb_storage.NewTidbConnectionManagerFromDsn(dsn)
		return tidb_storage.NewTidbStorage(ctx, tidb_storage.NewTidbStorageOptions().SetConnectionManager(manager))
	})
}
