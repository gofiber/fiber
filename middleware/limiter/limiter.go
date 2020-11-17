package limiter

import (
	"fmt"
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
		// Limiter settings
		max        = strconv.Itoa(cfg.Max)
		timestamp  = uint64(time.Now().Unix())
		expiration = uint64(cfg.Expiration.Seconds())
		mux        = &sync.RWMutex{}

		// Default store logic (if no Store is provided)
		entries = make(map[string]entry)
	)

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

		// Create new entry
		entry := entry{}

		// Lock entry
		mux.Lock()
		defer mux.Unlock()

		// Use Storage if provided
		if cfg.Storage != nil {
			val, err := cfg.Storage.Get(key)
			if val != nil && len(val) > 0 {
				if _, err := entry.UnmarshalMsg(val); err != nil {
					return err
				}
			}
			if err != nil && err.Error() != errNotExist {
				fmt.Println("[LIMITER]", err.Error())
			}
		} else {
			entry = entries[key]
		}

		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Set expiration if entry does not exist
		if entry.exp == 0 {
			entry.exp = ts + expiration

		} else if ts >= entry.exp {
			// Check if entry is expired
			entry.hits = 0
			entry.exp = ts + expiration
		}

		// Increment hits
		entry.hits++

		// Use Storage if provided
		if cfg.Storage != nil {
			// Marshal entry to bytes
			val, err := entry.MarshalMsg(nil)
			if err != nil {
				return err
			}

			// Pass value to Storage
			if err = cfg.Storage.Set(key, val, cfg.Expiration); err != nil {
				return err
			}
		} else {
			entries[key] = entry
		}

		// Calculate when it resets in seconds
		expire := entry.exp - ts

		// Set how many hits we have left
		remaining := cfg.Max - entry.hits

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
