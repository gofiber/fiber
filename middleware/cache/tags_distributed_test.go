package cache

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Encoding round-trip
// ---------------------------------------------------------------------------

func Test_encodeDecodeStringSet_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		nil,
		{},
		{"single"},
		{"a", "b", "c"},
		{"", "empty-first"},
		{"user:123", "product:456", "region:us-east-1"},
	}
	for _, input := range cases {
		got := decodeStringSet(encodeStringSet(input))
		if len(input) == 0 {
			require.Empty(t, got)
		} else {
			require.Equal(t, input, got)
		}
	}
}

func Test_decodeStringSet_Truncated(t *testing.T) {
	t.Parallel()

	full := encodeStringSet([]string{"hello", "world"})
	// Truncate at every byte boundary; must never panic
	for i := range len(full) {
		got := decodeStringSet(full[:i])
		require.LessOrEqual(t, len(got), 2)
	}
}

// ---------------------------------------------------------------------------
// distributedTagStore unit tests
// ---------------------------------------------------------------------------

func Test_distributedTagStore_AddAndHas(t *testing.T) {
	t.Parallel()

	store := memory.New()
	defer store.Close()
	d := newDistributedTagStore(store, 5*time.Minute)

	require.False(t, d.has("k1"))
	d.add("k1", []string{"a", "b"})
	require.True(t, d.has("k1"))

	// Forward and reverse indexes persisted in shared storage
	require.Contains(t, d.readSet(tagKeyPrefix+"a"), "k1")
	require.Contains(t, d.readSet(tagKeyPrefix+"b"), "k1")
	require.ElementsMatch(t, []string{"a", "b"}, d.readSet(tagRevKeyPrefix+"k1"))

	// Idempotent: second add does not duplicate entries
	d.add("k1", []string{"a", "b"})
	require.Equal(t, []string{"k1"}, d.readSet(tagKeyPrefix+"a"))

	// Empty tags is a no-op
	d.add("k2", nil)
	require.False(t, d.has("k2"))
}

func Test_distributedTagStore_Remove(t *testing.T) {
	t.Parallel()

	store := memory.New()
	defer store.Close()
	d := newDistributedTagStore(store, 5*time.Minute)

	d.add("k1", []string{"a", "b"})
	d.add("k2", []string{"a"})

	d.remove("k1")
	require.False(t, d.has("k1"))

	// "a" forward still contains k2; "b" forward is deleted (empty set)
	require.Equal(t, []string{"k2"}, d.readSet(tagKeyPrefix+"a"))
	require.Nil(t, d.readSet(tagKeyPrefix+"b"))
	// Reverse index for k1 is gone
	require.Nil(t, d.readSet(tagRevKeyPrefix+"k1"))

	// Removing a non-existent key is a safe no-op
	d.remove("nonexistent")
}

func Test_distributedTagStore_Invalidate(t *testing.T) {
	t.Parallel()

	store := memory.New()
	defer store.Close()
	d := newDistributedTagStore(store, 5*time.Minute)

	d.add("k1", []string{"a", "b"})
	d.add("k2", []string{"b", "c"})
	d.add("k3", []string{"c"})

	keys := d.invalidate([]string{"b"})
	sort.Strings(keys)
	require.Equal(t, []string{"k1", "k2"}, keys)

	// Affected keys removed from local index
	require.False(t, d.has("k1"))
	require.False(t, d.has("k2"))
	// Unaffected key still tracked
	require.True(t, d.has("k3"))

	// Shared reverse indexes: only the invalidated tag stripped
	require.Equal(t, []string{"a"}, d.readSet(tagRevKeyPrefix+"k1"))
	require.Equal(t, []string{"c"}, d.readSet(tagRevKeyPrefix+"k2"))
	// Forward index for "b" deleted; others intact
	require.Nil(t, d.readSet(tagKeyPrefix+"b"))
	require.Contains(t, d.readSet(tagKeyPrefix+"a"), "k1")
	require.Contains(t, d.readSet(tagKeyPrefix+"c"), "k2")
	require.Contains(t, d.readSet(tagKeyPrefix+"c"), "k3")
}

