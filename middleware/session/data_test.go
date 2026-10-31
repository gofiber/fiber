package session

import (
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionTestData returns pooled data holding the key/value pairs given as
// alternating arguments, and releases it when the test ends.
func sessionTestData(t *testing.T, pairs ...any) *data {
	t.Helper()

	d := acquireData()
	t.Cleanup(func() { releaseData(d) })
	for kv := range slices.Chunk(pairs, 2) {
		d.Set(kv[0], kv[1])
	}
	return d
}

// sessionTestDataCases are the key sets that the Keys and Len tests store.
var sessionTestDataCases = []struct {
	name  string
	pairs []any // alternating keys and values
	keys  []any
}{
	{name: "Empty data"},
	{name: "Single key", pairs: []any{"key1", "value1"}, keys: []any{"key1"}},
	{
		name: "Multiple keys", pairs: []any{"key1", "value1", "key2", "value2", "key3", "value3"},
		keys: []any{"key1", "key2", "key3"},
	},
}

func TestKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range sessionTestDataCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ElementsMatch(t, tc.keys, sessionTestData(t, tc.pairs...).Keys())
		})
	}

	t.Run("Concurrent access", func(t *testing.T) {
		t.Parallel()
		d := sessionTestData(t, "key1", "value1", "key2", "value2", "key3", "value3")

		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() { assert.Len(t, d.Keys(), 3, "Expected three keys") })
		}
		wg.Wait()
	})
}

func TestData_Len(t *testing.T) {
	t.Parallel()
	for _, tc := range sessionTestDataCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, len(tc.keys), sessionTestData(t, tc.pairs...).Len())
		})
	}

	t.Run("Concurrent access", func(t *testing.T) {
		t.Parallel()
		d := sessionTestData(t, "key1", "value1", "key2", "value2", "key3", "value3")

		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() { assert.Equal(t, 3, d.Len(), "Expected length to be 3 during concurrent access") })
		}
		wg.Wait()
	})
}

func TestData_Get(t *testing.T) {
	t.Parallel()
	t.Run("Nonexistent key", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, sessionTestData(t).Get("nonexistent-key"), "Expected nil for nonexistent key")
	})

	t.Run("Existing key", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "value1", sessionTestData(t, "key1", "value1").Get("key1"), "Expected value1 for key1")
	})
}

func TestData_Reset(t *testing.T) {
	t.Parallel()
	t.Run("Reset data", func(t *testing.T) {
		t.Parallel()
		d := sessionTestData(t, "key1", "value1", "key2", "value2")
		d.Reset()
		requireDataEmpty(t, d, "Expected data map to be empty after reset")
	})
}

func mapPointer(m map[any]any) uintptr {
	return reflect.ValueOf(m).Pointer()
}

func lockedMapPointer(d *data) uintptr {
	d.RLock()
	defer d.RUnlock()
	return mapPointer(d.Data)
}

func requireDataEmpty(t *testing.T, d *data, msg string) {
	t.Helper()
	d.RLock()
	defer d.RUnlock()
	require.Empty(t, d.Data, msg)
}

func TestData_ResetPreservesAllocation(t *testing.T) {
	t.Parallel()
	d := sessionTestData(t)

	originalPtr := lockedMapPointer(d)

	d.Set("key1", "value1")
	d.Set("key2", "value2")
	require.Equal(t, originalPtr, lockedMapPointer(d), "Expected map pointer to stay constant after writes")

	d.Reset()
	requireDataEmpty(t, d, "Expected data map to be empty after reset")
	require.Equal(t, originalPtr, lockedMapPointer(d), "Expected reset to preserve underlying map")

	d.Set("key3", "value3")
	require.Nil(t, d.Get("key1"), "Expected cleared key not to leak after reset")
	require.Equal(t, originalPtr, lockedMapPointer(d), "Expected map pointer to remain stable after further writes")
}

func TestData_PoolReuseDoesNotLeakEntries(t *testing.T) {
	t.Parallel()
	acquired := make([]*data, 0, 6)
	t.Cleanup(func() {
		for _, item := range acquired {
			releaseData(item)
		}
	})

	acquireWithCleanup := func() *data {
		d := acquireData()
		acquired = append(acquired, d)
		return d
	}

	// Acquire first outside acquireWithCleanup to avoid double-put:
	// it is explicitly released via releaseData below.
	first := acquireData()
	first.Set("key1", "value1")
	first.Set("key2", "value2")
	first.Reset()

	originalPtr := lockedMapPointer(first)
	releaseData(first)

	var reused *data
	for range 5 {
		candidate := acquireWithCleanup()
		if lockedMapPointer(candidate) == originalPtr {
			reused = candidate
			break
		}
		requireDataEmpty(t, candidate, "Expected pooled data to be empty when new instance is returned")
		require.Nil(t, candidate.Get("key2"), "Expected no leakage of prior entries on alternate pooled instance")
	}

	if reused == nil {
		t.Skip("sync.Pool returned a different instance; reuse cannot be asserted")
		return
	}

	require.Equal(t, originalPtr, lockedMapPointer(reused), "Expected pooled data to reuse cleared map")
	requireDataEmpty(t, reused, "Expected pooled data to be empty after reuse")
	require.Nil(t, reused.Get("key2"), "Expected no leakage of prior entries on reuse")

	reused.Set("key4", "value4")
	require.Equal(t, "value4", reused.Get("key4"), "Expected pooled map to accept new values")
}

