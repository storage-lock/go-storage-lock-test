//go:build storage_lock_dameng

package backends

import (
	"context"

	dameng_storage "github.com/storage-lock/go-dameng-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("dameng", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_DAMENG_DSN")
		if err != nil {
			return nil, err
		}
		manager := dameng_storage.NewDaMengConnectionManagerFromDSN(dsn)
		return dameng_storage.NewDaMengStorage(ctx, dameng_storage.NewDaMengStorageOptions().SetConnectionManager(manager))
	})
}
