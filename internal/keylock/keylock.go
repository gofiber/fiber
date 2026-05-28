package keylock

import (
	"hash/maphash"
	"sync"
)

// Locker serializes work per key while limiting global contention through shards.
type Locker struct {
	seed   maphash.Seed
	shards []shard
}

type shard struct {
	mu     sync.Mutex
	owners map[string]chan struct{}
}

// New creates a Locker with the provided shard count.
func New(shardCount int) *Locker {
	if shardCount <= 0 {
		shardCount = 1
	}

	return &Locker{
		seed:   maphash.MakeSeed(),
		shards: make([]shard, shardCount),
	}
}

// Lock acquires exclusive ownership for a key and returns a release function.
func (l *Locker) Lock(key string) func() {
	s := &l.shards[maphash.String(l.seed, key)%uint64(len(l.shards))]

	for {
		s.mu.Lock()
		if s.owners == nil {
			s.owners = make(map[string]chan struct{})
		}
		if wait, exists := s.owners[key]; exists {
			s.mu.Unlock()
			<-wait
			continue
		}

		owner := make(chan struct{})
		s.owners[key] = owner
		s.mu.Unlock()

		return func() {
			s.mu.Lock()
			if s.owners[key] == owner {
				delete(s.owners, key)
				s.mu.Unlock()
				close(owner)
			} else {
				s.mu.Unlock()
			}
		}
	}
}
