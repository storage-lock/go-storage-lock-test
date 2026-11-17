//go:build storage_lock_cassandra

package backends

import (
	"context"
	"strings"

	cassandra_storage "github.com/storage-lock/go-cassandra-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("cassandra", func(ctx context.Context) (storage.Storage, error) {
		hosts, err := framework.BackendFromEnv("STORAGE_LOCK_CASSANDRA_HOSTS")
		if err != nil {
			return nil, err
		}
		keyspace := "storage_lock"
		if value, err := framework.BackendFromEnv("STORAGE_LOCK_CASSANDRA_KEYSPACE"); err == nil {
			keyspace = value
		}
		options := cassandra_storage.NewCassandraStorageOptions().
			SetConnectionManager(cassandra_storage.NewCassandraConnectionManagerFromConfig(strings.Split(hosts, ","), keyspace)).
			SetKeyspace(keyspace)
		return cassandra_storage.NewCassandraStorage(ctx, options)
	})
}
