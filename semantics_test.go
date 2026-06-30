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

// 确保 context 被使用（某些 factory 可能需要）
var _ = context.Background
