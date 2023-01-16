package memory

import (
	"context"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
)

var testStore = New()

func Test_Storage_Memory_Set(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(context.TODO(), key, val, 0)
	utils.AssertEqual(t, nil, err)
}

func Test_Storage_Memory_Set_Override(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(context.TODO(), key, val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Set(context.TODO(), key, val, 0)
	utils.AssertEqual(t, nil, err)
}

func Test_Storage_Memory_Get(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(context.TODO(), key, val, 0)
	utils.AssertEqual(t, nil, err)

	result, err := testStore.Get(context.TODO(), key)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, val, result)
}

func Test_Storage_Memory_Set_Expiration(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
		exp = 1 * time.Second
	)

	err := testStore.Set(context.TODO(), key, val, exp)
	utils.AssertEqual(t, nil, err)

	time.Sleep(1100 * time.Millisecond)
}

func Test_Storage_Memory_Get_Expired(t *testing.T) {
	var (
		key = "john"
	)

	result, err := testStore.Get(context.TODO(), key)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Storage_Memory_Get_NotExist(t *testing.T) {
	t.Parallel()

	result, err := testStore.Get(context.TODO(), "notexist")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Storage_Memory_Delete(t *testing.T) {
	t.Parallel()
	var (
		key = "john"
		val = []byte("doe")
	)

	err := testStore.Set(context.TODO(), key, val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Delete(context.TODO(), key)
	utils.AssertEqual(t, nil, err)

	result, err := testStore.Get(context.TODO(), key)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Storage_Memory_Reset(t *testing.T) {
	t.Parallel()
	var (
		val = []byte("doe")
	)

	err := testStore.Set(context.TODO(), "john1", val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Set(context.TODO(), "john2", val, 0)
	utils.AssertEqual(t, nil, err)

	err = testStore.Reset(context.TODO())
	utils.AssertEqual(t, nil, err)

	result, err := testStore.Get(context.TODO(), "john1")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)

	result, err = testStore.Get(context.TODO(), "john2")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, len(result) == 0)
}

func Test_Storage_Memory_Close(t *testing.T) {
	t.Parallel()
	utils.AssertEqual(t, nil, testStore.Close(context.TODO()))
}

func Test_Storage_Memory_Conn(t *testing.T) {
	t.Parallel()
	utils.AssertEqual(t, true, testStore.Conn() != nil)
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
				d.Set(context.TODO(), key, value, ttl)
			}
			for _, key := range keys {
				_, _ = d.Get(context.TODO(), key)
			}
			for _, key := range keys {
				d.Delete(context.TODO(), key)
			}
		}
	})
}
