package storage_lock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/storage-lock/go-filesystem-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// FileSystemStorageFactory 为 FileSystemStorage 实现 StorageLockTestFactory
// 每个测试用例使用独立的临时目录来隔离状态
type FileSystemStorageFactory struct {
	workspace string
	storage   *file_system_storage.FileSystemStorage
}

// NewFileSystemStorageFactory 创建一个基于 FileSystemStorage 的测试 factory
func NewFileSystemStorageFactory(t *testing.T) *FileSystemStorageFactory {
	workspace, err := os.MkdirTemp("", "storage-lock-fs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	fs := file_system_storage.NewFileSystemStorage(workspace)
	ctx := context.Background()
	if err := fs.Init(ctx); err != nil {
		t.Fatalf("failed to init FileSystemStorage: %v", err)
	}
	return &FileSystemStorageFactory{
		workspace: workspace,
		storage:   fs,
	}
}

func (f *FileSystemStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *FileSystemStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with FileSystemStorage: %v", err)
	}
	return lock
}

func (f *FileSystemStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// 什么都不做，统一在 Cleanup 中清理临时目录
}

// Cleanup 清理临时目录，应在测试完成后调用
func (f *FileSystemStorageFactory) Cleanup() {
	_ = os.RemoveAll(f.workspace)
}

// 确保引用 filepath（用于可能的路径断言）
var _ = filepath.Join
