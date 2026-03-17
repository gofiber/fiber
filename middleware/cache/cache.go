// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"golang.org/x/sync/singleflight"
)

// timestampUpdatePeriod is the period that is used to check the cache expiration.
// It should not be too long to provide more or less acceptable expiration error, and,
// at the same time, it should not be too short to avoid overwhelming the system.
const timestampUpdatePeriod = 300 * time.Millisecond

// loadResult holds the response data returned from a singleflight load so waiters
// can apply it to their context without running the handler.
type loadResult struct {
	Body      []byte
	Status    int
	Ctype     []byte
	Cencoding []byte
	Headers   map[string][]byte
	Exp       uint64
}

// cache status
const (
	// cacheUnreachable: when cache was bypassed or is invalid
	cacheUnreachable = "unreachable"
	// cacheHit: cache served
	cacheHit = "hit"
	// cacheMiss: no cache record for the given key
	cacheMiss = "miss"
)

// directives
const (
	noCache = "no-cache"
	noStore = "no-store"
)

var ignoreHeaders = map[string]interface{}{
	"Connection":          nil,
	"Keep-Alive":          nil,
	"Proxy-Authenticate":  nil,
	"Proxy-Authorization": nil,
	"TE":                  nil,
	"Trailers":            nil,
	"Transfer-Encoding":   nil,
	"Upgrade":             nil,
	"Content-Type":        nil, // already stored explicitly by the cache manager
	"Content-Encoding":    nil, // already stored explicitly by the cache manager
}

