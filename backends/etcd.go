//go:build storage_lock_etcd

package backends

import (
	"context"
	"strings"

	etcd_storage "github.com/storage-lock/go-etcd-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("etcd", func(ctx context.Context) (storage.Storage, error) {
		endpoints, err := framework.BackendFromEnv("STORAGE_LOCK_ETCD_ENDPOINTS")
		if err != nil {
			return nil, err
		}
		return etcd_storage.NewEtcdStorage(ctx, etcd_storage.NewEtcdStorageOptionsWithEndpoints(strings.Split(endpoints, ",")))
	})
}
