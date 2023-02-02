package idempotency

import (
	"sync"
)

// Locker implements a spinlock for a string key.
type Locker interface {
	Lock(key string) error
	Unlock(key string) error
}

type MemoryLock struct {
	mu sync.Mutex

	keys map[string]*sync.Mutex
}

func (l *MemoryLock) Lock(key string) error {
	l.mu.Lock()
	mu, ok := l.keys[key]
	if !ok {
		mu = new(sync.Mutex)
		l.keys[key] = mu
	}
	l.mu.Unlock()

	mu.Lock()

	return nil
}

func (l *MemoryLock) Unlock(key string) error {
	l.mu.Lock()
	mu, ok := l.keys[key]
	l.mu.Unlock()
	if !ok {
		// This happens if we try to unlock an unknown key
		return nil
	}

	mu.Unlock()

	return nil
}

func NewMemoryLock() *MemoryLock {
	return &MemoryLock{
		keys: make(map[string]*sync.Mutex),
	}
}

var _ Locker = (*MemoryLock)(nil)
