package backends

import (
	"context"
	"os"
	"path/filepath"

	filesystem_storage "github.com/storage-lock/go-filesystem-storage"
	http_storage "github.com/storage-lock/go-http-storage"
	kv_storage "github.com/storage-lock/go-kv-storage"
	leveldb_storage "github.com/storage-lock/go-leveldb-storage"
	memory_storage "github.com/storage-lock/go-memory-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
	sqlite3_storage "github.com/storage-lock/go-sqlite3-storage"
)

func init() {
	framework.RegisterBackend("memory", func(context.Context) (storage.Storage, error) {
		return memory_storage.NewMemoryStorage(), nil
	})
	framework.RegisterBackend("filesystem", func(context.Context) (storage.Storage, error) {
		dir, err := os.MkdirTemp("", "storage-lock-filesystem-*")
		if err != nil {
			return nil, err
		}
		store := filesystem_storage.NewFileSystemStorage(dir)
		if err := store.Init(context.Background()); err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		return store, nil
	})
	framework.RegisterBackend("http", func(ctx context.Context) (storage.Storage, error) {
		return http_storage.NewHTTPStorage(ctx)
	})
	framework.RegisterBackend("kv", func(ctx context.Context) (storage.Storage, error) {
		return kv_storage.NewKVStorage(ctx)
	})
	framework.RegisterBackend("leveldb", func(ctx context.Context) (storage.Storage, error) {
		dir, err := os.MkdirTemp("", "storage-lock-leveldb-*")
		if err != nil {
			return nil, err
		}
		manager := leveldb_storage.NewLevelDBConnectionManagerFromPath(dir)
		return leveldb_storage.NewLevelDBStorage(ctx, leveldb_storage.NewLevelDBStorageOptions().SetConnectionManager(manager))
	})
	framework.RegisterBackend("sqlite3", func(ctx context.Context) (storage.Storage, error) {
		dir, err := os.MkdirTemp("", "storage-lock-sqlite3-*")
		if err != nil {
			return nil, err
		}
		manager, err := sqlite3_storage.NewSqlite3ConnectionManager(filepath.Join(dir, "semantics.db3"))
		if err != nil {
			return nil, err
		}
		return sqlite3_storage.NewSqlite3Storage(ctx, sqlite3_storage.NewSqlite3StorageOptions().SetConnectionManager(manager))
	})
}
