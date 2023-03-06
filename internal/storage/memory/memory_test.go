package memory

import (
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
)

var testStore = New()

func Test_Storage_Memory_Set(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)
}

func Test_Storage_Memory_Set_Override(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	err = testStore.Set(key, val, 0)
	require.NoError(t, err)
}

func Test_Storage_Memory_Get(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Equal(t, val, result)
}

func Test_Storage_Memory_Set_Expiration(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
		exp = 1 * time.Second
	)

	err := testStore.Set(key, val, exp)
	require.NoError(t, err)

	time.Sleep(1100 * time.Millisecond)
}

func Test_Storage_Memory_Get_Expired(t *testing.T) {
	key := "john"

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Equal(t, true, len(result) == 0)
}

func Test_Storage_Memory_Get_NotExist(t *testing.T) {
	t.Parallel()

	result, err := testStore.Get("notexist")
	require.NoError(t, err)
	require.Equal(t, true, len(result) == 0)
}

func Test_Storage_Memory_Delete(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	require.NoError(t, err)

	err = testStore.Delete(key)
	require.NoError(t, err)

	result, err := testStore.Get(key)
	require.NoError(t, err)
	require.Equal(t, true, len(result) == 0)
}

func Test_Storage_Memory_Reset(t *testing.T) {
	t.Parallel()
	val := []byte("doe")

	err := testStore.Set("john1", val, 0)
	require.NoError(t, err)

	err = testStore.Set("john2", val, 0)
	require.NoError(t, err)

	err = testStore.Reset()
	require.NoError(t, err)

	result, err := testStore.Get("john1")
	require.NoError(t, err)
	require.Equal(t, true, len(result) == 0)

	result, err = testStore.Get("john2")
	require.NoError(t, err)
	require.Equal(t, true, len(result) == 0)
}

func Test_Storage_Memory_Close(t *testing.T) {
	t.Parallel()
	require.NoError(t, testStore.Close())
}

func Test_Storage_Memory_Conn(t *testing.T) {
	t.Parallel()
	require.True(t, testStore.Conn() != nil)
}

// go test -v -run=^$ -bench=Benchmark_Storage_Memory -benchmem -count=4
func Benchmark_Storage_Memory(b *testing.B) {
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
				_, _ = d.Get(key)
			}
			for _, key := range keys {
				d.Delete(key)
			}
		}
	})
}
