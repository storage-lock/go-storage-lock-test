# go-storage-lock-test

分布式锁语义测试框架：验证基于 [storage-lock](https://github.com/storage-lock) 的存储实现是否正确实现了分布式锁的核心语义。

## 为什么需要这个框架

一个 Storage 实现即使通过了 [go-storage-test-helper](https://github.com/storage-lock/go-storage-test-helper) 的 CRUD 测试，也未必能正确支持分布式锁——因为分布式锁的语义（互斥、可重入、租约过期、看门狗续租）涉及更复杂的多步骤交互和并发场景。

本框架专注于测试**分布式锁的核心语义**，而非单个存储方法的 CRUD。

## 测试覆盖的语义

| 测试用例 | 验证内容 |
|---------|---------|
| `BasicLockUnlock` | 基本加锁/解锁流程 |
| `MutexExclusion` | 互斥性：同一 lockId 同一时刻只有一个 owner 持有 |
| `ReentrantLock` | 可重入：同 owner 多次 Lock，LockCount 递增，多次 UnLock 才真正释放 |
| `LeaseExpiration` | 租约过期：持有者"宕机"后，锁在租约到期后可被其他人抢占 |
| `WatchDogRenewal` | 看门狗续租：持有期间租约被持续续期，不会过期 |
| `ConcurrentCompetition` | 并发竞争：多协程争抢，临界区完全互斥，计数器无丢失无重复 |
| `NonOwnerUnlock` | 非持有者解锁应失败 |
| `ReacquireAfterRelease` | 释放后可被他人重新获取 |
| `FairQueue/ArrivalOrder` | 跨实例先来先服务：后创建的实例先登记，就先拿到锁 |
| `FairQueue/CanceledHeadHandsOff` | 队首取消后，下一位从存储队列里接手 |
| `FairQueue/UnfairClientCannotCut` | 没开公平选项的客户端不能插到已排队的人前面 |
| `FairQueue/ConcurrentArrival` | 多个竞争者同时登记，临界区依次进入且不重复 |

## 使用方法

存储实现方只需提供一个 `StorageLockTestFactory`，它能为每个测试用例创建独立的、状态干净的 `StorageLock` 实例：

```go
package my_storage_test

import (
    "context"
    "testing"
    "time"

    storage_lock_test "github.com/storage-lock/go-storage-lock-test"
    "github.com/storage-lock/go-storage"
    storage_lock "github.com/storage-lock/go-storage-lock"
    "github.com/storage-lock/go-memory-storage" // 替换为你的存储实现
)

type MyFactory struct {
    storage *memory_storage.MemoryStorage
}

func (f *MyFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
    return f.storage
}

func (f *MyFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
    options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
    options.LeaseExpireAfter = time.Second * 3
    options.LeaseRefreshInterval = time.Second * 1
    options.VersionMissRetryInterval = time.Millisecond * 5
    lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
    if err != nil {
        t.Fatalf("failed to create lock: %v", err)
    }
    return lock
}

func (f *MyFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
    // 清理资源
}

func TestMyStorageLockSemantics(t *testing.T) {
    factory := &MyFactory{storage: memory_storage.NewMemoryStorage()}
    storage_lock_test.TestStorageLockSemantics(t, factory)
}
```

## 框架自带的测试

不依赖外部服务的后端一次跑完：

```bash
go test -count=1 -timeout 60m -run TestLocalBackends .
```

覆盖 memory、filesystem、leveldb、kv、http、sqlite3。

全部后端（含依赖外部服务的）用 `run-all.sh`。它先导出每个服务的连接环境变量，再跑整个包的测试；哪个服务没在监听，对应的 `Test*StorageLockSemantics` 就跳过，不会让整次运行失败。RocksDB 不需要服务，但需要本机的 `librocksdb`。

```bash
docker compose -f docker/docker-compose.yml up -d
./run-all.sh
```

`docker/docker-compose.yml` 里的端口刻意避开默认值（Redis 16379、MySQL 13307、PostgreSQL 15432、memcache 11212、LedisDB 16380），避免和机器上已有的实例冲突。环境变量已设置时脚本沿用已有的值。

## 设计要点

- **隔离性**：每个测试用例使用独立的 lockId，状态互不干扰
- **NoopWatchDog**：提供不续租的看门狗，精确模拟"持有者宕机"场景，测试租约过期
- **并发竞争测试**：用原子计数器验证临界区互斥性和计数器完整性，是验证 CAS 正确性的核心
