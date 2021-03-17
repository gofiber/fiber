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
		mux        = &sync.RWMutex{}
		timestamp  = uint64(time.Now().Unix())
		expiration = uint64(cfg.Expiration.Seconds())
	)
	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage)

	// Update timestamp every second
	go func() {
		for {
			atomic.StoreUint64(&timestamp, uint64(time.Now().Unix()))
			time.Sleep(1 * time.Second)
		}
	}()

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Only cache GET methods
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		// Get key from request
		key := cfg.KeyGenerator(c)

		// Get entry from pool
		e := manager.get(key)

		// Lock entry and unlock when finished
		mux.Lock()
		defer mux.Unlock()

		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		if e.exp != 0 && ts >= e.exp {
			// Check if entry is expired
			manager.delete(key)
			// External storage saves body data with different key
			if cfg.Storage != nil {
				manager.delete(key + "_body")
			}
		} else if e.exp != 0 {
			// Separate body value to avoid msgp serialization
			// We can store raw bytes with Storage 👍
			if cfg.Storage != nil {
				e.body = manager.getRaw(key + "_body")
			}
			// Set response headers from cache
			c.Response().SetBodyRaw(e.body)
			c.Response().SetStatusCode(e.status)
			c.Response().Header.SetContentTypeBytes(e.ctype)
			if len(e.cencoding) > 0 {
				c.Response().Header.SetBytesV(fiber.HeaderContentEncoding, e.cencoding)
			}
			// Set Cache-Control header if enabled
			if cfg.CacheControl {
				maxAge := strconv.FormatUint(e.exp-ts, 10)
				c.Set(fiber.HeaderCacheControl, "public, max-age="+maxAge)
			}

			// Return response
			return nil
		}

		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
		}

		// Don't cache response if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return nil
		}

		// Cache response
		e.body = utils.SafeBytes(c.Response().Body())
		e.status = c.Response().StatusCode()
		e.ctype = utils.SafeBytes(c.Response().Header.ContentType())
		e.cencoding = utils.SafeBytes(c.Response().Header.Peek(fiber.HeaderContentEncoding))
		e.exp = ts + expiration

		// For external Storage we store raw body seperated
		if cfg.Storage != nil {
			manager.setRaw(key+"_body", e.body, cfg.Expiration)
			// avoid body msgp encoding
			e.body = nil
			manager.set(key, e, cfg.Expiration)
			manager.release(e)
		} else {
			// Store entry in memory
			manager.set(key, e, cfg.Expiration)
		}

		// Finish response
		return nil
	}
}
