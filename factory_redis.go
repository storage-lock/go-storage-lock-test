package storage_lock

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	redis_storage "github.com/storage-lock/go-redis-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// RedisStorageFactory 为 RedisStorage 实现 StorageLockTestFactory
type RedisStorageFactory struct {
	client  *redis.Client
	storage *redis_storage.RedisStorage
}

// NewRedisStorageFactory 创建一个基于 RedisStorage 的测试 factory
// 如果 STORAGE_LOCK_REDIS_URI 环境变量为空则返回 nil（调用方应 t.Skip）
func NewRedisStorageFactory(t *testing.T) *RedisStorageFactory {
	envName := "STORAGE_LOCK_REDIS_URI"
	uri := os.Getenv(envName)
	if uri == "" {
		return nil
	}

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6399",
	})
	if uri != "" {
		opts, err := redis.ParseURL(uri)
		if err != nil {
			t.Fatalf("failed to parse Redis URI: %v", err)
		}
		client = redis.NewClient(opts)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to connect to Redis: %v", err)
	}

	connectionManager := redis_storage.NewRedisConnectionManagerFromClient(client)
	s, err := redis_storage.NewRedisStorage(ctx, redis_storage.NewRedisStorageOptions().SetConnectionManager(connectionManager))
	if err != nil {
		t.Fatalf("failed to create RedisStorage: %v", err)
	}

	return &RedisStorageFactory{
		client:  client,
		storage: s,
	}
}

func (f *RedisStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *RedisStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with RedisStorage: %v", err)
	}
	return lock
}

func (f *RedisStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// Redis 无需特殊清理
}

// Cleanup 清理 Redis 测试数据
func (f *RedisStorageFactory) Cleanup() {
	if f.client != nil {
		_ = f.client.FlushDB(context.Background())
	}
}
