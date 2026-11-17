//go:build storage_lock_mariadb

package backends

import (
	"context"

	mariadb_storage "github.com/storage-lock/go-mariadb-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("mariadb", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_MARIADB_DSN")
		if err != nil {
			return nil, err
		}
		manager := mariadb_storage.NewMariadbConnectionManagerFromDsn(dsn)
		return mariadb_storage.NewMariadbStorage(ctx, mariadb_storage.NewMariadbStorageOptions().SetConnectionManager(manager))
	})
}
