//go:build storage_lock_percona

package backends

import (
	"context"

	_ "github.com/go-sql-driver/mysql"
	percona_storage "github.com/storage-lock/go-percona-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("percona", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_PERCONA_DSN")
		if err != nil {
			return nil, err
		}
		options := percona_storage.NewPerconaStorageOptions()
		options.ConnectionManager = storage.NewDsnConnectionManager("mysql", dsn)
		return percona_storage.NewPerconaStorage(ctx, options)
	})
}
