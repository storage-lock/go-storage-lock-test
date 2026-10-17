package storage_lock_test

import (
	"testing"

	storage_lock "github.com/storage-lock/go-storage-lock-test"
)

// localBackends 是不需要外部服务的存储后端。
// 每个后端跑同一套语义用例，新增用例只要挂进对应的 TestStorageLock_*Semantics 即可，
// 这里不用改。依赖外部服务的后端由各自的 Test 函数按环境变量决定跳过。
var localBackends = []struct {
	name    string
	factory func(t *testing.T) storage_lock.StorageLockTestFactory
}{
	{"memory", func(*testing.T) storage_lock.StorageLockTestFactory { return storage_lock.NewMemoryStorageFactory() }},
	{"filesystem", func(t *testing.T) storage_lock.StorageLockTestFactory {
		return storage_lock.NewFileSystemStorageFactory(t)
	}},
	{"leveldb", func(t *testing.T) storage_lock.StorageLockTestFactory {
		return storage_lock.NewLevelDBStorageFactory(t)
	}},
	{"kv", func(t *testing.T) storage_lock.StorageLockTestFactory { return storage_lock.NewKVStorageFactory(t) }},
	{"http", func(t *testing.T) storage_lock.StorageLockTestFactory { return storage_lock.NewHTTPStorageFactory(t) }},
	{"sqlite3", func(t *testing.T) storage_lock.StorageLockTestFactory {
		return storage_lock.NewSqlite3StorageFactory(t)
	}},
}

// TestLocalBackends 一次跑完所有本地后端的全部语义用例。
//
//	go test -count=1 -timeout 60m -run TestLocalBackends .
func TestLocalBackends(t *testing.T) {
	suites := []func(*testing.T, storage_lock.StorageLockTestFactory){
		storage_lock.TestStorageLockSemantics,
		storage_lock.TestStorageLock_ReadWriteLockSemantics,
		storage_lock.TestStorageLock_FairQueueSemantics,
		storage_lock.TestStorageLock_OwnershipSemantics,
		storage_lock.TestStorageLock_LifecycleSemantics,
	}
	for _, backend := range localBackends {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()
			factory := backend.factory(t)
			if cleaner, ok := factory.(interface{ Cleanup() }); ok {
				t.Cleanup(cleaner.Cleanup)
			}
			for _, suite := range suites {
				suite(t, factory)
			}
		})
	}
}
