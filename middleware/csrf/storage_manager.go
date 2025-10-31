package csrf

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/memory"
)

// msgp -file="storage_manager.go" -o="storage_manager_msgp.go" -tests=true -unexported
//
//go:generate msgp -o=storage_manager_msgp.go -tests=true -unexported
type item struct{}

const redactedKey = "[redacted]"

//msgp:ignore manager
//msgp:ignore storageManager
type storageManager struct {
	pool       sync.Pool       `msg:"-"` //nolint:revive // Ignore unexported type
	memory     *memory.Storage `msg:"-"` //nolint:revive // Ignore unexported type
	storage    fiber.Storage   `msg:"-"` //nolint:revive // Ignore unexported type
	redactKeys bool
}

func newStorageManager(storage fiber.Storage, redactKeys bool) *storageManager {
	// Create new storage handler
	storageManager := &storageManager{
		pool: sync.Pool{
			New: func() any {
				return new(item)
			},
		},
		redactKeys: redactKeys,
	}
	if storage != nil {
		// Use provided storage if provided
		storageManager.storage = storage
	} else {
		// Fallback to memory storage
		storageManager.memory = memory.New()
	}
	return storageManager
}

// get raw data from storage or memory
func (m *storageManager) getRaw(ctx context.Context, key string) ([]byte, error) {
	if m.storage != nil {
		raw, err := m.storage.GetWithContext(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("csrf: failed to get value from storage: %w", err)
		}
		return raw, nil
	}

	if value := m.memory.Get(key); value != nil {
		raw, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("csrf: unexpected value type %T in storage", value)
		}
		return raw, nil
	}

	return nil, nil
}

// set data to storage or memory
func (m *storageManager) setRaw(ctx context.Context, key string, raw []byte, exp time.Duration) error {
	if m.storage != nil {
		if err := m.storage.SetWithContext(ctx, key, raw, exp); err != nil {
			return fmt.Errorf("csrf: failed to store key %q: %w", m.logKey(key), err)
		}
		return nil
	}

	m.memory.Set(key, raw, exp)
	return nil
}

// delete data from storage or memory
func (m *storageManager) delRaw(ctx context.Context, key string) error {
	if m.storage != nil {
		if err := m.storage.DeleteWithContext(ctx, key); err != nil {
			return fmt.Errorf("csrf: failed to delete key %q: %w", m.logKey(key), err)
		}
		return nil
	}

	m.memory.Delete(key)
	return nil
}

func (m *storageManager) logKey(key string) string {
	if m.redactKeys {
		return redactedKey
	}
	return key
}
