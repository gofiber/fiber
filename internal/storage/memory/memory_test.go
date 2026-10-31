package memory

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
)

// requireKeys requires store to hold exactly n keys; none at all reads as nil.
func requireKeys(t *testing.T, store *Storage, n int) {
	t.Helper()

	keys, err := store.Keys()
	require.NoError(t, err)
	if n == 0 {
		require.Nil(t, keys)
		return
	}
	require.Len(t, keys, n)
}

// requireValue requires store to hold want under key.
func requireValue(t *testing.T, store *Storage, key string, want []byte) {
	t.Helper()

	got, err := store.Get(key)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// requireMissing requires store to hold no value under key.
func requireMissing(t *testing.T, store *Storage, key string) {
	t.Helper()

	got, err := store.Get(key)
	require.NoError(t, err)
	require.Empty(t, got)
}

// canceledContext returns a context that is already canceled.
func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// Test_Storage_Memory_CanceledContext pins that a WithContext variant called
// with an already canceled context fails with the context's error and leaves
// the store as it found it.
func Test_Storage_Memory_CanceledContext(t *testing.T) {
	t.Parallel()
	val := []byte("doe")

	tests := []struct {
		call func(ctx context.Context, store *Storage) ([]byte, error)
		name string
		// seeded are the keys stored, with val, before the call.
		seeded []string
	}{
		{name: "SetWithContext", call: func(ctx context.Context, store *Storage) ([]byte, error) {
			return nil, store.SetWithContext(ctx, "john", val, 0)
		}},
		{name: "GetWithContext", call: func(ctx context.Context, store *Storage) ([]byte, error) {
			return store.GetWithContext(ctx, "john")
		}, seeded: []string{"john"}},
		{name: "DeleteWithContext", call: func(ctx context.Context, store *Storage) ([]byte, error) {
			return nil, store.DeleteWithContext(ctx, "john")
		}, seeded: []string{"john"}},
		{name: "ResetWithContext", call: func(ctx context.Context, store *Storage) ([]byte, error) {
			return nil, store.ResetWithContext(ctx)
		}, seeded: []string{"john1", "john2"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testStore := New()
			for _, key := range tc.seeded {
				require.NoError(t, testStore.Set(key, val, 0))
			}
			requireKeys(t, testStore, len(tc.seeded))

			result, err := tc.call(canceledContext(), testStore)
			require.ErrorIs(t, err, context.Canceled)
			require.Nil(t, result)

			for _, key := range tc.seeded {
				requireValue(t, testStore, key, val)
			}
			requireKeys(t, testStore, len(tc.seeded))
		})
	}
}

func Test_Storage_Memory_Set_Override(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NoError(t, testStore.Set("john", []byte("doe"), 0))
	require.NoError(t, testStore.Set("john", []byte("doe"), 0))
	requireKeys(t, testStore, 1)
}

func Test_Storage_Memory_Get(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NoError(t, testStore.Set("john", []byte("doe"), 0))
	requireValue(t, testStore, "john", []byte("doe"))
	requireKeys(t, testStore, 1)
}

func Test_Storage_Memory_Set_Expiration(t *testing.T) {
	t.Parallel()
	testStore := New(Config{GCInterval: 300 * time.Millisecond})
	require.NoError(t, testStore.Set("john", []byte("doe"), 1*time.Second))

	// interval + expire + buffer
	time.Sleep(1500 * time.Millisecond)
	requireMissing(t, testStore, "john")
	requireKeys(t, testStore, 0)
}

// setWithinSecond stores key while the cached clock holds still.
func setWithinSecond(t *testing.T, store *Storage, key string, val []byte, exp time.Duration) uint32 {
	t.Helper()
	for range 100 {
		now := utils.Timestamp()
		require.NoError(t, store.Set(key, val, exp))
		if utils.Timestamp() == now {
			return now
		}
	}
	t.Fatal("cached clock kept ticking during Set")
	return 0
}

// expiryOf returns the whole-second expiry stored for key.
func expiryOf(store *Storage, key string) uint32 {
	store.mux.RLock()
	defer store.mux.RUnlock()
	return store.db[key].expiry
}

func Test_Storage_Memory_Set_ExpirationRoundsUp(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		exp  time.Duration
		// want is the expiry in seconds after the current one; 0 keeps it forever.
		want uint32
	}{
		{name: "sub-second rounds up to one second", exp: 500 * time.Millisecond, want: 1},
		{name: "whole seconds are kept", exp: time.Second, want: 1},
		{name: "fractions round up rather than down", exp: 1900 * time.Millisecond, want: 2},
		{name: "zero keeps the entry forever", exp: 0, want: 0},
		{name: "negative keeps the entry forever", exp: -time.Second, want: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testStore := New()
			now := setWithinSecond(t, testStore, "john", []byte("doe"), tc.exp)

			want := tc.want
			if want != 0 {
				want += now
			}
			require.Equal(t, want, expiryOf(testStore, "john"))

			result, err := testStore.Get("john")
			require.NoError(t, err)
			require.Equal(t, []byte("doe"), result, "a positive expiration must not be stored already expired")

			requireKeys(t, testStore, 1)
		})
	}
}

