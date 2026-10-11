package storage_lock_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-zookeeper/zk"
	zookeeper_storage "github.com/storage-lock/go-zookeeper-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// ZookeeperStorageFactory 为 ZookeeperStorage 实现 StorageLockTestFactory
type ZookeeperStorageFactory struct {
	client  *zk.Conn
	storage *zookeeper_storage.ZookeeperStorage
}

// NewZookeeperStorageFactory 创建一个基于 ZookeeperStorage 的测试 factory
// 如果 STORAGE_LOCK_ZOOKEEPER_HOSTS 环境变量为空则返回 nil（调用方应 t.Skip）
func NewZookeeperStorageFactory(t *testing.T) *ZookeeperStorageFactory {
	envName := "STORAGE_LOCK_ZOOKEEPER_HOSTS"
	hosts := os.Getenv(envName)
	if hosts == "" {
		return nil
	}

	ctx := context.Background()
	conn, _, err := zk.Connect([]string{hosts}, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to ZooKeeper: %v", err)
	}

	connectionManager := zookeeper_storage.NewZookeeperConnectionManagerFromClient(conn)
	s, err := zookeeper_storage.NewZookeeperStorage(ctx, zookeeper_storage.NewZookeeperStorageOptions().SetConnectionManager(connectionManager))
	if err != nil {
		t.Fatalf("failed to create ZookeeperStorage: %v", err)
	}

	return &ZookeeperStorageFactory{
		client:  conn,
		storage: s,
	}
}

func (f *ZookeeperStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *ZookeeperStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with ZookeeperStorage: %v", err)
	}
	return lock
}

func (f *ZookeeperStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// ZooKeeper 无需特殊清理
}

// Cleanup 清理 ZooKeeper 测试数据
func (f *ZookeeperStorageFactory) Cleanup() {
	if f.client != nil {
		// 递归删除测试路径
		deleteRecursive(f.client, "/storage_lock_test")
	}
}

// deleteRecursive 递归删除 ZooKeeper 路径
func deleteRecursive(conn *zk.Conn, path string) {
	children, _, err := conn.Children(path)
	if err != nil {
		return
	}
	for _, child := range children {
		deleteRecursive(conn, path+"/"+child)
	}
	conn.Delete(path, -1)
}
