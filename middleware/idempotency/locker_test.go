package idempotency_test

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/middleware/idempotency"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// go test -run Test_MemoryLock
func Test_MemoryLock(t *testing.T) {
	t.Parallel()

	l := idempotency.NewMemoryLock()

	{
		err := l.Lock("a")
		require.NoError(t, err)
	}
	{
		done := make(chan struct{})
		go func() {
			defer close(done)

			err := l.Lock("a")
			assert.NoError(t, err)
		}()

		select {
		case <-done:
			t.Fatal("lock acquired again")
		case <-time.After(time.Second):
		}
	}

	{
		err := l.Lock("b")
		require.NoError(t, err)
	}
	{
		err := l.Unlock("b")
		require.NoError(t, err)
	}
	{
		err := l.Lock("b")
		require.NoError(t, err)
	}

	{
		err := l.Unlock("c")
		require.NoError(t, err)
	}

	{
		err := l.Lock("d")
		require.NoError(t, err)
	}
}

func Benchmark_MemoryLock(b *testing.B) {
	keys := make([]string, b.N)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
	}

	lock := idempotency.NewMemoryLock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := keys[i]
		if err := lock.Lock(key); err != nil {
			b.Fatal(err)
		}
		if err := lock.Unlock(key); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_MemoryLock_Parallel(b *testing.B) {
	// In order to prevent using repeated keys I pre-allocate keys
	keys := make([]string, 1_000_000)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
	}

	b.Run("UniqueKeys", func(b *testing.B) {
		lock := idempotency.NewMemoryLock()
		var keyI atomic.Int32
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				i := int(keyI.Add(1)) % len(keys)
				key := keys[i]
				if err := lock.Lock(key); err != nil {
					b.Fatal(err)
				}
				if err := lock.Unlock(key); err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("Repeated3TimesKeys", func(b *testing.B) {
		lock := idempotency.NewMemoryLock()
		var keyI atomic.Int32
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				// Division by 3 ensures that index will be repreated exactly 3 times
				i := int(keyI.Add(1)) / 3 % len(keys)
				key := keys[i]
				if err := lock.Lock(key); err != nil {
					b.Fatal(err)
				}
				if err := lock.Unlock(key); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
