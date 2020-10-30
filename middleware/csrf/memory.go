package csrf

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

type memoryStorage struct {
	sync.RWMutex
	ctx    context.Context
	tokens map[string]int64
	gc     time.Duration
}

// newMemoryStorage - Creates new in-memory storage for CSRF tokens
func newMemoryStorage(ctx context.Context) fiber.Storage {
	storage := &memoryStorage{
		ctx:    ctx,
		tokens: make(map[string]int64),
		gc:     10 * time.Second,
	}
	go storage.collect(ctx)
	return storage
}

func (m *memoryStorage) Get(id string) ([]byte, error) {
	m.RLock()
	t, ok := m.tokens[id]
	m.RUnlock()
	// Check if token exist or expired
	if !ok || time.Now().Unix() >= t {
		return nil, errors.New("csrf: invalid key")
	}

	return utils.GetBytes(id), nil
}

// Set session value. `exp` will be zero for no expiration.
func (m *memoryStorage) Set(id string, value []byte, exp time.Duration) error {
	m.Lock()
	m.tokens[id] = time.Now().Unix() + int64(exp.Seconds())
	m.Unlock()

	return nil
}

// Delete session value
func (m *memoryStorage) Delete(id string) error {
	m.Lock()
	delete(m.tokens, id)
	m.Unlock()
	return nil
}

// Clear clears the storage
func (m *memoryStorage) Clear() error {
	m.Lock()
	for k := range m.tokens {
		delete(m.tokens, k)
	}
	m.Unlock()
	return nil
}

func (m *memoryStorage) collect(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.gc):
			now := time.Now().Unix()
			m.Lock()
			for t := range m.tokens {
				if now >= m.tokens[t] {
					delete(m.tokens, t)
				}
			}
			m.Unlock()
		}
	}
}
