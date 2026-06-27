package keylock

import (
	"hash/maphash"
	"math/bits"
	"runtime"
	"sync"
)

// shardsPerProc scales the shard count with the available parallelism. Only
// GOMAXPROCS goroutines can contend for a shard mutex at the same instant, and
// the mutex guards just the short map bookkeeping (not the per-key critical
// section), so a small multiple of GOMAXPROCS keeps false collisions rare.
const shardsPerProc = 4

// Locker serializes work per key while limiting global contention through shards.
type Locker struct {
	shards []shard
	seed   maphash.Seed
	mask   uint64
}

type shard struct {
	owners map[string]chan struct{}
	mu     sync.Mutex
}

// New creates a Locker whose shard count adapts to the available parallelism
// (GOMAXPROCS) with minShards as a floor, so larger machines get more shards
// without a hardcoded ceiling. The effective count is rounded up to a power of
// two so the hot path selects a shard with a bitmask instead of a modulo. A
// non-positive minShards is treated as 1.
func New(minShards int) *Locker {
	count := shardCount(minShards)

	return &Locker{
		seed:   maphash.MakeSeed(),
		shards: make([]shard, count),
		//nolint:gosec // G115: count is a power of two >= 1, so count-1 is non-negative
		mask: uint64(count - 1),
	}
}

// shardCount returns the next power of two greater than or equal to
// max(minShards, shardsPerProc*GOMAXPROCS), with a floor of 1.
func shardCount(minShards int) int {
	if minShards < 1 {
		minShards = 1
	}

	n := max(shardsPerProc*runtime.GOMAXPROCS(0), minShards)

	return nextPow2(n)
}

// nextPow2 returns the smallest power of two greater than or equal to n.
func nextPow2(n int) int {
	if n <= 1 {
		return 1
	}

	return 1 << bits.Len(uint(n-1))
}

// Lock acquires exclusive ownership for a key and returns a release function.
func (l *Locker) Lock(key string) func() {
	s := &l.shards[maphash.String(l.seed, key)&l.mask]

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
