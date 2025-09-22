package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/memory"
)

// msgp -file="manager.go" -o="manager_msgp.go" -tests=false -unexported
//
//go:generate msgp -o=manager_msgp.go -tests=false -unexported
type item struct {
	currHits int
	prevHits int
	exp      uint64
}

//msgp:ignore manager
type manager struct {
	pool    sync.Pool
	memory  *memory.Storage
	storage fiber.Storage
}

const redactedKey = "[redacted]"

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
		// Fallback too memory storage
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
	e.prevHits = 0
	e.currHits = 0
	e.exp = 0
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(ctx context.Context, key string) (*item, error) {
	if m.storage != nil {
		raw, err := m.storage.GetWithContext(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("limiter: failed to get key %s from storage: %w", redactedKey, err)
		}
		if raw != nil {
			it := m.acquire()
			if _, err := it.UnmarshalMsg(raw); err != nil {
				m.release(it)
				return nil, fmt.Errorf("limiter: failed to unmarshal key %s: %w", redactedKey, err)
			}
			return it, nil
		}
		return m.acquire(), nil
	}

	value := m.memory.Get(key)
	if value == nil {
		return m.acquire(), nil
	}

	it, ok := value.(*item)
	if !ok {
		return nil, fmt.Errorf("limiter: unexpected entry type %T for key %s", value, redactedKey)
	}

	return it, nil
}

// set data to storage or memory
func (m *manager) set(ctx context.Context, key string, it *item, exp time.Duration) error {
	if m.storage != nil {
		raw, err := it.MarshalMsg(nil)
		if err != nil {
			m.release(it)
			return fmt.Errorf("limiter: failed to marshal key %s: %w", redactedKey, err)
		}
		if err := m.storage.SetWithContext(ctx, key, raw, exp); err != nil {
			m.release(it)
			return fmt.Errorf("limiter: failed to store key %s: %w", redactedKey, err)
		}
		m.release(it)
		return nil
	}

	m.memory.Set(key, it, exp)
	return nil
}
