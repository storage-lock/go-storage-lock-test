package storage_lock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// TestStorageLock_LifecycleSemantics 对任意 Storage 运行锁生命周期语义测试。
// 覆盖空参数拒绝、解锁幂等、观察接口、写意图防饥饿、重入上限和 WithLock 的自动释放。
func TestStorageLock_LifecycleSemantics(t *testing.T, factory StorageLockTestFactory) {
	ctx := context.Background()
	cases := []struct {
		name string
		fn   func(t *testing.T, ctx context.Context, factory StorageLockTestFactory)
	}{
		{"RejectsEmptyOwner", testLifecycleRejectsEmptyOwner},
		{"DoubleUnlockFails", testLifecycleDoubleUnlockFails},
		{"ObservationAPIs", testLifecycleObservationAPIs},
		{"WriterIntentBlocksNewReaders", testLifecycleWriterIntentBlocksNewReaders},
		{"ReentrantDepthLimit", testLifecycleReentrantDepthLimit},
		{"WithLockReleasesOnError", testLifecycleWithLockReleasesOnError},
		{"ReadLockContention", testLifecycleReadLockContention},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			c.fn(t, ctx, &isolatedFactory{inner: factory, lockId: "test-lifecycle-" + c.name})
		})
	}
}

// testLifecycleRejectsEmptyOwner 验证空 owner 在加锁、解锁、续租、读锁上都被拒绝，
// 并且不会在存储里留下记录。
func testLifecycleRejectsEmptyOwner(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	if err := lock.Lock(ctx, ""); !errors.Is(err, storage_lock.ErrOwnerIdEmpty) {
		t.Fatalf("空 owner 加锁应返回 ErrOwnerIdEmpty，实际 %v", err)
	}
	if err := lock.UnLock(ctx, ""); !errors.Is(err, storage_lock.ErrOwnerIdEmpty) {
		t.Fatalf("空 owner 解锁应返回 ErrOwnerIdEmpty，实际 %v", err)
	}
	if err := lock.Renew(ctx, ""); !errors.Is(err, storage_lock.ErrOwnerIdEmpty) {
		t.Fatalf("空 owner 续租应返回 ErrOwnerIdEmpty，实际 %v", err)
	}
	if err := lock.RLock(ctx, ""); !errors.Is(err, storage_lock.ErrOwnerIdEmpty) {
		t.Fatalf("空 owner 读锁应返回 ErrOwnerIdEmpty，实际 %v", err)
	}
	held, err := lock.IsLocked(ctx)
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if held {
		t.Fatal("空 owner 被拒绝后不应留下有效持有记录")
	}
}

// testLifecycleDoubleUnlockFails 验证第二次解锁失败，且失败后锁仍然可以被别人获取。
func testLifecycleDoubleUnlockFails(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	if err := lock.Lock(ctx, "owner"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := lock.UnLock(ctx, "owner"); err != nil {
		t.Fatalf("first unlock: %v", err)
	}
	if err := lock.UnLock(ctx, "owner"); err == nil {
		t.Fatal("第二次解锁不应成功")
	}
	if err := lock.Lock(ctx, "other"); err != nil {
		t.Fatalf("重复解锁之后别人应该能拿到锁: %v", err)
	}
	if err := lock.UnLock(ctx, "other"); err != nil {
		t.Fatalf("cleanup unlock: %v", err)
	}
}

// testLifecycleObservationAPIs 验证 Inspect、TTL、GetOwner、IsLocked 与真实持有状态一致，
// 释放后全部转为空闲。
func testLifecycleObservationAPIs(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	if held, err := lock.IsLocked(ctx); err != nil || held {
		t.Fatalf("未加锁时应为空闲: held=%v err=%v", held, err)
	}
	if err := lock.Lock(ctx, "observer"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	status, err := lock.Inspect(ctx)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !status.Held || status.Information.OwnerId != "observer" || status.Information.LockCount != 1 {
		t.Fatalf("Inspect 与持有状态不符: %+v", status)
	}
	if status.RemainingLease <= 0 {
		t.Fatalf("持有中的剩余租期应大于 0: %v", status.RemainingLease)
	}
	ttl, err := lock.TTL(ctx, "observer")
	if err != nil || ttl <= 0 {
		t.Fatalf("TTL 应大于 0: ttl=%v err=%v", ttl, err)
	}
	owner, err := lock.GetOwner(ctx)
	if err != nil || owner != "observer" {
		t.Fatalf("GetOwner 应返回 observer: owner=%q err=%v", owner, err)
	}
	if _, err := lock.TTL(ctx, "stranger"); err == nil {
		t.Fatal("不是持有者时 TTL 应返回错误")
	}
	if err := lock.UnLock(ctx, "observer"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if held, err := lock.IsLocked(ctx); err != nil || held {
		t.Fatalf("释放后应为空闲: held=%v err=%v", held, err)
	}
	if _, err := lock.GetOwner(ctx); err == nil {
		t.Fatal("释放后 GetOwner 应返回错误")
	}
}

// testLifecycleWriterIntentBlocksNewReaders 验证写者排队期间，新读者进不来，
// 已经在读的人不受影响，写者取消后新读者恢复可进入。
func testLifecycleWriterIntentBlocksNewReaders(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	if err := lock.RLock(ctx, "reader"); err != nil {
		t.Fatalf("reader: %v", err)
	}
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- lock.WLock(waitCtx, "writer") }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		status, err := lock.Inspect(ctx)
		if err == nil && status.WriteIntentOwner == "writer" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("写者没有登记写意图")
		}
		time.Sleep(5 * time.Millisecond)
	}

	lateCtx, lateCancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer lateCancel()
	if err := lock.RLock(lateCtx, "late-reader"); err == nil {
		t.Fatal("写者排队后新读者不应进入")
	}
	// 已在读的人可以重入，写意图只拦新读者。
	if err := lock.RLock(ctx, "reader"); err != nil {
		t.Fatalf("已有读者应能重入: %v", err)
	}
	if err := lock.RUnLock(ctx, "reader"); err != nil {
		t.Fatalf("reentry release: %v", err)
	}

	cancel()
	if err := <-done; err == nil {
		t.Fatal("被取消的写者不应拿到锁")
	}
	if err := lock.RLock(ctx, "late-reader"); err != nil {
		t.Fatalf("写者取消后新读者应能进入: %v", err)
	}
	if err := lock.RUnLock(ctx, "late-reader"); err != nil {
		t.Fatalf("late release: %v", err)
	}
	if err := lock.RUnLock(ctx, "reader"); err != nil {
		t.Fatalf("reader release: %v", err)
	}
}

