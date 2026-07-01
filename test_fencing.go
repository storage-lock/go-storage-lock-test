package storage_lock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	memory_storage "github.com/storage-lock/go-memory-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"github.com/stretchr/testify/assert"
)

// testFencingTokenMonotonic 验证漏洞 F 的修复：栅栏令牌（fencing token）。
//
// 场景（模拟"持有者 A 停顿/宕机、租约过期、B 合法抢占"这一租约锁的经典安全边界）：
//  1. A 用 NoopWatchDog 持锁（不续租），取得栅栏令牌 tokenA
//  2. 等待租约自然过期
//  3. B 抢占过期锁成功，取得栅栏令牌 tokenB
//  4. 断言 tokenB > tokenA —— 令牌跨所有权变更【严格单调递增】，这正是栅栏保证：
//     即便停顿恢复的 A 仍以为自己持锁，它手里的 tokenA 已小于 tokenB，
//     被保护资源侧只需拒绝"令牌 <= 已见过的最大值"的写入，即可挡住迟到的 A。
//  5. 断言 A 此刻查询令牌返回 ErrLockNotBelongYou —— A 能自知已失去锁。
func testFencingTokenMonotonic(t *testing.T) {
	ctx := context.Background()

	// A、B 共享同一存储与同一 lockId，各自是独立的 StorageLock 实例（模拟两个进程/客户端）
	sharedStorage := memory_storage.NewMemoryStorage()
	lockId := "test-FencingTokenMonotonic"

	newLock := func() *storage_lock.StorageLock {
		options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
		options.LeaseExpireAfter = time.Second * 3 // 最小允许租约
		options.LeaseRefreshInterval = time.Second * 1
		options.VersionMissRetryInterval = time.Millisecond * 5
		// 关键：用 NoopWatchDog，持锁后不续租，租约会自然过期，从而能被抢占
		options.WatchDogFactory = &NoopWatchDogFactory{}
		lock, err := storage_lock.NewStorageLockWithOptions(sharedStorage, options)
		assert.Nil(t, err)
		return lock
	}

	lockA := newLock()
	lockB := newLock()

	ownerA := "owner-A"
	ownerB := "owner-B"

	// 1. A 加锁并取栅栏令牌
	err := lockA.Lock(ctx, ownerA)
	assert.Nil(t, err, "A 加锁应成功")

	tokenA, err := lockA.GetFencingToken(ctx, ownerA)
	assert.Nil(t, err, "A 持锁期间应能取得栅栏令牌")
	assert.True(t, tokenA > 0, "令牌应为正")

	// 2. 等租约过期（LeaseExpireAfter=3s，NoopWatchDog 不续租）
	time.Sleep(time.Second*3 + time.Millisecond*500)

	// 3. B 抢占过期锁，取栅栏令牌
	err = lockB.Lock(ctx, ownerB)
	assert.Nil(t, err, "租约过期后 B 应能抢占加锁")

	tokenB, err := lockB.GetFencingToken(ctx, ownerB)
	assert.Nil(t, err, "B 持锁期间应能取得栅栏令牌")

	// 4. 核心断言：令牌严格单调递增，这是栅栏令牌消除"停顿恢复的 A 破坏被保护资源"的关键
	assert.True(t, tokenB > tokenA,
		"栅栏令牌必须跨所有权变更严格单调递增：tokenB(%d) 应 > tokenA(%d)", tokenB, tokenA)

	// 5. A 此刻查询令牌应发现锁已不属于自己——A 能自知失锁，不应再对被保护资源动手
	_, err = lockA.GetFencingToken(ctx, ownerA)
	assert.True(t, errors.Is(err, storage_lock.ErrLockNotBelongYou),
		"A 失去锁后查询令牌应返回 ErrLockNotBelongYou，实际: %v", err)

	// 收尾：B 释放锁
	err = lockB.UnLock(ctx, ownerB)
	assert.Nil(t, err, "B 释放锁应成功")
}
