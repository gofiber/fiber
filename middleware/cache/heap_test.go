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
	firstIdx := h.put("same-key", 1, 1, 10, "", purgeTag{})
	firstGen := h.generation(firstIdx)
	require.True(t, h.matches("same-key", firstIdx, firstGen))
	require.False(t, h.matches("same-key", -1, firstGen))
	require.False(t, h.matches("same-key", len(h.indices), firstGen))

	key, size := h.remove(firstIdx)
	require.Equal(t, "same-key", key)
	require.Equal(t, uint(10), size)
	require.False(t, h.matches("same-key", firstIdx, firstGen))

	secondIdx := h.put("same-key", 2, 2, 20, "", purgeTag{})
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
	a := h.put("m|a", 1, 1, 1, "m", purgeTag{})
	b := h.put("m|b", 2, 2, 1, "m", purgeTag{})
	h.put("plain", 3, 3, 1, "", purgeTag{})

	h.remove(a)
	require.True(t, h.tracks("m"))
	require.Nil(t, h.takeOrphans())

	h.remove(b)
	require.False(t, h.tracks("m"))
	h.put("m|b", 4, 4, 1, "m", purgeTag{}) // a replacement stored under it again
	require.Nil(t, h.takeOrphans())

	h.remove(h.keys["m|b"])
	require.Equal(t, []string{"m"}, h.takeOrphans())
	require.Nil(t, h.takeOrphans())
}

func TestIndexedHeapPutBackNodesLeaveNoOrphans(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	const nodes = 100
	for i := range nodes {
		h.put("key"+strconv.Itoa(i), uint64(i), uint64(i), 1, "manifest"+strconv.Itoa(i), purgeTag{})
	}

	for range 50 {
		var popped []heapEntry
		for h.Len() > 0 {
			popped = append(popped, h.entries[0])
			h.removeFirst()
		}
		require.Len(t, h.orphans, nodes)

		for _, entry := range slices.Backward(popped) {
			h.putWithGeneration(entry.key, entry.exp, entry.expires, entry.bytes, entry.gen, entry.manifest, entry.tag)
		}
		require.Empty(t, h.orphans)
		require.Equal(t, nodes, h.Len())
	}
	require.Nil(t, h.takeOrphans())
}

func TestIndexedHeapEvictsEntriesDueTogetherInStoringOrder(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	const nodes = 64
	for i := range nodes {
		h.put(strconv.Itoa(i), 7, 7, 1, "", purgeTag{})
	}
	for i := range nodes {
		key, _ := h.removeFirst()
		require.Equal(t, strconv.Itoa(i), key)
	}
}

