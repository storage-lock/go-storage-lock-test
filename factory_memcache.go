package storage_lock_test

import (
	"context"
	"os"
	"testing"
	"time"

	memcache_storage "github.com/storage-lock/go-memcache-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"github.com/bradfitz/gomemcache/memcache"
)

// MemcacheStorageFactory 为 MemcacheStorage 实现 StorageLockTestFactory
type MemcacheStorageFactory struct {
	client  *memcache.Client
	storage *memcache_storage.MemcacheStorage
}

// NewMemcacheStorageFactory 创建一个基于 MemcacheStorage 的测试 factory
// 如果 STORAGE_LOCK_MEMCACHE_SERVERS 环境变量为空则返回 nil（调用方应 t.Skip）
func NewMemcacheStorageFactory(t *testing.T) *MemcacheStorageFactory {
	envName := "STORAGE_LOCK_MEMCACHE_SERVERS"
	servers := os.Getenv(envName)
	if servers == "" {
		return nil
	}

	ctx := context.Background()
	client := memcache.New(servers)
	if err := client.Ping(); err != nil {
		t.Fatalf("failed to connect to memcache: %v", err)
	}

	connectionManager := memcache_storage.NewMemcacheConnectionManagerFromClient(client)
	s, err := memcache_storage.NewMemcacheStorage(ctx, memcache_storage.NewMemcacheStorageOptions().
		SetConnectionManager(connectionManager).
		SetTimeProvider(&localTimeProvider{}))
	if err != nil {
		t.Fatalf("failed to create MemcacheStorage: %v", err)
	}

	return &MemcacheStorageFactory{
		client:  client,
		storage: s,
	}
}

func (f *MemcacheStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *MemcacheStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with MemcacheStorage: %v", err)
	}
	return lock
}

func (f *MemcacheStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// memcache 无需特殊清理
}

// Cleanup 清理 memcache 测试数据
func (f *MemcacheStorageFactory) Cleanup() {
	if f.client != nil {
		_ = f.client.FlushAll()
	}
}

// localTimeProvider 本地时间源，用于无 NTP 访问的测试环境
type localTimeProvider struct{}

func (p *localTimeProvider) GetTime(ctx context.Context) (time.Time, error) {
	return time.Now(), nil
}
