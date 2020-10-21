package limiter

import (
	"bytes"
	"encoding/gob"
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

	// Store is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Store Storage

	// Internally used - if true, the simpler method of two maps is used in order to keep
	// execution time down.
	usingCustomStore bool
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

// trackedSession is the type used for session tracking
type trackedSession struct {
	Hits      int
	ResetTime uint64
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
		if cfg.Store != nil {
			cfg.usingCustomStore = true
		}
	}

	// Limiter settings
	var max = strconv.Itoa(cfg.Max)
	var sessions = make(map[string]trackedSession)
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

		// Lock mux (prevents values changing between retrieval and reassignment, which can and does
		// break things)
		mux.Lock()

		var session trackedSession

		if cfg.usingCustomStore {
			// Load data from store
			fromStore, err := cfg.Store.Get(key)
			if err != nil {
				return err
			}

			if len(fromStore) == 0 {
				// Assume this means item not found.
				session = trackedSession{}
			} else {
				// Decode bytes using gob
				var buf bytes.Buffer
				_, _ = buf.Write(fromStore)
				dec := gob.NewDecoder(&buf)
				err := dec.Decode(&session)
				if err != nil {
					return err
				}
			}
		} else {
			// Load data from in-memory map
			session = sessions[key]
		}

		// Set unix timestamp if not exist
		ts := atomic.LoadUint64(&timestamp)
		if session.ResetTime == 0 {
			session.ResetTime = ts + duration
		} else if ts >= session.ResetTime {
			session.Hits = 0
			session.ResetTime = ts + duration
		}

		// Increment key hits
		session.Hits++

		if cfg.usingCustomStore {
			// Convert session struct into bytes
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			err := enc.Encode(session)
			if err != nil {
				return err
			}

			// Store those bytes
			err = cfg.Store.Set(key, buf.Bytes(), cfg.Duration)
			if err != nil {
				return err
			}
		} else {
			sessions[key] = session
		}

		// Get current hits
		hitCount := session.Hits

		// Calculate when it resets in seconds
		resetTime := session.ResetTime - ts

		// Set how many hits we have left
		remaining := cfg.Max - hitCount

		mux.Unlock()

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
