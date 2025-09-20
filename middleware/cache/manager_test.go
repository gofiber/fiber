package cache

import (
	"context"
	"testing"
	"time"

	utils "github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_manager_get(t *testing.T) {
	t.Parallel()
	cacheManager := newManager(nil)
	t.Run("Item not found in cache", func(t *testing.T) {
		t.Parallel()
		it, err := cacheManager.get(context.Background(), utils.UUID())
		require.ErrorIs(t, err, errCacheMiss)
		assert.Nil(t, it)
	})
	t.Run("Item found in cache", func(t *testing.T) {
		t.Parallel()
		id := utils.UUID()
		cacheItem := cacheManager.acquire()
		cacheItem.body = []byte("test-body")
		require.NoError(t, cacheManager.set(context.Background(), id, cacheItem, 10*time.Second))
		it, err := cacheManager.get(context.Background(), id)
		require.NoError(t, err)
		assert.NotNil(t, it)
	})
}
