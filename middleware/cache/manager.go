package cache

import (
	"context"
	"errors"
	"fmt"
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

const redactedKey = "[redacted]"

var errCacheMiss = errors.New("cache: miss")

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
func (m *manager) get(ctx context.Context, key string) (*item, error) {
	if m.storage != nil {
		raw, err := m.storage.GetWithContext(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("cache: failed to get key %s from storage: %w", redactedKey, err)
		}
		if raw == nil {
			return nil, errCacheMiss
		}

		it := m.acquire()
		if _, err := it.UnmarshalMsg(raw); err != nil {
			m.release(it)
			return nil, fmt.Errorf("cache: failed to unmarshal key %s: %w", redactedKey, err)
		}

		return it, nil
	}

	if value := m.memory.Get(key); value != nil {
		it, ok := value.(*item)
		if !ok {
			return nil, fmt.Errorf("cache: unexpected entry type %T for key %s", value, redactedKey)
		}
		return it, nil
	}

	return nil, errCacheMiss
}

// get raw data from storage or memory
func (m *manager) getRaw(ctx context.Context, key string) ([]byte, error) {
	if m.storage != nil {
		raw, err := m.storage.GetWithContext(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("cache: failed to get raw key %s from storage: %w", redactedKey, err)
		}
		if raw == nil {
			return nil, errCacheMiss
		}
		return raw, nil
	}

	if value := m.memory.Get(key); value != nil {
		raw, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("cache: unexpected raw entry type %T for key %s", value, redactedKey)
		}
		return raw, nil
	}

	return nil, errCacheMiss
}

// set data to storage or memory
func (m *manager) set(ctx context.Context, key string, it *item, exp time.Duration) error {
	if m.storage != nil {
		raw, err := it.MarshalMsg(nil)
		if err != nil {
			m.release(it)
			return fmt.Errorf("cache: failed to marshal key %s: %w", redactedKey, err)
		}
		if err := m.storage.SetWithContext(ctx, key, raw, exp); err != nil {
			m.release(it)
			return fmt.Errorf("cache: failed to store key %s: %w", redactedKey, err)
		}
		m.release(it)
		return nil
	}

	m.memory.Set(key, it, exp)
	return nil
}

// set data to storage or memory
func (m *manager) setRaw(ctx context.Context, key string, raw []byte, exp time.Duration) error {
	if m.storage != nil {
		if err := m.storage.SetWithContext(ctx, key, raw, exp); err != nil {
			return fmt.Errorf("cache: failed to store raw key %s: %w", redactedKey, err)
		}
		return nil
	}

	m.memory.Set(key, raw, exp)
	return nil
}

// delete data from storage or memory
func (m *manager) del(ctx context.Context, key string) error {
	if m.storage != nil {
		if err := m.storage.DeleteWithContext(ctx, key); err != nil {
			return fmt.Errorf("cache: failed to delete key %s: %w", redactedKey, err)
		}
		return nil
	}

	m.memory.Delete(key)
	return nil
}
