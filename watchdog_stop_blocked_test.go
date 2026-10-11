package storage_lock_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/golang-infrastructure/go-iterator"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// slowStorage 装饰一个真实 Storage，在指定的写/读方法上人为阻塞到 ctx 取消，
// 用于模拟"存储慢/挂"场景，钉死看门狗 Stop 必须能打断卡在存储 RPC 里的 goroutine（问题3）。
type slowStorage struct {
	inner        storage.Storage
	blockOnCalls map[string]chan struct{} // 方法名 -> 阻塞信号；方法进入后阻塞到该 chan close 或 ctx 取消
	mu           sync.Mutex
	calls        map[string]int
}

func newSlowStorage(inner storage.Storage) *slowStorage {
	return &slowStorage{
		inner:        inner,
		blockOnCalls: make(map[string]chan struct{}),
		calls:        make(map[string]int),
	}
}

// armBlock 让指定方法下次进入时阻塞，返回一个 chan，测试 close 它即放行。
func (s *slowStorage) armBlock(method string) chan struct{} {
	ch := make(chan struct{})
	s.mu.Lock()
	s.blockOnCalls[method] = ch
	s.mu.Unlock()
	return ch
}

func (s *slowStorage) waitOrCancel(ctx context.Context, method string) error {
	s.mu.Lock()
	s.calls[method]++
	ch, ok := s.blockOnCalls[method]
	s.mu.Unlock()
	if ok {
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *slowStorage) GetName() string { return s.inner.GetName() }
func (s *slowStorage) Capabilities() []storage.StorageCapability {
	return s.inner.Capabilities()
}
func (s *slowStorage) Init(ctx context.Context) error { return s.inner.Init(ctx) }

func (s *slowStorage) Get(ctx context.Context, lockId string) (string, error) {
	if err := s.waitOrCancel(ctx, "Get"); err != nil {
		return "", err
	}
	return s.inner.Get(ctx, lockId)
}

func (s *slowStorage) UpdateWithVersion(ctx context.Context, lockId string, exceptedVersion, newVersion storage.Version, lockInformation *storage.LockInformation) error {
	if err := s.waitOrCancel(ctx, "UpdateWithVersion"); err != nil {
		return err
	}
	return s.inner.UpdateWithVersion(ctx, lockId, exceptedVersion, newVersion, lockInformation)
}

func (s *slowStorage) CreateWithVersion(ctx context.Context, lockId string, version storage.Version, lockInformation *storage.LockInformation) error {
	if err := s.waitOrCancel(ctx, "CreateWithVersion"); err != nil {
		return err
	}
	return s.inner.CreateWithVersion(ctx, lockId, version, lockInformation)
}

func (s *slowStorage) DeleteWithVersion(ctx context.Context, lockId string, exceptedVersion storage.Version, lockInformation *storage.LockInformation) error {
	if err := s.waitOrCancel(ctx, "DeleteWithVersion"); err != nil {
		return err
	}
	return s.inner.DeleteWithVersion(ctx, lockId, exceptedVersion, lockInformation)
}

func (s *slowStorage) GetTime(ctx context.Context) (time.Time, error) {
	if err := s.waitOrCancel(ctx, "GetTime"); err != nil {
		return time.Time{}, err
	}
	return s.inner.GetTime(ctx)
}

func (s *slowStorage) Close(ctx context.Context) error { return s.inner.Close(ctx) }

func (s *slowStorage) List(ctx context.Context) (iterator.Iterator[*storage.LockInformation], error) {
	return s.inner.List(ctx)
}

// TestWatchDogStopInterruptsBlockedStorageRPC 钉死问题3（liveness）：
// 看门狗 goroutine 卡在慢存储的 Get/Update 调用里时，Stop 必须能通过取消 runCtx
// 立即打断它，使 goroutine 迅速退出，而不是干等到子 ctx 自然超时（LeaseRefreshInterval）
// 或 Stop 自身 ctx 超时后才返回、遗留存活 goroutine。
//
// 修复前：refreshLeaseExpiredTime 的子 ctx 用 context.WithTimeout(context.Background(), ...)，
// 不监听 stop/runCtx，Stop 无法打断卡在存储 RPC 里的 goroutine。
// 修复后：子 ctx 从 runCtx 派生，Stop 调 runCancel 取消它，存储调用立刻返回。
func TestWatchDogStopInterruptsBlockedStorageRPC(t *testing.T) {
	factory := NewMemoryStorageFactory()
	ctx := context.Background()
	storageImpl := factory.CreateStorage(ctx, t)

	// 短租约 + 短刷新间隔，让看门狗很快进入第一次 refresh
	options := storage_lock.NewStorageLockOptionsWithLockId("test-watchdog-stop-blocked")
	options.LeaseExpireAfter = time.Second * 6
	options.LeaseRefreshInterval = time.Second * 2
	options.VersionMissRetryInterval = time.Millisecond * 50
	clock := newBlockingClock()
	options.SetTimeProvider(clock)
	lock, err := storage_lock.NewStorageLockWithOptions(storageImpl, options)
	if err != nil {
		t.Fatalf("NewStorageLockWithOptions: %v", err)
	}

	// 正常加锁，看门狗随之启动
	owner := "owner-stop-test"
	mustLock(t, lock, owner)

	// 看门狗已启动。让它进入第一次 refresh：refreshLeaseExpiredTime 读到记录后，
	// 用 GetTime 计算新的过期时间。阻塞点放在注入的时间源上：续租必经它，而 UnLock 写墓碑
	// 用的是调用方自己的 ctx，不受看门狗 runCtx 取消的影响，所以能干净地只卡住看门狗。
	// 不能阻塞 UpdateWithVersion：释放锁统一写墓碑，墓碑本身就是一次 UpdateWithVersion，
	// 阻塞它会把 UnLock 和看门狗一起卡住。也不能阻塞 Get：tryUnlock 读记录时同样会调 Get。
	// 也不能直接拦 storage.GetTime：内存存储的实现忽略 ctx，取消信号根本传不到那里。
	// UnLock 校验租约时也会取时，用的是调用方的 ctx。阻塞必须放过这个 ctx，否则 UnLock
	// 会和看门狗堵在同一个时间源上，测到的是调用方把自己卡住。
	// 这里的 ctx 只用来标记"调用方"，不带超时：超时要等看门狗确认卡住后再计，
	// 否则等待看门狗进入的那一秒会把预算耗光。
	blockCh := clock.arm()
	// 兜底放行阻塞信号，确保任何失败路径下卡住的 goroutine 也能继续（避免泄漏到后续测试）
	defer func() {
		select {
		case <-blockCh:
		default:
			close(blockCh)
		}
	}()

	// 等待看门狗确实进入并被阻塞在时间源上
	deadline := time.Now().Add(time.Second * 5)
	for time.Now().Before(deadline) {
		if clock.callCount() > 0 {
			break
		}
		time.Sleep(time.Millisecond * 20)
	}
	gotCalls := clock.callCount()
	if gotCalls == 0 {
		t.Fatalf("看门狗未在预期时间内进入 refresh 的 GetTime 调用，无法验证 Stop 打断")
	}

	// 此时 goroutine 卡在 GetTime 的 waitOrCancel 上。Stop 必须迅速返回且 goroutine 退出。
	// 给 UnLock 一个远小于 LeaseRefreshInterval(2s) 的超时——若修复有效，Stop 里 runCancel 会立即取消
	// runCtx（进而取消 refresh 子 ctx），GetTime 立即返回 ctx.Err()，goroutine 退出，done close。
	// 修复前子 ctx 不监听 runCtx，Stop 只能干等到自身 stopCtx 超时，UnLock 报 context deadline exceeded。
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer stopCancel()
	stopBegin := time.Now()
	err = lock.UnLock(stopCtx, owner)
	elapsed := time.Since(stopBegin)

	// 硬断言1：UnLock（含 Stop）必须远小于子 ctx 自然超时(2s)与 stopCtx(500ms)。
	// 修复前会卡满 500ms 返回 deadline exceeded。
	if err != nil {
		t.Fatalf("UnLock 应在 Stop 打断阻塞后迅速成功，实际 err=%v 耗时=%v（问题3未修复：Stop 未打断卡在存储 RPC 的 goroutine）", err, elapsed)
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("UnLock/Stop 耗时 %v，远超预期（应被 runCancel 立即打断），问题3未修复", elapsed)
	}

	// 硬断言2：放开阻塞信号后，看门狗 goroutine 不应再续租（Stop 已让它退出）。
	// 用 GetTime 调用数是否增长判定 goroutine 存活。
	close(blockCh)
	gotAfter := clock.callCount()
	time.Sleep(time.Millisecond * 300)
	if got := clock.callCount(); got > gotAfter {
		t.Fatalf("Stop 后看门狗 goroutine 仍在续租（GetTime %d -> %d），未真正退出，问题3未修复", gotAfter, got)
	}
}

func mustLock(t *testing.T, lock *storage_lock.StorageLock, owner string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	if err := lock.Lock(ctx, owner); err != nil {
		t.Fatalf("Lock 失败: %v", err)
	}
}

// blockingClock 是一个可按需卡住的时间源。arm 之后，只有看门狗发起的 GetTime 会阻塞到
// 放行或 ctx 取消；调用方自己的取时照常返回。
// 区分两者靠的是 ctx 里有没有标记：UnLock 写墓碑、校验租约时用的是调用方传入的 ctx，
// 看门狗续租用的是从 runCtx 派生的子 ctx。如果一视同仁地阻塞，UnLock 会和看门狗堵在
// 同一个时间源上，测到的就是调用方把自己卡住，而不是 Stop 打不断看门狗。
type blockingClock struct {
	mu    sync.Mutex
	block chan struct{}
	calls int
}

func newBlockingClock() *blockingClock { return &blockingClock{} }

func (c *blockingClock) arm() chan struct{} {
	ch := make(chan struct{})
	c.mu.Lock()
	c.block = ch
	c.calls = 0
	c.mu.Unlock()
	return ch
}

func (c *blockingClock) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *blockingClock) GetTime(ctx context.Context) (time.Time, error) {
	c.mu.Lock()
	ch := c.block
	// 只阻塞看门狗的取时。调用方的 UnLock 也会取时，放过它，否则两边堵在同一个时间源上。
	blocked := ch != nil && storage_lock.IsWatchDogContext(ctx)
	if blocked {
		c.calls++
	}
	c.mu.Unlock()
	if blocked {
		select {
		case <-ch:
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		}
	}
	return time.Now(), nil
}
