package storage_lock

import (
	"context"
	"os"
	"testing"

	filesystem_storage "github.com/storage-lock/go-filesystem-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// FileSystemStorageFactory 为公开 filesystem backend 提供语义测试实例。
type FileSystemStorageFactory struct {
	workspace string
	storage   *filesystem_storage.FileSystemStorage
}

func NewFileSystemStorageFactory(t *testing.T) *FileSystemStorageFactory {
	t.Helper()
	workspace := t.TempDir()
	store := filesystem_storage.NewFileSystemStorage(workspace)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("initialize filesystem storage: %v", err)
	}
	return &FileSystemStorageFactory{workspace: workspace, storage: store}
}

func (f *FileSystemStorageFactory) CreateStorage(context.Context, *testing.T) storage.Storage {
	return f.storage
}

func (f *FileSystemStorageFactory) NewStorageLock(_ context.Context, t *testing.T, lockID string) *storage_lock.StorageLock {
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, testLockOptions(lockID))
	if err != nil {
		t.Fatalf("create filesystem StorageLock: %v", err)
	}
	return lock
}

func (*FileSystemStorageFactory) CloseStorageLock(context.Context, *testing.T, *storage_lock.StorageLock) {
}

func (f *FileSystemStorageFactory) Cleanup() { _ = os.RemoveAll(f.workspace) }