func TestData_Delete(t *testing.T) {
	t.Parallel()
	t.Run("Delete existing key", func(t *testing.T) {
		t.Parallel()
		d := sessionTestData(t, "key1", "value1")
		d.Delete("key1")
		require.Nil(t, d.Get("key1"), "Expected nil for deleted key")
	})

	t.Run("Delete nonexistent key", func(t *testing.T) {
		t.Parallel()
		d := sessionTestData(t)
		d.Delete("nonexistent-key")
		// No assertion needed, just ensure no panic or error
	})
}

func Test_ValueMayAlias(t *testing.T) {
	t.Parallel()
	type userID int
	type name string
	type scalarStruct struct{ N int }

	now := time.Now()

	tests := []struct {
		value any
		name  string
		alias bool
	}{
		{name: "nil", value: nil, alias: false},
		{name: "string", value: "fenny", alias: false},
		{name: "named string", value: name("fenny"), alias: false},
		{name: "int", value: 42, alias: false},
		{name: "named int", value: userID(42), alias: false},
		{name: "bool", value: true, alias: false},
		{name: "float", value: 1.5, alias: false},
		{name: "duration", value: time.Hour, alias: false},
		{name: "time", value: now, alias: false},
		{name: "time pointer", value: &now, alias: true},
		{name: "bytes", value: []byte("fenny"), alias: true},
		{name: "slice", value: []string{"fenny"}, alias: true},
		{name: "map", value: map[string]string{}, alias: true},
		{name: "pointer", value: new(int), alias: true},
		// A struct is copied, but a field could still alias, so it stays conservative.
		{name: "struct", value: scalarStruct{}, alias: true},
		{name: "array", value: [2]int{}, alias: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.alias, valueMayAlias(tt.value))
		})
	}
}

func Test_Data_DirtyTracking(t *testing.T) {
	t.Parallel()

	// cleanData returns pooled data holding key/value with the dirty flag cleared,
	// standing in for the state a successful decode leaves behind.
	cleanData := func(t *testing.T, key, value any) *data {
		t.Helper()
		d := sessionTestData(t, key, value)
		d.dirty.Store(false)
		return d
	}

	t.Run("Set marks dirty", func(t *testing.T) {
		t.Parallel()
		d := cleanData(t, "user", "fenny")
		d.Set("role", "admin")
		require.True(t, d.dirty.Load())
	})

	t.Run("Delete marks dirty", func(t *testing.T) {
		t.Parallel()
		d := cleanData(t, "user", "fenny")
		d.Delete("user")
		require.True(t, d.dirty.Load())
	})

	t.Run("Reset marks dirty", func(t *testing.T) {
		t.Parallel()
		d := cleanData(t, "user", "fenny")
		d.Reset()
		require.True(t, d.dirty.Load())
	})

	t.Run("Get of a scalar stays clean", func(t *testing.T) {
		t.Parallel()
		d := cleanData(t, "user", "fenny")
		require.Equal(t, "fenny", d.Get("user"))
		require.False(t, d.dirty.Load())
	})

	t.Run("Get of a missing key stays clean", func(t *testing.T) {
		t.Parallel()
		d := cleanData(t, "user", "fenny")
		require.Nil(t, d.Get("absent"))
		require.False(t, d.dirty.Load())
	})

	t.Run("Get of an aliasable value marks dirty", func(t *testing.T) {
		t.Parallel()
		d := cleanData(t, "prefs", map[string]string{"theme": "dark"})
		require.NotNil(t, d.Get("prefs"))
		require.True(t, d.dirty.Load())
	})

	t.Run("Keys of scalar keys stays clean", func(t *testing.T) {
		t.Parallel()
		d := cleanData(t, "user", "fenny")
		require.Equal(t, []any{"user"}, d.Keys())
		require.False(t, d.dirty.Load())
	})

	t.Run("Keys of an aliasable key marks dirty", func(t *testing.T) {
		t.Parallel()
		key := new(int)
		d := cleanData(t, key, "fenny")
		require.Equal(t, []any{any(key)}, d.Keys())
		require.True(t, d.dirty.Load())
	})
}
