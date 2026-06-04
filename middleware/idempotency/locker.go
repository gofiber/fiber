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
	shard := l.getShard(key)

	for {
		shard.mu.Lock()
		lock, ok := shard.keys[key]
		if !ok {
			lock = &countedLock{}
			shard.keys[key] = lock
		}
		lock.locked++
		shard.mu.Unlock()

		lock.mu.Lock()

		shard.mu.Lock()

		currentLock, ok := shard.keys[key]
		if ok && currentLock == lock {
			shard.mu.Unlock()
			return nil
		}
		lock.locked--
		lock.mu.Unlock()
		shard.mu.Unlock()
	}
}

// Unlock releases the lock associated with the provided key.
func (l *MemoryLock) Unlock(key string) error {
	shard := l.getShard(key)
	shard.mu.Lock()
	lock, ok := shard.keys[key]
	if !ok {
		// This happens if we try to unlock an unknown key
		shard.mu.Unlock()
		return nil
	}
	lock.mu.Unlock()

	lock.locked--
	if lock.locked == 0 {
		delete(shard.keys, key)
	}
	shard.mu.Unlock()
	return nil
}

func (l *MemoryLock) getShard(key string) *lockerShard {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	hash := uint32(offset32)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= prime32
	}
	return l.shards[hash%numShards]
}

var _ Locker = (*MemoryLock)(nil)
