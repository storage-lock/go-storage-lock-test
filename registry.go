package storage_lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// StorageBuilder 为一个具体存储实现创建干净实例。
// 环境不满足时返回 ErrBackendUnavailable，套件会跳过而不是失败。
// 返回的 Storage 必须是这个实现自己的类型，套件会拒绝缺少 CapabilityCAS 的实例。
type StorageBuilder func(ctx context.Context) (storage.Storage, error)

// ErrBackendUnavailable 表示这次测试没有可用的后端，不是锁语义失败。
var ErrBackendUnavailable = fmt.Errorf("storage backend is not available")

type registeredBackend struct {
	name    string
	builder StorageBuilder
}

var (
	registryMu sync.Mutex
	registry   []registeredBackend
)

// RegisterBackend 登记一个具体存储实现。
// 同名重复登记会 panic，避免两个实现悄悄覆盖彼此。
// 通常放在实现库的 TestMain 或 init 中调用。
func RegisterBackend(name string, builder StorageBuilder) {
	if name == "" || builder == nil {
		panic("storage lock test backend name and builder are required")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, backend := range registry {
		if backend.name == name {
			panic("storage lock test backend already registered: " + name)
		}
	}
	registry = append(registry, registeredBackend{name: name, builder: builder})
}

// RegisteredBackends 返回已登记的实现名称，顺序与登记顺序一致。
func RegisteredBackends() []string {
	registryMu.Lock()
	defer registryMu.Unlock()
	names := make([]string, len(registry))
	for i, backend := range registry {
		names[i] = backend.name
	}
	return names
}

// TestRegisteredBackends 对每个已登记、当前可用、且声明了 CapabilityCAS 的实现
// 运行同一套锁语义、读写锁、公平队列、所有权和生命周期用例。
// 不可用的实现跳过；能连上但没有 CAS 的实现失败，避免占位库被当成通过。
func TestRegisteredBackends(t *testing.T) {
	registryMu.Lock()
	backends := append([]registeredBackend(nil), registry...)
	registryMu.Unlock()
	if len(backends) == 0 {
		t.Skip("no storage backends registered")
	}
	suites := []func(*testing.T, StorageLockTestFactory){
		TestStorageLockSemantics,
		TestStorageLock_ReadWriteLockSemantics,
		TestStorageLock_FairQueueSemantics,
		TestStorageLock_OwnershipSemantics,
		TestStorageLock_LifecycleSemantics,
	}
	for _, backend := range backends {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			factory, err := newRegisteredFactory(context.Background(), backend)
			if errors.Is(err, ErrBackendUnavailable) {
				t.Skip(err.Error())
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(factory.Cleanup)
			for _, suite := range suites {
				suite(t, factory)
			}
		})
	}
}

type registeredFactory struct {
	storage storage.Storage
	cleanup func()
}

func newRegisteredFactory(ctx context.Context, backend registeredBackend) (*registeredFactory, error) {
	value, err := backend.builder(ctx)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, ErrBackendUnavailable
	}
	if !storage.HasCapability(value, storage.CapabilityCAS) {
		return nil, fmt.Errorf("%s does not declare CapabilityCAS", backend.name)
	}
	return &registeredFactory{storage: value, cleanup: func() { _ = value.Close(context.Background()) }}, nil
}

func (f *registeredFactory) CreateStorage(context.Context, *testing.T) storage.Storage {
	return f.storage
}

func (f *registeredFactory) NewStorageLock(_ context.Context, t *testing.T, lockID string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockID)
	options.LeaseExpireAfter = TestLockOptions.LeaseExpireAfter
	options.LeaseRefreshInterval = TestLockOptions.LeaseRefreshInterval
	options.VersionMissRetryInterval = TestLockOptions.VersionMissRetryInterval
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("create storage lock for %s: %v", f.storage.GetName(), err)
	}
	return lock
}

func (f *registeredFactory) CloseStorageLock(context.Context, *testing.T, *storage_lock.StorageLock) {
}

func (f *registeredFactory) Cleanup() {
	if f.cleanup != nil {
		f.cleanup()
	}
}

// BackendFromEnv 在环境变量为空时返回不可用错误，供各实现库的 builder 复用。
func BackendFromEnv(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("%w: %s is empty", ErrBackendUnavailable, name)
	}
	return value, nil
}
