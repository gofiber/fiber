// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Expiration is the time that an cached response will live
	//
	// Optional. Default: 5 * time.Minute
	Expiration time.Duration

	// CacheControl enables client side caching if set to true
	//
	// Optional. Default: false
	CacheControl bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	Expiration:   5 * time.Minute,
	CacheControl: false,
}

// cache is the manager to store the cached responses
type cache struct {
	sync.RWMutex
	entries    map[string]entry
	expiration int64
}

// entry defines the cached response
type entry struct {
	body        []byte
	contentType []byte
	statusCode  int
	expiration  int64
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.Next == nil {
			cfg.Next = ConfigDefault.Next
		}
		if int(cfg.Expiration.Seconds()) == 0 {
			cfg.Expiration = ConfigDefault.Expiration
		}
	}

	// Nothing to cache
	if int(cfg.Expiration.Seconds()) < 0 {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	// Initialize db
	db := &cache{
		entries:    make(map[string]entry),
		expiration: int64(cfg.Expiration.Seconds()),
	}
	// Remove expired entries
	go func() {
		for {
			// GC the entries every 10 seconds to avoid
			time.Sleep(10 * time.Second)
			db.Lock()
			for k := range db.entries {
				if time.Now().Unix() >= db.entries[k].expiration {
					delete(db.entries, k)
				}
			}
			db.Unlock()
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
		key := c.Path()

		// Find cached entry
		db.RLock()
		resp, ok := db.entries[key]
		db.RUnlock()
		if ok {
			// Check if entry is expired
			if time.Now().Unix() >= resp.expiration {
				db.Lock()
				delete(db.entries, key)
				db.Unlock()
			} else {
				// Set response headers from cache
				c.Response().SetBodyRaw(resp.body)
				c.Response().SetStatusCode(resp.statusCode)
				c.Response().Header.SetContentTypeBytes(resp.contentType)
				// Set Cache-Control header if enabled
				if cfg.CacheControl {
					maxAge := strconv.FormatInt(resp.expiration-time.Now().Unix(), 10)
					c.Set(fiber.HeaderCacheControl, "max-age="+maxAge)
				}
				return nil
			}
		}

		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
		}

		// Cache response
		db.Lock()
		db.entries[key] = entry{
			body:        c.Response().Body(),
			statusCode:  c.Response().StatusCode(),
			contentType: c.Response().Header.ContentType(),
			expiration:  time.Now().Unix() + db.expiration,
		}
		db.Unlock()

		// Finish response
		return nil
	}
}
