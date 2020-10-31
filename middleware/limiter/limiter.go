package limiter

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

//go:generate msgp -unexported
//msgp:ignore Config

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

	// DEPRECATED: Use Expiration instead
	Duration time.Duration

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration

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

	// Store is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Store fiber.Storage

	// Internally used - if true, the simpler method of two maps is used in order to keep
	// execution time down.
	defaultStore bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:       nil,
	Max:        5,
	Expiration: 1 * time.Minute,
	Key: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
	defaultStore: true,
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
		if int(cfg.Duration.Seconds()) <= 0 && int(cfg.Expiration.Seconds()) <= 0 {
			cfg.Expiration = ConfigDefault.Expiration
		}
		if int(cfg.Duration.Seconds()) > 0 {
			fmt.Println("[LIMITER] Duration is deprecated, please use Expiration")
			if cfg.Expiration != ConfigDefault.Expiration {
				cfg.Expiration = cfg.Duration
			}
		}
		if cfg.Key == nil {
			cfg.Key = ConfigDefault.Key
		}
		if cfg.LimitReached == nil {
			cfg.LimitReached = ConfigDefault.LimitReached
		}
		if cfg.Store == nil {
			cfg.defaultStore = true
		}
	}

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
		key := cfg.Key(c)

		// Create new entry
		entry := entry{}

		// Lock entry
		mux.Lock()
		defer mux.Unlock()

		// Use default memory storage
		if cfg.defaultStore {
			entry = entries[key]
		} else { // Use custom storage
			storeEntry, err := cfg.Store.Get(key)
			if err != nil {
				return err
			}
			// Only decode if we found an entry
			if len(storeEntry) > 0 {
				// Decode bytes using msgp
				if _, err := entry.UnmarshalMsg(storeEntry); err != nil {
					return err
				}
			}
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

		// Use default memory storage
		if cfg.defaultStore {
			entries[key] = entry
		} else { // Use custom storage
			data, err := entry.MarshalMsg(nil)
			if err != nil {
				return err
			}

			// Pass bytes to Storage
			if err = cfg.Store.Set(key, data, cfg.Expiration); err != nil {
				return err
			}
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

// replacer for strconv.FormatUint
// func appendInt(buf *bytebufferpool.ByteBuffer, v int) (int, error) {
// 	old := len(buf.B)
// 	buf.B = fasthttp.AppendUint(buf.B, v)
// 	return len(buf.B) - old, nil
// }
