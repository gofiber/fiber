package session

import (
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
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		keys := d.Keys()
		require.Empty(t, keys, "Expected no keys in empty data")
	})

	// Test case: Single key
	t.Run("Single key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		d.Set("key1", "value1")
		keys := d.Keys()
		require.Len(t, keys, 1, "Expected one key")
		require.Contains(t, keys, "key1", "Expected key1 to be present")
	})

	// Test case: Multiple keys
	t.Run("Multiple keys", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
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
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
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
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		length := d.Len()
		require.Equal(t, 0, length, "Expected length to be 0 for empty data")
	})

	// Test case: Single key
	t.Run("Single key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		d.Set("key1", "value1")
		length := d.Len()
		require.Equal(t, 1, length, "Expected length to be 1 when one key is set")
	})

	// Test case: Multiple keys
	t.Run("Multiple keys", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
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
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		d.Set("key1", "value1")
		d.Set("key2", "value2")
		d.Set("key3", "value3")

		done := make(chan bool)
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

	// Test case: Non-existent key
	t.Run("Non-existent key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		value := d.Get("non-existent-key")
		require.Nil(t, value, "Expected nil for non-existent key")
	})

	// Test case: Existing key
	t.Run("Existing key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
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
		defer dataPool.Put(d)
		d.Set("key1", "value1")
		d.Set("key2", "value2")
		d.Reset()
		require.Empty(t, d.Data, "Expected data map to be empty after reset")
	})
}

func TestData_Delete(t *testing.T) {
	t.Parallel()

	// Test case: Delete existing key
	t.Run("Delete existing key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		d.Set("key1", "value1")
		d.Delete("key1")
		value := d.Get("key1")
		require.Nil(t, value, "Expected nil for deleted key")
	})

	// Test case: Delete non-existent key
	t.Run("Delete non-existent key", func(t *testing.T) {
		t.Parallel()
		d := acquireData()
		defer dataPool.Put(d)
		d.Reset() // Ensure data is reset
		d.Delete("non-existent-key")
		// No assertion needed, just ensure no panic or error
	})
}
