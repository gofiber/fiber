package csrf

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/memory"
)

// go:generate msgp
// msgp -file="storage_manager.go" -o="storage_manager_msgp.go" -tests=false -unexported
type item struct{}

//msgp:ignore manager
type storageManager struct {
	pool    sync.Pool
	memory  *memory.Storage
	storage fiber.Storage
}

func newStorageManager(storage fiber.Storage) *storageManager {
	// Create new storage handler
	storageManager := &storageManager{
		pool: sync.Pool{
			New: func() interface{} {
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
func (m *storageManager) getRaw(key string) []byte {
	var raw []byte
	if m.storage != nil {
		raw, _ = m.storage.Get(key) //nolint:errcheck // TODO: Do not ignore error
	} else {
		raw, _ = m.memory.Get(key).([]byte) //nolint:errcheck // TODO: Do not ignore error
	}
	return raw
}

// set data to storage or memory
func (m *storageManager) setRaw(key string, raw []byte, exp time.Duration) {
	if m.storage != nil {
		_ = m.storage.Set(key, raw, exp) //nolint:errcheck // TODO: Do not ignore error
	} else {
		// Storage layer handles defensive copying of keys and values
		m.memory.Set(key, raw, exp)
	}
}

// delete data from storage or memory
func (m *storageManager) delRaw(key string) {
	if m.storage != nil {
		_ = m.storage.Delete(key) //nolint:errcheck // TODO: Do not ignore error
	} else {
		m.memory.Delete(key)
	}
}
