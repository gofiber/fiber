package cache

import (
	"context"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_manager_get(t *testing.T) {
	t.Parallel()
	cacheManager := newManager(nil, true)
	t.Run("Item not found in cache", func(t *testing.T) {
		t.Parallel()
		it, err := cacheManager.get(context.Background(), utils.UUIDv4())
		require.ErrorIs(t, err, errCacheMiss)
		assert.Nil(t, it)
	})
	t.Run("Item found in cache", func(t *testing.T) {
		t.Parallel()
		id := utils.UUIDv4()
		cacheItem := cacheManager.acquire()
		cacheItem.body = []byte("test-body")
		require.NoError(t, cacheManager.set(context.Background(), id, cacheItem, 10*time.Second))
		it, err := cacheManager.get(context.Background(), id)
		require.NoError(t, err)
		assert.NotNil(t, it)
	})
}

func Test_manager_logKey(t *testing.T) {
	t.Parallel()

	redactedManager := newManager(nil, true)
	assert.Equal(t, redactedKey, redactedManager.logKey("secret"))

	plainManager := newManager(nil, false)
	assert.Equal(t, "secret", plainManager.logKey("secret"))
}

// Test_manager_release_ResetsEveryField pins that an item handed back to the
// pool carries nothing from its last use: a field left set would surface in
// whatever entry the pooled item is reused for next.
func Test_manager_release_ResetsEveryField(t *testing.T) {
	t.Parallel()

	m := newManager(memory.New(), true)
	e := m.acquire()
	*e = populatedItem()
	m.release(e)
	require.Equal(t, item{}, *e)
}
