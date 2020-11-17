// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Nothing to cache
	if int(cfg.Expiration.Seconds()) < 0 {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	var (
		// Cache settings
		timestamp  = uint64(time.Now().Unix())
		expiration = uint64(cfg.Expiration.Seconds())
	)

	// create storage handler
	store := &storage{
		cfg:     &cfg,
		mux:     &sync.RWMutex{},
		entries: make(map[string]*entry),
	}

	// Update timestamp every second
	go func() {
		for {
			atomic.StoreUint64(&timestamp, uint64(time.Now().Unix()))
			time.Sleep(750 * time.Millisecond)
		}
	}()

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Only cache GET methods
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		// Get key from request
		key := cfg.KeyGenerator(c)

		// Get/Create new entry
		var entry = store.get(key)

		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Set expiration if entry does not exist
		if entry.exp == 0 {
			entry.exp = ts + expiration

		} else if ts >= entry.exp {
			// Check if entry is expired
			store.delete(key)
		} else {
			// Set response headers from cache
			c.Send(entry.body)
			c.Status(entry.status)
			c.Response().Header.SetContentTypeBytes(entry.cType)

			// Set Cache-Control header if enabled
			if cfg.CacheControl {
				maxAge := strconv.FormatUint(entry.exp-ts, 10)
				c.Set(fiber.HeaderCacheControl, "public, max-age="+maxAge)
			}

			// Return response
			return nil
		}

		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
		}

		// Cache response
		entry.status = c.Response().StatusCode()
		entry.body = utils.CopyBytes(c.Response().Body())
		entry.cType = utils.CopyBytes(c.Response().Header.ContentType())

		store.set(key, entry)

		// Finish response
		return nil
	}
}
