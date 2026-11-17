//go:build storage_lock_oss

package backends

import (
	"context"

	oss_storage "github.com/storage-lock/go-oss-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("oss", func(ctx context.Context) (storage.Storage, error) {
		endpoint, err := framework.BackendFromEnv("STORAGE_LOCK_OSS_ENDPOINT")
		if err != nil {
			return nil, err
		}
		accessKey, err := framework.BackendFromEnv("STORAGE_LOCK_OSS_ACCESS_KEY")
		if err != nil {
			return nil, err
		}
		secretKey, err := framework.BackendFromEnv("STORAGE_LOCK_OSS_SECRET_KEY")
		if err != nil {
			return nil, err
		}
		bucket := "storage-lock-test"
		if value, err := framework.BackendFromEnv("STORAGE_LOCK_OSS_BUCKET"); err == nil {
			bucket = value
		}
		options := oss_storage.NewOSSStorageOptions().
			SetConnectionManager(oss_storage.NewOSSConnectionManagerFromEndpoint(endpoint, accessKey, secretKey, bucket)).
			SetBucketName(bucket)
		return oss_storage.NewOSSStorage(ctx, options)
	})
}
