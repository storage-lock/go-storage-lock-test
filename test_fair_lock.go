package storage_lock

import (
	"context"
	"testing"

	storage_lock "github.com/storage-lock/go-storage-lock"
	"github.com/stretchr/testify/require"
)

// TestStorageLock_FairLockSemantics 直接针对公开的 FairLock 类型运行互斥与释放作用域用例。
//
// 之前套件只测了 StorageLock + SetFair，从没实例化过 FairLock 本身，
// 所以 v0.0.6/v0.0.7 里 FairLock 用固定 owner 导致的互斥失效一直没被发现。
// 这里每个 FairLock 都带自己的 ownerId，模拟两个独立客户端连同一个存储。
func TestStorageLock_FairLockSemantics(t *testing.T, factory StorageLockTestFactory) {
	ctx := context.Background()
	store := factory.CreateStorage(ctx, t)
	const lockID = "fair-lock-mutex"

	t.Run("TwoInstancesAreMutuallyExclusive", func(t *testing.T) {
		holder, err := storage_lock.NewFairLock(store, lockID)
		require.NoError(t, err)
		other, err := storage_lock.NewFairLock(store, lockID)
		require.NoError(t, err)

		require.NoError(t, holder.Lock(ctx, "owner-A"))
		// 第二个客户端必须是忙，不能因为共用 owner 变成重入。
		require.Error(t, other.TryLock(ctx, "owner-B"), "第二个客户端不能拿到已被占用的公平锁")

		require.NoError(t, holder.UnLock(ctx, "owner-A"))
		require.NoError(t, other.TryLock(ctx, "owner-B"))
		require.NoError(t, other.UnLock(ctx, "owner-B"))
	})

	t.Run("UnlockDoesNotReleaseAnotherOwner", func(t *testing.T) {
		a, err := storage_lock.NewFairLock(store, lockID)
		require.NoError(t, err)
		b, err := storage_lock.NewFairLock(store, lockID)
		require.NoError(t, err)

		require.NoError(t, a.Lock(ctx, "owner-A"))
		require.Error(t, b.UnLock(ctx, "owner-B"), "非持有者不能释放公平锁")
		require.Error(t, b.TryLock(ctx, "owner-B"), "A 未释放时 B 不能获得")

		require.NoError(t, a.UnLock(ctx, "owner-A"))
		require.NoError(t, b.Lock(ctx, "owner-B"))
		require.NoError(t, b.UnLock(ctx, "owner-B"))
	})
}