func Test_Storage_Memory_GCInterval_SubSecond(t *testing.T) {
	t.Parallel()
	require.Equal(t, 100*time.Millisecond, configDefault(Config{GCInterval: 100 * time.Millisecond}).GCInterval)
	require.Equal(t, ConfigDefault.GCInterval, configDefault(Config{}).GCInterval)
	require.Equal(t, ConfigDefault.GCInterval, configDefault(Config{GCInterval: -time.Second}).GCInterval)
	require.Equal(t, ConfigDefault.GCInterval, configDefault().GCInterval)

	testStore := New(Config{GCInterval: 100 * time.Millisecond})
	require.Equal(t, 100*time.Millisecond, testStore.gcInterval)
	require.NoError(t, testStore.Set("john", []byte("doe"), time.Second))

	// The collector, not just Get's own expiry check, must drop the entry.
	require.Eventually(t, func() bool {
		testStore.mux.RLock()
		defer testStore.mux.RUnlock()
		_, ok := testStore.db["john"]
		return !ok
	}, 5*time.Second, 50*time.Millisecond)
}

func Test_Storage_Memory_Set_Long_Expiration_with_Keys(t *testing.T) {
	t.Parallel()
	testStore := New()
	requireKeys(t, testStore, 0)
	require.NoError(t, testStore.Set("john", []byte("doe"), 3*time.Second))

	time.Sleep(1100 * time.Millisecond)
	requireKeys(t, testStore, 1)

	time.Sleep(4000 * time.Millisecond)
	requireMissing(t, testStore, "john")
	requireKeys(t, testStore, 0)
}

func Test_Storage_Memory_Get_NotExist(t *testing.T) {
	t.Parallel()
	testStore := New()
	requireMissing(t, testStore, "notexist")
	requireKeys(t, testStore, 0)
}

func Test_Storage_Memory_Delete(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NoError(t, testStore.Set("john", []byte("doe"), 0))
	requireKeys(t, testStore, 1)
	require.NoError(t, testStore.Delete("john"))
	requireMissing(t, testStore, "john")
	requireKeys(t, testStore, 0)
}

func Test_Storage_Memory_Reset(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NoError(t, testStore.Set("john1", []byte("doe"), 0))
	require.NoError(t, testStore.Set("john2", []byte("doe"), 0))
	requireKeys(t, testStore, 2)
	require.NoError(t, testStore.Reset())
	requireMissing(t, testStore, "john1")
	requireMissing(t, testStore, "john2")
	requireKeys(t, testStore, 0)
}

// Test_Storage_Memory_Close_GCPanic verifies that Close does not deadlock if
// gc() panics during initialization (e.g. NewTicker with a non-positive
// interval). The defer for gcExited must run before any code that can panic.
func Test_Storage_Memory_Close_GCPanic(t *testing.T) {
	t.Parallel()
	store := &Storage{
		db:         make(map[string]Entry),
		gcInterval: 0, // NewTicker panics on non-positive duration
		done:       make(chan struct{}),
		gcExited:   make(chan struct{}),
	}

	// Launch gc directly; recover the expected panic so the test does not crash.
	gcPanicked := make(chan any, 1)
	go func() {
		defer func() { gcPanicked <- recover() }()
		store.gc()
	}()

	done := make(chan error, 1)
	go func() { done <- store.Close() }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Close deadlocked after gc panic")
	}

	require.NotNil(t, <-gcPanicked, "expected gc() to panic on zero gcInterval")
}

func Test_Storage_Memory_Close_Idempotent(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NoError(t, testStore.Close())

	// After the first Close returns, the gc goroutine must have exited.
	select {
	case <-testStore.gcExited:
	default:
		t.Fatal("gc goroutine still running after Close returned")
	}

	// Subsequent concurrent Close calls must neither block nor panic.
	var wg sync.WaitGroup
	errCh := make(chan error, 4)
	for range 4 {
		wg.Go(func() { errCh <- testStore.Close() })
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errCh)
		for err := range errCh {
			require.NoError(t, err)
		}
	case <-time.After(time.Second):
		t.Fatal("concurrent Close blocked")
	}
}

