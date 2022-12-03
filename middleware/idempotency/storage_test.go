package idempotency_test

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/middleware/idempotency"

	"github.com/stretchr/testify/require"
)

// go test -run Test_MemoryStorage
func Test_MemoryStorage(t *testing.T) {
	t.Parallel()

	s := idempotency.NewMemoryStorage(time.Minute)

	{
		val, err := s.Get("a")
		require.NoError(t, err)
		require.Nil(t, val)
	}
	{
		err := s.Set("a", []byte("hello a"), time.Millisecond)
		require.NoError(t, err)
	}
	{
		val, err := s.Get("a")
		require.NoError(t, err)
		require.Equal(t, []byte("hello a"), val)
	}

	{
		err := s.Set("b", []byte("hello b 1"), time.Millisecond)
		require.NoError(t, err)
	}
	{
		val, err := s.Get("b")
		require.NoError(t, err)
		require.Equal(t, []byte("hello b 1"), val)
	}
	{
		err := s.Set("b", []byte("hello b 2"), 10*time.Millisecond)
		require.NoError(t, err)
	}

	time.Sleep(2 * time.Millisecond)

	{
		val, err := s.Get("a")
		require.NoError(t, err)
		require.Nil(t, val)
	}

	{
		val, err := s.Get("b")
		require.NoError(t, err)
		require.Equal(t, []byte("hello b 2"), val)
	}
}
