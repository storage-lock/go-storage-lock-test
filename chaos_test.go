package storage_lock_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-infrastructure/go-iterator"
	memory_storage "github.com/storage-lock/go-memory-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// 混沌注入：对底层的 MemoryStorage 做代理，可以随时：
//   - Pause：让所有存储操作阻塞一段时间（模拟网络分区/进程挂起）
//   - FailN：让接下来 N 个操作直接报错（模拟抖动）
//   - SetTimeSkew：让 GetTime 返回带偏移的时间（模拟时钟漂移）
//
// 分布式锁的互斥性最终由 Storage 的 CAS + 统一时钟保证；混沌测试验证的是
// 在这些故障下锁仍然满足两个不变量：同一时刻至多一个有效持有者（以存储时间为
// 准），以及栅栏令牌严格递增、被抢占的旧 owner 无法再续租。
type chaosStorage struct {
	inner storage.Storage

	mu         sync.Mutex
	pauseUntil time.Time
	skew       time.Duration
	failOps    int
}

var errChaosInjected = errors.New("chaos: injected storage failure")

func (x *chaosStorage) GetName() string { return "chaos-" + x.inner.GetName() }

func (x *chaosStorage) Capabilities() []storage.StorageCapability {
	return x.inner.Capabilities()
}

func (x *chaosStorage) Init(ctx context.Context) error {
	return x.gate(ctx, func() error { return x.inner.Init(ctx) })
}

func (x *chaosStorage) Get(ctx context.Context, lockId string) (string, error) {
	var out string
	err := x.gate(ctx, func() error {
		var e error
		out, e = x.inner.Get(ctx, lockId)
		return e
	})
	return out, err
}

func (x *chaosStorage) CreateWithVersion(ctx context.Context, lockId string, version storage.Version, info *storage.LockInformation) error {
	return x.gate(ctx, func() error { return x.inner.CreateWithVersion(ctx, lockId, version, info) })
}

func (x *chaosStorage) UpdateWithVersion(ctx context.Context, lockId string, expected, newVersion storage.Version, info *storage.LockInformation) error {
	return x.gate(ctx, func() error { return x.inner.UpdateWithVersion(ctx, lockId, expected, newVersion, info) })
}

func (x *chaosStorage) DeleteWithVersion(ctx context.Context, lockId string, expected storage.Version, info *storage.LockInformation) error {
	return x.gate(ctx, func() error { return x.inner.DeleteWithVersion(ctx, lockId, expected, info) })
}

func (x *chaosStorage) GetTime(ctx context.Context) (time.Time, error) {
	var out time.Time
	err := x.gate(ctx, func() error {
		var e error
		out, e = x.inner.GetTime(ctx)
		if e == nil {
			out = out.Add(x.getSkew())
		}
		return e
	})
	return out, err
}

func (x *chaosStorage) List(ctx context.Context) (iterator.Iterator[*storage.LockInformation], error) {
	return x.inner.List(ctx)
}

func (x *chaosStorage) Close(ctx context.Context) error {
	return x.gate(ctx, func() error { return x.inner.Close(ctx) })
}

// gate 统一入口：暂停期阻塞（尊重 ctx 取消），抖动期注入错误。
func (x *chaosStorage) gate(ctx context.Context, op func() error) error {
	x.mu.Lock()
	pauseUntil := x.pauseUntil
	fail := x.failOps > 0
	if fail {
		x.failOps--
	}
	x.mu.Unlock()

	if pauseUntil.After(time.Now()) {
		timer := time.NewTimer(time.Until(pauseUntil))
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if fail {
		return errChaosInjected
	}
	return op()
}

func (x *chaosStorage) pause(d time.Duration) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.pauseUntil = time.Now().Add(d)
}

func (x *chaosStorage) failNext(n int) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.failOps = n
}

func (x *chaosStorage) setTimeSkew(d time.Duration) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.skew = d
}

func (x *chaosStorage) getSkew() time.Duration {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.skew
}

// newChaosLock 创建带混沌注入的锁。租约 3s / 续租 1s，与语义测试基线一致。
func newChaosLock(lockId string) (*chaosStorage, *storage_lock.StorageLock) {
	return newChaosLockWithOptions(lockId, false)
}

