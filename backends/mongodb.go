//go:build storage_lock_mongodb

package backends

import (
	"context"

	mongodb_storage "github.com/storage-lock/go-mongodb-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("mongodb", func(ctx context.Context) (storage.Storage, error) {
		uri, err := framework.BackendFromEnv("STORAGE_LOCK_MONGO_URI")
		if err != nil {
			return nil, err
		}
		return mongodb_storage.NewMongoStorage(ctx, mongodb_storage.NewMongoStorageOptionsWithURI(uri))
	})
}
