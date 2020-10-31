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

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Expiration is the time that an cached response will live
	//
	// Optional. Default: 1 * time.Minute
	Expiration time.Duration

	// CacheControl enables client side caching if set to true
	//
	// Optional. Default: false
	CacheControl bool

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
	Next:         nil,
	Expiration:   1 * time.Minute,
	CacheControl: false,
	defaultStore: true,
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
		if cfg.Store == nil {
			cfg.defaultStore = true
		}
	}

	var (
		// Cache settings
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

	// Nothing to cache
	if int(cfg.Expiration.Seconds()) < 0 {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	// Remove expired entries
	if cfg.defaultStore {
		go func() {
			for {
				// GC the entries every 10 seconds
				time.Sleep(10 * time.Second)
				mux.Lock()
				for k := range entries {
					if atomic.LoadUint64(&timestamp) >= entries[k].exp {
						delete(entries, k)
					}
				}
				mux.Unlock()
			}
		}()
	}

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

		// Create new entry
		var entry entry
		var entryBody []byte

		// Lock entry
		mux.Lock()
		defer mux.Unlock()

		// Check if we need to use the default in-memory storage
		if cfg.defaultStore {
			entry = entries[key]

		} else {
			// Load data from store
			storeEntry, err := cfg.Store.Get(key)
			if err != nil {
				return err
			}

			// Only decode if we found an entry
			if storeEntry != nil {
				// Decode bytes using msgp
				if _, err := entry.UnmarshalMsg(storeEntry); err != nil {
					return err
				}
			}

			if entryBody, err = cfg.Store.Get(key + "_body"); err != nil {
				return err
			}
		}

		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Set expiration if entry does not exist
		if entry.exp == 0 {
			entry.exp = ts + expiration

		} else if ts >= entry.exp {
			// Check if entry is expired
			// Use default memory storage
			if cfg.defaultStore {
				delete(entries, key)
			} else { // Use custom storage
				if err := cfg.Store.Delete(key); err != nil {
					return err
				}
				if err := cfg.Store.Delete(key + "_body"); err != nil {
					return err
				}
			}

		} else {
			if cfg.defaultStore {
				c.Response().SetBodyRaw(entry.body)
			} else {
				c.Response().SetBodyRaw(entryBody)
			}
			// Set response headers from cache
			c.Response().SetStatusCode(entry.status)
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
		entryBody = utils.SafeBytes(c.Response().Body())
		entry.status = c.Response().StatusCode()
		entry.cType = utils.SafeBytes(c.Response().Header.ContentType())

		// Use default memory storage
		if cfg.defaultStore {
			entry.body = entryBody
			entries[key] = entry

		} else {
			// Use custom storage
			data, err := entry.MarshalMsg(nil)
			if err != nil {
				return err
			}

			// Pass bytes to Storage
			if err = cfg.Store.Set(key, data, cfg.Expiration); err != nil {
				return err
			}

			// Pass bytes to Storage
			if err = cfg.Store.Set(key+"_body", entryBody, cfg.Expiration); err != nil {
				return err
			}
		}

		// Finish response
		return nil
	}
}