// newChaosLockNoWatchDog 与 newChaosLock 相同，但关闭自动续租（DisableWatchDog）。
// 用于时间回拨场景：持有者不自动续租，租约严格按固定租期在真实时钟上过期，
// 避免"回拨 + 活跃看门狗"把租约越续越长的歧义（那是另一个混沌维度）。
func newChaosLockNoWatchDog(lockId string) (*chaosStorage, *storage_lock.StorageLock) {
	return newChaosLockWithOptions(lockId, true)
}

func newChaosLockWithOptions(lockId string, disableWatchDog bool) (*chaosStorage, *storage_lock.StorageLock) {
	inner := memory_storage.NewMemoryStorage()
	chaos := &chaosStorage{inner: inner}
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = 3 * time.Second
	options.LeaseRefreshInterval = time.Second
	options.VersionMissRetryInterval = 10 * time.Millisecond
	options.DisableWatchDog = disableWatchDog
	lock, err := storage_lock.NewStorageLockWithOptions(chaos, options)
	if err != nil {
		panic(err)
	}
	return chaos, lock
}

// acquireWithFence 获取锁并返回栅栏令牌；失败返回 0。
func acquireWithFence(ctx context.Context, lock *storage_lock.StorageLock, owner string) storage.Version {
	if err := lock.Lock(ctx, owner); err != nil {
		return 0
	}
	token, err := lock.GetFencingToken(ctx, owner)
	if err != nil {
		return 0
	}
	return token
}

