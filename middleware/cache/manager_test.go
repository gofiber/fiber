package cache

import (
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
)

func Test_manager_get(t *testing.T) {
	t.Parallel()
	cacheManager := newManager(nil)
	t.Run("Item not found in cache", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, cacheManager.get(utils.UUID()))
	})
	t.Run("Item found in cache", func(t *testing.T) {
		t.Parallel()
		id := utils.UUID()
		cacheItem := cacheManager.acquire()
		cacheItem.body = []byte("test-body")
		cacheManager.set(id, cacheItem, 10*time.Second)
		assert.NotNil(t, cacheManager.get(id))
	})
}
