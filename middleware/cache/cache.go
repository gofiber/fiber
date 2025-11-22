// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"context"
	"errors"
	"fmt"
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

var cacheableStatusCodes = map[int]struct{}{
	fiber.StatusOK:                          {},
	fiber.StatusNonAuthoritativeInformation: {},
	fiber.StatusNoContent:                   {},
	fiber.StatusPartialContent:              {},
	fiber.StatusMultipleChoices:             {},
	fiber.StatusMovedPermanently:            {},
	fiber.StatusNotFound:                    {},
	fiber.StatusMethodNotAllowed:            {},
	fiber.StatusGone:                        {},
	fiber.StatusRequestURITooLong:           {},
	fiber.StatusNotImplemented:              {},
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	redactKeys := !cfg.DisableValueRedaction

	maskKey := func(key string) string {
		if redactKeys {
			return redactedKey
		}
		return key
	}

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
	manager := newManager(cfg.Storage, redactKeys)
	// Create indexed heap for tracking expirations ( see heap.go )
	heap := &indexedHeap{}
	// count stored bytes (sizes of response bodies)
	var storedBytes uint

	// Update timestamp in the configured interval
	go func() {
		ticker := time.NewTicker(timestampUpdatePeriod)
		defer ticker.Stop()
		for range ticker.C {
			atomic.StoreUint64(&timestamp, uint64(time.Now().Unix())) //nolint:gosec //Not a concern
		}
	}()

	// Delete key from both manager and storage
	deleteKey := func(ctx context.Context, dkey string) error {
		if err := manager.del(ctx, dkey); err != nil {
			return err
		}
		// External storage saves body data with different key
		if cfg.Storage != nil {
			if err := manager.del(ctx, dkey+"_body"); err != nil {
				return err
			}
		}
		return nil
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

		reqCtx := c.Context()

		// Get entry from pool
		e, err := manager.get(reqCtx, key)
		if err != nil && !errors.Is(err, errCacheMiss) {
			return err
		}

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
				if err := deleteKey(reqCtx, key); err != nil {
					if e != nil {
						manager.release(e)
					}
					mux.Unlock()
					return fmt.Errorf("cache: failed to delete expired key %q: %w", maskKey(key), err)
				}
				idx := e.heapidx
				manager.release(e)
				if cfg.MaxBytes > 0 {
					_, size := heap.remove(idx)
					storedBytes -= size
				}
			} else if e.exp != 0 && !hasRequestDirective(c, noCache) {
				// Separate body value to avoid msgp serialization
				// We can store raw bytes with Storage 👍
				if cfg.Storage != nil {
					rawBody, err := manager.getRaw(reqCtx, key+"_body")
					if err != nil {
						manager.release(e)
						mux.Unlock()
						return cacheBodyFetchError(maskKey, key, err)
					}
					e.body = rawBody
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
				// Set Cache-Control header if not disabled and not already set
				if !cfg.DisableCacheControl && len(c.Response().Header.Peek(fiber.HeaderCacheControl)) == 0 {
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
		if _, ok := cacheableStatusCodes[c.Response().StatusCode()]; !ok {
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
				keyToRemove, size := heap.removeFirst()
				if err := deleteKey(reqCtx, keyToRemove); err != nil {
					return fmt.Errorf("cache: failed to delete key %q while evicting: %w", maskKey(keyToRemove), err)
				}
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
			for key, value := range c.Response().Header.All() {
				// create real copy
				keyS := string(key)
				if _, ok := ignoreHeaders[keyS]; !ok {
					e.headers[keyS] = utils.CopyBytes(value)
				}
			}
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
		var heapIdx int
		if cfg.MaxBytes > 0 {
			heapIdx = heap.put(key, e.exp, bodySize)
			e.heapidx = heapIdx
			storedBytes += bodySize
		}

		cleanupOnStoreError := func(ctx context.Context, releaseEntry, rawStored bool) error {
			var cleanupErr error
			if cfg.MaxBytes > 0 {
				_, size := heap.remove(heapIdx)
				storedBytes -= size
			}
			if releaseEntry {
				manager.release(e)
			}
			if rawStored {
				rawKey := key + "_body"
				if err := manager.del(ctx, rawKey); err != nil {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cache: failed to delete raw key %q after store error: %w", maskKey(rawKey), err))
				}
			}
			return cleanupErr
		}

		// For external Storage we store raw body separated
		if cfg.Storage != nil {
			if err := manager.setRaw(reqCtx, key+"_body", e.body, expiration); err != nil {
				if cleanupErr := cleanupOnStoreError(reqCtx, true, false); cleanupErr != nil {
					err = errors.Join(err, cleanupErr)
				}
				return err
			}
			// avoid body msgp encoding
			e.body = nil
			if err := manager.set(reqCtx, key, e, expiration); err != nil {
				if cleanupErr := cleanupOnStoreError(reqCtx, false, true); cleanupErr != nil {
					err = errors.Join(err, cleanupErr)
				}
				return err
			}
		} else {
			// Store entry in memory
			if err := manager.set(reqCtx, key, e, expiration); err != nil {
				if cleanupErr := cleanupOnStoreError(reqCtx, true, false); cleanupErr != nil {
					err = errors.Join(err, cleanupErr)
				}
				return err
			}
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

func cacheBodyFetchError(mask func(string) string, key string, err error) error {
	if errors.Is(err, errCacheMiss) {
		return fmt.Errorf("cache: no cached body for key %q: %w", mask(key), err)
	}
	return err
}

// parseMaxAge extracts the max-age directive from a Cache-Control header.
func parseMaxAge(cc string) (time.Duration, bool) {
	for part := range strings.SplitSeq(cc, ",") {
		part = utils.Trim(utils.ToLower(part), ' ')
		if after, ok := strings.CutPrefix(part, "max-age="); ok {
			if sec, err := strconv.Atoi(after); err == nil {
				return time.Duration(sec) * time.Second, true
			}
		}
	}
	return 0, false
}