func TestIndexedHeapRemoveExpiredGoesByExpiry(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	h.put("asset", 300, 1_000_000, 1, "", purgeTag{}) // due at 300, fresh for long after
	h.put("api", 430, 430, 1, "", purgeTag{})         // due when it expires, at 430
	c := h.put("page", 500, 2000, 1, "", purgeTag{})

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

	rng := rand.New(rand.NewPCG(3, 4))
	live := map[string]uint64{} // key -> when it expires
	idx := map[string]int{}
	for i := range 2000 {
		switch op := rng.IntN(4); {
		case op == 0 || len(live) == 0:
			key, expires := strconv.Itoa(i), rng.Uint64N(1000)
			idx[key] = h.put(key, rng.Uint64N(1000), expires, 1, "", purgeTag{})
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

// heapTestWrites stands in for purgeTable.ends: the last write at each slot ends the entries whose
// request began by then.
type heapTestWrites map[uint64]uint64

func (w heapTestWrites) ends(slot, stored uint64) bool {
	last := w[slot]
	return last != 0 && stored <= last
}

func TestIndexedHeapRemoveEndedTakesWhatAWriteEnded(t *testing.T) {
	t.Parallel()
	writes := heapTestWrites{}
	h := newIndexedHeap()
	h.ends = writes.ends
	h.put("a", 1, 1000, 1, "", purgeTag{slot: 1, stored: 10})
	h.put("b", 2, 1000, 1, "", purgeTag{slot: 1, stored: 30})
	h.put("stale", 3, 50, 1, "", purgeTag{slot: 2, stored: 10})
	h.put("c", 4, 1000, 1, "", purgeTag{slot: 3, stored: 10})
	_, found := h.removeEnded()
	require.False(t, found, "nothing is ended before a write")

	writes[1] = 20 // after a began, before b did
	h.flag(1)
	entry, found := h.removeEnded()
	require.True(t, found)
	require.Equal(t, "a", entry.key)
	_, found = h.removeEnded()
	require.False(t, found, "b, first in the slot now, is live")
	require.Empty(t, h.flagged)

	entry, found = h.removeExpired(100)
	require.True(t, found)
	require.Equal(t, "stale", entry.key)

	// Stored after the write by a request that began before it: ended on arrival, it goes ahead of b
	// and flags the slot itself.
	h.put("late", 5, 1000, 1, "", purgeTag{slot: 1, stored: 15})
	entry, found = h.removeEnded()
	require.True(t, found)
	require.Equal(t, "late", entry.key)
	_, found = h.removeEnded()
	require.False(t, found, "b is live")

	writes[3] = 40
	h.flag(3)
	entry, found = h.removeEnded()
	require.True(t, found)
	require.Equal(t, "c", entry.key)
	require.NotContains(t, h.slots, uint64(3), "a slot left empty is forgotten")
	require.Empty(t, h.flagged)
	h.flag(9)
	require.Empty(t, h.flagged, "a slot without entries is not flagged")

	key, _ := h.removeFirst()
	require.Equal(t, "b", key)
	require.Empty(t, h.slots)

	unpurged := &indexedHeap{}
	unpurged.put("x", 1, 1, 1, "", purgeTag{slot: 1})
	unpurged.flag(1)
	_, found = unpurged.removeEnded()
	require.False(t, found, "without ends, nothing is ended")
}

func TestIndexedHeapSlotListsFollowEveryChange(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(5, 6))
	writes := heapTestWrites{}
	h := newIndexedHeap()
	h.ends = writes.ends
	model := map[uint64][]string{} // each slot's keys in list order: ended on arrival first, then the rest as stored
	idx, tags := map[string]int{}, map[string]purgeTag{}
	drop := func(key string) {
		slot := tags[key].slot
		model[slot] = slices.DeleteFunc(model[slot], func(k string) bool { return k == key })
		if len(model[slot]) == 0 {
			delete(model, slot)
		}
		delete(idx, key)
		delete(tags, key)
	}
	for clock := uint64(10); clock < 3010; clock++ {
		switch op := rng.IntN(5); {
		case op == 0 || len(idx) == 0:
			key := strconv.FormatUint(clock, 10)
			tag := purgeTag{slot: rng.Uint64N(4), stored: clock - rng.Uint64N(3)} // some began before the last write
			idx[key], tags[key] = h.put(key, rng.Uint64N(1000), rng.Uint64N(1000), 1, "", tag), tag
			if writes.ends(tag.slot, tag.stored) {
				model[tag.slot] = append([]string{key}, model[tag.slot]...)
			} else {
				model[tag.slot] = append(model[tag.slot], key)
			}
		case op == 1:
			key, _ := h.removeFirst()
			drop(key)
		case op == 2:
			for key := range idx {
				removed, _ := h.remove(idx[key])
				require.Equal(t, key, removed)
				drop(key)
				break
			}
		case op == 3:
			slot := rng.Uint64N(4)
			writes[slot] = clock
			h.flag(slot)
		default:
			entry, found := h.removeEnded()
			if !found {
				for key, tag := range tags {
					require.False(t, writes.ends(tag.slot, tag.stored), "ended entry %s left behind", key)
				}
				break
			}
			require.True(t, writes.ends(entry.tag.slot, entry.tag.stored))
			require.Equal(t, model[entry.tag.slot][0], entry.key, "only the first entry of a slot is taken")
			drop(entry.key)
		}

		require.Len(t, h.slots, len(model))
		for slot, keys := range model {
			var got []string
			prev, list := 0, h.slots[slot]
			for at := list.head; at != 0; at = h.links[at-1].next {
				require.Equal(t, prev, h.links[at-1].prev, "linked both ways")
				got = append(got, h.entries[h.indices[at-1]].key)
				prev = at
			}
			require.Equal(t, prev, list.tail)
			require.Equal(t, keys, got)
		}
		for slot := range h.flagged {
			require.Contains(t, h.slots, slot)
		}
	}
}

func TestIndexedHeapRemoveKeepsOrder(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))
	h := &indexedHeap{}
	idx := make([]int, 200)
	for i := range idx {
		exp := rng.Uint64N(1000)
		idx[i] = h.put(strconv.Itoa(i), exp, exp, uint(exp), "", purgeTag{})
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

func TestIndexedHeapFreedSlotsHoldNoKeys(t *testing.T) {
	t.Parallel()
	h := &indexedHeap{}
	a := h.put("a", 1, 1, 1, "m", purgeTag{})
	b := h.put("b", 2, 2, 1, "m", purgeTag{})
	h.remove(a)
	h.removeFirst()
	for _, entry := range h.entries[:cap(h.entries)] {
		require.Empty(t, entry.key)
		require.Empty(t, entry.manifest)
	}
	require.Equal(t, b, h.put("c", 3, 3, 1, "", purgeTag{}), "a freed index is still reused")
}

// Sequential: testing.AllocsPerRun panics during a parallel test.
func TestIndexedHeapEvictionDoesNotAllocate(t *testing.T) {
	h := &indexedHeap{ends: func(slot, _ uint64) bool { return slot == 0 }} // a write ended every entry of slot 0
	for i := range 64 {
		h.put(strconv.Itoa(i), uint64(i), uint64(i), 1, "", purgeTag{slot: uint64(i % 4)})
	}
	exp := uint64(64)
	allocs := testing.AllocsPerRun(100, func() {
		entry, ended := h.removeEnded()
		if !ended {
			entry.key, entry.bytes = h.removeFirst()
		}
		h.put(entry.key, exp, exp, entry.bytes, "", purgeTag{slot: exp % 4})
		exp++
	})
	require.Zero(t, allocs)
}
