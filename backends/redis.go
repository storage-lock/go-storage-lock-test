//go:build storage_lock_redis

package backends

import (
	"context"

	redis_storage "github.com/storage-lock/go-redis-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("redis", func(ctx context.Context) (storage.Storage, error) {
		uri, err := framework.BackendFromEnv("STORAGE_LOCK_REDIS_URI")
		if err != nil {
			return nil, err
		}
		return redis_storage.NewRedisStorage(ctx, redis_storage.NewRedisStorageOptionsWithURI(uri))
	})
}
