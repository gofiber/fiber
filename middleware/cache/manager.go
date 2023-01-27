package cache

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/memory"
)

// go:generate msgp
// msgp -file="manager.go" -o="manager_msgp.go" -tests=false -unexported
// don't forget to replace the msgp import path to:
// "github.com/gofiber/fiber/v2/internal/msgp"
type item struct {
	body      []byte
	ctype     []byte
	cencoding []byte
	status    int
	exp       uint64
	headers   map[string][]byte
	// used for finding the item in an indexed heap
	heapidx int
}

//msgp:ignore manager
type manager struct {
	pool    sync.Pool
	memory  *memory.Storage
	storage fiber.Storage
}

func newManager(storage fiber.Storage) *manager {
	// Create new storage handler
	manager := &manager{
		pool: sync.Pool{
			New: func() interface{} {
				return new(item)
			},
		},
	}
	if storage != nil {
		// Use provided storage if provided
		manager.storage = storage
	} else {
		// Fallback to memory storage
		manager.memory = memory.New()
	}
	return manager
}

// acquire returns an *entry from the sync.Pool
func (m *manager) acquire() *item {
	return m.pool.Get().(*item)
}

// release and reset *entry to sync.Pool
func (m *manager) release(e *item) {
	// don't release item if we using memory storage
	if m.storage != nil {
		return
	}
	e.body = nil
	e.ctype = nil
	e.status = 0
	e.exp = 0
	e.headers = nil
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(ctx context.Context, key string) (it *item) {
	if m.storage != nil {
		it = m.acquire()
		if raw, _ := m.storage.Get(ctx, key); raw != nil {
			if _, err := it.UnmarshalMsg(raw); err != nil {
				return
			}
		}
		return
	}
	if it, _ = m.memory.Get(ctx, key).(*item); it == nil {
		it = m.acquire()
	}
	return
}

// get raw data from storage or memory
func (m *manager) getRaw(ctx context.Context, key string) (raw []byte) {
	if m.storage != nil {
		raw, _ = m.storage.Get(ctx, key)
	} else {
		raw, _ = m.memory.Get(ctx, key).([]byte)
	}
	return
}

// set data to storage or memory
func (m *manager) set(ctx context.Context, key string, it *item, exp time.Duration) {
	if m.storage != nil {
		if raw, err := it.MarshalMsg(nil); err == nil {
			_ = m.storage.Set(ctx, key, raw, exp)
		}
	} else {
		m.memory.Set(ctx, key, it, exp)
	}
}

// set data to storage or memory
func (m *manager) setRaw(ctx context.Context, key string, raw []byte, exp time.Duration) {
	if m.storage != nil {
		_ = m.storage.Set(ctx, key, raw, exp)
	} else {
		m.memory.Set(ctx, key, raw, exp)
	}
}

// delete data from storage or memory
func (m *manager) delete(ctx context.Context, key string) {
	if m.storage != nil {
		_ = m.storage.Delete(ctx, key)
	} else {
		m.memory.Delete(ctx, key)
	}
}
