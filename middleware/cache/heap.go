package cache

import (
	"container/heap"
	"crypto/rand"
	"encoding/binary"
	"fmt"
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
	exp      uint64 // when the entry is due to be evicted
	expires  uint64 // when the entry stops being fresh
	gen      uint64
	bytes    uint
	idx      int
	tag      purgeTag
}

// purgeTag is how a write finds the entries it ends (RFC 9111 §4.4): the purgeTable slot of
// the entry's path, and the stamp taken when its request began.
type purgeTag struct {
	slot, stored uint64
}

// slotLink holds an index's neighbors in its slot's list, each one above its index, so 0 is none.
type slotLink struct {
	prev, next int
}

// slotList holds the first and last index of a slot's list, one above as in slotLink.
type slotList struct {
	head, tail int
}

type expiryNode struct {
	expires uint64
	idx     int
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
	orphans   map[string]struct{}
	// Slice the heap is built on
	entries []heapEntry
	// Mapping "index" to position in heap slice
	indices []int
	// A second heap by when entries stop being fresh, and each index's position in it (see removeExpired).
	byExpiry  []expiryNode
	expiryPos []int
	// ends reports whether a write ended an entry, as purgeTable.ends does; nil ends none.
	ends func(slot, stored uint64) bool
	// Each purge slot's indices in storing order, those ended on arrival first, and the slots
	// whose first entries a write may have ended (see removeEnded).
	slots   map[uint64]slotList
	flagged map[uint64]struct{}
	links   []slotLink
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

// Less implements heap.Interface, ordering by when entries are due to be evicted, then by storing order.
func (h *indexedHeap) Less(i, j int) bool {
	a, b := &h.entries[i], &h.entries[j]
	if a.exp != b.exp {
		return a.exp < b.exp
	}
	// Generations wrap from a random start (see nextGeneration): a is older if b is under half the range ahead.
	ahead := b.gen - a.gen
	return ahead != 0 && ahead < 1<<63
}

// Swap implements heap.Interface and swaps the entries at the provided indices.
func (h *indexedHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.indices[h.entries[i].idx] = i
	h.indices[h.entries[j].idx] = j
}

// Push implements heap.Interface and inserts a new entry into the heap.
func (h *indexedHeap) Push(x any) {
	entry := x.(heapEntry) //nolint:forcetypeassert,errcheck // Forced type assertion required to implement the heap.Interface interface
	h.pushInternal(&entry)
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
			if h.orphans == nil {
				h.orphans = make(map[string]struct{})
			}
			h.orphans[entry.manifest] = struct{}{}
		}
	}
	h.expiryRemove(entry.idx)
	h.unlink(entry.idx, entry.tag.slot)
	h.entries = h.entries[0 : n-1]
	return entry
}

func (h *indexedHeap) pushInternal(entry *heapEntry) {
	if h.keys == nil {
		h.keys = make(map[string]int)
	}
	h.keys[entry.key] = entry.idx
	if entry.manifest != "" {
		if h.manifests == nil {
			h.manifests = make(map[string]int)
		}
		h.manifests[entry.manifest]++
		// Stored under again, or put back after a failed eviction: no longer an orphan.
		delete(h.orphans, entry.manifest)
	}
	h.indices[entry.idx] = len(h.entries)
	h.entries = append(h.entries, *entry)
	h.expiryPush(entry.idx, entry.expires)
	h.link(entry.idx, entry.tag)
}

// link appends idx to its slot's list or, when a write ended it already, puts it first and flags the slot.
func (h *indexedHeap) link(idx int, tag purgeTag) {
	for len(h.links) <= idx {
		h.links = append(h.links, slotLink{})
	}
	if h.slots == nil {
		h.slots = make(map[uint64]slotList)
	}
	list, self := h.slots[tag.slot], idx+1
	if h.ends != nil && h.ends(tag.slot, tag.stored) {
		h.links[idx] = slotLink{next: list.head}
		if list.head != 0 {
			h.links[list.head-1].prev = self
		} else {
			list.tail = self
		}
		list.head = self
		h.slots[tag.slot] = list
		h.flag(tag.slot)
		return
	}
	h.links[idx] = slotLink{prev: list.tail}
	if list.tail != 0 {
		h.links[list.tail-1].next = self
	} else {
		list.head = self
	}
	list.tail = self
	h.slots[tag.slot] = list
}

// unlink takes idx out of its slot's list, and forgets a slot left empty.
func (h *indexedHeap) unlink(idx int, slot uint64) {
	link, list := h.links[idx], h.slots[slot]
	if link.prev != 0 {
		h.links[link.prev-1].next = link.next
	} else {
		list.head = link.next
	}
	if link.next != 0 {
		h.links[link.next-1].prev = link.prev
	} else {
		list.tail = link.prev
	}
	if list.head == 0 {
		delete(h.slots, slot)
		delete(h.flagged, slot)
		return
	}
	h.slots[slot] = list
}