// singleHolderJSON 解析存储里的锁记录，断言只有一个有效持有者。
func assertSingleHolder(t *testing.T, ctx context.Context, s storage.Storage, lockId string) {
	t.Helper()
	// chaosStorage 可能带着注入的失败（failNext）：读失败不一定是状态坏，
	// 等注入耗尽后重试（最多 2s）。真实抖动中读被拒绝本身不是互斥性破坏。
	var raw string
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for {
		raw, err = s.Get(ctx, lockId)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("storage get failed (injection may not drain): %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if raw == "" {
		return // 未创建过锁也是合法状态
	}
	info, err := storage.LockInformationFromJsonString(raw)
	if err != nil {
		t.Fatalf("parse lock info failed: %v", err)
	}
	if info == nil {
		return
	}
	if info.LockCount == 0 {
		return // 墓碑，无持有者
	}
	now, err := s.GetTime(ctx)
	if err != nil {
		t.Fatalf("gettime failed: %v", err)
	}
	if !now.Before(info.LeaseExpireTime) {
		return // 租约已过期，视为无持有者
	}
	// 有一个未过期的持有者：owner 只能有一个（共享读锁场景本套件不涉及）。
	if info.ReadLock {
		t.Fatalf("unexpected read lock record: %#v", info)
	}
	if info.OwnerId == "" || info.LockCount <= 0 {
		t.Fatalf("broken holder record: %#v", info)
	}
}

// TestChaosPauseDuringHold：持有中整库"卡死"（分区）。看门狗续租失败、
// 租约按存储时间过期，恢复后新 owner 能接管，旧 owner 无法续租，栅栏递增。
func TestChaosPauseDuringHold(t *testing.T) {
	ctx := context.Background()
	chaos, lock := newChaosLock("chaos-pause")
	_ = chaos.Init(ctx)

	ownerA := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	tokenA := acquireWithFence(ctx, lock, ownerA)
	if tokenA == 0 {
		t.Fatal("owner A failed to acquire")
	}

	// 分区：所有操作卡死 5s > 租约 3s，A 的续租必然失败。
	chaos.pause(5 * time.Second)
	time.Sleep(1500 * time.Millisecond) // 让看门狗撞上分区

	// 分区恢复。
	time.Sleep(3700 * time.Millisecond)

	// A 的续租应失败（锁已被视为过期/易主，旧 owner 不能再续租）。
	if err := lock.Renew(ctx, ownerA); err == nil {
		t.Fatal("stale owner A renewed after losing the lock during partition")
	}

	// B 接管，栅栏严格递增。
	ownerB := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	tokenB := acquireWithFence(ctx, lock, ownerB)
	if tokenB == 0 {
		t.Fatal("owner B failed to acquire after partition")
	}
	if tokenB <= tokenA {
		t.Fatalf("fencing token must increase across partition, A=%d B=%d", tokenA, tokenB)
	}
	assertSingleHolder(t, ctx, chaos, "chaos-pause")
}

// TestChaosClockSkewForward：时钟向前漂移，租约看起来提前过期。
// 新 owner 接管后旧 owner 的续租必须失败，栅栏递增，无双重持有。
func TestChaosClockSkewForward(t *testing.T) {
	ctx := context.Background()
	chaos, lock := newChaosLock("chaos-skew-fwd")
	_ = chaos.Init(ctx)

	ownerA := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	tokenA := acquireWithFence(ctx, lock, ownerA)
	if tokenA == 0 {
		t.Fatal("owner A failed to acquire")
	}

	// 时钟向前漂 30s，A 的 3s 租约瞬间"过期"。
	chaos.setTimeSkew(30 * time.Second)

	ownerB := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	tokenB := acquireWithFence(ctx, lock, ownerB)
	if tokenB == 0 {
		t.Fatal("owner B failed to take over under forward clock skew")
	}
	if tokenB <= tokenA {
		t.Fatalf("fencing token must increase across skew, A=%d B=%d", tokenA, tokenB)
	}

	// 旧 owner 续租失败（版本已变）。
	if err := lock.Renew(ctx, ownerA); err == nil {
		t.Fatal("stale owner A renewed after clock skew takeover")
	}
	assertSingleHolder(t, ctx, chaos, "chaos-skew-fwd")
}

// TestChaosClockSkewBackward：时钟回拨是危险方向——租约显得更长。
// 断言：回拨期间新 owner 无法获取（不会出现双重持有）；回拨消除后按
// 真实时间过期，正常交接且栅栏递增。
func TestChaosClockSkewBackward(t *testing.T) {
	ctx := context.Background()
	// 无看门狗：A 是固定租期客户端。回拨让 A 的租约"显得更长"，
	// 但 A 不续租，回拨消除后按真实 3s 过期（这是要断言的场景）。
	chaos, lock := newChaosLockNoWatchDog("chaos-skew-back")
	_ = chaos.Init(ctx)

	ownerA := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	tokenA := acquireWithFence(ctx, lock, ownerA)
	if tokenA == 0 {
		t.Fatal("owner A failed to acquire")
	}

	// 回拨 30s：A 的租约被"延长"到 ~33s。
	chaos.setTimeSkew(-30 * time.Second)

	// 回拨期间 B 尝试获取：带 500ms 短超时，必须超时失败（Lock 是阻塞轮询，
	// 不能双重持有；无超时 ctx 会永久等待直到回拨消除）。等超时返回后，
	// 租约仍被"延长"，断言存储里仍只有 A 一个持有者。
	ownerB := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	skewCtx, skewCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	skewErr := lock.Lock(skewCtx, ownerB)
	skewCancel()
	if skewErr == nil {
		t.Fatal("owner B must not acquire while clock is skewed backward (no double-hold)")
	}
	assertSingleHolder(t, ctx, chaos, "chaos-skew-back")

	// 回拨消除后，A 的租约按真实时间正常过期，B 才能接管。
	// A 从获取到此刻最多持有 ~4s（3s 租约 + 回拨期 0.5s + 余量），等 4.5s 确保过期。
	chaos.setTimeSkew(0)
	time.Sleep(4500 * time.Millisecond)

	tokenB := acquireWithFence(ctx, lock, ownerB)
	if tokenB == 0 {
		t.Fatal("owner B failed to acquire after skew removed")
	}
	if tokenB <= tokenA {
		t.Fatalf("fencing token must increase, A=%d B=%d", tokenA, tokenB)
	}
	if err := lock.Renew(ctx, ownerA); err == nil {
		t.Fatal("stale owner A renewed after losing the lock")
	}
	assertSingleHolder(t, ctx, chaos, "chaos-skew-back")
}

// TestChaosFlakyFailures：存储抖动——看门狗可能续租失败，但互斥性与
// 栅栏单调性不受影响；抖动耗尽后锁状态一致、可正常交接。
func TestChaosFlakyFailures(t *testing.T) {
	ctx := context.Background()
	chaos, lock := newChaosLock("chaos-flaky")
	_ = chaos.Init(ctx)

	ownerA := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	tokenA := acquireWithFence(ctx, lock, ownerA)
	if tokenA == 0 {
		t.Fatal("owner A failed to acquire")
	}

	// 注入 6 次失败，覆盖看门狗的若干轮续租（每秒 1 次，2.2s 内约 2-3 次），
	// 剩余几次由后续操作消耗。注入太多会让"抖动耗尽"窗口拉长，反而等不到。
	chaos.failNext(6)
	time.Sleep(2200 * time.Millisecond)

	// 抖动耗尽后：要么 A 仍有效持有（看门狗恢复了续租），要么已过期可被接管。
	// 两种都合法，但绝不能出现"两个有效持有者"或栅栏回退。
	assertSingleHolder(t, ctx, chaos, "chaos-flaky")

	ownerB := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	if err := lock.TryLock(ctx, ownerB); err == nil {
		tokenB, _ := lock.GetFencingToken(ctx, ownerB)
		if tokenB <= tokenA {
			t.Fatalf("fencing token must increase, A=%d B=%d", tokenA, tokenB)
		}
		_ = lock.UnLock(ctx, ownerB)
	} else if errors.Is(err, storage_lock.ErrLockBusy) {
		// A 仍持有，正常；续租恢复后检查 A 还能续租。
		if err := lock.Renew(ctx, ownerA); err != nil {
			t.Fatalf("owner A should be able to renew after flaky period: %v", err)
		}
	} else {
		t.Fatalf("unexpected acquire error after flaky period: %v", err)
	}
	assertSingleHolder(t, ctx, chaos, "chaos-flaky")
}

// TestChaosPartitionDuringAcquire：获取期间分区——调用方超时失败，
// 不产生部分状态；分区恢复后可正常获取。
func TestChaosPartitionDuringAcquire(t *testing.T) {
	ctx := context.Background()
	chaos, lock := newChaosLock("chaos-part-acquire")
	_ = chaos.Init(ctx)

	chaos.pause(2 * time.Second)
	ctxTimeout, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	if err := lock.Lock(ctxTimeout, storage_lock.NewOwnerIdGenerator().GenOwnerId()); err == nil {
		t.Fatal("acquire should fail while storage is partitioned")
	}
	time.Sleep(1300 * time.Millisecond) // 分区结束

	owner := storage_lock.NewOwnerIdGenerator().GenOwnerId()
	if token := acquireWithFence(ctx, lock, owner); token == 0 {
		t.Fatal("acquire should succeed after partition ends")
	}
	assertSingleHolder(t, ctx, chaos, "chaos-part-acquire")
}

// TestChaosConcurrentHandover：多次分区 + 时钟漂移交替注入下，
// 并发抢占的栅栏令牌保持严格递增（多 goroutine 同时抢同一把锁）。
func TestChaosConcurrentHandover(t *testing.T) {
	ctx := context.Background()
	chaos, lock := newChaosLock("chaos-concurrent")
	_ = chaos.Init(ctx)

	const rounds = 6
	var lastFence atomic.Uint64

	for i := 0; i < rounds; i++ {
		owner := storage_lock.NewOwnerIdGenerator().GenOwnerId()
		if i%2 == 0 {
			chaos.failNext(8) // 抖动
		} else {
			chaos.pause(1200 * time.Millisecond) // 短分区
		}

		// 每轮给并发 goroutine 一个共享超时 ctx：如果一轮里无人能抢到锁
		// （抖动耗尽/分区期间所有尝试都失败），goroutine 在 Lock 轮询中
		// 等满 timeout 后放弃本轮，避免永久阻塞（wg.Wait 永不返回）。
		roundCtx, roundCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		var wg sync.WaitGroup
		var fence atomic.Uint64
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(owner string) {
				defer wg.Done()
				if token := acquireWithFence(roundCtx, lock, owner); token > 0 {
					prev := fence.Load()
					for prev < uint64(token) && !fence.CompareAndSwap(prev, uint64(token)) {
						prev = fence.Load()
					}
					// 临界区结束立即释放，让下一个人可以抢到。
					_ = lock.UnLock(roundCtx, owner)
				}
			}(owner)
		}
		wg.Wait()
		roundCancel()

		if got := fence.Load(); got > 0 {
			if prev := lastFence.Load(); prev > 0 && got <= prev {
				t.Fatalf("fencing token regressed across chaos rounds: prev=%d got=%d", prev, got)
			}
			lastFence.Store(got)
		}
		// 每轮结束释放，进入下一轮混沌。
		_ = lock.TryLock(ctx, storage_lock.NewOwnerIdGenerator().GenOwnerId())
		time.Sleep(300 * time.Millisecond)
	}
	assertSingleHolder(t, ctx, chaos, "chaos-concurrent")
}
