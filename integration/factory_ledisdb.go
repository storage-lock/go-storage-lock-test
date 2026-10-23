package integration

import (
	"context"
	"os"
	"testing"
	"time"

	ledisdb_storage "github.com/storage-lock/go-ledisdb-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// LedisDBStorageFactory 为 LedisDBStorage 实现 StorageLockTestFactory
// LedisDB 本轮从占位实现升级为真实 Lua CAS 实现（兼容 Redis 协议），接入完整语义套件。
type LedisDBStorageFactory struct {
	storage *ledisdb_storage.LedisDBStorage
}

// NewLedisDBStorageFactory 创建一个基于 LedisDBStorage 的测试 factory
// 如果 STORAGE_LOCK_LEDISDB_URI 环境变量为空则返回 nil（调用方应 t.Skip）
func NewLedisDBStorageFactory(t *testing.T) *LedisDBStorageFactory {
	uri := os.Getenv("STORAGE_LOCK_LEDISDB_URI")
	if uri == "" {
		return nil
	}
	ctx := context.Background()
	s, err := ledisdb_storage.NewLedisDBStorage(ctx, ledisdb_storage.NewLedisDBStorageOptionsWithURI(uri))
	if err != nil {
		t.Fatalf("failed to create LedisDBStorage: %v", err)
	}
	return &LedisDBStorageFactory{storage: s}
}

func (f *LedisDBStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *LedisDBStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	// LedisDB 无服务端时钟（不支持 TIME），Capabilities 不声明 ReliableTime，
	// 此处注入本地时间源兜底，对齐 memcache/mongo factory 的做法。
	options.SetTimeProvider(&localTimeProvider{})
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with LedisDBStorage: %v", err)
	}
	return lock
}

func (f *LedisDBStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
}
