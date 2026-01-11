package session

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeys(t *testing.T) {
	t.Parallel()

	// Test case: Empty data
	t.Run("Empty data", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		keys := d.Keys()
		require.Empty(t, keys, "Expected no keys in empty data")
	})

	// Test case: Single key
	t.Run("Single key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		keys := d.Keys()
		require.Len(t, keys, 1, "Expected one key")
		require.Contains(t, keys, "key1", "Expected key1 to be present")
	})

	// Test case: Multiple keys
	t.Run("Multiple keys", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		d.Set("key2", "value2")
		d.Set("key3", "value3")
		keys := d.Keys()
		require.Len(t, keys, 3, "Expected three keys")
		require.Contains(t, keys, "key1", "Expected key1 to be present")
		require.Contains(t, keys, "key2", "Expected key2 to be present")
		require.Contains(t, keys, "key3", "Expected key3 to be present")
	})

	// Test case: Concurrent access
	t.Run("Concurrent access", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		d.Set("key2", "value2")
		d.Set("key3", "value3")

		done := make(chan bool)
		go func() {
			keys := d.Keys()
			assert.Len(t, keys, 3, "Expected three keys")
			done <- true
		}()
		go func() {
			keys := d.Keys()
			assert.Len(t, keys, 3, "Expected three keys")
			done <- true
		}()
		<-done
		<-done
	})
}

func TestData_Len(t *testing.T) {
	t.Parallel()

	// Test case: Empty data
	t.Run("Empty data", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		length := d.Len()
		require.Equal(t, 0, length, "Expected length to be 0 for empty data")
	})

	// Test case: Single key
	t.Run("Single key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		length := d.Len()
		require.Equal(t, 1, length, "Expected length to be 1 when one key is set")
	})

	// Test case: Multiple keys
	t.Run("Multiple keys", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		d.Set("key2", "value2")
		d.Set("key3", "value3")
		length := d.Len()
		require.Equal(t, 3, length, "Expected length to be 3 when three keys are set")
	})

	// Test case: Concurrent access
	t.Run("Concurrent access", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		d.Set("key2", "value2")
		d.Set("key3", "value3")

		done := make(chan bool, 2) // Buffered channel with size 2
		go func() {
			length := d.Len()
			assert.Equal(t, 3, length, "Expected length to be 3 during concurrent access")
			done <- true
		}()
		go func() {
			length := d.Len()
			assert.Equal(t, 3, length, "Expected length to be 3 during concurrent access")
			done <- true
		}()
		<-done
		<-done
	})
}

func TestData_Get(t *testing.T) {
	t.Parallel()

	// Test case: Nonexistent key
	t.Run("Nonexistent key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		value := d.Get("nonexistent-key")
		require.Nil(t, value, "Expected nil for nonexistent key")
	})

	// Test case: Existing key
	t.Run("Existing key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		value := d.Get("key1")
		require.Equal(t, "value1", value, "Expected value1 for key1")
	})
}

func TestData_Reset(t *testing.T) {
	t.Parallel()

	// Test case: Reset data
	t.Run("Reset data", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		d.Set("key1", "value1")
		d.Set("key2", "value2")
		d.Reset()
		require.Empty(t, d.Data, "Expected data map to be empty after reset")
	})
}

func mapPointer(m map[any]any) uintptr {
	return reflect.ValueOf(m).Pointer()
}

func TestData_ResetPreservesAllocation(t *testing.T) {
	t.Parallel()

	d := acquireData()
	d.Reset() // Ensure clean state from pool
	t.Cleanup(func() {
		d.Reset()
		dataPool.Put(d)
	})

	originalPtr := mapPointer(d.Data)

	d.Set("key1", "value1")
	d.Set("key2", "value2")
	require.Equal(t, originalPtr, mapPointer(d.Data), "Expected map pointer to stay constant after writes")

	d.Reset()
	require.Empty(t, d.Data, "Expected data map to be empty after reset")
	require.Equal(t, originalPtr, mapPointer(d.Data), "Expected reset to preserve underlying map")

	d.Set("key3", "value3")
	require.Nil(t, d.Get("key1"), "Expected cleared key not to leak after reset")
	require.Equal(t, originalPtr, mapPointer(d.Data), "Expected map pointer to remain stable after further writes")
}

func TestData_PoolReuseDoesNotLeakEntries(t *testing.T) {
	t.Parallel()

	acquired := make([]*data, 0, 6)
	t.Cleanup(func() {
		for _, item := range acquired {
			item.Reset()
			dataPool.Put(item)
		}
	})

	acquireWithCleanup := func() *data {
		d := acquireData()
		acquired = append(acquired, d)
		return d
	}

	first := acquireWithCleanup()
	first.Set("key1", "value1")
	first.Set("key2", "value2")
	first.Reset()

	originalPtr := mapPointer(first.Data)
	dataPool.Put(first)

	var reused *data
	for i := 0; i < 5; i++ {
		candidate := acquireWithCleanup()
		if mapPointer(candidate.Data) == originalPtr {
			reused = candidate
			break
		}
		require.Empty(t, candidate.Data, "Expected pooled data to be empty when new instance is returned")
		require.Nil(t, candidate.Get("key2"), "Expected no leakage of prior entries on alternate pooled instance")
	}

	if reused == nil {
		t.Skip("sync.Pool returned a different instance; reuse cannot be asserted")
		return
	}

	require.Equal(t, originalPtr, mapPointer(reused.Data), "Expected pooled data to reuse cleared map")
	require.Empty(t, reused.Data, "Expected pooled data to be empty after reuse")
	require.Nil(t, reused.Get("key2"), "Expected no leakage of prior entries on reuse")

	reused.Set("key4", "value4")
	require.Equal(t, "value4", reused.Get("key4"), "Expected pooled map to accept new values")
}

func TestData_Delete(t *testing.T) {
	t.Parallel()

	// Test case: Delete existing key
	t.Run("Delete existing key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Set("key1", "value1")
		d.Delete("key1")
		value := d.Get("key1")
		require.Nil(t, value, "Expected nil for deleted key")
	})

	// Test case: Delete nonexistent key
	t.Run("Delete nonexistent key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		d.Reset() // Ensure clean state from pool
		defer dataPool.Put(d)
		defer d.Reset()
		d.Delete("nonexistent-key")
		// No assertion needed, just ensure no panic or error
	})
}
