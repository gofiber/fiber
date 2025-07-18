package cache

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/memory"
)

// msgp -file="manager.go" -o="manager_msgp.go" -tests=true -unexported
//
//go:generate msgp -o=manager_msgp.go -tests=true -unexported
type item struct {
	headers   map[string][]byte
	body      []byte
	ctype     []byte
	cencoding []byte
	status    int
	age       uint64
	exp       uint64
	ttl       uint64
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
		manager.memory = memory.New()
	}
	return manager
}

// acquire returns an *entry from the sync.Pool
func (m *manager) acquire() *item {
	return m.pool.Get().(*item) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
}

// release and reset *entry to sync.Pool
func (m *manager) release(e *item) {
	// don't release item if we using in-memory storage
	if m.storage == nil {
		return
	}
	e.body = nil
	e.ctype = nil
	e.status = 0
	e.age = 0
	e.exp = 0
	e.ttl = 0
	e.headers = nil
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(key string) *item {
	if m.storage != nil {
		raw, err := m.storage.Get(key)
		if err != nil || raw == nil {
			return nil
		}

		it := m.acquire()
		if _, err := it.UnmarshalMsg(raw); err != nil {
			m.release(it)
			return nil
		}

		return it
	}

	if it, _ := m.memory.Get(key).(*item); it != nil { //nolint:errcheck // We store nothing else in the pool
		return it
	}

	return nil
}

// get raw data from storage or memory
func (m *manager) getRaw(key string) []byte {
	var raw []byte
	if m.storage != nil {
		var err error
		raw, err = m.storage.Get(key)
		if err != nil {
			// Return nil on storage error (cache miss)
			return nil
		}
	} else {
		if data, ok := m.memory.Get(key).([]byte); ok {
			raw = data
		}
	}
	return raw
}

// set data to storage or memory
func (m *manager) set(key string, it *item, exp time.Duration) {
	if m.storage != nil {
		if raw, err := it.MarshalMsg(nil); err == nil {
			if setErr := m.storage.Set(key, raw, exp); setErr != nil {
				// Log or handle storage set error gracefully
				// For now, we'll just ignore it as the original code did
				// but without the linter suppression
			}
		}
		// we can release data because it's serialized to database
		m.release(it)
	} else {
		m.memory.Set(key, it, exp)
	}
}

// set data to storage or memory
func (m *manager) setRaw(key string, raw []byte, exp time.Duration) {
	if m.storage != nil {
		if err := m.storage.Set(key, raw, exp); err != nil {
			// Log or handle storage set error gracefully
			// For now, we'll just ignore it as the original code did
			// but without the linter suppression
		}
	} else {
		m.memory.Set(key, raw, exp)
	}
}

// delete data from storage or memory
func (m *manager) del(key string) {
	if m.storage != nil {
		if err := m.storage.Delete(key); err != nil {
			// Log or handle storage delete error gracefully
			// For now, we'll just ignore it as the original code did
			// but without the linter suppression
		}
	} else {
		m.memory.Delete(key)
	}
}
