package idempotency

import (
	"hash/fnv"
	"sync"
)

const numShards = 32

// Locker implements a spinlock for a string key.
type Locker interface {
	Lock(key string) error
	Unlock(key string) error
}

type countedLock struct {
	mu     sync.Mutex
	locked int
}

type lockerShard struct {
	keys map[string]*countedLock
	mu   sync.Mutex
}

// MemoryLock coordinates access to idempotency keys using in-memory locks.
// MemoryLock is safe for concurrent use.
type MemoryLock struct {
	shards []*lockerShard
}

// NewMemoryLock creates a MemoryLock ready for use.
func NewMemoryLock() *MemoryLock {

	shards := make([]*lockerShard, numShards)
	for i := range numShards {
		shards[i] = &lockerShard{
			keys: make(map[string]*countedLock),
		}
	}

	return &MemoryLock{
		shards: shards,
	}
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

func (l *MemoryLock) getShard(key string) *lockerShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return l.shards[h.Sum32()%numShards]
}

var _ Locker = (*MemoryLock)(nil)
