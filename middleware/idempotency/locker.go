package idempotency

import (
	"sync"
)

// Locker implements a spinlock for a string key.
type Locker interface {
	Lock(key string) error
	Unlock(key string) error
}

type countedLock struct {
	mu     sync.Mutex
	locked int
}

type MemoryLock struct {
	keys map[string]*countedLock
	mu   sync.Mutex
}

func (l *MemoryLock) Lock(key string) error {
	l.mu.Lock()
	lock, ok := l.keys[key]
	if !ok {
		lock = new(countedLock)
		l.keys[key] = lock
	}
	lock.locked++
	l.mu.Unlock()

	lock.mu.Lock()

	return nil
}

func (l *MemoryLock) Unlock(key string) error {
	l.mu.Lock()
	lock, ok := l.keys[key]
	if !ok {
		// This happens if we try to unlock an unknown key
		l.mu.Unlock()
		return nil
	}

	lock.locked--
	if lock.locked <= 0 {
		// This happens if countedLock is used to Lock and Unlock the same number of times
		// So, we can delete the key to prevent memory leak
		delete(l.keys, key)
	}
	l.mu.Unlock()

	lock.mu.Unlock()

	return nil
}

func NewMemoryLock() *MemoryLock {
	return &MemoryLock{
		keys: make(map[string]*countedLock),
	}
}

var _ Locker = (*MemoryLock)(nil)