// flag has removeEnded look at slot, whose first entries a write may have ended. A slot that holds
// no entry stays unflagged, so the flags never outnumber the entries.
func (h *indexedHeap) flag(slot uint64) {
	if _, ok := h.slots[slot]; !ok {
		return
	}
	if h.flagged == nil {
		h.flagged = make(map[uint64]struct{})
	}
	h.flagged[slot] = struct{}{}
}

// removeEnded removes an entry a write ended (RFC 9111 §4.4), which no request can be served, from the
// front of a flagged slot, and unflags a slot whose first entry is live. An ended entry behind a live one,
// left by a request that ran wholly between a write's stamp and its mark, waits for a lookup.
func (h *indexedHeap) removeEnded() (heapEntry, bool) {
	if h.ends == nil || len(h.flagged) == 0 {
		return heapEntry{}, false // spare an eviction without a write the map iteration
	}
	for slot := range h.flagged {
		if list, ok := h.slots[slot]; ok {
			realIdx := h.indices[list.head-1]
			if entry := h.entries[realIdx]; h.ends(slot, entry.tag.stored) {
				h.removeInternal(realIdx)
				return entry, true
			}
		}
		delete(h.flagged, slot)
	}
	return heapEntry{}, false
}

func (h *indexedHeap) expiryPush(idx int, expires uint64) {
	for len(h.expiryPos) <= idx {
		h.expiryPos = append(h.expiryPos, 0)
	}
	h.expiryPos[idx] = len(h.byExpiry)
	h.byExpiry = append(h.byExpiry, expiryNode{expires: expires, idx: idx})
	h.expiryUp(len(h.byExpiry) - 1)
}

func (h *indexedHeap) expiryRemove(idx int) {
	pos, last := h.expiryPos[idx], len(h.byExpiry)-1
	if pos != last {
		h.expirySwap(pos, last)
	}
	h.byExpiry = h.byExpiry[:last]
	if pos != last {
		h.expiryDown(pos)
		h.expiryUp(pos)
	}
}

func (h *indexedHeap) expirySwap(i, j int) {
	h.byExpiry[i], h.byExpiry[j] = h.byExpiry[j], h.byExpiry[i]
	h.expiryPos[h.byExpiry[i].idx] = i
	h.expiryPos[h.byExpiry[j].idx] = j
}

func (h *indexedHeap) expiryUp(j int) {
	for j > 0 {
		i := (j - 1) / 2
		if h.byExpiry[i].expires <= h.byExpiry[j].expires {
			return
		}
		h.expirySwap(i, j)
		j = i
	}
}

func (h *indexedHeap) expiryDown(i int) {
	n := len(h.byExpiry)
	for {
		j := 2*i + 1
		if j >= n {
			return
		}
		if r := j + 1; r < n && h.byExpiry[r].expires < h.byExpiry[j].expires {
			j = r
		}
		if h.byExpiry[j].expires >= h.byExpiry[i].expires {
			return
		}
		h.expirySwap(i, j)
		i = j
	}
}

func (h *indexedHeap) removeExpired(now uint64) (heapEntry, bool) {
	if len(h.byExpiry) == 0 || h.byExpiry[0].expires > now {
		return heapEntry{}, false
	}
	realIdx := h.indices[h.byExpiry[0].idx]
	entry := h.entries[realIdx]
	h.removeInternal(realIdx)
	return entry, true
}

// tracks reports whether a live entry was stored under the Vary manifest.
func (h *indexedHeap) tracks(manifest string) bool {
	return h.manifests[manifest] > 0
}

// takeOrphans hands over the manifests no live entry has been stored under since their last one left.
func (h *indexedHeap) takeOrphans() []string {
	if len(h.orphans) == 0 {
		return nil
	}
	orphans := make([]string, 0, len(h.orphans))
	for manifest := range h.orphans {
		orphans = append(orphans, manifest)
	}
	clear(h.orphans)
	return orphans
}

// findKey finds the live node for a key without scanning the expiration heap.
func (h *indexedHeap) findKey(key string) (int, bool) {
	idx, ok := h.keys[key]
	return idx, ok
}

// Returns index to track entry
func (h *indexedHeap) put(key string, exp, expires uint64, bytes uint, manifest string, tag purgeTag) int {
	return h.putWithGeneration(key, exp, expires, bytes, h.nextGeneration(), manifest, tag)
}

// putWithGeneration restores an existing entry without changing its persisted
// identity. Its heap index may change if another entry reused the old slot.
func (h *indexedHeap) putWithGeneration(key string, exp, expires uint64, bytes uint, gen uint64, manifest string, tag purgeTag) int {
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
	h.pushInternal(&heapEntry{
		key: key, manifest: manifest, exp: exp, expires: expires, gen: gen, idx: idx, bytes: bytes, tag: tag,
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
