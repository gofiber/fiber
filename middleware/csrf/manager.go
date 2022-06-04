package csrf

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
)

// go:generate msgp
// msgp -file="manager.go" -o="manager_msgp.go" -tests=false -unexported
type item struct{}

//msgp:ignore manager
type manager struct {
	pool    sync.Pool
	storage fiber.Storage
}

func newManager(storage fiber.Storage) *manager {
	// Create new storage handler
	manager := &manager{
		pool: sync.Pool{
			New: func() any {
				return new(item)
			},
		},
	}

	if storage != nil {
		// Use provided storage if provided
		manager.storage = storage
	} else {
		// Fallback to memory storage
		manager.storage = memory.New(1)
	}

	return manager
}

// acquire returns an *entry from the sync.Pool
func (m *manager) acquire() *item {
	return m.pool.Get().(*item)
}

// release and reset *entry to sync.Pool
func (m *manager) release(e *item) {
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(key string) (it *item) {
	it = m.acquire()
	if raw, _ := m.storage.Get(key); raw != nil {
		if _, err := it.UnmarshalMsg(raw); err != nil {
			return
		}
	}

	return
}

// get raw data from storage or memory
func (m *manager) getRaw(key string) (raw []byte) {
	raw, _ = m.storage.Get(key)

	return
}

// set data to storage or memory
func (m *manager) set(key string, it *item, exp time.Duration) {
	if raw, err := it.MarshalMsg(nil); err == nil {
		_ = m.storage.Set(key, raw, exp)
	}
}

// set data to storage or memory
func (m *manager) setRaw(key string, raw []byte, exp time.Duration) {
	_ = m.storage.Set(key, raw, exp)
}

// delete data from storage or memory
func (m *manager) delete(key string) {
	_ = m.storage.Delete(key)
}
