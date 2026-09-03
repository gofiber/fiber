package memory

import (
	"math"
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
	require.Eventually(t, func() bool {
		return store.Get(key) == nil
	}, 3*time.Second, 10*time.Millisecond)

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

// setWithinSecond stores key while the cached clock holds still.
func setWithinSecond(t *testing.T, store *Storage, key string, val any, ttl time.Duration) uint32 {
	t.Helper()
	for range 100 {
		now := utils.Timestamp()
		store.Set(key, val, ttl)
		if utils.Timestamp() == now {
			return now
		}
	}
	t.Fatal("cached clock kept ticking during Set")
	return 0
}

// expiryOf returns the whole-second expiry stored for key.
func expiryOf(store *Storage, key string) uint32 {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.data[key].e
}

func Test_Memory_TTLRoundsUp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		ttl  time.Duration
		// want is the expiry in seconds after the current one; 0 keeps it forever.
		want uint32
	}{
		{name: "sub-second rounds up to one second", ttl: 500 * time.Millisecond, want: 1},
		{name: "whole seconds are kept", ttl: time.Second, want: 1},
		{name: "fractions round up rather than down", ttl: 1900 * time.Millisecond, want: 2},
		{name: "zero keeps the entry forever", ttl: 0, want: 0},
		{name: "negative keeps the entry forever", ttl: -time.Second, want: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := New()
			now := setWithinSecond(t, store, "key", "value", tc.ttl)

			want := tc.want
			if want != 0 {
				want += now
			}
			require.Equal(t, want, expiryOf(store, "key"))
			require.Equal(t, "value", store.Get("key"), "a positive ttl must not be stored already expired")
		})
	}
}

func Test_Memory_SubSecondTTLExpires(t *testing.T) {
	t.Parallel()

	store := New()
	store.Set("key", "value", 500*time.Millisecond)
	require.Equal(t, "value", store.Get("key"))
	require.Eventually(t, func() bool {
		return store.Get("key") == nil
	}, 3*time.Second, 10*time.Millisecond)
}

// go test -v -run=^$ -bench=Benchmark_Memory -benchmem -count=4
func Benchmark_Memory(b *testing.B) {
	keyLength := 1000
	keys := make([]string, keyLength)
	for i := range keyLength {
		keys[i] = utils.UUIDv4()
	}
	value := []byte("joe")

	ttl := 2 * time.Second
	b.Run("fiber_memory", func(b *testing.B) {
		d := New()
		b.ReportAllocs()
		for b.Loop() {
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

func Test_Memory_CeilSecondsSaturates(t *testing.T) {
	t.Parallel()

	// Rounding must not overflow into a wrapped, tiny TTL.
	require.Equal(t, uint32(math.MaxUint32), ceilSeconds(time.Duration(math.MaxInt64)))
	require.Equal(t, uint32(math.MaxUint32), ceilSeconds(time.Duration(math.MaxInt64)-1))
	require.Equal(t, uint32(1), ceilSeconds(time.Nanosecond))
	require.Equal(t, uint32(2), ceilSeconds(time.Second+time.Nanosecond))
}

func Test_Memory_MaxTTLDoesNotWrap(t *testing.T) {
	t.Parallel()

	store := New()
	store.Set("max", "v", time.Duration(math.MaxInt64))

	// The absolute expiry must saturate rather than wrap past the current
	// timestamp, which would expire an effectively infinite TTL at once.
	require.Equal(t, "v", store.Get("max"))
	require.Equal(t, uint32(math.MaxUint32), expiryOf(store, "max"))
}
