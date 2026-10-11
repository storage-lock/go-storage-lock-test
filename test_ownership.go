package storage_lock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// TestStorageLock_OwnershipSemantics 对任意 Storage 运行持有权语义测试。
// 这些用例验证的是锁记录上的不变量：fencing token 单调、读锁升级不剥夺其他读者、
// 手动续租能顶住过期、持有权被抢走后业务上下文被取消。
func TestStorageLock_OwnershipSemantics(t *testing.T, factory StorageLockTestFactory) {
	ctx := context.Background()
	cases := []struct {
		name string
		fn   func(t *testing.T, ctx context.Context, factory StorageLockTestFactory)
	}{
		{"FencingTokenMonotonic", testOwnershipFencingMonotonic},
		{"StaleOwnerRejected", testOwnershipStaleOwnerRejected},
		{"UpgradeOnlyReader", testOwnershipUpgradeOnlyReader},
		{"UpgradeKeepsOtherReaders", testOwnershipUpgradeKeepsOtherReaders},
		{"DowngradeAllowsReader", testOwnershipDowngradeAllowsReader},
		{"ManualRenewalExtendsLease", testOwnershipManualRenewal},
		{"LostLeaseCancelsWork", testOwnershipLostCancelsWork},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			c.fn(t, ctx, &isolatedFactory{inner: factory, lockId: "test-ownership-" + c.name})
		})
	}
}

// testOwnershipFencingMonotonic 验证每次所有权转移都拿到严格更大的 token。
// 资源侧用这个 token 拒绝旧持有者在租约过期后继续写入。
func testOwnershipFencingMonotonic(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-ownership-FencingTokenMonotonic"
	options := shortLeaseOptions(lockId)
	options.WatchDogFactory = &NoopWatchDogFactory{}

	first, err := storage_lock.NewStorageLockWithOptions(s, options)
	if err != nil {
		t.Fatalf("create first lock: %v", err)
	}
	if err := first.Lock(ctx, "owner-a"); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	tokenA, err := first.GetFencingToken(ctx, "owner-a")
	if err != nil {
		t.Fatalf("token of owner-a: %v", err)
	}
	if err := first.UnLock(ctx, "owner-a"); err != nil {
		t.Fatalf("unlock owner-a: %v", err)
	}

	second, err := storage_lock.NewStorageLockWithOptions(s, shortLeaseOptions(lockId))
	if err != nil {
		t.Fatalf("create second lock: %v", err)
	}
	if err := second.Lock(ctx, "owner-b"); err != nil {
		t.Fatalf("second lock: %v", err)
	}
	tokenB, err := second.GetFencingToken(ctx, "owner-b")
	if err != nil {
		t.Fatalf("token of owner-b: %v", err)
	}
	if tokenB <= tokenA {
		t.Fatalf("fencing token 必须严格递增: owner-a=%d owner-b=%d", tokenA, tokenB)
	}

	// 旧持有者拿自己过期的身份既续不上租，也读不到新 token。
	if err := first.Renew(ctx, "owner-a"); err == nil {
		t.Fatal("旧持有者不应续上别人的租约")
	}
	if _, err := first.GetFencingToken(ctx, "owner-a"); err == nil {
		t.Fatal("旧持有者不应读到当前 fencing token")
	}
	if err := second.UnLock(ctx, "owner-b"); err != nil {
		t.Fatalf("unlock owner-b: %v", err)
	}
}

// testOwnershipStaleOwnerRejected 验证租约过期被抢后，原持有者的解锁和续租都被拒绝，
// 新持有者的记录保持不动。
func testOwnershipStaleOwnerRejected(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-ownership-StaleOwnerRejected"
	options := shortLeaseOptions(lockId)
	options.WatchDogFactory = &NoopWatchDogFactory{}
	stale, err := storage_lock.NewStorageLockWithOptions(s, options)
	if err != nil {
		t.Fatalf("create stale lock: %v", err)
	}
	if err := stale.Lock(ctx, "stale"); err != nil {
		t.Fatalf("stale lock: %v", err)
	}
	time.Sleep(leaseExpiryWait)

	fresh, err := storage_lock.NewStorageLockWithOptions(s, shortLeaseOptions(lockId))
	if err != nil {
		t.Fatalf("create fresh lock: %v", err)
	}
	acquireCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := fresh.Lock(acquireCtx, "fresh"); err != nil {
		t.Fatalf("fresh lock after expiry: %v", err)
	}

	if err := stale.UnLock(ctx, "stale"); err == nil {
		t.Fatal("过期的旧持有者不应解锁成功")
	}
	if err := stale.Renew(ctx, "stale"); err == nil {
		t.Fatal("过期的旧持有者不应续租成功")
	}
	status, err := fresh.Inspect(ctx)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if status.Information.OwnerId != "fresh" || !status.Held {
		t.Fatalf("新持有者的记录被旧持有者改动了: owner=%s held=%v", status.Information.OwnerId, status.Held)
	}
	if err := fresh.UnLock(ctx, "fresh"); err != nil {
		t.Fatalf("unlock fresh: %v", err)
	}
}

