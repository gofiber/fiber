package storage

import (
	"errors"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

type memoryStorage struct {
	sync.RWMutex
	tokens map[string]int64
}

// NewMemoryStorage - Creates new in-memory storage for CSRF tokens
func NewMemoryStorage() fiber.Storage {
	storage := &memoryStorage{
		tokens: make(map[string]int64),
	}
	go storage.gc()
	return storage
}

func (m *memoryStorage) Get(id string) ([]byte, error) {
	m.RLock()
	t, ok := m.tokens[id]
	m.RUnlock()
	// Check if token exist or expired
	if !ok || time.Now().Unix() >= t {
		return nil, errors.New("invalid key")
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
	delete(m.tokens, id)
	return nil
}

// Clear clears the storage
func (m *memoryStorage) Clear() error {
	for k := range m.tokens {
		delete(m.tokens, k)
	}
	return nil
}

func (m *memoryStorage) gc() {
	for {
		// GC the tokens every 10 seconds to avoid
		time.Sleep(10 * time.Second)
		m.Lock()
		for t := range m.tokens {
			if time.Now().Unix() >= m.tokens[t] {
				delete(m.tokens, t)
			}
		}
		m.Unlock()
	}
}
