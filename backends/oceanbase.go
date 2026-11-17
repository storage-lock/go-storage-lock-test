//go:build storage_lock_oceanbase

package backends

import (
	"context"

	oceanbase_storage "github.com/storage-lock/go-oceanbase-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("oceanbase", func(ctx context.Context) (storage.Storage, error) {
		dsn, err := framework.BackendFromEnv("STORAGE_LOCK_OCEANBASE_DSN")
		if err != nil {
			return nil, err
		}
		manager := oceanbase_storage.NewOceanBaseConnectionManagerFromDSN(dsn)
		return oceanbase_storage.NewOceanBaseStorage(ctx, oceanbase_storage.NewOceanBaseStorageOptions().SetConnectionManager(manager))
	})
}
