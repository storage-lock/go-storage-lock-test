//go:build storage_lock_oracle

package backends

import (
	"context"

	oracle_storage "github.com/storage-lock/go-oracle-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("oracle", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_ORACLE_DSN")
		if err != nil {
			return nil, err
		}
		manager := oracle_storage.NewOracleConnectionManagerFromDSN(dsn)
		return oracle_storage.NewOracleStorage(ctx, oracle_storage.NewOracleStorageOptions().SetConnectionManager(manager))
	})
}
