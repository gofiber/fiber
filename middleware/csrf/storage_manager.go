package csrf

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/memory"
	"github.com/gofiber/utils/v2"
)

// msgp -file="storage_manager.go" -o="storage_manager_msgp.go" -tests=true -unexported
//
//go:generate msgp -o=storage_manager_msgp.go -tests=true -unexported
type item struct{}

//msgp:ignore manager
type storageManager struct {
	pool    sync.Pool       `msg:"-"` //nolint:revive // Ignore unexported type
	memory  *memory.Storage `msg:"-"` //nolint:revive // Ignore unexported type
	storage fiber.Storage   `msg:"-"` //nolint:revive // Ignore unexported type
}

func newStorageManager(storage fiber.Storage) *storageManager {
	// Create new storage handler
	storageManager := &storageManager{
		pool: sync.Pool{
			New: func() any {
				return new(item)
			},
		},
	}
	if storage != nil {
		// Use provided storage if provided
		storageManager.storage = storage
	} else {
		// Fallback too memory storage
		storageManager.memory = memory.New()
	}
	return storageManager
}

// get raw data from storage or memory
func (m *storageManager) getRaw(ctx context.Context, key string) []byte {
	var raw []byte
	if m.storage != nil {
		raw, _ = m.storage.GetWithContext(ctx, key) //nolint:errcheck // TODO: Do not ignore error
	} else {
		raw, _ = m.memory.Get(key).([]byte) //nolint:errcheck // TODO: Do not ignore error
	}
	return raw
}

// set data to storage or memory
func (m *storageManager) setRaw(ctx context.Context, key string, raw []byte, exp time.Duration) {
	if m.storage != nil {
		_ = m.storage.SetWithContext(ctx, key, raw, exp) //nolint:errcheck // TODO: Do not ignore error
	} else {
		// the key is crucial in crsf and sometimes a reference to another value which can be reused later(pool/unsafe values concept), so a copy is made here
		m.memory.Set(utils.CopyString(key), raw, exp)
	}
}

// delete data from storage or memory
func (m *storageManager) delRaw(ctx context.Context, key string) {
	if m.storage != nil {
		_ = m.storage.DeleteWithContext(ctx, key) //nolint:errcheck // TODO: Do not ignore error
	} else {
		m.memory.Delete(key)
	}
}
