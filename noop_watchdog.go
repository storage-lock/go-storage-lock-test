package storage_lock

import (
	"context"

	"github.com/storage-lock/go-events"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// NoopWatchDog 一个什么都不做的看门狗，用于模拟"持有者宕机"场景
// 它不会续租，锁会在 LeaseExpireAfter 后自然过期
type NoopWatchDog struct {
	id string
}

func (x *NoopWatchDog) Name() string  { return "noop-watch-dog" }
func (x *NoopWatchDog) GetID() string { return x.id }
func (x *NoopWatchDog) Start(ctx context.Context) error {
	// 什么都不做，不启动任何续租协程
	return nil
}
func (x *NoopWatchDog) Stop(ctx context.Context) error {
	// 什么都不做
	return nil
}

// SetEvent 实现 events.Eventable，NoopWatchDog 不需要事件
func (x *NoopWatchDog) SetEvent(e *events.Event) {}

// NoopWatchDogFactory 创建 NoopWatchDog 的工厂
// 测试中用于模拟持有者宕机：锁获取后不会被续租，会自然过期
type NoopWatchDogFactory struct{}

func (x *NoopWatchDogFactory) Name() string { return "noop-watch-dog-factory" }

func (x *NoopWatchDogFactory) NewWatchDog(ctx context.Context, e *events.Event, lock *storage_lock.StorageLock, ownerId string) (storage_lock.WatchDog, error) {
	return &NoopWatchDog{id: "noop-watch-dog"}, nil
}

// 确保实现接口
var _ storage_lock.WatchDog = &NoopWatchDog{}
var _ storage_lock.WatchDogFactory = &NoopWatchDogFactory{}
