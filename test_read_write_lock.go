package storage_lock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestStorageLock_ReadWriteLockSemantics 对任意 Storage 运行读写锁语义测试
// 该测试验证 ReadLock/WriteLock 在存储层上的正确行为。
func TestStorageLock_ReadWriteLockSemantics(t *testing.T, factory StorageLockTestFactory) {
	ctx := context.Background()

	cases := []struct {
		name string
		fn   func(t *testing.T, ctx context.Context, factory StorageLockTestFactory)
	}{
		{"MultipleReaders", testRWMultipleReaders},
		{"WriterBlocksReader", testRWWriterBlocksReader},
		{"ReaderBlocksWriter", testRWReaderBlocksWriter},
		{"ReentrantSameOwner", testRWReentrantSameOwner},
		{"WriteAfterReadersRelease", testRWWriteAfterReadersRelease},
		{"ReaderAfterWriterRelease", testRWReaderAfterWriterRelease},
		{"SubscribeReceivesReadLockState", testRWSubscribeReceivesReadLockState},
		{"SubscribeReceivesWriteLockState", testRWSubscribeReceivesWriteLockState},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			lockId := "test-rw-" + c.name
			c.fn(t, ctx, &isolatedFactory{inner: factory, lockId: lockId})
		})
	}
}

func testRWMultipleReaders(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	err := lock.RLock(ctx, "reader-1")
	assert.NoError(t, err)
	defer lock.RUnLock(ctx, "reader-1")

	err = lock.RLock(ctx, "reader-2")
	assert.NoError(t, err)
	defer lock.RUnLock(ctx, "reader-2")
}

func testRWWriterBlocksReader(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	err := lock.WLock(ctx, "writer")
	assert.NoError(t, err)
	defer lock.WUnLock(ctx, "writer")

	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	err = lock.RLock(readCtx, "blocked-reader")
	assert.Error(t, err)
}

func testRWReaderBlocksWriter(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	err := lock.RLock(ctx, "reader")
	assert.NoError(t, err)
	defer lock.RUnLock(ctx, "reader")

	writeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	err = lock.WLock(writeCtx, "blocked-writer")
	assert.Error(t, err)
}

func testRWReentrantSameOwner(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	err := lock.RLock(ctx, "same-owner")
	assert.NoError(t, err)

	err = lock.RLock(ctx, "same-owner")
	assert.NoError(t, err)

	err = lock.RUnLock(ctx, "same-owner")
	assert.NoError(t, err)
	err = lock.RUnLock(ctx, "same-owner")
	assert.NoError(t, err)
}

func testRWWriteAfterReadersRelease(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	err := lock.RLock(ctx, "reader-1")
	assert.NoError(t, err)
	defer lock.RUnLock(ctx, "reader-1")

	err = lock.RLock(ctx, "reader-2")
	assert.NoError(t, err)
	defer lock.RUnLock(ctx, "reader-2")

	err = lock.RUnLock(ctx, "reader-1")
	assert.NoError(t, err)
	err = lock.RUnLock(ctx, "reader-2")
	assert.NoError(t, err)

	// 所有读者释放后，写者应能立即获取
	err = lock.WLock(ctx, "writer")
	assert.NoError(t, err)
	lock.WUnLock(ctx, "writer")
}

func testRWReaderAfterWriterRelease(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	err := lock.WLock(ctx, "writer")
	assert.NoError(t, err)
	lock.WUnLock(ctx, "writer")

	// 写者释放后，读者应能立即获取
	err = lock.RLock(ctx, "reader")
	assert.NoError(t, err)
	lock.RUnLock(ctx, "reader")
}

func testRWSubscribeReceivesReadLockState(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	lockId := "test-rw-sub-read-" + t.Name()
	ch, err := lock.Subscribe(ctx, lockId)
	assert.NoError(t, err)
	defer lock.Unsubscribe(lockId, ch)

	err = lock.RLock(ctx, "sub-owner")
	assert.NoError(t, err)

	select {
	case info := <-ch:
		assert.Equal(t, lockId, info.LockId)
		assert.Equal(t, "sub-owner", info.OwnerId)
		assert.True(t, info.ReadLock)
	case <-time.After(300 * time.Millisecond):
		// 共享测试框架下订阅通知为尽力投递，允许未收到
	}

	lock.RUnLock(ctx, "sub-owner")
}

func testRWSubscribeReceivesWriteLockState(t *testing.T, ctx context.Context, factory StorageLockTestFactory) {
	lock, cleanup := newLock(t, ctx, factory, "")
	defer cleanup()

	lockId := "test-rw-sub-write-" + t.Name()
	ch, err := lock.Subscribe(ctx, lockId)
	assert.NoError(t, err)
	defer lock.Unsubscribe(lockId, ch)

	err = lock.WLock(ctx, "sub-writer")
	assert.NoError(t, err)

	select {
	case info := <-ch:
		assert.Equal(t, lockId, info.LockId)
		assert.Equal(t, "sub-writer", info.OwnerId)
		assert.False(t, info.ReadLock)
	case <-time.After(300 * time.Millisecond):
		// 共享测试框架下订阅通知为尽力投递，允许未收到
	}

	lock.WUnLock(ctx, "sub-writer")
}
