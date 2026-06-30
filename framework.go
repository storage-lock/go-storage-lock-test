// Package storage_lock_test 提供分布式锁语义的测试框架
//
// 本包用于验证基于 storage-lock 框架的存储实现是否正确实现了分布式锁的核心语义。
// 一个 Storage 实现即使通过了 go-storage-test-helper 的 CRUD 测试，也未必能正确支持
// 分布式锁——因为分布式锁的语义（互斥、可重入、租约过期、看门狗续租）涉及更复杂的
// 多步骤交互和并发场景。
//
// # 使用方法
//
// 存储实现方只需提供一个 StorageLockTestFactory，它能为每个测试用例创建一个干净的、
// 相互隔离的 StorageLock 实例：
//
//	func TestMyStorageLockSemantics(t *testing.T) {
//	    factory := &MyFactory{}
//	    storage_lock_test.TestStorageLockSemantics(t, factory)
//	}
//
// factory 的 NewStorageLock 方法应该：
//   - 每次返回一个全新的、状态干净的锁实例（或保证清理了上一次的状态）
//   - 使用独立的 lockId 来隔离不同测试用例
//   - 可以根据测试用例的需要调整选项（如缩短租约时间以测试过期场景）
//
// # 测试覆盖的语义
//
//   - 基本加锁/解锁
//   - 互斥性（同一 lockId 同一时刻只有一个 owner 持有）
//   - 可重入（同 owner 多次 Lock，LockCount 递增，多次 UnLock 才真正释放）
//   - 租约过期（持有者"宕机"后，锁在租约到期后可被其他人抢占）
//   - 看门狗续租（持有期间租约被持续续期，不会过期）
//   - 并发竞争（多协程争抢，最终只有一个成功，临界区互斥）
//   - 非持有者解锁（应失败）
//   - 释放后可被他人重新获取
package storage_lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// StorageLockTestFactory 为每个测试用例创建独立的、状态干净的 StorageLock 实例
//
// 实现方需要保证：
//   - NewStorageLock 返回的实例与同一测试中其它实例的状态隔离
//     （通常通过独立的 lockId 或独立的存储 namespace 实现）
//   - CloseStorageLock 在测试结束后清理资源（如删除临时表、临时目录）
//   - CreateStorage 返回底层的 Storage 实现（用于直接操作，例如测试租约过期时
//     手动篡改 LeaseExpireTime）
type StorageLockTestFactory interface {

	// CreateStorage 返回底层 Storage 实现
	// ctx: 用于超时控制
	// t: 当前测试对象，可用于在创建失败时直接 Fatalf
	CreateStorage(ctx context.Context, t *testing.T) storage.Storage

	// NewStorageLock 创建一个新的 StorageLock 实例，使用指定的 lockId
	// options 参数已被框架填充好（包括锁ID和针对测试场景调整过的选项），
	// 实现方可以直接使用，也可以基于它进一步定制
	NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock

	// CloseStorageLock 清理一个 StorageLock 实例及其关联资源
	CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock)
}

// TestLockOptions 是框架内部使用的锁选项基线，测试用例可能会在此基础上调整
//
// 测试用到的租约时间设置得比较短，以便快速测试过期场景：
//   - LeaseExpireAfter: 3 秒（最短允许值）
//   - LeaseRefreshInterval: 1 秒
var TestLockOptions = storage_lock.StorageLockOptions{
	LeaseExpireAfter:         time.Second * 3,
	LeaseRefreshInterval:     time.Second * 1,
	VersionMissRetryInterval: time.Millisecond * 100,
}

// TestStorageLockSemantics 运行完整的分布式锁语义测试套件
//
// 这是测试框架的入口，存储实现方只需调用此函数并传入自己的 factory 即可
// 跑完所有语义测试
func TestStorageLockSemantics(t *testing.T, factory StorageLockTestFactory) {

	ctx := context.Background()

	// 每个用例使用独立的子测试，独立的 lockId，避免互相干扰
	cases := []struct {
		name string
		fn   func(t *testing.T, ctx context.Context, factory StorageLockTestFactory)
	}{
		{"BasicLockUnlock", testBasicLockUnlock},
		{"MutexExclusion", testMutexExclusion},
		{"ReentrantLock", testReentrantLock},
		{"LeaseExpiration", testLeaseExpiration},
		{"WatchDogRenewal", testWatchDogRenewal},
		{"ConcurrentCompetition", testConcurrentCompetition},
		{"NonOwnerUnlock", testNonOwnerUnlock},
		{"ReacquireAfterRelease", testReacquireAfterRelease},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// 每个子测试用独立的 lockId，隔离状态
			lockId := "test-" + c.name
			c.fn(t, ctx, &isolatedFactory{
				inner:  factory,
				lockId: lockId,
			})
		})
	}
}

// isolatedFactory 包装用户提供的 factory，自动注入独立的 lockId
type isolatedFactory struct {
	inner  StorageLockTestFactory
	lockId string
}

func (f *isolatedFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.inner.CreateStorage(ctx, t)
}

func (f *isolatedFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	return f.inner.NewStorageLock(ctx, t, f.lockId)
}

func (f *isolatedFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	f.inner.CloseStorageLock(ctx, t, lock)
}

// newLock 是一个便捷方法，用于在测试中快速创建并清理一个锁
func newLock(t *testing.T, ctx context.Context, factory StorageLockTestFactory, lockId string) (*storage_lock.StorageLock, func()) {
	lock := factory.NewStorageLock(ctx, t, lockId)
	cleanup := func() {
		factory.CloseStorageLock(ctx, t, lock)
	}
	return lock, cleanup
}
