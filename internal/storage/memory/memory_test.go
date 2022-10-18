package memory

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
)

var testStore = New()

func Test_Memory_Set(t *testing.T) {
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	utils.AssertEqual(t, nil, err)
}

func Test_Memory_Set_Override(t *testing.T) {
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Set(key, val, 0)
	utils.AssertEqual(t, nil, err)
}

func Test_Memory_Get(t *testing.T) {
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	utils.AssertEqual(t, nil, err)

	result, err := testStore.Get(key)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, val, result)
}

func Test_Memory_Set_Expiration(t *testing.T) {
	var (
		key = "john"
		val = []byte("doe")
		exp = 1 * time.Second
	)

	err := testStore.Set(key, val, exp)
	utils.AssertEqual(t, nil, err)

	time.Sleep(1100 * time.Millisecond)
}

func Test_Memory_Get_Expired(t *testing.T) {
	var (
		key = "john"
	)

	result, err := testStore.Get(key)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Memory_Get_NotExist(t *testing.T) {

	result, err := testStore.Get("notexist")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Memory_Delete(t *testing.T) {
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(key, val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Delete(key)
	utils.AssertEqual(t, nil, err)

	result, err := testStore.Get(key)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Memory_Reset(t *testing.T) {
	var (
		val = []byte("doe")
	)

	err := testStore.Set("john1", val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Set("john2", val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Reset()
	utils.AssertEqual(t, nil, err)

	result, err := testStore.Get("john1")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)

	result, err = testStore.Get("john2")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Memory_Close(t *testing.T) {
	utils.AssertEqual(t, nil, testStore.Close())
}

func Test_Memory_Conn(t *testing.T) {
	utils.AssertEqual(t, true, testStore.Conn() != nil)
}

// go test -run Test_Memory -v -race

func Test_Memory(t *testing.T) {
	var store = New()
	var (
		key = "john"
		val = []byte("doe")
		exp = 1 * time.Second
	)

	store.Set(key, val, 0)

	result, error := store.Get(key)
	utils.AssertEqual(t, val, result)
	utils.AssertEqual(t, nil, error)

	result, error = store.Get("empty")
	utils.AssertEqual(t, nil, result)
	utils.AssertEqual(t, nil, error)

	store.Set(key, val, exp)
	time.Sleep(1100 * time.Millisecond)

	result, error = store.Get(key)
	utils.AssertEqual(t, nil, result)
	utils.AssertEqual(t, nil, error)

	store.Set(key, val, 0)
	result, error = store.Get(key)
	utils.AssertEqual(t, val, result)
	utils.AssertEqual(t, nil, error)

	store.Delete(key)
	result, error = store.Get(key)
	utils.AssertEqual(t, nil, result)
	utils.AssertEqual(t, nil, error)

	store.Set("john", val, 0)
	store.Set("doe", val, 0)
	store.Reset()

	result, error = store.Get("john")
	utils.AssertEqual(t, nil, result)
	utils.AssertEqual(t, nil, error)

	result, error = store.Get("doe")
	utils.AssertEqual(t, nil, result)
	utils.AssertEqual(t, nil, error)

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
				_, _ = d.Get(key)
			}
			for _, key := range keys {
				d.Delete(key)
			}
		}
	})
}
