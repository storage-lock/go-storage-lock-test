//go:build storage_lock_postgresql

package backends

import (
	"context"

	postgresql_storage "github.com/storage-lock/go-postgresql-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("postgresql", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_POSTGRESQL_DSN")
		if err != nil {
			return nil, err
		}
		manager := postgresql_storage.NewPostgresqlConnectionGetterFromDsn(dsn)
		return postgresql_storage.NewPostgresqlStorage(ctx, postgresql_storage.NewPostgresqlStorageOptions().SetConnectionManager(manager))
	})
}
