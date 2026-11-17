//go:build storage_lock_rocksdb

package backends

import (
	"context"
	"os"

	rocksdb_storage "github.com/storage-lock/go-rocksdb-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("rocksdb", func(ctx context.Context) (storage.Storage, error) {
		dir, err := os.MkdirTemp("", "storage-lock-rocksdb-*")
		if err != nil {
			return nil, err
		}
		return rocksdb_storage.NewRocksDBStorage(ctx, rocksdb_storage.NewRocksDBStorageOptions().SetConnectionManager(rocksdb_storage.NewRocksDBConnectionManager(dir)))
	})
}
