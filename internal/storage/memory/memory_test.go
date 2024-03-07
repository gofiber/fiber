package memory

import (
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

func Test_Storage_Memory_Set_Expiration(t *testing.T) {
	t.Parallel()
	var (
		testStore = New()
		key       = "john"
		val       = []byte("doe")
		exp       = 1 * time.Second
	)

	err := testStore.Set(key, val, exp)
	require.NoError(t, err)

	time.Sleep(1100 * time.Millisecond)

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Zero(t, len(result))

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
		exp       = 5 * time.Second
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
	require.Zero(t, len(result))

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
}

func Test_Storage_Memory_Get_NotExist(t *testing.T) {
	t.Parallel()
	testStore := New()
	result, err := testStore.Get("notexist")
	require.NoError(t, err)
	require.Zero(t, len(result))

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
	require.Zero(t, len(result))

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
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
	require.Zero(t, len(result))

	result, err = testStore.Get("john2")
	require.NoError(t, err)
	require.Zero(t, len(result))

	keys, err = testStore.Keys()
	require.NoError(t, err)
	require.Nil(t, keys)
}

func Test_Storage_Memory_Close(t *testing.T) {
	t.Parallel()
	testStore := New()
	require.NoError(t, testStore.Close())
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
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = testStore.Set("john", []byte("doe"), 0) //nolint: errcheck // error not needed for benchmark
	}
}

func Benchmark_Memory_Set_Parallel(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = testStore.Set("john", []byte("doe"), 0) //nolint: errcheck // error not needed for benchmark
		}
	})
}

func Benchmark_Memory_Set_Asserted(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = testStore.Get("john") //nolint: errcheck // error not needed for benchmark
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
			_, _ = testStore.Get("john") //nolint: errcheck // error not needed for benchmark
		}
	})
}

func Benchmark_Memory_Get_Asserted(b *testing.B) {
	testStore := New()
	err := testStore.Set("john", []byte("doe"), 0)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = testStore.Set("john", []byte("doe"), 0) //nolint: errcheck // error not needed for benchmark
		_ = testStore.Delete("john")                //nolint: errcheck // error not needed for benchmark
	}
}

func Benchmark_Memory_SetAndDelete_Parallel(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = testStore.Set("john", []byte("doe"), 0) //nolint: errcheck // error not needed for benchmark
			_ = testStore.Delete("john")                //nolint: errcheck // error not needed for benchmark
		}
	})
}

func Benchmark_Memory_SetAndDelete_Asserted(b *testing.B) {
	testStore := New()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
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