func Test_distributedTagStore_InvalidateMultipleTags(t *testing.T) {
	t.Parallel()

	store := memory.New()
	defer store.Close()
	d := newDistributedTagStore(store, 5*time.Minute)

	d.add("k1", []string{"a", "b"})
	d.add("k2", []string{"b", "c"})
	d.add("k3", []string{"c", "d"})

	keys := d.invalidate([]string{"a", "c"})
	sort.Strings(keys)
	require.Equal(t, []string{"k1", "k2", "k3"}, keys)

	// All affected keys gone from local
	require.False(t, d.has("k1"))
	require.False(t, d.has("k2"))
	require.False(t, d.has("k3"))

	// Only the invalidated tags stripped from reverse indexes
	require.Equal(t, []string{"b"}, d.readSet(tagRevKeyPrefix+"k1"))
	require.Equal(t, []string{"b"}, d.readSet(tagRevKeyPrefix+"k2"))
	require.Equal(t, []string{"d"}, d.readSet(tagRevKeyPrefix+"k3"))

	// Forward indexes for "a" and "c" deleted; "b" and "d" intact
	require.Nil(t, d.readSet(tagKeyPrefix+"a"))
	require.Nil(t, d.readSet(tagKeyPrefix+"c"))
	require.ElementsMatch(t, []string{"k1", "k2"}, d.readSet(tagKeyPrefix+"b"))
	require.Equal(t, []string{"k3"}, d.readSet(tagKeyPrefix+"d"))
}

// ---------------------------------------------------------------------------
// Cross-instance coordination
// ---------------------------------------------------------------------------

func Test_distributedTagStore_CrossInstance(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	defer shared.Close()

	d1 := newDistributedTagStore(shared, 5*time.Minute)
	d2 := newDistributedTagStore(shared, 5*time.Minute)

	// Instance 1 populates the shared index
	d1.add("k1", []string{"user:1"})
	d1.add("k2", []string{"user:1", "product:x"})

	// Instance 2 invalidates – finds keys written by instance 1
	keys := d2.invalidate([]string{"user:1"})
	sort.Strings(keys)
	require.Equal(t, []string{"k1", "k2"}, keys)

	// Shared forward index for "user:1" is gone
	require.Nil(t, d2.readSet(tagKeyPrefix+"user:1"))

	// k2 still has "product:x" in its shared reverse index
	require.Equal(t, []string{"product:x"}, d2.readSet(tagRevKeyPrefix+"k2"))

	// Instance 1's local index is stale (it doesn't know about the remote
	// invalidation until it re-populates on the next cache hit)
	require.True(t, d1.has("k1"))
	require.True(t, d1.has("k2"))
}

func Test_distributedTagStore_CrossInstanceBothDirections(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	defer shared.Close()

	d1 := newDistributedTagStore(shared, 5*time.Minute)
	d2 := newDistributedTagStore(shared, 5*time.Minute)

	// Each instance adds entries under the same tag
	d1.add("k1", []string{"shared-tag"})
	d2.add("k2", []string{"shared-tag"})

	// Either instance sees both keys in the shared forward index
	fwd := d1.readSet(tagKeyPrefix + "shared-tag")
	require.ElementsMatch(t, []string{"k1", "k2"}, fwd)

	// Instance 1 invalidates and collects keys from both instances
	keys := d1.invalidate([]string{"shared-tag"})
	sort.Strings(keys)
	require.Equal(t, []string{"k1", "k2"}, keys)

	// Shared forward index is cleared
	require.Nil(t, d1.readSet(tagKeyPrefix+"shared-tag"))
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func Test_distributedTagStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	store := memory.New()
	defer store.Close()
	d := newDistributedTagStore(store, 5*time.Minute)

	const n = 100
	var wg sync.WaitGroup

	// Concurrent adds with 10 shared tags
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			d.add(fmt.Sprintf("key:%d", i), []string{fmt.Sprintf("tag:%d", i%10)})
		}(i)
	}
	wg.Wait()

	for i := range n {
		require.True(t, d.has(fmt.Sprintf("key:%d", i)))
	}

	// Concurrently invalidate tags 0–4
	wg.Add(5)
	for i := range 5 {
		go func(i int) {
			defer wg.Done()
			d.invalidate([]string{fmt.Sprintf("tag:%d", i)})
		}(i)
	}
	wg.Wait()

	for i := range n {
		if i%10 < 5 {
			require.False(t, d.has(fmt.Sprintf("key:%d", i)),
				"key:%d under tag %d should be invalidated", i, i%10)
		} else {
			require.True(t, d.has(fmt.Sprintf("key:%d", i)),
				"key:%d under tag %d should still exist", i, i%10)
		}
	}
}
