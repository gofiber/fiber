package limiter

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Max number of recent connections during `Duration` seconds before sending a 429 response
	//
	// Default: 5
	Max int

	// Duration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Duration time.Duration

	// Key allows you to generate custom keys, by default c.IP() is used
	//
	// Default: func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	Key func(*fiber.Ctx) string

	// LimitReached is called when a request hits the limit
	//
	// Default: func(c *fiber.Ctx) error {
	//   return c.SendStatus(fiber.StatusTooManyRequests)
	// }
	LimitReached fiber.Handler
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:     nil,
	Max:      5,
	Duration: 1 * time.Minute,
	Key: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
}

// X-RateLimit-* headers
const (
	xRateLimitLimit     = "X-RateLimit-Limit"
	xRateLimitRemaining = "X-RateLimit-Remaining"
	xRateLimitReset     = "X-RateLimit-Reset"
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
		if cfg.Max <= 0 {
			cfg.Max = ConfigDefault.Max
		}
		if int(cfg.Duration.Seconds()) <= 0 {
			cfg.Duration = ConfigDefault.Duration
		}
		if cfg.Key == nil {
			cfg.Key = ConfigDefault.Key
		}
		if cfg.LimitReached == nil {
			cfg.LimitReached = ConfigDefault.LimitReached
		}
	}

	// Limiter settings
	var max = strconv.Itoa(cfg.Max)
	var hits = make(map[string]int)
	var reset = make(map[string]uint64)
	var timestamp = uint64(time.Now().Unix())
	var duration = uint64(cfg.Duration.Seconds())

	// mutex for parallel read and write access
	mux := &sync.Mutex{}

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

		// Get key (default is the remote IP)
		key := cfg.Key(c)

		// Lock map
		mux.Lock()

		// Set unix timestamp if not exist
		ts := atomic.LoadUint64(&timestamp)
		if reset[key] == 0 {
			reset[key] = ts + duration
		} else if ts >= reset[key] {
			hits[key] = 0
			reset[key] = ts + duration
		}

		// Increment key hits
		hits[key]++

		// Get current hits
		hitCount := hits[key]

		// Calculate when it resets in seconds
		resetTime := reset[key] - ts

		// Unlock map
		mux.Unlock()

		// Set how many hits we have left
		remaining := cfg.Max - hitCount

		// Check if hits exceed the cfg.Max
		if remaining < 0 {
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			c.Set(fiber.HeaderRetryAfter, strconv.FormatUint(resetTime, 10))

			// Call LimitReached handler
			return cfg.LimitReached(c)
		}

		// We can continue, update RateLimit headers
		c.Set(xRateLimitLimit, max)
		c.Set(xRateLimitRemaining, strconv.Itoa(remaining))
		c.Set(xRateLimitReset, strconv.FormatUint(resetTime, 10))

		// Continue stack
		return c.Next()
	}
}
