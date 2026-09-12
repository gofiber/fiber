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

func Test_Storage_Memory_Set(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func Test_Storage_Memory_SetWithContext(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := testStore.SetWithContext(ctx, key, val, 0)
	require.ErrorIs(t, err, context.Canceled)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
}

func Test_Storage_Memory_Set_Override(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	err = testStore.Set(key, val, 0)
	require.NoError(t, err)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func Test_Storage_Memory_Get(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, result)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func Test_Storage_Memory_GetWithContext(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := testStore.GetWithContext(ctx, key)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func Test_Storage_Memory_Set_Expiration(t *testing.T) {
	t.Parallel()
	var (
		testStore = New(Config{
			GCInterval: 300 * time.Millisecond,
		})
		key = "john"
		val = []byte("doe")
		exp = 1 * time.Second
	)

	err := testStore.Set(key, val, exp)
	require.NoError(t, err)

	// interval + expire + buffer
	time.Sleep(1500 * time.Millisecond)

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Empty(t, result)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
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

			keys, err := testStore.Keys()
			require.NoError(t, err)
			require.Len(t, keys, 1)
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
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
		exp       = 3 * time.Second
	)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)

	err = testStore.Set(key, val, exp)
	require.NoError(t, err)

	time.Sleep(1100 * time.Millisecond)

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)

	time.Sleep(4000 * time.Millisecond)
	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Empty(t, result)

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
}

func Test_Storage_Memory_Get_NotExist(t *testing.T) {
	t.Parallel()
	testStore := New()
	result, err := testStore.Get("notexist")
	require.NoError(t, err)
	require.Empty(t, result)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
}

func Test_Storage_Memory_Delete(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)

	err = testStore.Delete(key)
	require.NoError(t, err)

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Empty(t, result)

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
}

func Test_Storage_Memory_DeleteWithContext(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = testStore.DeleteWithContext(ctx, key)
	require.ErrorIs(t, err, context.Canceled)

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, result)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func Test_Storage_Memory_Reset(t *testing.T) {
	t.Parallel()
	testStore := New()
	val := []byte("doe")

	err := testStore.Set("john1", val, 0)
	require.NoError(t, err)

	err = testStore.Set("john2", val, 0)
	require.NoError(t, err)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 2)

	err = testStore.Reset()
	require.NoError(t, err)

	result, err := testStore.Get("john1")
	require.NoError(t, err)
	require.Empty(t, result)

	result, err = testStore.Get("john2")
	require.NoError(t, err)
	require.Empty(t, result)

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
}

func Test_Storage_Memory_ResetWithContext(t *testing.T) {
	t.Parallel()
	testStore := New()
	val := []byte("doe")

	err := testStore.Set("john1", val, 0)
	require.NoError(t, err)

	err = testStore.Set("john2", val, 0)
	require.NoError(t, err)

	keys, err := testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = testStore.ResetWithContext(ctx)
	require.ErrorIs(t, err, context.Canceled)

	result, err := testStore.Get("john1")
	require.NoError(t, err)
	require.Equal(t, val, result)

	result, err = testStore.Get("john2")
	require.NoError(t, err)
	require.Equal(t, val, result)

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func Test_Storage_Memory_Close(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NoError(t, testStore.Close())
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
		defer func() {
			gcPanicked <- recover()
		}()
		store.gc()
	}()

	done := make(chan error, 1)
	go func() {
		done <- store.Close()
	}()

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
		wg.Go(func() {
			errCh <- testStore.Close()
		})
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

// Benchmarks for Set operation
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
		err := testStore.Set("john", []byte("doe"), 0)
		require.NoError(b, err)
	}
}

func Benchmark_Memory_Set_Asserted_Parallel(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := testStore.Set("john", []byte("doe"), 0)
			require.NoError(b, err)
		}
	})
}

// Benchmarks for Get operation
func Benchmark_Memory_Get(b *testing.B) {
	testStore := New()
	err := testStore.Set("john", []byte("doe"), 0)
	require.NoError(b, err)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = testStore.Get("john") //nolint:errcheck // error not needed for benchmark
	}
}

func Benchmark_Memory_Get_Parallel(b *testing.B) {
	testStore := New()
	err := testStore.Set("john", []byte("doe"), 0)
	require.NoError(b, err)

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
	err := testStore.Set("john", []byte("doe"), 0)
	require.NoError(b, err)

	b.ReportAllocs()

	for b.Loop() {
		_, err := testStore.Get("john")
		require.NoError(b, err)
	}
}

func Benchmark_Memory_Get_Asserted_Parallel(b *testing.B) {
	testStore := New()
	err := testStore.Set("john", []byte("doe"), 0)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := testStore.Get("john")
			require.NoError(b, err)
		}
	})
}

// Benchmarks for SetAndDelete operation
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
