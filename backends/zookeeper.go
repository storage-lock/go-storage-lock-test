//go:build storage_lock_zookeeper

package backends

import (
	"context"
	"strings"

	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
	zookeeper_storage "github.com/storage-lock/go-zookeeper-storage"
)

func init() {
	framework.RegisterBackend("zookeeper", func(ctx context.Context) (storage.Storage, error) {
		hosts, err := framework.BackendFromEnv("STORAGE_LOCK_ZOOKEEPER_HOSTS")
		if err != nil {
			return nil, err
		}
		return zookeeper_storage.NewZookeeperStorage(ctx, zookeeper_storage.NewZookeeperStorageOptionsWithHosts(strings.Split(hosts, ",")))
	})
}
