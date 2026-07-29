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
	mu sync.Mutex

	keys map[string]*countedLock
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
	l.mu.Unlock()

	lock.mu.Unlock()

	l.mu.Lock()
	lock.locked--
	if lock.locked <= 0 {
		// No waiters left, so drop the entry instead of leaking one mutex per key
		delete(l.keys, key)
	}
	l.mu.Unlock()

	return nil
}

func NewMemoryLock() *MemoryLock {
	return &MemoryLock{
		keys: make(map[string]*countedLock),
	}
}

var _ Locker = (*MemoryLock)(nil)
