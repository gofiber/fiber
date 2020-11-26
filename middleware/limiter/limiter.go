package limiter

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	// Storage ErrNotExist
	errNotExist = "key does not exist"

	// X-RateLimit-* headers
	xRateLimitLimit     = "X-RateLimit-Limit"
	xRateLimitRemaining = "X-RateLimit-Remaining"
	xRateLimitReset     = "X-RateLimit-Reset"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	var (
		// Limiter variables
		mux        = &sync.RWMutex{}
		max        = strconv.Itoa(cfg.Max)
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
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get key from request
		key := cfg.KeyGenerator(c)

		// Lock entry
		mux.Lock()

		// Get entry from pool and release when finished
		e := manager.get(key)

		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Set expiration if entry does not exist
		if e.exp == 0 {
			e.exp = ts + expiration

		} else if ts >= e.exp {
			// Check if entry is expired
			e.hits = 0
			e.exp = ts + expiration
		}

		// Increment hits
		e.hits++

		// Calculate when it resets in seconds
		expire := e.exp - ts

		// Set how many hits we have left
		remaining := cfg.Max - e.hits

		// Update storage
		manager.set(key, e, cfg.Expiration)

		// Unlock entry
		mux.Unlock()

		// Check if hits exceed the cfg.Max
		if remaining < 0 {
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			c.Set(fiber.HeaderRetryAfter, strconv.FormatUint(expire, 10))

			// Call LimitReached handler
			return cfg.LimitReached(c)
		}

		// We can continue, update RateLimit headers
		c.Set(xRateLimitLimit, max)
		c.Set(xRateLimitRemaining, strconv.Itoa(remaining))
		c.Set(xRateLimitReset, strconv.FormatUint(expire, 10))

		// Continue stack
		return c.Next()
	}
}