// New creates a new middleware handler. When Config.SingleFlight is true, concurrent
// cache misses for the same key are coalesced (single-flight): only one request runs
// the handler and populates the cache; others wait and share the result, preventing
// cache stampede. Recommend SingleFlight: true for high-concurrency deployments.
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
		mux       = &sync.RWMutex{}
		timestamp = uint64(time.Now().Unix())
		sf        singleflight.Group
	)
	// Create a manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage)
	// Create an indexed heap to track expirations ( see heap.go )
	heap := &indexedHeap{}
	// Count bytes stored (sizes of response bodies)
	var storedBytes uint = 0

	// Update timestamp in the configured interval
	go func() {
		for {
			atomic.StoreUint64(&timestamp, uint64(time.Now().Unix()))
			time.Sleep(timestampUpdatePeriod)
		}
	}()

	// Delete a key from both manager and storage
	deleteKey := func(dkey string) {
		manager.delete(dkey)
		// External storage saves body data with a different key
		if cfg.Storage != nil {
			manager.delete(dkey + "_body")
		}
	}

	// Return a new handler
	return func(c *fiber.Ctx) error {
		// -------------------------------------------------------------------------
		// Refrain from caching
		if hasRequestDirective(c, noStore) {
			return c.Next()
		}

		// -------------------------------------------------------------------------
		// Only cache selected methods
		var isExists bool
		for _, method := range cfg.Methods {
			if c.Method() == method {
				isExists = true
			}
		}

		if !isExists {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.Next()
		}

		// -------------------------------------------------------------------------
		// Get key from request
		// TODO(allocation optimization): try to minimize the allocation from 2 to 1
		key := cfg.KeyGenerator(c) + "_" + c.Method()

		// Get entry from pool
		e := manager.get(key)

		// Lock entry
		mux.Lock()

		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Check if entry has expired
		if e.exp != 0 && ts >= e.exp {
			deleteKey(key)
			if cfg.MaxBytes > 0 {
				_, size := heap.remove(e.heapidx)
				storedBytes -= size
			}
		} else if e.exp != 0 && !hasRequestDirective(c, noCache) {
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
			if e.headers != nil {
				for k, v := range e.headers {
					c.Response().Header.SetBytesV(k, v)
				}
			}

			// Set Cache-Control header if enabled
			if cfg.CacheControl {
				maxAge := strconv.FormatUint(e.exp-ts, 10)
				c.Set(fiber.HeaderCacheControl, "public, max-age="+maxAge)
			}

			c.Set(cfg.CacheHeader, cacheHit)

			mux.Unlock()

			// Return response
			return nil
		}

		// make sure we're not blocking concurrent requests - do unlock
		mux.Unlock()

		// -------------------------------------------------------------------------
		// Single-flight path (optional)
		// Handle concurrent cache misses (single-flight) -> mitigate cache stampede
		if cfg.SingleFlight {
			// Single-flight: one request runs the handler and populates cache; others wait and share the result.
			v, err, shared := sf.Do(key, func() (any, error) {
				if err := c.Next(); err != nil {
					return nil, err
				}

				// Begin critical section: lock entry and timestamp
				mux.Lock()
				defer mux.Unlock()
				ts := atomic.LoadUint64(&timestamp)
				e := manager.get(key)
				bodySize := uint(len(c.Response().Body()))

				expiration := cfg.Expiration
				if cfg.ExpirationGenerator != nil {
					expiration = cfg.ExpirationGenerator(c, &cfg)
				}
				exp := ts + uint64(expiration.Seconds())
				res := loadResult{
					Body:      utils.CopyBytes(c.Response().Body()),
					Status:    c.Response().StatusCode(),
					Ctype:     utils.CopyBytes(c.Response().Header.ContentType()),
					Cencoding: utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderContentEncoding)),
					Exp:       exp,
				}

				// Store response headers if enabled
				if cfg.StoreResponseHeaders {
					res.Headers = make(map[string][]byte)
					c.Response().Header.VisitAll(
						func(k []byte, v []byte) {
							keyS := string(k)
							if _, ok := ignoreHeaders[keyS]; !ok {
								res.Headers[keyS] = utils.CopyBytes(v)
							}
						},
					)
				}

				// If middleware marks request for bypass, return result without caching.
				if cfg.Next != nil && cfg.Next(c) {
					return res, nil
				}
				// Skip caching if body won't fit into cache.
				if cfg.MaxBytes > 0 && bodySize > cfg.MaxBytes {
					return res, nil
				}
				// Evict oldest entries if cache is full.
				if cfg.MaxBytes > 0 {
					for storedBytes+bodySize > cfg.MaxBytes {
						removedKey, size := heap.removeFirst()
						deleteKey(removedKey)
						storedBytes -= size
					}
				}

				// Overwrite pool entry with the new result.
				e.body = res.Body
				e.status = res.Status
				e.ctype = res.Ctype
				e.cencoding = res.Cencoding
				e.headers = res.Headers
				e.exp = res.Exp

				// Update cache size tracking if enabled.
				if cfg.MaxBytes > 0 {
					e.heapidx = heap.put(key, e.exp, bodySize)
					storedBytes += bodySize
				}

				// Store entry in external storage if enabled.
				if cfg.Storage != nil {
					manager.setRaw(key+"_body", e.body, expiration)
					// Avoid body msgp encoding.
					e.body = nil
					manager.set(key, e, expiration)
					manager.release(e)
				} else {
					// Store entry in memory.
					manager.set(key, e, expiration)
				}
				return res, nil
			})
			if err != nil {
				return err
			}

			// If result was shared (other request already populated cache), apply it to our context.
			if shared {
				// Waiter: apply shared result to our context
				res := v.(loadResult)
				c.Response().SetBodyRaw(res.Body)
				c.Response().SetStatusCode(res.Status)
				c.Response().Header.SetContentTypeBytes(res.Ctype)

				// Set content encoding if defined.
				if len(res.Cencoding) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderContentEncoding, res.Cencoding)
				}

				// Pass headers if defined.
				if res.Headers != nil {
					for k, v := range res.Headers {
						c.Response().Header.SetBytesV(k, v)
					}
				}

				// Set Cache-Control header if enabled.
				if cfg.CacheControl {
					ts := atomic.LoadUint64(&timestamp)
					maxAge := strconv.FormatUint(res.Exp-ts, 10)
					c.Set(fiber.HeaderCacheControl, "public, max-age="+maxAge)
				}
			}

			// Set cache status header.
			c.Set(cfg.CacheHeader, cacheMiss)
			return nil
		}

		// Otherwise, the default non-single-flight path.

		// Continue stack, return err to Fiber if exists
		if err := c.Next(); err != nil {
			return err
		}

		// lock entry back and unlock on finish
		mux.Lock()
		defer mux.Unlock()

		// Don't cache response if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		// Don't try to cache if body won't fit into cache
		bodySize := uint(len(c.Response().Body()))
		if cfg.MaxBytes > 0 && bodySize > cfg.MaxBytes {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		// Remove oldest to make room for new
		if cfg.MaxBytes > 0 {
			for storedBytes+bodySize > cfg.MaxBytes {
				key, size := heap.removeFirst()
				deleteKey(key)
				storedBytes -= size
			}
		}

		// Cache response
		e.body = utils.CopyBytes(c.Response().Body())
		e.status = c.Response().StatusCode()
		e.ctype = utils.CopyBytes(c.Response().Header.ContentType())
		e.cencoding = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderContentEncoding))

		// Store all response headers
		// (more: https://datatracker.ietf.org/doc/html/rfc2616#section-13.5.1)
		if cfg.StoreResponseHeaders {
			e.headers = make(map[string][]byte)
			c.Response().Header.VisitAll(
				func(key []byte, value []byte) {
					// create real copy
					keyS := string(key)
					if _, ok := ignoreHeaders[keyS]; !ok {
						e.headers[keyS] = utils.CopyBytes(value)
					}
				},
			)
		}

		// default cache expiration
		expiration := cfg.Expiration
		// Calculate expiration by response header or other setting
		if cfg.ExpirationGenerator != nil {
			expiration = cfg.ExpirationGenerator(c, &cfg)
		}
		e.exp = ts + uint64(expiration.Seconds())

		// Store entry in heap
		if cfg.MaxBytes > 0 {
			e.heapidx = heap.put(key, e.exp, bodySize)
			storedBytes += bodySize
		}

		// For external Storage we store raw body separated
		if cfg.Storage != nil {
			manager.setRaw(key+"_body", e.body, expiration)
			// avoid body msgp encoding
			e.body = nil
			manager.set(key, e, expiration)
			manager.release(e)
		} else {
			// Store entry in memory
			manager.set(key, e, expiration)
		}

		c.Set(cfg.CacheHeader, cacheMiss)

		// Finish response
		return nil
	}
}

// Check if request has a directive.
func hasRequestDirective(c *fiber.Ctx, directive string) bool {
	return strings.Contains(c.Get(fiber.HeaderCacheControl), directive)
}
