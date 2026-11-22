package cache

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexedHeapGenerationSurvivesIndexReuse(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	firstIdx := h.put("same-key", 1, 1, 10, "")
	firstGen := h.generation(firstIdx)
	require.True(t, h.matches("same-key", firstIdx, firstGen))
	require.False(t, h.matches("same-key", -1, firstGen))
	require.False(t, h.matches("same-key", len(h.indices), firstGen))

	key, size := h.remove(firstIdx)
	require.Equal(t, "same-key", key)
	require.Equal(t, uint(10), size)
	require.False(t, h.matches("same-key", firstIdx, firstGen))

	secondIdx := h.put("same-key", 2, 2, 20, "")
	secondGen := h.generation(secondIdx)
	require.Equal(t, firstIdx, secondIdx)
	require.NotEqual(t, firstGen, secondGen)
	require.False(t, h.matches("same-key", firstIdx, firstGen))
	require.True(t, h.matches("same-key", secondIdx, secondGen))
	require.False(t, h.matches("other-key", secondIdx, secondGen))
	require.Equal(t, 1, h.Len())
	require.Equal(t, uint(20), h.entries[0].bytes)
}

func TestIndexedHeapOrphansAManifestWithItsLastEntry(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	a := h.put("m|a", 1, 1, 1, "m")
	b := h.put("m|b", 2, 2, 1, "m")
	h.put("plain", 3, 3, 1, "")

	h.remove(a)
	require.True(t, h.tracks("m"))
	require.Nil(t, h.takeOrphans())

	h.remove(b)
	require.False(t, h.tracks("m"))
	h.put("m|b", 4, 4, 1, "m") // a replacement stored under it again
	require.Nil(t, h.takeOrphans())

	h.remove(h.keys["m|b"])
	require.Equal(t, []string{"m"}, h.takeOrphans())
	require.Nil(t, h.takeOrphans())
}

// Nodes a failed eviction pops and puts back leave no orphans, however often it repeats.
func TestIndexedHeapPutBackNodesLeaveNoOrphans(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	const nodes = 100
	for i := range nodes {
		h.put("key"+strconv.Itoa(i), uint64(i), uint64(i), 1, "manifest"+strconv.Itoa(i))
	}

	for range 50 {
		var popped []heapEntry
		for h.Len() > 0 {
			popped = append(popped, h.entries[0])
			h.removeFirst()
		}
		require.Len(t, h.orphans, nodes)

		for _, entry := range slices.Backward(popped) {
			h.putWithGeneration(entry.key, entry.exp, entry.expires, entry.bytes, entry.gen, entry.manifest)
		}
		require.Empty(t, h.orphans)
		require.Equal(t, nodes, h.Len())
	}
	require.Nil(t, h.takeOrphans())
}

// Entries due together, as those stored in one second are, go in the order they were stored.
func TestIndexedHeapEvictsEntriesDueTogetherInStoringOrder(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	const nodes = 64
	for i := range nodes {
		h.put(strconv.Itoa(i), 7, 7, 1, "")
	}
	for i := range nodes {
		key, _ := h.removeFirst()
		require.Equal(t, strconv.Itoa(i), key)
	}
}

// removeExpired goes by expiry, and the two orderings stay in step through every removal.
func TestIndexedHeapRemoveExpiredGoesByExpiry(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	h.put("asset", 300, 1_000_000, 1, "") // due at 300, fresh for long after
	h.put("api", 430, 430, 1, "")         // due when it expires, at 430
	c := h.put("page", 500, 2000, 1, "")

	_, expired := h.removeExpired(429)
	require.False(t, expired)
	entry, expired := h.removeExpired(500)
	require.True(t, expired)
	require.Equal(t, "api", entry.key)
	_, expired = h.removeExpired(500)
	require.False(t, expired, "the rest is fresh")

	h.remove(c)
	key, _ := h.removeFirst()
	require.Equal(t, "asset", key)
	require.Zero(t, h.Len())
	require.Empty(t, h.byExpiry)

	// Random puts and removals of every kind, against what is left.
	rng := rand.New(rand.NewPCG(3, 4))
	live := map[string]uint64{} // key -> when it expires
	idx := map[string]int{}
	for i := range 2000 {
		switch op := rng.IntN(4); {
		case op == 0 || len(live) == 0:
			key, expires := strconv.Itoa(i), rng.Uint64N(1000)
			idx[key] = h.put(key, rng.Uint64N(1000), expires, 1, "")
			live[key] = expires
		case op == 1:
			key, _ := h.removeFirst()
			delete(live, key)
		case op == 2:
			for key := range live {
				removed, _ := h.remove(idx[key])
				require.Equal(t, key, removed)
				delete(live, key)
				break
			}
		default:
			now := rng.Uint64N(1000)
			soonest, found := uint64(0), false
			for _, expires := range live {
				if expires <= now && (!found || expires < soonest) {
					soonest, found = expires, true
				}
			}
			entry, expired := h.removeExpired(now)
			require.Equal(t, found, expired)
			if expired {
				require.Equal(t, soonest, entry.expires)
				require.Equal(t, live[entry.key], entry.expires)
				delete(live, entry.key)
			}
		}
		require.Len(t, h.byExpiry, len(live))
		require.Equal(t, len(live), h.Len())
	}
}

func TestIndexedHeapRemoveKeepsOrder(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))
	h := &indexedHeap{}
	idx := make([]int, 200)
	for i := range idx {
		exp := rng.Uint64N(1000)
		idx[i] = h.put(strconv.Itoa(i), exp, exp, uint(exp), "")
	}
	for _, i := range rng.Perm(len(idx))[:100] {
		key, _ := h.remove(idx[i])
		require.Equal(t, strconv.Itoa(i), key)
	}

	var last uint
	for h.Len() > 0 {
		key, size := h.removeFirst()
		require.GreaterOrEqual(t, size, last)
		require.NotContains(t, h.keys, key)
		last = size
	}
	require.Empty(t, h.keys)
}

// Sequential: testing.AllocsPerRun panics during a parallel test.
func TestIndexedHeapEvictionDoesNotAllocate(t *testing.T) {
	h := &indexedHeap{}
	for i := range 64 {
		h.put(strconv.Itoa(i), uint64(i), uint64(i), 1, "")
	}
	exp := uint64(64)
	allocs := testing.AllocsPerRun(100, func() {
		key, size := h.removeFirst()
		h.put(key, exp, exp, size, "")
		exp++
	})
	require.Zero(t, allocs)
}
