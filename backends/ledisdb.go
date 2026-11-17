//go:build storage_lock_ledisdb

package backends

import (
	"context"

	ledisdb_storage "github.com/storage-lock/go-ledisdb-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("ledisdb", func(ctx context.Context) (storage.Storage, error) {
		uri, err := framework.BackendFromEnv("STORAGE_LOCK_LEDISDB_URI")
		if err != nil {
			return nil, err
		}
		return ledisdb_storage.NewLedisDBStorage(ctx, ledisdb_storage.NewLedisDBStorageOptionsWithURI(uri))
	})
}
