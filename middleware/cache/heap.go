package cache

import (
	"container/heap"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
)

var (
	heapGenerationOnce sync.Once
	heapGeneration     atomic.Uint64
)

func seedHeapGeneration() {
	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		panic(fmt.Errorf("cache: failed to initialize heap generation: %w", err))
	}
	heapGeneration.Store(binary.LittleEndian.Uint64(seed[:]))
}

func newIndexedHeap() *indexedHeap {
	heapGenerationOnce.Do(seedHeapGeneration)
	return &indexedHeap{}
}

type heapEntry struct {
	key      string
	manifest string // the Vary manifest the entry was stored under, if any
	exp      uint64
	gen      uint64
	bytes    uint
	idx      int
}

// indexedHeap is a regular min-heap that allows finding
// elements in constant time. It does so by handing out special indices
// and tracking entry movement.
//
// indexedHeap is used for quickly finding entries with the lowest
// expiration timestamp and deleting arbitrary entries.
type indexedHeap struct {
	// One live heap index per cache key.
	keys map[string]int
	// Live entries per Vary manifest, and the manifests whose last entry left since takeOrphans.
	manifests map[string]int
	orphans   []string
	// Slice the heap is built on
	entries []heapEntry
	// Mapping "index" to position in heap slice
	indices []int
	// Max index handed out
	maxidx int
}

// nextGeneration is safe to call without the cache lock, including when
// MaxBytes is disabled and no heap entries are tracked. A process-random
// starting point keeps persisted entries from matching a new process's heap.
func (*indexedHeap) nextGeneration() uint64 {
	heapGenerationOnce.Do(seedHeapGeneration)
	for {
		if gen := heapGeneration.Add(1); gen != 0 {
			return gen
		}
	}
}

// Len implements heap.Interface by reporting the number of entries in the heap.
func (h *indexedHeap) Len() int {
	return len(h.entries)
}

// Less implements heap.Interface and orders entries by expiration time.
func (h *indexedHeap) Less(i, j int) bool {
	return h.entries[i].exp < h.entries[j].exp
}

// Swap implements heap.Interface and swaps the entries at the provided indices.
func (h *indexedHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.indices[h.entries[i].idx] = i
	h.indices[h.entries[j].idx] = j
}

// Push implements heap.Interface and inserts a new entry into the heap.
func (h *indexedHeap) Push(x any) {
	h.pushInternal(x.(heapEntry)) //nolint:forcetypeassert,errcheck // Forced type assertion required to implement the heap.Interface interface
}

// Pop implements heap.Interface and removes the last entry from the heap.
func (h *indexedHeap) Pop() any {
	return h.popInternal()
}

func (h *indexedHeap) popInternal() heapEntry {
	n := len(h.entries)
	entry := h.entries[n-1]
	if h.keys[entry.key] == entry.idx {
		delete(h.keys, entry.key)
	}
	if entry.manifest != "" {
		h.manifests[entry.manifest]--
		if h.manifests[entry.manifest] == 0 {
			delete(h.manifests, entry.manifest)
			h.orphans = append(h.orphans, entry.manifest)
		}
	}
	h.entries = h.entries[0 : n-1]
	return entry
}

func (h *indexedHeap) pushInternal(entry heapEntry) {
	if h.keys == nil {
		h.keys = make(map[string]int)
	}
	h.keys[entry.key] = entry.idx
	if entry.manifest != "" {
		if h.manifests == nil {
			h.manifests = make(map[string]int)
		}
		h.manifests[entry.manifest]++
	}
	h.indices[entry.idx] = len(h.entries)
	h.entries = append(h.entries, entry)
}

// tracks reports whether a live entry was stored under the Vary manifest.
func (h *indexedHeap) tracks(manifest string) bool {
	return h.manifests[manifest] > 0
}

// takeOrphans hands over the manifests no live entry has been stored under since their last one left.
func (h *indexedHeap) takeOrphans() []string {
	// A replaced entry's manifest is tracked again by now; keeping the array spares a store an allocation.
	h.orphans = slices.DeleteFunc(h.orphans, h.tracks)
	if len(h.orphans) == 0 {
		return nil
	}
	orphans := h.orphans
	h.orphans = nil
	return orphans
}

// findKey finds the live node for a key without scanning the expiration heap.
func (h *indexedHeap) findKey(key string) (int, bool) {
	idx, ok := h.keys[key]
	return idx, ok
}

// Returns index to track entry
func (h *indexedHeap) put(key string, exp uint64, bytes uint, manifest string) int {
	return h.putWithGeneration(key, exp, bytes, h.nextGeneration(), manifest)
}

// putWithGeneration restores an existing entry without changing its persisted
// identity. Its heap index may change if another entry reused the old slot.
func (h *indexedHeap) putWithGeneration(key string, exp uint64, bytes uint, gen uint64, manifest string) int {
	idx := 0
	if len(h.entries) < h.maxidx {
		// Steal index from previously removed entry
		// capacity > size is guaranteed
		n := len(h.entries)
		idx = h.entries[:n+1][n].idx
	} else {
		idx = h.maxidx
		h.maxidx++
		h.indices = append(h.indices, idx)
	}
	// Push manually to avoid allocation
	h.pushInternal(heapEntry{
		key: key, manifest: manifest, exp: exp, gen: gen, idx: idx, bytes: bytes,
	})
	heap.Fix(h, h.Len()-1)
	return idx
}

// generation returns the identity of a live heap entry. Callers hold the cache lock.
func (h *indexedHeap) generation(idx int) uint64 {
	return h.entries[h.indices[idx]].gen
}

// matches checks both the recycled index and the lifetime of its current entry.
func (h *indexedHeap) matches(key string, idx int, gen uint64) bool {
	if idx < 0 || idx >= len(h.indices) {
		return false
	}
	realIdx := h.indices[idx]
	if realIdx < 0 || realIdx >= len(h.entries) {
		return false
	}
	entry := h.entries[realIdx]
	return entry.idx == idx && entry.key == key && entry.gen == gen
}

func (h *indexedHeap) removeInternal(realIdx int) (key string, size uint) { //nolint:nonamedreturns // gocritic unnamedResult prefers named key and size when removing heap entries
	// heap.Remove, unrolled so Pop does not box the entry on every eviction.
	last := len(h.entries) - 1
	if realIdx != last {
		h.Swap(realIdx, last)
	}
	x := h.popInternal()
	if realIdx != last {
		heap.Fix(h, realIdx)
	}
	return x.key, x.bytes
}

// Remove entry by index
func (h *indexedHeap) remove(idx int) (key string, size uint) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming returned key and size pair
	return h.removeInternal(h.indices[idx])
}

// Remove entry with lowest expiration time
func (h *indexedHeap) removeFirst() (key string, size uint) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming returned key and size pair
	return h.removeInternal(0)
}
