//go:build storage_lock_leofs

package backends

import (
	"context"

	s3_storage "github.com/storage-lock/go-s3-storage"
	leofs_storage "github.com/storage-lock/go-leofs-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("leofs", func(ctx context.Context) (storage.Storage, error) {
		endpoint, err := framework.BackendFromEnv("STORAGE_LOCK_S3_ENDPOINT")
		if err != nil {
			return nil, err
		}
		accessKey, err := framework.BackendFromEnv("STORAGE_LOCK_S3_ACCESS_KEY")
		if err != nil {
			return nil, err
		}
		secretKey, err := framework.BackendFromEnv("STORAGE_LOCK_S3_SECRET_KEY")
		if err != nil {
			return nil, err
		}
		bucket := "storage-lock-test"
		if value, err := framework.BackendFromEnv("STORAGE_LOCK_S3_BUCKET"); err == nil {
			bucket = value
		}
		options := s3_storage.NewS3StorageOptions().
			SetConnectionManager(s3_storage.NewS3ConnectionManagerFromEndpoint(endpoint, accessKey, secretKey, false)).
			SetBucketName(bucket)
		return leofs_storage.NewLeoFSStorage(ctx, options)
	})
}
