//go:build storage_lock_obs

package backends

import (
	"context"

	obs_storage "github.com/storage-lock/go-obs-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("obs", func(ctx context.Context) (storage.Storage, error) {
		endpoint, err := framework.BackendFromEnv("STORAGE_LOCK_OBS_ENDPOINT")
		if err != nil {
			return nil, err
		}
		accessKey, err := framework.BackendFromEnv("STORAGE_LOCK_OBS_ACCESS_KEY")
		if err != nil {
			return nil, err
		}
		secretKey, err := framework.BackendFromEnv("STORAGE_LOCK_OBS_SECRET_KEY")
		if err != nil {
			return nil, err
		}
		bucket := "storage-lock-test"
		if value, err := framework.BackendFromEnv("STORAGE_LOCK_OBS_BUCKET"); err == nil {
			bucket = value
		}
		options := obs_storage.NewOBSStorageOptions().
			SetConnectionManager(obs_storage.NewOBSConnectionManager(endpoint, accessKey, secretKey)).
			SetBucketName(bucket)
		return obs_storage.NewOBSStorage(ctx, options)
	})
}
