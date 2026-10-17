package storage_lock

import (
	"context"
	"os"
	"testing"
	"time"

	etcd_storage "github.com/storage-lock/go-etcd-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdStorageFactory 为 EtcdStorage 实现 StorageLockTestFactory
type EtcdStorageFactory struct {
	client  *clientv3.Client
	storage *etcd_storage.EtcdStorage
}

// NewEtcdStorageFactory 创建一个基于 EtcdStorage 的测试 factory
// 如果 STORAGE_LOCK_ETCD_ENDPOINTS 环境变量为空则返回 nil（调用方应 t.Skip）
func NewEtcdStorageFactory(t *testing.T) *EtcdStorageFactory {
	envName := "STORAGE_LOCK_ETCD_ENDPOINTS"
	endpoints := os.Getenv(envName)
	if endpoints == "" {
		return nil
	}

	ctx := context.Background()
	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{endpoints},
	})
	if err != nil {
		t.Fatalf("failed to connect to etcd: %v", err)
	}

	connectionManager := etcd_storage.NewEtcdConnectionManagerFromClient(client)
	s, err := etcd_storage.NewEtcdStorage(ctx, etcd_storage.NewEtcdStorageOptions().SetConnectionManager(connectionManager))
	if err != nil {
		t.Fatalf("failed to create EtcdStorage: %v", err)
	}

	return &EtcdStorageFactory{
		client:  client,
		storage: s,
	}
}

func (f *EtcdStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *EtcdStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with EtcdStorage: %v", err)
	}
	return lock
}

func (f *EtcdStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// etcd 无需特殊清理
}

// Cleanup 清理 etcd 测试数据
func (f *EtcdStorageFactory) Cleanup() {
	if f.client != nil {
		_, _ = f.client.Delete(context.Background(), "storage_lock_test", clientv3.WithPrefix())
	}
}
