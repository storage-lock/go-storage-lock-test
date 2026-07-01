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

// TestFencingTokenMonotonic 验证栅栏令牌（fencing token）跨所有权变更严格单调递增，
// 这是消除"持有者停顿/宕机、租约过期被抢占后仍破坏被保护资源"这一租约锁经典安全边界的关键。
func TestFencingTokenMonotonic(t *testing.T) {
	testFencingTokenMonotonic(t)
}

// 确保 context 被使用（某些 factory 可能需要）
var _ = context.Background