// testLifecycleReentrantDepthLimit 验证达到上限后继续加锁被拒绝，且拒绝不改变已有深度。
func testLifecycleReentrantDepthLimit(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-lifecycle-ReentrantDepthLimit"
	options := shortLeaseOptions(lockId)
	options.MaxReentrantDepth = 2
	lock, err := storage_lock.NewStorageLockWithOptions(s, options)
	if err != nil {
		t.Fatalf("create lock: %v", err)
	}
	if err := lock.Lock(ctx, "owner"); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if err := lock.Lock(ctx, "owner"); err != nil {
		t.Fatalf("second lock: %v", err)
	}
	if err := lock.Lock(ctx, "owner"); !errors.Is(err, storage_lock.ErrMaxReentrantDepth) {
		t.Fatalf("第三次加锁应返回 ErrMaxReentrantDepth，实际 %v", err)
	}
	current, err := s.Get(ctx, lockId)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	info, err := storage.LockInformationFromJsonString(current)
	if err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if info.LockCount != 2 {
		t.Fatalf("被拒绝的加锁不应改变深度: %d", info.LockCount)
	}
	if err := lock.UnLock(ctx, "owner"); err != nil {
		t.Fatalf("unlock one: %v", err)
	}
	if err := lock.UnLock(ctx, "owner"); err != nil {
		t.Fatalf("unlock two: %v", err)
	}
}

// testLifecycleWithLockReleasesOnError 验证回调返回错误时锁也被释放，别人立刻可以获取。
func testLifecycleWithLockReleasesOnError(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	sentinel := errors.New("business failed")
	var token storage.Version
	err := lock.WithLock(ctx, func(_ context.Context, got storage.Version) error {
		token = got
		if got == 0 {
			t.Error("WithLock 应提供非零 fencing token")
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithLock 应原样返回业务错误，实际 %v", err)
	}
	if token == 0 {
		t.Fatal("回调没有拿到 fencing token")
	}
	if err := lock.Lock(ctx, "other"); err != nil {
		t.Fatalf("回调失败后锁应已释放: %v", err)
	}
	if err := lock.UnLock(ctx, "other"); err != nil {
		t.Fatalf("cleanup unlock: %v", err)
	}
}

// testLifecycleReadLockContention 验证多个读者可以同时进入，写者必须等全部读者离开。
func testLifecycleReadLockContention(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	const readers = 5
	started := make(chan struct{}, readers)
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			owner := "reader-" + itoa(id)
			if err := lock.RLock(ctx, owner); err != nil {
				t.Errorf("reader %d: %v", id, err)
				return
			}
			started <- struct{}{}
			<-release
			if err := lock.RUnLock(ctx, owner); err != nil {
				t.Errorf("reader %d release: %v", id, err)
			}
		}(i)
	}
	for i := 0; i < readers; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("只有 %d 个读者进入，共享读锁没有生效", i)
		}
	}

	writerDone := make(chan error, 1)
	go func() { writerDone <- lock.WLock(ctx, "writer") }()
	select {
	case err := <-writerDone:
		t.Fatalf("读者还在时写者不应拿到锁: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	if err := waitResult(t, writerDone); err != nil {
		t.Fatalf("读者全部离开后写者应拿到锁: %v", err)
	}
	if err := lock.WUnLock(ctx, "writer"); err != nil {
		t.Fatalf("writer release: %v", err)
	}
}