func Test_Storage_Memory_Conn(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NotNil(t, testStore.Conn())
}

func Benchmark_Memory_Set(b *testing.B) {
	testStore := New()
	b.ReportAllocs()

	for b.Loop() {
		_ = testStore.Set("john", []byte("doe"), 0) //nolint:errcheck // error not needed for benchmark
	}
}

func Benchmark_Memory_Set_Parallel(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = testStore.Set("john", []byte("doe"), 0) //nolint:errcheck // error not needed for benchmark
		}
	})
}

func Benchmark_Memory_Set_Asserted(b *testing.B) {
	testStore := New()
	b.ReportAllocs()

	for b.Loop() {
		require.NoError(b, testStore.Set("john", []byte("doe"), 0))
	}
}

func Benchmark_Memory_Set_Asserted_Parallel(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			require.NoError(b, testStore.Set("john", []byte("doe"), 0))
		}
	})
}

func Benchmark_Memory_Get(b *testing.B) {
	testStore := New()
	require.NoError(b, testStore.Set("john", []byte("doe"), 0))
	b.ReportAllocs()

	for b.Loop() {
		_, _ = testStore.Get("john") //nolint:errcheck // error not needed for benchmark
	}
}

func Benchmark_Memory_Get_Parallel(b *testing.B) {
	testStore := New()
	require.NoError(b, testStore.Set("john", []byte("doe"), 0))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = testStore.Get("john") //nolint:errcheck // error not needed for benchmark
		}
	})
}

func Benchmark_Memory_Get_Asserted(b *testing.B) {
	testStore := New()
	require.NoError(b, testStore.Set("john", []byte("doe"), 0))
	b.ReportAllocs()

	for b.Loop() {
		_, err := testStore.Get("john")
		require.NoError(b, err)
	}
}

func Benchmark_Memory_Get_Asserted_Parallel(b *testing.B) {
	testStore := New()
	require.NoError(b, testStore.Set("john", []byte("doe"), 0))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := testStore.Get("john")
			require.NoError(b, err)
		}
	})
}

func Benchmark_Memory_SetAndDelete(b *testing.B) {
	testStore := New()
	b.ReportAllocs()

	for b.Loop() {
		_ = testStore.Set("john", []byte("doe"), 0) //nolint:errcheck // error not needed for benchmark
		_ = testStore.Delete("john")                //nolint:errcheck // error not needed for benchmark
	}
}

func Benchmark_Memory_SetAndDelete_Parallel(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = testStore.Set("john", []byte("doe"), 0) //nolint:errcheck // error not needed for benchmark
			_ = testStore.Delete("john")                //nolint:errcheck // error not needed for benchmark
		}
	})
}

func Benchmark_Memory_SetAndDelete_Asserted(b *testing.B) {
	testStore := New()
	b.ReportAllocs()

	for b.Loop() {
		err := testStore.Set("john", []byte("doe"), 0)
		require.NoError(b, err)

		err = testStore.Delete("john")
		require.NoError(b, err)
	}
}

func Benchmark_Memory_SetAndDelete_Asserted_Parallel(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := testStore.Set("john", []byte("doe"), 0)
			require.NoError(b, err)

			err = testStore.Delete("john")
			require.NoError(b, err)
		}
	})
}

func Test_Storage_Memory_CeilSecondsSaturates(t *testing.T) {
	t.Parallel()
	// Rounding must not overflow into a wrapped, tiny expiration.
	require.Equal(t, uint32(math.MaxUint32), ceilSeconds(time.Duration(math.MaxInt64)))
	require.Equal(t, uint32(math.MaxUint32), ceilSeconds(time.Duration(math.MaxInt64)-1))
	require.Equal(t, uint32(1), ceilSeconds(time.Nanosecond))
	require.Equal(t, uint32(2), ceilSeconds(time.Second+time.Nanosecond))
}

func Test_Storage_Memory_MaxExpirationDoesNotWrap(t *testing.T) {
	t.Parallel()
	store := New()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.Set("max", []byte("v"), time.Duration(math.MaxInt64)))

	// The absolute expiry must saturate rather than wrap past the current
	// timestamp, which would expire an effectively infinite TTL at once.
	got, err := store.Get("max")
	require.NoError(t, err)
	require.Equal(t, []byte("v"), got)
	require.Equal(t, uint32(math.MaxUint32), expiryOf(store, "max"))
}
