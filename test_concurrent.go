package storage_lock_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testConcurrentCompetition 测试并发竞争：多个协程争抢同一把锁，
// 验证：
//  1. 临界区互斥——任意时刻最多只有一个协程在临界区内
//  2. 计数器最终值正确——所有协程的递增都完整执行（没有丢失或重复）
func testConcurrentCompetition(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "concurrent")
	defer cleanup()

	const goroutineCount = 10
	const incrementsPerGoroutine = 20

	var counter int64        // 受锁保护的临界资源
	var inCriticalSection int64 // 当前在临界区内的协程数（必须始终 <= 1）
	var maxConcurrent int64  // 观察到的最大并发数（必须 == 1，验证互斥）
	var lockAcquireCount int64

	var wg sync.WaitGroup
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ownerId := "owner-concurrent-" + itoa(id)
			for j := 0; j < incrementsPerGoroutine; j++ {

				// 获取锁，使用较长超时以应对高竞争
				lockCtx, cancel := context.WithTimeout(ctx, time.Second*30)
				err := lock.Lock(lockCtx, ownerId)
				if err != nil {
					cancel()
					t.Errorf("goroutine %d failed to acquire lock: %v", id, err)
					return
				}
				atomic.AddInt64(&lockAcquireCount, 1)

				// 进入临界区：增加并发计数
				cur := atomic.AddInt64(&inCriticalSection, 1)
				// 更新观察到的最大并发数
				for {
					max := atomic.LoadInt64(&maxConcurrent)
					if cur <= max || atomic.CompareAndSwapInt64(&maxConcurrent, max, cur) {
						break
					}
				}

				// 模拟临界区工作
				atomic.AddInt64(&counter, 1)
				time.Sleep(time.Microsecond * 10)

				// 离开临界区
				atomic.AddInt64(&inCriticalSection, -1)

				// 释放锁
				err = lock.UnLock(lockCtx, ownerId)
				cancel()
				if err != nil {
					t.Errorf("goroutine %d failed to release lock: %v", id, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	expected := int64(goroutineCount * incrementsPerGoroutine)

	// 验证计数器正确（所有递增都执行了，无丢失无重复）
	assert.Equal(t, expected, counter,
		"counter should equal expected, got %d expected %d", counter, expected)

	// 验证互斥性：临界区任意时刻最多只有一个协程
	assert.Equal(t, int64(1), atomic.LoadInt64(&maxConcurrent),
		"max concurrent in critical section should be 1, got %d", maxConcurrent)

	// 验证加锁次数（应该等于总递增次数）
	assert.Equal(t, expected, atomic.LoadInt64(&lockAcquireCount),
		"lock acquire count should equal expected")

	t.Logf("Concurrent test passed: %d goroutines x %d increments = %d, maxConcurrent=%d",
		goroutineCount, incrementsPerGoroutine, counter, maxConcurrent)
}

// itoa 简单的整数转字符串，避免引入 strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