// testOwnershipUpgradeOnlyReader 验证唯一读者可以把读锁升级成独占锁，升级后别人进不来。
func testOwnershipUpgradeOnlyReader(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()
	if err := lock.RLock(ctx, "reader"); err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if err := lock.TryUpgrade(ctx, "reader"); err != nil {
		t.Fatalf("唯一读者升级应该成功: %v", err)
	}
	status, err := lock.Inspect(ctx)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if status.Information.ReadLock || status.Information.OwnerId != "reader" || status.Information.LockCount != 1 {
		t.Fatalf("升级后应为独占锁: %+v", status.Information)
	}
	shortCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if err := lock.RLock(shortCtx, "other"); err == nil {
		t.Fatal("升级后其他读者不应进入")
	}
	if err := lock.UnLock(ctx, "reader"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
}

// testOwnershipUpgradeKeepsOtherReaders 验证还有其他读者时升级失败，且两个人的读锁都保持原样。
func testOwnershipUpgradeKeepsOtherReaders(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()
	if err := lock.RLock(ctx, "reader-a"); err != nil {
		t.Fatalf("reader-a: %v", err)
	}
	if err := lock.RLock(ctx, "reader-b"); err != nil {
		t.Fatalf("reader-b: %v", err)
	}
	err := lock.TryUpgrade(ctx, "reader-a")
	if !errors.Is(err, storage_lock.ErrUpgradeConflict) {
		t.Fatalf("有其他读者时升级应返回 ErrUpgradeConflict，实际 %v", err)
	}
	status, err := lock.Inspect(ctx)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !status.Information.ReadLock || status.Information.LockCount != 2 {
		t.Fatalf("升级失败后读锁应保持两层: %+v", status.Information)
	}
	if status.Information.ReadDepth("reader-a") != 1 || status.Information.ReadDepth("reader-b") != 1 {
		t.Fatalf("两个读者的持有深度都应还在: %+v", status.Information.ReadOwners)
	}
	if err := lock.RUnLock(ctx, "reader-b"); err != nil {
		t.Fatalf("release reader-b: %v", err)
	}
	if err := lock.RUnLock(ctx, "reader-a"); err != nil {
		t.Fatalf("release reader-a: %v", err)
	}
}

// testOwnershipDowngradeAllowsReader 验证独占锁降级后变成共享读锁，其他读者可以进入。
func testOwnershipDowngradeAllowsReader(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()
	if err := lock.Lock(ctx, "writer"); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	if err := lock.Downgrade(ctx, "writer"); err != nil {
		t.Fatalf("downgrade: %v", err)
	}
	if err := lock.RLock(ctx, "reader"); err != nil {
		t.Fatalf("降级后读者应该能进入: %v", err)
	}
	status, err := lock.Inspect(ctx)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !status.Information.ReadLock || status.Information.LockCount != 2 {
		t.Fatalf("降级后应是两层共享读锁: %+v", status.Information)
	}
	if err := lock.RUnLock(ctx, "writer"); err != nil {
		t.Fatalf("release writer: %v", err)
	}
	if err := lock.RUnLock(ctx, "reader"); err != nil {
		t.Fatalf("release reader: %v", err)
	}
}

// testOwnershipManualRenewal 验证关闭看门狗后，持有者自己续租也能把过期时间顶到租期之外。
func testOwnershipManualRenewal(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-ownership-ManualRenewalExtendsLease"
	options := shortLeaseOptions(lockId)
	options.DisableWatchDog = true
	lock, err := storage_lock.NewStorageLockWithOptions(s, options)
	if err != nil {
		t.Fatalf("create lock: %v", err)
	}
	if err := lock.Lock(ctx, "owner"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	before, err := s.Get(ctx, lockId)
	if err != nil {
		t.Fatalf("read before renew: %v", err)
	}
	beforeInfo, err := storage.LockInformationFromJsonString(before)
	if err != nil {
		t.Fatalf("decode before renew: %v", err)
	}
	time.Sleep(time.Second)
	if err := lock.Renew(ctx, "owner"); err != nil {
		t.Fatalf("renew: %v", err)
	}
	after, err := s.Get(ctx, lockId)
	if err != nil {
		t.Fatalf("read after renew: %v", err)
	}
	afterInfo, err := storage.LockInformationFromJsonString(after)
	if err != nil {
		t.Fatalf("decode after renew: %v", err)
	}
	if !afterInfo.LeaseExpireTime.After(beforeInfo.LeaseExpireTime) {
		t.Fatalf("续租没有把过期时间往后推: before=%v after=%v", beforeInfo.LeaseExpireTime, afterInfo.LeaseExpireTime)
	}
	if afterInfo.LockCount != 1 {
		t.Fatalf("续租不应改变重入深度: %d", afterInfo.LockCount)
	}

	// 等到续租前的那个过期时间已经过去，别人仍然拿不走。
	// 用存储返回的时间而不是本机墙钟：有的存储时间只精确到秒，墙钟对不齐。
	deadline := time.Now().Add(5 * time.Second)
	for {
		now, err := s.GetTime(ctx)
		if err != nil {
			t.Fatalf("storage time: %v", err)
		}
		if !now.Before(beforeInfo.LeaseExpireTime) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("存储时间没有前进到续租前的过期点")
		}
		time.Sleep(100 * time.Millisecond)
	}
	other := factory.NewStorageLock(ctx, t, lockId)
	shortCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if err := other.Lock(shortCtx, "other"); err == nil {
		t.Fatal("手动续租有效期间别人不应拿到锁")
	}
	if err := lock.UnLock(ctx, "owner"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
}

// testOwnershipLostCancelsWork 验证持有者停掉看门狗、租约被别人抢过后，
// BeginLease 交给业务的 context 以 ErrLockLost 取消。
func testOwnershipLostCancelsWork(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	s := factory.CreateStorage(ctx, t)
	lockId := "test-ownership-LostLeaseCancelsWork"
	options := shortLeaseOptions(lockId)
	options.LeaseRefreshInterval = 200 * time.Millisecond
	holder, err := storage_lock.NewStorageLockWithOptions(s, options)
	if err != nil {
		t.Fatalf("create holder: %v", err)
	}
	owned, stop, err := holder.BeginLease(ctx, "holder")
	if err != nil {
		t.Fatalf("begin lease: %v", err)
	}
	defer stop()

	// 等看门狗至少完成一次续租，确认它在运行，再把租约改成已过期。
	// 看门狗还在按 LeaseRefreshInterval 续租，读到的版本随时会被它写掉，
	// 所以这里按 CAS 失败重试，直到抢在下一次续租之前把过期记录写进去。
	// 版本必须同时前进：看门狗拿旧版本续租会被 CAS 拒绝，随后读到过期记录并判定持有权丢失。
	time.Sleep(time.Second)
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for {
		current, err := s.Get(ctx, lockId)
		if err != nil {
			t.Fatalf("read lease: %v", err)
		}
		info, err := storage.LockInformationFromJsonString(current)
		if err != nil {
			t.Fatalf("decode lease: %v", err)
		}
		expired := *info
		expired.Version = info.Version + 1
		expired.LeaseExpireTime = time.Now().Add(-time.Second)
		lastErr = s.UpdateWithVersion(ctx, lockId, info.Version, expired.Version, &expired)
		if lastErr == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expire lease: %v", lastErr)
		}
	}

	select {
	case <-owned.Done():
		if !errors.Is(context.Cause(owned), storage_lock.ErrLockLost) {
			t.Fatalf("业务上下文应以 ErrLockLost 取消，实际 %v", context.Cause(owned))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("持有权丢失后业务上下文没有取消")
	}
}

const leaseExpiryWait = 3500 * time.Millisecond

// shortLeaseOptions 返回测试用的短租约配置。3 秒是框架允许的最短租期。
func shortLeaseOptions(lockId string) *storage_lock.StorageLockOptions {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = 3 * time.Second
	options.LeaseRefreshInterval = time.Second
	options.VersionMissRetryInterval = 50 * time.Millisecond
	return options
}
