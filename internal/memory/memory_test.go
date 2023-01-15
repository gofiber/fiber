package memory

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_Memory -v -race

func Test_Memory(t *testing.T) {
	t.Parallel()
	var store = New()
	var (
		key             = "john"
		val interface{} = []byte("doe")
		exp             = 1 * time.Second
	)

	store.Set(key, val, 0)
	store.Set(key, val, 0)

	result := store.Get(key)
	utils.AssertEqual(t, val, result)

	result = store.Get("empty")
	utils.AssertEqual(t, nil, result)

	store.Set(key, val, exp)
	time.Sleep(1100 * time.Millisecond)

	result = store.Get(key)
	utils.AssertEqual(t, nil, result)

	store.Set(key, val, 0)
	result = store.Get(key)
	utils.AssertEqual(t, val, result)

	store.Delete(key)
	result = store.Get(key)
	utils.AssertEqual(t, nil, result)

	store.Set("john", val, 0)
	store.Set("doe", val, 0)
	store.Reset()

	result = store.Get("john")
	utils.AssertEqual(t, nil, result)

	result = store.Get("doe")
	utils.AssertEqual(t, nil, result)
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
