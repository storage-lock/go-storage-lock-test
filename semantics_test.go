package storage_lock_test

import (
	"context"
	"testing"
)

// TestMemoryStorageLockSemantics 对 MemoryStorage 运行完整的分布式锁语义测试
func TestMemoryStorageLockSemantics(t *testing.T) {
	factory := NewMemoryStorageFactory()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestFileSystemStorageLockSemantics 对 FileSystemStorage 运行完整的分布式锁语义测试
func TestFileSystemStorageLockSemantics(t *testing.T) {
	factory := NewFileSystemStorageFactory(t)
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestRedisStorageLockSemantics 对 RedisStorage 运行完整的分布式锁语义测试
func TestRedisStorageLockSemantics(t *testing.T) {
	factory := NewRedisStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_REDIS_URI not set, skipping Redis semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestLevelDBStorageLockSemantics 对 LevelDBStorage 运行完整的分布式锁语义测试
func TestLevelDBStorageLockSemantics(t *testing.T) {
	factory := NewLevelDBStorageFactory(t)
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestEtcdStorageLockSemantics 对 EtcdStorage 运行完整的分布式锁语义测试
func TestEtcdStorageLockSemantics(t *testing.T) {
	factory := NewEtcdStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_ETCD_ENDPOINTS not set, skipping etcd semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestKVStorageLockSemantics 对 KVStorage 运行完整的分布式锁语义测试
func TestKVStorageLockSemantics(t *testing.T) {
	factory := NewKVStorageFactory(t)
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestHTTPStorageLockSemantics 对 HTTPStorage 运行完整的分布式锁语义测试
func TestHTTPStorageLockSemantics(t *testing.T) {
	factory := NewHTTPStorageFactory(t)
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestLedisDBStorageLockSemantics 对 LedisDBStorage 运行完整的分布式锁语义测试
func TestLedisDBStorageLockSemantics(t *testing.T) {
	factory := NewLedisDBStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_LEDISDB_URI not set, skipping LedisDB semantics test")
	}
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestRocksDBStorageLockSemantics 对 RocksDBStorage 运行完整的分布式锁语义测试
func TestRocksDBStorageLockSemantics(t *testing.T) {
	factory := NewRocksDBStorageFactory(t)
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestSqlite3StorageLockSemantics 对 Sqlite3Storage 运行完整的分布式锁语义测试
// sqlite3 是 SQL 家族第一个接入语义套件的代表（文件型，无需外部服务端）
func TestSqlite3StorageLockSemantics(t *testing.T) {
	factory := NewSqlite3StorageFactory(t)
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestMysqlStorageLockSemantics 对 MysqlStorage 运行完整的分布式锁语义测试
// mysql 是 SQL 家族（sql-based 基类派生 13 仓）的服务端实证代表
func TestMysqlStorageLockSemantics(t *testing.T) {
	factory := NewMysqlStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_MYSQL_DSN not set, skipping MySQL semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestPostgresqlStorageLockSemantics 对 PostgresqlStorage 运行完整的分布式锁语义测试
// postgres 有独立 SqlProvider（$ 占位符方言），单独实证
func TestPostgresqlStorageLockSemantics(t *testing.T) {
	factory := NewPostgresqlStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_POSTGRESQL_DSN not set, skipping PostgreSQL semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestMemcacheStorageLockSemantics 对 MemcacheStorage 运行完整的分布式锁语义测试
func TestMemcacheStorageLockSemantics(t *testing.T) {
	factory := NewMemcacheStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_MEMCACHE_SERVERS not set, skipping memcache semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestZookeeperStorageLockSemantics 对 ZookeeperStorage 运行完整的分布式锁语义测试
func TestZookeeperStorageLockSemantics(t *testing.T) {
	factory := NewZookeeperStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_ZOOKEEPER_HOSTS not set, skipping ZooKeeper semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestS3StorageLockSemantics 对 S3Storage 运行完整的分布式锁语义测试
// S3 不支持原子条件删除，通过墓碑降级路径验证互斥性
func TestS3StorageLockSemantics(t *testing.T) {
	factory := NewS3StorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_S3_ENDPOINT not set, skipping S3 semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestMongoDBStorageLockSemantics 对 MongoStorage 运行完整的分布式锁语义测试
func TestMongoDBStorageLockSemantics(t *testing.T) {
	factory := NewMongoDBStorageFactory(t)
	if factory == nil {
		t.Skip("STORAGE_LOCK_MONGO_URI not set, skipping MongoDB semantics test")
	}
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
	TestStorageLock_ReadWriteLockSemantics(t, factory)
	TestStorageLock_FairQueueSemantics(t, factory)
	TestStorageLock_OwnershipSemantics(t, factory)
	TestStorageLock_LifecycleSemantics(t, factory)
}

// TestTombstoneReleasePath 验证"不支持原子条件删除"的存储介质（如对象存储）
// 能通过墓碑降级路径正确释放与抢占锁，互斥性不被破坏。
func TestTombstoneReleasePath(t *testing.T) {
	testTombstoneReleasePath(t)
}

// TestTombstoneRLockReclaim 验证 RLock 抢占墓碑记录不会死循环（回归测试）
func TestTombstoneRLockReclaim(t *testing.T) {
	testTombstoneRLockReclaim(t)
}

// TestFencingTokenMonotonic 验证栅栏令牌（fencing token）跨所有权变更严格单调递增，
// 这是消除"持有者停顿/宕机、租约过期被抢占后仍破坏被保护资源"这一租约锁经典安全边界的关键。
func TestFencingTokenMonotonic(t *testing.T) {
	testFencingTokenMonotonic(t)
}

// 确保 context 被使用（某些 factory 可能需要）
var _ = context.Background
