package idempotency

import (
	"sync"

	"github.com/gofiber/fiber/v3"
)

// maxLockWaiters is how many requests MemoryLock lets wait for one key: each holds a connection and a goroutine.
const maxLockWaiters = 100

// ErrTooManyWaiters is returned by MemoryLock when maxLockWaiters requests already wait for the key.
var ErrTooManyWaiters = fiber.NewError(fiber.StatusConflict, "too many requests are waiting for this idempotency key")

// Locker implements a spinlock for a string key.
type Locker interface {
	Lock(key string) error
	Unlock(key string) error
}

type countedLock struct {
	mu     sync.Mutex
	locked int
}

// MemoryLock coordinates access to idempotency keys using in-memory locks.
// MemoryLock is safe for concurrent use.
type MemoryLock struct {
	keys map[string]*countedLock
	mu   sync.Mutex
}

// Lock acquires the lock for the provided key, creating it when necessary.
func (l *MemoryLock) Lock(key string) error {
	l.mu.Lock()
	if l.keys == nil {
		l.keys = make(map[string]*countedLock)
	}
	lock, ok := l.keys[key]
	if !ok {
		lock = new(countedLock)
		l.keys[key] = lock
	}
	if lock.locked > maxLockWaiters {
		l.mu.Unlock()
		return ErrTooManyWaiters
	}
	lock.locked++
	l.mu.Unlock()

	lock.mu.Lock()

	return nil
}

// Unlock releases the lock associated with the provided key.
func (l *MemoryLock) Unlock(key string) error {
	l.mu.Lock()
	lock, ok := l.keys[key]
	if !ok {
		// This happens if we try to unlock an unknown key
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()

	lock.mu.Unlock()

	l.mu.Lock()
	lock.locked--
	if lock.locked <= 0 {
		// This happens if countedLock is used to Lock and Unlock the same number of times
		// So, we can delete the key to prevent memory leak
		delete(l.keys, key)
	}
	l.mu.Unlock()

	return nil
}

// NewMemoryLock creates a MemoryLock ready for use.
func NewMemoryLock() *MemoryLock {
	return &MemoryLock{
		keys: make(map[string]*countedLock),
	}
}

var _ Locker = (*MemoryLock)(nil)
