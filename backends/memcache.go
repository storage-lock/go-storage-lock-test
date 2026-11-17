//go:build storage_lock_memcache

package backends

import (
	"context"
	"strings"

	memcache_storage "github.com/storage-lock/go-memcache-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("memcache", func(ctx context.Context) (storage.Storage, error) {
		servers, err := framework.BackendFromEnv("STORAGE_LOCK_MEMCACHE_SERVERS")
		if err != nil {
			return nil, err
		}
		return memcache_storage.NewMemcacheStorage(ctx, memcache_storage.NewMemcacheStorageOptionsWithServers(strings.Split(servers, ",")))
	})
}
