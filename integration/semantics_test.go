package integration_test

import (
	"testing"

	framework "github.com/storage-lock/go-storage-lock-test"
	backends "github.com/storage-lock/go-storage-lock-test/integration"
)

// storageLockSuites runs the exported suites against one backend factory.
func storageLockSuites(t *testing.T, factory framework.StorageLockTestFactory) {
	t.Helper()
	framework.TestStorageLockSemantics(t, factory)
	framework.TestStorageLock_ReadWriteLockSemantics(t, factory)
	framework.TestStorageLock_FairQueueSemantics(t, factory)
	framework.TestStorageLock_OwnershipSemantics(t, factory)
	framework.TestStorageLock_LifecycleSemantics(t, factory)
}

// TestMemoryStorageLockSemantics 对 MemoryStorage 运行完整的分布式锁语义测试
func TestRedisStorageLockSemantics(t *testing.T) {
	factory := backends.NewRedisStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_REDIS_URI not set, skipping Redis semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestLevelDBStorageLockSemantics 对 LevelDBStorage 运行完整的分布式锁语义测试
func TestLevelDBStorageLockSemantics(t *testing.T) {
	factory := backends.NewLevelDBStorageFactory(t)
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestEtcdStorageLockSemantics 对 EtcdStorage 运行完整的分布式锁语义测试
func TestEtcdStorageLockSemantics(t *testing.T) {
	factory := backends.NewEtcdStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_ETCD_ENDPOINTS not set, skipping etcd semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestKVStorageLockSemantics 对 KVStorage 运行完整的分布式锁语义测试
func TestKVStorageLockSemantics(t *testing.T) {
	factory := backends.NewKVStorageFactory(t)
	storageLockSuites(t, factory)
}

// TestHTTPStorageLockSemantics 对 HTTPStorage 运行完整的分布式锁语义测试
func TestHTTPStorageLockSemantics(t *testing.T) {
	factory := backends.NewHTTPStorageFactory(t)
	storageLockSuites(t, factory)
}

// TestLedisDBStorageLockSemantics 对 LedisDBStorage 运行完整的分布式锁语义测试
func TestLedisDBStorageLockSemantics(t *testing.T) {
	factory := backends.NewLedisDBStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_LEDISDB_URI not set, skipping LedisDB semantics test")
	}
	storageLockSuites(t, factory)
}

// TestRocksDBStorageLockSemantics 对 RocksDBStorage 运行完整的分布式锁语义测试
func TestRocksDBStorageLockSemantics(t *testing.T) {
	factory := backends.NewRocksDBStorageFactory(t)
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestSqlite3StorageLockSemantics 对 Sqlite3Storage 运行完整的分布式锁语义测试
// sqlite3 是 SQL 家族第一个接入语义套件的代表（文件型，无需外部服务端）
func TestMysqlStorageLockSemantics(t *testing.T) {
	factory := backends.NewMysqlStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_MYSQL_DSN not set, skipping MySQL semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestPostgresqlStorageLockSemantics 对 PostgresqlStorage 运行完整的分布式锁语义测试
// postgres 有独立 SqlProvider（$ 占位符方言），单独实证
func TestPostgresqlStorageLockSemantics(t *testing.T) {
	factory := backends.NewPostgresqlStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_POSTGRESQL_DSN not set, skipping PostgreSQL semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestMemcacheStorageLockSemantics 对 MemcacheStorage 运行完整的分布式锁语义测试
func TestMemcacheStorageLockSemantics(t *testing.T) {
	factory := backends.NewMemcacheStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_MEMCACHE_SERVERS not set, skipping memcache semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestZookeeperStorageLockSemantics 对 ZookeeperStorage 运行完整的分布式锁语义测试
func TestZookeeperStorageLockSemantics(t *testing.T) {
	factory := backends.NewZookeeperStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_ZOOKEEPER_HOSTS not set, skipping ZooKeeper semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestS3StorageLockSemantics 对 S3Storage 运行完整的分布式锁语义测试
// S3 不支持原子条件删除，通过墓碑降级路径验证互斥性
func TestS3StorageLockSemantics(t *testing.T) {
	factory := backends.NewS3StorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_S3_ENDPOINT not set, skipping S3 semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}

// TestMongoDBStorageLockSemantics 对 MongoStorage 运行完整的分布式锁语义测试
func TestMongoDBStorageLockSemantics(t *testing.T) {
	factory := backends.NewMongoDBStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_MONGO_URI not set, skipping MongoDB semantics test")
	}
	defer factory.Cleanup()
	storageLockSuites(t, factory)
}
