package cache

import (
	"container/heap"
)

type heapEntry struct {
	key   string
	exp   uint64
	bytes uint
	idx   int
}

// indexedHeap is a regular min-heap that allows finding
// elements in constant time. It does so by handing out special indices
// and tracking entry movement.
//
// indexdedHeap is used for quickly finding entries with the lowest
// expiration timestamp and deleting arbitrary entries.
type indexedHeap struct {
	// Slice the heap is built on
	entries []heapEntry
	// Mapping "index" to position in heap slice
	indices []int
	// Max index handed out
	maxidx int
}

func (h indexedHeap) Len() int {
	return len(h.entries)
}

func (h indexedHeap) Less(i, j int) bool {
	return h.entries[i].exp < h.entries[j].exp
}

func (h indexedHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.indices[h.entries[i].idx] = i
	h.indices[h.entries[j].idx] = j
}

func (h *indexedHeap) Push(x interface{}) {
	h.pushInternal(x.(heapEntry)) //nolint:forcetypeassert // Forced type assertion required to implement the heap.Interface interface
}

func (h *indexedHeap) Pop() interface{} {
	n := len(h.entries)
	h.entries = h.entries[0 : n-1]
	return h.entries[0:n][n-1]
}

func (h *indexedHeap) pushInternal(entry heapEntry) {
	h.indices[entry.idx] = len(h.entries)
	h.entries = append(h.entries, entry)
}

// Returns index to track entry
func (h *indexedHeap) put(key string, exp uint64, bytes uint) int {
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
		key: key, exp: exp, idx: idx, bytes: bytes,
	})
	heap.Fix(h, h.Len()-1)
	return idx
}

func (h *indexedHeap) removeInternal(realIdx int) (string, uint) {
	x := heap.Remove(h, realIdx).(heapEntry) //nolint:forcetypeassert,errcheck // Forced type assertion required to implement the heap.Interface interface
	return x.key, x.bytes
}

// Remove entry by index
func (h *indexedHeap) remove(idx int) (string, uint) {
	return h.removeInternal(h.indices[idx])
}

// Remove entry with lowest expiration time
func (h *indexedHeap) removeFirst() (string, uint) {
	return h.removeInternal(0)
}
