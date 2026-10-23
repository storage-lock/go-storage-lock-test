package integration

import (
	"time"

	storage_lock "github.com/storage-lock/go-storage-lock"
)

func integrationLockOptions(lockID string) *storage_lock.StorageLockOptions {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockID)
	options.LeaseExpireAfter = 3 * time.Second
	options.LeaseRefreshInterval = time.Second
	options.VersionMissRetryInterval = 5 * time.Millisecond
	return options
}
