// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// timestampUpdatePeriod is the period which is used to check the cache expiration.
// It should not be too long to provide more or less acceptable expiration error, and in the same
// time it should not be too short to avoid overwhelming of the system
const timestampUpdatePeriod = 300 * time.Millisecond

// cache status
// unreachable: when cache is bypass, or invalid
// hit: cache is served
// miss: do not have cache record
const (
	cacheUnreachable = "unreachable"
	cacheHit         = "hit"
	cacheMiss        = "miss"
)

// directives
const (
	noCache = "no-cache"
	noStore = "no-store"
)

var ignoreHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"TE":                  {},
	"Trailers":            {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"Content-Type":        {}, // already stored explicitly by the cache manager
	"Content-Encoding":    {}, // already stored explicitly by the cache manager
}

var cacheableStatusCodes = map[int]bool{
	fiber.StatusOK:                          true,
	fiber.StatusNonAuthoritativeInformation: true,
	fiber.StatusNoContent:                   true,
	fiber.StatusPartialContent:              true,
	fiber.StatusMultipleChoices:             true,
	fiber.StatusMovedPermanently:            true,
	fiber.StatusNotFound:                    true,
	fiber.StatusMethodNotAllowed:            true,
	fiber.StatusGone:                        true,
	fiber.StatusRequestURITooLong:           true,
	fiber.StatusNotImplemented:              true,
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Nothing to cache
	if int(cfg.Expiration.Seconds()) < 0 {
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	var (
		// Cache settings
		mux       = &sync.RWMutex{}
		timestamp = uint64(time.Now().Unix()) //nolint:gosec //Not a concern
	)
	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage)
	// Create indexed heap for tracking expirations ( see heap.go )
	heap := &indexedHeap{}
	// count stored bytes (sizes of response bodies)
	var storedBytes uint

	// Update timestamp in the configured interval
	go func() {
		for {
			atomic.StoreUint64(&timestamp, uint64(time.Now().Unix())) //nolint:gosec //Not a concern
			time.Sleep(timestampUpdatePeriod)
		}
	}()

	// Delete key from both manager and storage
	deleteKey := func(dkey string) {
		manager.del(dkey)
		// External storage saves body data with different key
		if cfg.Storage != nil {
			manager.del(dkey + "_body")
		}
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Refrain from caching
		if hasRequestDirective(c, noStore) {
			return c.Next()
		}

		requestMethod := c.Method()

		// Only cache selected methods
		if !slices.Contains(cfg.Methods, requestMethod) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.Next()
		}

		// Get key from request
		// TODO(allocation optimization): try to minimize the allocation from 2 to 1
		key := cfg.KeyGenerator(c) + "_" + requestMethod

		// Get entry from pool
		e := manager.get(key)

		// Lock entry
		mux.Lock()

		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Cache Entry found
		if e != nil {
			// Invalidate cache if requested
			if cfg.CacheInvalidator != nil && cfg.CacheInvalidator(c) {
				e.exp = ts - 1
			}

			// Check if entry is expired
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
				for k, v := range e.headers {
					c.Response().Header.SetBytesV(k, v)
				}
				// Set Cache-Control header if enabled and not already set
				if cfg.CacheControl && len(c.Response().Header.Peek(fiber.HeaderCacheControl)) == 0 {
					maxAge := strconv.FormatUint(e.exp-ts, 10)
					c.Set(fiber.HeaderCacheControl, "public, max-age="+maxAge)
				}

				// RFC-compliant Age header (RFC 9111)
				resident := e.ttl - (e.exp - ts)
				age := strconv.FormatUint(e.age+resident, 10)
				c.Response().Header.Set(fiber.HeaderAge, age)

				c.Set(cfg.CacheHeader, cacheHit)

				// release item allocated from storage
				if cfg.Storage != nil {
					manager.release(e)
				}

				mux.Unlock()

				// Return response
				return nil
			}
		}

		// make sure we're not blocking concurrent requests - do unlock
		mux.Unlock()

		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
		}

		// Respect server cache-control: no-store
		if strings.Contains(utils.ToLower(string(c.Response().Header.Peek(fiber.HeaderCacheControl))), noStore) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		// Don't cache response if status code is not cacheable
		if !cacheableStatusCodes[c.Response().StatusCode()] {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
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

		e = manager.acquire()
		// Cache response
		e.body = utils.CopyBytes(c.Response().Body())
		e.status = c.Response().StatusCode()
		e.ctype = utils.CopyBytes(c.Response().Header.ContentType())
		e.cencoding = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderContentEncoding))

		ageVal := uint64(0)
		if b := c.Response().Header.Peek(fiber.HeaderAge); len(b) > 0 {
			if v, err := fasthttp.ParseUint(b); err == nil {
				ageVal = uint64(v) //nolint:gosec //Not a concern
			}
		} else {
			c.Response().Header.Set(fiber.HeaderAge, "0")
		}
		e.age = ageVal

		// Store all response headers
		// (more: https://datatracker.ietf.org/doc/html/rfc2616#section-13.5.1)
		if cfg.StoreResponseHeaders {
			e.headers = make(map[string][]byte)
			c.Response().Header.All()(func(key, value []byte) bool {
				// create real copy
				keyS := string(key)
				if _, ok := ignoreHeaders[keyS]; !ok {
					e.headers[keyS] = utils.CopyBytes(value)
				}
				return true
			})
		}

		// default cache expiration
		expiration := cfg.Expiration
		if v, ok := parseMaxAge(string(c.Response().Header.Peek(fiber.HeaderCacheControl))); ok {
			expiration = v
		}
		// Calculate expiration by response header or other setting
		if cfg.ExpirationGenerator != nil {
			expiration = cfg.ExpirationGenerator(c, &cfg)
		}
		e.exp = ts + uint64(expiration.Seconds())
		e.ttl = uint64(expiration.Seconds())

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

// Check if request has directive
func hasRequestDirective(c fiber.Ctx, directive string) bool {
	cc := c.Get(fiber.HeaderCacheControl)
	ccLen := len(cc)
	dirLen := len(directive)
	for i := 0; i <= ccLen-dirLen; i++ {
		if !utils.EqualFold(cc[i:i+dirLen], directive) {
			continue
		}
		if i > 0 {
			prev := cc[i-1]
			if prev != ' ' && prev != ',' {
				continue
			}
		}
		if i+dirLen == ccLen || cc[i+dirLen] == ',' {
			return true
		}
	}

	return false
}

// parseMaxAge extracts the max-age directive from a Cache-Control header.
func parseMaxAge(cc string) (time.Duration, bool) {
	for _, part := range strings.Split(cc, ",") {
		part = utils.Trim(utils.ToLower(part), ' ')
		if strings.HasPrefix(part, "max-age=") {
			if sec, err := strconv.Atoi(strings.TrimPrefix(part, "max-age=")); err == nil {
				return time.Duration(sec) * time.Second, true
			}
		}
	}
	return 0, false
}
