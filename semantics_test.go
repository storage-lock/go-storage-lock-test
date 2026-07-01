package storage_lock_test

import (
	"context"
	"testing"
)

// TestMemoryStorageLockSemantics 对 MemoryStorage 运行完整的分布式锁语义测试
func TestMemoryStorageLockSemantics(t *testing.T) {
	factory := NewMemoryStorageFactory()
	TestStorageLockSemantics(t, factory)
}

// TestFileSystemStorageLockSemantics 对 FileSystemStorage 运行完整的分布式锁语义测试
func TestFileSystemStorageLockSemantics(t *testing.T) {
	factory := NewFileSystemStorageFactory(t)
	defer factory.Cleanup()
	TestStorageLockSemantics(t, factory)
}

// TestTombstoneReleasePath 验证"不支持原子条件删除"的存储介质（如对象存储）
// 能通过墓碑降级路径正确释放与抢占锁，互斥性不被破坏。
func TestTombstoneReleasePath(t *testing.T) {
	testTombstoneReleasePath(t)
}

// 确保 context 被使用（某些 factory 可能需要）
var _ = context.Background
