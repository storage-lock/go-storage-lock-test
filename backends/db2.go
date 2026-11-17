//go:build storage_lock_db2

package backends

import (
	"context"

	db2_storage "github.com/storage-lock/go-db2-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("db2", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_DB2_DSN")
		if err != nil {
			return nil, err
		}
		manager := db2_storage.NewDB2ConnectionManagerFromDSN(dsn)
		return db2_storage.NewDB2Storage(ctx, db2_storage.NewDB2StorageOptions().SetConnectionManager(manager))
	})
}
