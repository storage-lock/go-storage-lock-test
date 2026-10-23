package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	s3_storage "github.com/storage-lock/go-s3-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

// S3StorageFactory 为 S3Storage 实现 StorageLockTestFactory
type S3StorageFactory struct {
	client  *minio.Client
	storage *s3_storage.S3Storage
	bucket  string
}

// NewS3StorageFactory 创建一个基于 S3Storage 的测试 factory
// 如果 STORAGE_LOCK_S3_ENDPOINT 环境变量为空则返回 nil（调用方应 t.Skip）
func NewS3StorageFactory(t *testing.T) *S3StorageFactory {
	envName := "STORAGE_LOCK_S3_ENDPOINT"
	endpoint := os.Getenv(envName)
	if endpoint == "" {
		return nil
	}

	accessKey := os.Getenv("STORAGE_LOCK_S3_ACCESS_KEY")
	secretKey := os.Getenv("STORAGE_LOCK_S3_SECRET_KEY")
	bucket := os.Getenv("STORAGE_LOCK_S3_BUCKET")
	if bucket == "" {
		bucket = "storage-lock-test"
	}

	ctx := context.Background()
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to create S3 client: %v", err)
	}

	// 确保 bucket 存在
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatalf("failed to check bucket: %v", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			t.Fatalf("failed to create bucket: %v", err)
		}
	}

	connectionManager := s3_storage.NewS3ConnectionManagerFromClient(client)
	s, err := s3_storage.NewS3Storage(ctx, &s3_storage.S3StorageOptions{
		ConnectionManager: connectionManager,
		BucketName:        bucket,
		KeyPrefix:         "storage_lock_test",
		TimeProvider:      &s3_storage.LocalTimeProvider{},
	})
	if err != nil {
		t.Fatalf("failed to create S3Storage: %v", err)
	}

	return &S3StorageFactory{
		client:  client,
		storage: s,
		bucket:  bucket,
	}
}

func (f *S3StorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *S3StorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with S3Storage: %v", err)
	}
	return lock
}

func (f *S3StorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// S3 无需特殊清理
}

// Cleanup 清理 S3 测试数据
func (f *S3StorageFactory) Cleanup() {
	if f.client != nil {
		ctx := context.Background()
		prefix := "storage_lock_test/lock/"
		ch := f.client.ListObjects(ctx, f.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
		for obj := range ch {
			if obj.Err == nil {
				_ = f.client.RemoveObject(ctx, f.bucket, obj.Key, minio.RemoveObjectOptions{})
			}
		}
	}
}
