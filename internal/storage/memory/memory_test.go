package memory

import (
	"context"
	"sync"
	"testing"
	"time"

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
