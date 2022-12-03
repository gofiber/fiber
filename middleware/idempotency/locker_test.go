package idempotency_test

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/middleware/idempotency"

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
			require.NoError(t, err)
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
