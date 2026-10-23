package integration

import (
	"context"
	"os"
	"testing"
	"time"

	mongodb_storage "github.com/storage-lock/go-mongodb-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBStorageFactory 为 MongoStorage 实现 StorageLockTestFactory
type MongoDBStorageFactory struct {
	client         *mongo.Client
	databaseName   string
	collectionName string
	storage        *mongodb_storage.MongoStorage
}

// localClockTimeProvider 用本机时钟代替公共 NTP 作为时间源。
// MongoStorage 默认依赖公共 NTP 服务器获取时间，但测试环境未必有公网 UDP 出网权限，
// 语义测试关心的是锁互斥/CAS/RW-lock/Subscribe 是否正确，并不依赖真实的 NTP 精度，
// 所以这里注入一个不依赖外网的时间源，避免测试被无关的网络可达性问题拖慢或卡住。
type localClockTimeProvider struct{}

func (localClockTimeProvider) GetTime(ctx context.Context) (time.Time, error) {
	return time.Now(), nil
}

// NewMongoDBStorageFactory 创建一个基于 MongoStorage 的测试 factory
// 如果 STORAGE_LOCK_MONGO_URI 环境变量为空则返回 nil（调用方应 t.Skip）
func NewMongoDBStorageFactory(t *testing.T) *MongoDBStorageFactory {
	envName := "STORAGE_LOCK_MONGO_URI"
	uri := os.Getenv(envName)
	if uri == "" {
		return nil
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("failed to connect to MongoDB: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("failed to ping MongoDB: %v", err)
	}

	databaseName := storage.DefaultStorageDatabaseName
	collectionName := storage.DefaultStorageTableName + "_semantics"

	connectionManager := mongodb_storage.NewMongoConnectionManagerFromClient(client)
	mongoOptions := mongodb_storage.NewMongoStorageOptions().
		SetConnectionManager(connectionManager).
		SetDatabaseName(databaseName).
		SetCollectionName(collectionName).
		SetTimeProvider(localClockTimeProvider{})
	s, err := mongodb_storage.NewMongoStorage(ctx, mongoOptions)
	if err != nil {
		t.Fatalf("failed to create MongoStorage: %v", err)
	}

	return &MongoDBStorageFactory{
		client:         client,
		databaseName:   databaseName,
		collectionName: collectionName,
		storage:        s,
	}
}

func (f *MongoDBStorageFactory) CreateStorage(ctx context.Context, t *testing.T) storage.Storage {
	return f.storage
}

func (f *MongoDBStorageFactory) NewStorageLock(ctx context.Context, t *testing.T, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = time.Second * 3
	options.LeaseRefreshInterval = time.Second * 1
	options.VersionMissRetryInterval = time.Millisecond * 5
	lock, err := storage_lock.NewStorageLockWithOptions(f.storage, options)
	if err != nil {
		t.Fatalf("failed to create StorageLock with MongoStorage: %v", err)
	}
	return lock
}

func (f *MongoDBStorageFactory) CloseStorageLock(ctx context.Context, t *testing.T, lock *storage_lock.StorageLock) {
	// MongoDB 无需特殊清理
}

// Cleanup 清理 MongoDB 测试数据
func (f *MongoDBStorageFactory) Cleanup() {
	if f.client == nil {
		return
	}
	ctx := context.Background()
	_, _ = f.client.Database(f.databaseName).Collection(f.collectionName).DeleteMany(ctx, bson.M{})
	_ = f.client.Disconnect(ctx)
}
