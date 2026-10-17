package storage_lock

import (
	"context"
	"sync"
	"testing"
	"time"

	storage_lock "github.com/storage-lock/go-storage-lock"
)

// TestStorageLock_FairQueueSemantics 对任意 Storage 运行跨实例公平排队测试。
// 用例里的每把锁都是独立创建的 StorageLock，只通过底层存储共享状态，
// 用来证明顺序来自存储记录而不是某个进程里的内存队列。
func TestStorageLock_FairQueueSemantics(t *testing.T, factory StorageLockTestFactory) {
	ctx := context.Background()
	cases := []struct {
		name string
		fn   func(t *testing.T, ctx context.Context, factory StorageLockTestFactory)
	}{
		{"ArrivalOrder", testFairArrivalOrder},
		{"CanceledHeadHandsOff", testFairCanceledHeadHandsOff},
		{"UnfairClientCannotCut", testFairUnfairClientCannotCut},
		{"ConcurrentArrival", testFairConcurrentArrival},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			c.fn(t, ctx, &isolatedFactory{inner: factory, lockId: "test-fair-" + c.name})
		})
	}
}

// fairFactory 把工厂创建出来的锁切到公平模式。工厂本身不知道这个选项，
// 所以每个具体存储实现都不用改就能跑这组用例。
type fairFactory struct {
	inner StorageLockTestFactory
}

func (f fairFactory) newLock(t *testing.T, ctx context.Context, lockId string) *storage_lock.StorageLock {
	t.Helper()
	lock := f.inner.NewStorageLock(ctx, t, lockId)
	lock.SetFair(true)
	return lock
}

func testFairArrivalOrder(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	wrapped := fairFactory{inner: factory}
	holder := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, holder)
	first := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, first)
	second := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, second)

	if err := holder.Lock(ctx, "holder"); err != nil {
		t.Fatalf("holder lock: %v", err)
	}

	// 后创建的实例先登记，登记顺序以存储里的 CAS 落库顺序为准。
	secondDone := make(chan error, 1)
	go func() { secondDone <- second.Lock(ctx, "second") }()
	waitForWaiter(t, holder, "second")

	firstDone := make(chan error, 1)
	go func() { firstDone <- first.Lock(ctx, "first") }()
	waitForWaiterCount(t, holder, 2)

	if err := holder.UnLock(ctx, "holder"); err != nil {
		t.Fatalf("holder unlock: %v", err)
	}
	if err := waitResult(t, secondDone); err != nil {
		t.Fatalf("先登记的实例应该先拿到锁: %v", err)
	}
	assertStillWaiting(t, firstDone)

	if err := second.UnLock(ctx, "second"); err != nil {
		t.Fatalf("second unlock: %v", err)
	}
	if err := waitResult(t, firstDone); err != nil {
		t.Fatalf("队首释放后队尾应该接手: %v", err)
	}
	if err := first.UnLock(ctx, "first"); err != nil {
		t.Fatalf("first unlock: %v", err)
	}
}

func testFairCanceledHeadHandsOff(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	wrapped := fairFactory{inner: factory}
	holder := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, holder)
	head := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, head)
	tail := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, tail)

	if err := holder.Lock(ctx, "holder"); err != nil {
		t.Fatalf("holder lock: %v", err)
	}
	headCtx, cancelHead := context.WithCancel(ctx)
	defer cancelHead()
	headDone := make(chan error, 1)
	go func() { headDone <- head.Lock(headCtx, "head") }()
	waitForWaiter(t, holder, "head")

	tailDone := make(chan error, 1)
	go func() { tailDone <- tail.Lock(ctx, "tail") }()
	waitForWaiterCount(t, holder, 2)

	cancelHead()
	if err := waitResult(t, headDone); err == nil {
		t.Fatal("被取消的队首不应该拿到锁")
	}
	if err := holder.UnLock(ctx, "holder"); err != nil {
		t.Fatalf("holder unlock: %v", err)
	}
	if err := waitResult(t, tailDone); err != nil {
		t.Fatalf("队首取消后下一位应该接手: %v", err)
	}
	if err := tail.UnLock(ctx, "tail"); err != nil {
		t.Fatalf("tail unlock: %v", err)
	}
}

func testFairUnfairClientCannotCut(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	wrapped := fairFactory{inner: factory}
	holder := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, holder)
	queued := wrapped.newLock(t, ctx, "")
	defer factory.CloseStorageLock(ctx, t, queued)
	// 这个实例保持工厂的默认配置，也就是不排队，代表一个没开公平选项的客户端。
	cutter := factory.NewStorageLock(ctx, t, "")
	defer factory.CloseStorageLock(ctx, t, cutter)

	if err := holder.Lock(ctx, "holder"); err != nil {
		t.Fatalf("holder lock: %v", err)
	}
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- queued.Lock(waitCtx, "queued") }()
	waitForWaiter(t, holder, "queued")
	if err := holder.UnLock(ctx, "holder"); err != nil {
		t.Fatalf("holder unlock: %v", err)
	}

	// 等到排队的人已经从空闲记录上把锁拿走，再让不排队的客户端抢同一次交接。
	// 交接期间它的 CAS 版本已经落后，不能插到队首前面。
	if err := waitResult(t, done); err != nil {
		t.Fatalf("排队者应该拿到锁: %v", err)
	}
	err := cutter.TryLock(ctx, "cutter")
	if err == nil {
		_ = cutter.UnLock(ctx, "cutter")
		t.Fatal("没开公平选项的客户端抢走了已经有人排队的锁")
	}
	if err := queued.UnLock(ctx, "queued"); err != nil {
		t.Fatalf("queued unlock: %v", err)
	}
}

func testFairConcurrentArrival(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	wrapped := fairFactory{inner: factory}
	const contenders = 6
	entered := make(chan string, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock := wrapped.newLock(t, ctx, "")
			defer factory.CloseStorageLock(ctx, t, lock)
			owner := lock.NewOwnerId()
			if err := lock.Lock(ctx, owner); err != nil {
				t.Errorf("lock %s: %v", owner, err)
				return
			}
			entered <- owner
			time.Sleep(20 * time.Millisecond)
			if err := lock.UnLock(ctx, owner); err != nil {
				t.Errorf("unlock %s: %v", owner, err)
			}
		}()
	}
	wg.Wait()
	close(entered)

	seen := map[string]bool{}
	order := make([]string, 0, contenders)
	for owner := range entered {
		if seen[owner] {
			t.Fatalf("临界区同时进入了两次: %s", owner)
		}
		seen[owner] = true
		order = append(order, owner)
	}
	if len(order) != contenders {
		t.Fatalf("应有 %d 个竞争者依次进入，实际 %d", contenders, len(order))
	}
}

func waitForWaiter(t *testing.T, lock *storage_lock.StorageLock, owner string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		waiters, err := lock.Waiters(context.Background())
		if err == nil {
			for _, waiter := range waiters {
				if waiter.OwnerId == owner {
					return
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s 没有登记进存储里的等待队列", owner)
}

func waitForWaiterCount(t *testing.T, lock *storage_lock.StorageLock, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		waiters, err := lock.Waiters(context.Background())
		if err == nil && len(waiters) >= count {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待队列没有达到 %d 人", count)
}

func waitResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("获取锁超时")
		return nil
	}
}

// assertStillWaiting 确认队尾在队首持有期间没有拿到锁。
func assertStillWaiting(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("队尾在队首释放前就拿到了锁: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
}
