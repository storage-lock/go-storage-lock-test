package storage_lock

import (
	"context"
	"errors"
	"testing"

	memory_storage "github.com/storage-lock/go-memory-storage"
	"github.com/storage-lock/go-storage"
)

type capabilityStorage struct {
	storage.Storage
	capabilities []storage.StorageCapability
}

func (s capabilityStorage) Capabilities() []storage.StorageCapability { return s.capabilities }

func TestRegisteredFactoryRejectsStorageWithoutCAS(t *testing.T) {
	backend := registeredBackend{
		name: "placeholder",
		builder: func(context.Context) (storage.Storage, error) {
			return capabilityStorage{
				Storage:      memory_storage.NewMemoryStorage(),
				capabilities: []storage.StorageCapability{storage.CapabilityReliableTime},
			}, nil
		},
	}
	_, err := newRegisteredFactory(context.Background(), backend)
	if err == nil || errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("storage without CAS must fail closed, got %v", err)
	}
}

func TestRegisteredFactorySkipsUnavailable(t *testing.T) {
	_, err := newRegisteredFactory(context.Background(), registeredBackend{
		name: "offline",
		builder: func(context.Context) (storage.Storage, error) {
			return nil, ErrBackendUnavailable
		},
	})
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("unavailable backend should skip, got %v", err)
	}
}

func TestMemoryBackendRegistrationBuildsLock(t *testing.T) {
	factory, err := newRegisteredFactory(context.Background(), registeredBackend{
		name: "memory",
		builder: func(context.Context) (storage.Storage, error) {
			return memory_storage.NewMemoryStorage(), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock := factory.NewStorageLock(context.Background(), t, "registry-memory")
	if err := lock.TryLock(context.Background(), "owner"); err != nil {
		t.Fatal(err)
	}
}
