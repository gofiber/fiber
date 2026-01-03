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
	headers         map[string][]byte
	body            []byte
	ctype           []byte
	cencoding       []byte
	cacheControl    []byte
	expires         []byte
	etag            []byte
	date            uint64
	status          int
	age             uint64
	exp             uint64
	ttl             uint64
	forceRevalidate bool
	revalidate      bool
	shareable       bool
	private         bool
	// used for finding the item in an indexed heap
	heapidx int
}

//msgp:ignore manager
type manager struct {
	pool       sync.Pool
	memory     *memory.Storage
	storage    fiber.Storage
	redactKeys bool
}

const redactedKey = "[redacted]"

var errCacheMiss = errors.New("cache: miss")

func newManager(storage fiber.Storage, redactKeys bool) *manager {
	// Create new storage handler
	manager := &manager{
		pool: sync.Pool{
			New: func() any {
				return new(item)
			},
		},
		redactKeys: redactKeys,
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
	e.cacheControl = nil
	e.expires = nil
	e.etag = nil
	e.ctype = nil
	e.cencoding = nil
	e.date = 0
	e.status = 0
	e.age = 0
	e.exp = 0
	e.ttl = 0
	e.forceRevalidate = false
	e.revalidate = false
	e.headers = nil
	e.shareable = false
	e.private = false
	e.heapidx = 0
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(ctx context.Context, key string) (*item, error) {
	if m.storage != nil {
		raw, err := m.storage.GetWithContext(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("cache: failed to get key %q from storage: %w", m.logKey(key), err)
		}
		if raw == nil {
			return nil, errCacheMiss
		}

		it := m.acquire()
		if _, err := it.UnmarshalMsg(raw); err != nil {
			m.release(it)
			return nil, fmt.Errorf("cache: failed to unmarshal key %q: %w", m.logKey(key), err)
		}

		return it, nil
	}

	if value := m.memory.Get(key); value != nil {
		it, ok := value.(*item)
		if !ok {
			return nil, fmt.Errorf("cache: unexpected entry type %T for key %q", value, m.logKey(key))
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
			return nil, fmt.Errorf("cache: failed to get raw key %q from storage: %w", m.logKey(key), err)
		}
		if raw == nil {
			return nil, errCacheMiss
		}
		return raw, nil
	}

	if value := m.memory.Get(key); value != nil {
		raw, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("cache: unexpected raw entry type %T for key %q", value, m.logKey(key))
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
			return fmt.Errorf("cache: failed to marshal key %q: %w", m.logKey(key), err)
		}
		if err := m.storage.SetWithContext(ctx, key, raw, exp); err != nil {
			m.release(it)
			return fmt.Errorf("cache: failed to store key %q: %w", m.logKey(key), err)
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
			return fmt.Errorf("cache: failed to store raw key %q: %w", m.logKey(key), err)
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
			return fmt.Errorf("cache: failed to delete key %q: %w", m.logKey(key), err)
		}
		return nil
	}

	m.memory.Delete(key)
	return nil
}

func (m *manager) logKey(key string) string {
	if m.redactKeys {
		return redactedKey
	}
	return key
}
