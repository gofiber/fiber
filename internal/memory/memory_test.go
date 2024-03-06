package memory

import (
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
)

// go test -run Test_Memory -v -race
func Test_Memory(t *testing.T) {
	t.Parallel()
	store := New()
	var (
		key     = "john-internal"
		val any = []byte("doe")
		exp     = 1 * time.Second
	)

	// Set key with value
	store.Set(key, val, 0)
	result := store.Get(key)
	require.Equal(t, val, result)

	// Get non-existing key
	result = store.Get("empty")
	require.Nil(t, result)

	// Set key with value and ttl
	store.Set(key, val, exp)
	time.Sleep(1100 * time.Millisecond)
	result = store.Get(key)
	require.Nil(t, result)

	// Set key with value and no expiration
	store.Set(key, val, 0)
	result = store.Get(key)
	require.Equal(t, val, result)

	// Delete key
	store.Delete(key)
	result = store.Get(key)
	require.Nil(t, result)

	// Reset all keys
	store.Set("john-reset", val, 0)
	store.Set("doe-reset", val, 0)
	store.Reset()

	// Check if all keys are deleted
	result = store.Get("john-reset")
	require.Nil(t, result)
	result = store.Get("doe-reset")
	require.Nil(t, result)
}

// go test -v -run=^$ -bench=Benchmark_Memory -benchmem -count=4
func Benchmark_Memory(b *testing.B) {
	keyLength := 1000
	keys := make([]string, keyLength)
	for i := 0; i < keyLength; i++ {
		keys[i] = utils.UUID()
	}
	value := []byte("joe")

	ttl := 2 * time.Second
	b.Run("fiber_memory", func(b *testing.B) {
		d := New()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			for _, key := range keys {
				d.Set(key, value, ttl)
			}
			for _, key := range keys {
				_ = d.Get(key)
			}
			for _, key := range keys {
				d.Delete(key)
			}
		}
	})
}
