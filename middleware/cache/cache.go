// Special thanks to @codemicro for helping with this middleware: github.com/codemicro/fiber-cache
package cache

import (
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

	// // Hydrate is run before the response is returned to the client.
	// // Because this middleware is backend-agnostic, it makes no assumptions
	// // about what you want to do with cached response other than caching the statuscode,
	// // content-type and response body. Hydrate allows you to alter the cached response.
	// //
	// // Optional. Default: nil
	// Hydrate fiber.Handler
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:       nil,
	Expiration: 5 * time.Minute,
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

// Internal variables
var (
	db   *cache
	once sync.Once
)

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

	// Initialize db once
	once.Do(func() {
		db = &cache{
			entries:    make(map[string]entry),
			expiration: int64(cfg.Expiration.Seconds()),
		}
		// Remove expired entries
		go func() {
			for {
				time.Sleep(1 * time.Minute)
				for k := range db.entries {
					if time.Now().Unix() >= db.entries[k].expiration {
						delete(db.entries, k)
					}
				}
			}
		}()
	})

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

		// Fine cached entry
		db.RLock()
		resp, ok := db.entries[key]
		db.RUnlock()
		if ok {
			// Set response headers from cache
			c.Response().SetBodyRaw(resp.body)
			c.Response().SetStatusCode(resp.statusCode)
			c.Response().Header.SetContentTypeBytes(resp.contentType)
			// // Hydrate response if defined
			// if cfg.Hydrate != nil {
			// 	return cfg.Hydrate(c)
			// }
			return nil
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
