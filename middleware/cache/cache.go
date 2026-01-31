// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
)

// timestampUpdatePeriod is the period which is used to check the cache expiration.
// It should not be too long to provide more or less acceptable expiration error, and in the same
// time it should not be too short to avoid overwhelming of the system
const timestampUpdatePeriod = 300 * time.Millisecond

// buffer size for hexpool
const hexLen = sha256.Size * 2

// cache status
// unreachable: when cache is bypass, or invalid
// hit: cache is served
// miss: do not have cache record
const (
	cacheUnreachable = "unreachable"
	cacheHit         = "hit"
	cacheMiss        = "miss"
)

type expirationSource uint8

const (
	expirationSourceConfig expirationSource = iota
	expirationSourceMaxAge
	expirationSourceSMaxAge
	expirationSourceExpires
	expirationSourceGenerator
)

// directives
const (
	noCache          = "no-cache"
	noStore          = "no-store"
	privateDirective = "private"
)

type requestCacheDirectives struct {
	maxAge   uint64
	maxStale uint64
	minFresh uint64

	maxAgeSet    bool
	maxStaleSet  bool
	maxStaleAny  bool
	minFreshSet  bool
	noStore      bool
	noCache      bool
	onlyIfCached bool
}

var ignoreHeaders = map[string]struct{}{
	"Age":                 {},
	"Cache-Control":       {}, // already stored explicitly by the cache manager
	"Connection":          {},
	"Content-Encoding":    {}, // already stored explicitly by the cache manager
	"Content-Type":        {}, // already stored explicitly by the cache manager
	"Date":                {},
	"ETag":                {}, // already stored explicitly by the cache manager
	"Expires":             {}, // already stored explicitly by the cache manager
	"Last-Modified":       {}, // already stored explicitly by the cache manager
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"TE":                  {},
	"Trailers":            {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

var cacheableStatusCodes = map[int]struct{}{
	fiber.StatusOK:                          {},
	fiber.StatusNonAuthoritativeInformation: {},
	fiber.StatusNoContent:                   {},
	fiber.StatusPartialContent:              {},
	fiber.StatusMultipleChoices:             {},
	fiber.StatusMovedPermanently:            {},
	fiber.StatusPermanentRedirect:           {},
	fiber.StatusNotFound:                    {},
	fiber.StatusMethodNotAllowed:            {},
	fiber.StatusGone:                        {},
	fiber.StatusRequestURITooLong:           {},
	fiber.StatusNotImplemented:              {},
}

// cacheInvalidatorLocalKey is the Locals key used to pass the tag
// invalidation function from the middleware to downstream handlers.
const cacheInvalidatorLocalKey = "__fiber_cache_invalidator"

// InvalidateTags removes all cached responses associated with any of the
// provided tags. The cache middleware must be present in the handler chain
// (registered via app.Use or on the same route group) for the invalidation
// function to be available.
func InvalidateTags(c fiber.Ctx, tags ...string) error {
	fn, ok := c.Locals(cacheInvalidatorLocalKey).(func(context.Context, ...string) error)
	if !ok || fn == nil {
		return errors.New("cache: InvalidateTags requires the cache middleware to be registered on this route")
	}
	return fn(c.Context(), tags...)
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	type evictionCandidate struct {
		key     string
		size    uint
		exp     uint64
		heapIdx int
	}

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
		timestamp = safeUnixSeconds(time.Now())
	)
	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage, redactKeys)
	// Create indexed heap for tracking expirations ( see heap.go )
	heap := &indexedHeap{}
	// count stored bytes (sizes of response bodies)
	var storedBytes uint
	// Tag index for tag-based cache invalidation
	var ti tagStore
	if cfg.Tags != nil || cfg.ResponseTags != nil {
		if cfg.Storage != nil {
			ti = newDistributedTagStore(cfg.Storage, cfg.Expiration)
		} else {
			ti = newTagIndex()
		}
	}
	// Pre-classify reject patterns for efficient runtime matching
	var reject *rejectMatcher
	if len(cfg.RejectTags) > 0 {
		reject = newRejectMatcher(cfg.RejectTags)
	}
	// Key → heap index mapping for O(1) lookup during tag invalidation
	var keyHeapIdx map[string]int
	if cfg.MaxBytes > 0 {
		keyHeapIdx = make(map[string]int)
	}
	// Pool for hex encoding buffers
	hexBufPool := &sync.Pool{
		New: func() any {
			buf := make([]byte, hexLen)
			return &buf
		},
	}
	hashAuthorization := makeHashAuthFunc(hexBufPool)
	buildVaryKey := makeBuildVaryKeyFunc(hexBufPool)

	// Update timestamp in the configured interval
	go func() {
		ticker := time.NewTicker(timestampUpdatePeriod)
		defer ticker.Stop()
		for range ticker.C {
			atomic.StoreUint64(&timestamp, safeUnixSeconds(time.Now()))
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

	removeHeapEntry := func(entryKey string, heapIdx int) {
		if cfg.MaxBytes == 0 {
			return
		}

		if heapIdx < 0 || heapIdx >= len(heap.indices) {
			return
		}

		indexedIdx := heap.indices[heapIdx]
		if indexedIdx < 0 || indexedIdx >= len(heap.entries) {
			return
		}

		entry := heap.entries[indexedIdx]
		if entry.idx != heapIdx || entry.key != entryKey {
			return
		}

		_, size := heap.remove(heapIdx)
		storedBytes -= size
		if keyHeapIdx != nil {
			delete(keyHeapIdx, entryKey)
		}
	}

	refreshHeapIndex := func(ctx context.Context, candidate evictionCandidate) error {
		entry, err := manager.get(ctx, candidate.key)
		if err != nil {
			if errors.Is(err, errCacheMiss) {
				return nil
			}
			return fmt.Errorf("cache: failed to reload key %q after eviction failure: %w", maskKey(candidate.key), err)
		}

		entry.heapidx = candidate.heapIdx

		remainingTTL := max(time.Until(secondsToTime(entry.exp)), 0)

		if err := manager.set(ctx, candidate.key, entry, remainingTTL); err != nil {
			return fmt.Errorf("cache: failed to restore heap index for key %q: %w", maskKey(candidate.key), err)
		}

		return nil
	}

	// invalidateFn is stored in Locals so InvalidateTags can reach it.
	// Lock ordering: ti.mu is never held while mux is held, and vice versa.
	invalidateFn := func(ctx context.Context, tags ...string) error {
		if ti == nil {
			return nil
		}
		// Collect keys (acquires and releases ti.mu)
		keys := ti.invalidate(tags)
		if len(keys) == 0 {
			return nil
		}
		// Remove from heap (acquires and releases mux; ti.mu is not held)
		if cfg.MaxBytes > 0 {
			mux.Lock()
			for _, k := range keys {
				if idx, ok := keyHeapIdx[k]; ok {
					removeHeapEntry(k, idx)
				}
			}
			mux.Unlock()
		}
		// Delete from storage
		var errs error
		for _, k := range keys {
			if delErr := deleteKey(ctx, k); delErr != nil {
				errs = errors.Join(errs, delErr)
			}
		}
		return errs
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Expose invalidation function for all requests in this middleware chain
		c.Locals(cacheInvalidatorLocalKey, invalidateFn)

		hasAuthorization := len(c.Request().Header.Peek(fiber.HeaderAuthorization)) > 0
		reqCacheControl := c.Request().Header.Peek(fiber.HeaderCacheControl)
		reqDirectives := parseRequestCacheControl(reqCacheControl)
		if !reqDirectives.noCache {
			reqPragma := utils.UnsafeString(c.Request().Header.Peek(fiber.HeaderPragma))
			if hasDirective(reqPragma, noCache) {
				reqDirectives.noCache = true
			}
		}

		// Refrain from caching
		if reqDirectives.noStore {
			return c.Next()
		}

		requestMethod := c.Method()

		// Only cache selected methods
		if !slices.Contains(cfg.Methods, requestMethod) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.Next()
		}

		// Get key from request
		baseKey := cfg.KeyGenerator(c) + "_" + requestMethod
		manifestKey := baseKey + "|vary"
		if hasAuthorization {
			authHash := hashAuthorization(c.Request().Header.Peek(fiber.HeaderAuthorization))
			baseKey += "_auth_" + authHash
			manifestKey = baseKey + "|vary"
		}
		key := baseKey

		reqCtx := c.Context()

		varyNames, hasVaryManifest, err := loadVaryManifest(reqCtx, manager, manifestKey)
		if err != nil {
			return err
		}
		if len(varyNames) > 0 {
			key += buildVaryKey(varyNames, &c.Request().Header)
		}

		// Get entry from pool
		e, err := manager.get(reqCtx, key)
		if err != nil && !errors.Is(err, errCacheMiss) {
			return err
		}

		// Re-populate tag index from persisted entry (recovers across restarts)
		if e != nil && ti != nil && len(e.tags) > 0 && !ti.has(key) {
			ti.add(key, e.tags)
		}

		entryAge := uint64(0)
		revalidate := false
		oldHeapIdx := -1 // Track old heap index for replacement during revalidation

		handleMinFresh := func(now uint64) {
			if e == nil || !reqDirectives.minFreshSet {
				return
			}
			remainingFreshness := remainingFreshness(e, now)
			if remainingFreshness < reqDirectives.minFresh {
				revalidate = true
				oldHeapIdx = e.heapidx
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
			}
		}

		// Lock entry
		mux.Lock()
		locked := true
		unlock := func() {
			if locked {
				mux.Unlock()
				locked = false
			}
		}
		relock := func() {
			if !locked {
				mux.Lock()
				locked = true
			}
		}
		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Cache Entry found
		if e != nil {
			entryAge = cachedResponseAge(e, ts)
			if reqDirectives.maxAgeSet && (reqDirectives.maxAge == 0 || entryAge > reqDirectives.maxAge) {
				revalidate = true
				oldHeapIdx = e.heapidx
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
			}

			handleMinFresh(ts)
		}

		if e != nil && e.ttl == 0 && e.forceRevalidate {
			revalidate = true
			oldHeapIdx = e.heapidx
			if cfg.Storage != nil {
				manager.release(e)
			}
			e = nil
		}

		if e != nil && e.ttl == 0 && e.exp != 0 && ts >= e.exp {
			unlock()
			if err := deleteKey(reqCtx, key); err != nil {
				if cfg.Storage != nil {
					manager.release(e)
				}
				return fmt.Errorf("cache: failed to delete expired key %q: %w", maskKey(key), err)
			}
			relock()
			removeHeapEntry(key, e.heapidx)
			if cfg.Storage != nil {
				manager.release(e)
			}
			e = nil
			unlock()
			if ti != nil {
				ti.remove(key)
			}
			c.Set(cfg.CacheHeader, cacheUnreachable)
			goto continueRequest
		}

		if e != nil {
			entryHasPrivate := e != nil && e.private
			if !entryHasPrivate && cfg.StoreResponseHeaders && len(e.headers) > 0 {
				if cc, ok := lookupCachedHeader(e.headers, fiber.HeaderCacheControl); ok && hasDirective(utils.UnsafeString(cc), privateDirective) {
					entryHasPrivate = true
				}
			}
			requestNoCache := reqDirectives.noCache

			// Invalidate cache if requested
			if cfg.CacheInvalidator != nil && cfg.CacheInvalidator(c) {
				e.exp = ts - 1
			}

			entryHasExpiration := e != nil && e.exp != 0
			entryExpired := entryHasExpiration && ts >= e.exp
			staleness := uint64(0)
			if entryExpired {
				staleness = ts - e.exp
			}
			allowStale := entryExpired && (reqDirectives.maxStaleAny || (reqDirectives.maxStaleSet && staleness <= reqDirectives.maxStale))

			if entryExpired && e.revalidate {
				revalidate = true
				oldHeapIdx = e.heapidx
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
			}

			handleMinFresh(ts)

			if revalidate {
				unlock()
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					return c.SendStatus(fiber.StatusGatewayTimeout)
				}
				goto continueRequest
			}

			servedStale := false

			switch {
			case entryExpired && !allowStale:
				unlock()
				if err := deleteKey(reqCtx, key); err != nil {
					if e != nil {
						manager.release(e)
					}
					return fmt.Errorf("cache: failed to delete expired key %q: %w", maskKey(key), err)
				}
				relock()
				idx := e.heapidx
				manager.release(e)
				removeHeapEntry(key, idx)
				e = nil
				unlock()
				if ti != nil {
					ti.remove(key)
				}
			case entryHasPrivate:
				unlock()
				if err := deleteKey(reqCtx, key); err != nil {
					if e != nil {
						manager.release(e)
					}
					return fmt.Errorf("cache: failed to delete private response for key %q: %w", maskKey(key), err)
				}
				relock()
				removeHeapEntry(key, e.heapidx)
				if cfg.Storage != nil && e != nil {
					manager.release(e)
				}
				e = nil
				unlock()
				if ti != nil {
					ti.remove(key)
				}
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					return c.SendStatus(fiber.StatusGatewayTimeout)
				}
				return c.Next()
			case entryHasExpiration && !requestNoCache:
				servedStale = entryExpired
				if hasAuthorization && !e.shareable {
					if cfg.Storage != nil {
						manager.release(e)
					}
					unlock()
					c.Set(cfg.CacheHeader, cacheUnreachable)
					return c.Next()
				}

				// Check conditional request headers (RFC 7232).
				// ETag and Last-Modified are in the cached item; no body load needed for 304.
				{
					ifNoneMatch := c.Request().Header.Peek(fiber.HeaderIfNoneMatch)
					ifModSince := c.Request().Header.Peek(fiber.HeaderIfModifiedSince)
					notModified := false
					if len(ifNoneMatch) > 0 && len(e.etag) > 0 {
						notModified = etagWeakMatch(ifNoneMatch, e.etag)
					} else if len(ifModSince) > 0 && e.lastModified != 0 {
						if modTime, parseErr := fasthttp.ParseHTTPDate(ifModSince); parseErr == nil {
							notModified = !secondsToTime(e.lastModified).After(modTime)
						}
					}

					if notModified {
						unlock()
						c.Response().SetStatusCode(fiber.StatusNotModified)
						if len(e.etag) > 0 {
							c.Response().Header.SetBytesV(fiber.HeaderETag, e.etag)
						}
						if e.lastModified != 0 {
							lmBytes := fasthttp.AppendHTTPDate(nil, secondsToTime(e.lastModified))
							c.Response().Header.SetBytesV(fiber.HeaderLastModified, lmBytes)
						}
						if len(e.cacheControl) > 0 {
							c.Response().Header.SetBytesV(fiber.HeaderCacheControl, e.cacheControl)
						}
						if !cfg.DisableCacheControl && len(c.Response().Header.Peek(fiber.HeaderCacheControl)) == 0 {
							remaining := uint64(0)
							if e.exp > ts {
								remaining = e.exp - ts
							}
							c.Set(fiber.HeaderCacheControl, buildCacheControl(remaining, e.revalidate))
						}
						clampedDate := clampDateSeconds(e.date, ts)
						dateValue := fasthttp.AppendHTTPDate(nil, secondsToTime(clampedDate))
						c.Response().Header.SetBytesV(fiber.HeaderDate, dateValue)
						c.Set(cfg.CacheHeader, cacheHit)
						if cfg.Storage != nil {
							manager.release(e)
						}
						return nil
					}
				}

				// Separate body value to avoid msgp serialization
				// We can store raw bytes with Storage 👍
				if cfg.Storage != nil {
					unlock()
					rawBody, err := manager.getRaw(reqCtx, key+"_body")
					if err != nil {
						manager.release(e)
						return cacheBodyFetchError(maskKey, key, err)
					}
					e.body = rawBody
				} else {
					unlock()
				}
				// Set response headers from cache
				c.Response().SetBodyRaw(e.body)
				c.Response().SetStatusCode(e.status)
				c.Response().Header.SetContentTypeBytes(e.ctype)
				if len(e.cencoding) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderContentEncoding, e.cencoding)
				}
				if len(e.cacheControl) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderCacheControl, e.cacheControl)
				}
				if len(e.expires) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderExpires, e.expires)
				}
				if len(e.etag) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderETag, e.etag)
				}
				if e.lastModified != 0 {
					lmBytes := fasthttp.AppendHTTPDate(nil, secondsToTime(e.lastModified))
					c.Response().Header.SetBytesV(fiber.HeaderLastModified, lmBytes)
				}
				clampedDate := clampDateSeconds(e.date, ts)
				dateValue := fasthttp.AppendHTTPDate(nil, secondsToTime(clampedDate))
				c.Response().Header.SetBytesV(fiber.HeaderDate, dateValue)
				for i := range e.headers {
					h := e.headers[i]
					c.Response().Header.SetBytesKV(h.key, h.value)
				}
				// Set Cache-Control header if not disabled and not already set
				if !cfg.DisableCacheControl && len(c.Response().Header.Peek(fiber.HeaderCacheControl)) == 0 {
					remaining := uint64(0)
					if e.exp > ts {
						remaining = e.exp - ts
					}
					c.Set(fiber.HeaderCacheControl, buildCacheControl(remaining, e.revalidate))
				}

				const maxDeltaSeconds = uint64(math.MaxInt32)
				ageSeconds := min(entryAge, maxDeltaSeconds)

				// RFC-compliant Age header (RFC 9111)
				age := utils.FormatUint(ageSeconds)
				c.Response().Header.Set(fiber.HeaderAge, age)
				appendWarningHeaders(&c.Response().Header, servedStale, isHeuristicFreshness(e, &cfg, entryAge))

				c.Set(cfg.CacheHeader, cacheHit)

				// release item allocated from storage
				if cfg.Storage != nil {
					manager.release(e)
				}

				// Return response
				return nil
			default:
				// no cached response to serve
			}
		}

		if e == nil && revalidate {
			unlock()
			c.Set(cfg.CacheHeader, cacheUnreachable)
			if reqDirectives.onlyIfCached {
				return c.SendStatus(fiber.StatusGatewayTimeout)
			}
			goto continueRequest
		}

		if e == nil && reqDirectives.onlyIfCached {
			unlock()
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.SendStatus(fiber.StatusGatewayTimeout)
		}

		// make sure we're not blocking concurrent requests - do unlock
		unlock()

	continueRequest:
		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
		}


		// Generate ETag from response body if configured
		if cfg.ETagGenerator != nil {
			if etag := cfg.ETagGenerator(c, c.Response().Body()); etag != "" {
				c.Response().Header.Set(fiber.HeaderETag, etag)
			}
		}
		// Auto-generate ETag from body hash when none is already set
		if cfg.EnableETag && len(c.Response().Header.Peek(fiber.HeaderETag)) == 0 {
			c.Response().Header.Set(fiber.HeaderETag, generateETag(c.Response().Body()))
		}
		// Generate Last-Modified if configured
		if cfg.LastModifiedGenerator != nil {
			if lm := cfg.LastModifiedGenerator(c); !lm.IsZero() {
				lmBytes := fasthttp.AppendHTTPDate(nil, lm.UTC())
				c.Response().Header.SetBytesV(fiber.HeaderLastModified, lmBytes)
			}
		}
		// Auto-set Last-Modified to now when none is already set
		if cfg.EnableLastModified && len(c.Response().Header.Peek(fiber.HeaderLastModified)) == 0 {
			c.Response().Header.SetBytesV(fiber.HeaderLastModified, fasthttp.AppendHTTPDate(nil, time.Now().UTC()))
		}

		// Evaluate conditional request headers on the revalidation /
		// first-miss path (RFC 9110 §8.3).  The cache-hit path already
		// handles this using stored e.etag / e.lastModified; this block
		// covers the case where c.Next() ran and the ETag or Last-Modified
		// was set by the handler, a generator, or auto-generation above.
		// Per RFC 9110 §8.3: If-None-Match takes precedence;
		// If-Modified-Since is ignored when If-None-Match is present.
		{
			ifNoneMatch := c.Request().Header.Peek(fiber.HeaderIfNoneMatch)
			storedETag := c.Response().Header.Peek(fiber.HeaderETag)
			notModified := false
			if len(ifNoneMatch) > 0 && len(storedETag) > 0 {
				notModified = etagWeakMatch(ifNoneMatch, storedETag)
			} else if ifModSince := c.Request().Header.Peek(fiber.HeaderIfModifiedSince); len(ifModSince) > 0 {
				if lm := c.Response().Header.Peek(fiber.HeaderLastModified); len(lm) > 0 {
					if modTime, parseErr := fasthttp.ParseHTTPDate(ifModSince); parseErr == nil {
						if lmTime, lmErr := fasthttp.ParseHTTPDate(lm); lmErr == nil {
							notModified = !lmTime.After(modTime)
						}
					}
				}
			}
			if notModified {
				c.Response().ResetBody()
				c.Response().SetStatusCode(fiber.StatusNotModified)
				c.Set(cfg.CacheHeader, cacheMiss)
				return nil
			}
		}

		cacheControlBytes := c.Response().Header.Peek(fiber.HeaderCacheControl)
		respCacheControl := parseResponseCacheControl(cacheControlBytes)
		varyHeader := utils.UnsafeString(c.Response().Header.Peek(fiber.HeaderVary))
		hasPrivate := respCacheControl.hasPrivate
		hasNoCache := respCacheControl.hasNoCache
		varyNames, varyHasStar := parseVary(varyHeader)

		// Respect server cache-control: no-store
		if respCacheControl.hasNoStore {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		if hasPrivate || hasNoCache || varyHasStar {
			if e != nil {
				if err := deleteKey(reqCtx, key); err != nil {
					if cfg.Storage != nil {
						manager.release(e)
					}
					return fmt.Errorf("cache: failed to delete cached response for key %q: %w", maskKey(key), err)
				}
				mux.Lock()
				removeHeapEntry(key, e.heapidx)
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
				mux.Unlock()
				if ti != nil {
					ti.remove(key)
				}
			}

			if hasVaryManifest {
				if err := manager.del(reqCtx, manifestKey); err != nil {
					return fmt.Errorf("cache: failed to delete stale vary manifest %q: %w", maskKey(manifestKey), err)
				}
			}

			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		shouldStoreVaryManifest := len(varyNames) > 0
		if len(varyNames) > 0 {
			if key == baseKey {
				key += buildVaryKey(varyNames, &c.Request().Header)
			}
		} else if hasVaryManifest {
			if err := manager.del(reqCtx, manifestKey); err != nil {
				return fmt.Errorf("cache: failed to delete stale vary manifest %q: %w", maskKey(manifestKey), err)
			}
		}

		isSharedCacheAllowed := allowsSharedCacheDirectives(respCacheControl)
		if hasAuthorization && !isSharedCacheAllowed {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		sharedCacheMode := !hasAuthorization || isSharedCacheAllowed

		// Don't cache response if status code is not cacheable
		if _, ok := cacheableStatusCodes[c.Response().StatusCode()]; !ok {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

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

		// Compute tags before eviction so tag-based rejection short-circuits early
		var entryTags []string
		if cfg.Tags != nil {
			entryTags = append(entryTags, cfg.Tags(c)...)
		}
		if cfg.ResponseTags != nil {
			entryTags = append(entryTags, cfg.ResponseTags(c, c.Response().Body())...)
		}
		if reject != nil && reject.matchesAny(entryTags) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		// Eviction loop: atomically reserve space for new entry and evict old entries.
		// Strategy:
		// 1. Under lock: reserve space by pre-incrementing storedBytes, then collect entries to evict
		// 2. Outside lock: perform I/O deletions
		// 3. On deletion failure: restore storedBytes and return error
		// 4. Track reservation with a flag; unreserve on early return via defer
		var spaceReserved bool
		defer func() {
			// If we reserved space but the entry was not successfully added to heap, unreserve it
			if cfg.MaxBytes > 0 && spaceReserved {
				mux.Lock()
				storedBytes -= bodySize
				mux.Unlock()
			}
		}()

		if cfg.MaxBytes > 0 {
			mux.Lock()
			// Reserve space for the new entry first
			storedBytes += bodySize
			spaceReserved = true

			// Now evict entries until we're under the limit
			var keysToRemove []string
			var sizesToRemove []uint
			var candidates []evictionCandidate

			for storedBytes > cfg.MaxBytes {
				if heap.Len() == 0 {
					// Can't evict more, unreserve space and fail
					storedBytes -= bodySize
					// Set spaceReserved to false so the deferred cleanup does not unreserve again
					spaceReserved = false
					mux.Unlock()
					return errors.New("cache: insufficient space and no entries to evict")
				}
				next := heap.entries[0]
				keyToRemove, size := heap.removeFirst()
				if keyHeapIdx != nil {
					delete(keyHeapIdx, keyToRemove)
				}
				keysToRemove = append(keysToRemove, keyToRemove)
				sizesToRemove = append(sizesToRemove, size)
				candidates = append(candidates, evictionCandidate{
					key:  keyToRemove,
					size: size,
					exp:  next.exp,
				})
				storedBytes -= size
			}
			mux.Unlock()

			// Perform deletions outside the lock
			if len(keysToRemove) > 0 {
				for i, keyToRemove := range keysToRemove {
					delErr := deleteKey(reqCtx, keyToRemove)
					if delErr == nil {
						if ti != nil {
							ti.remove(keyToRemove)
						}
						continue
					}

					// Deletion failed: restore storedBytes for failed deletions
					mux.Lock()
					// Restore sizes of entries we failed to delete
					for j := i; j < len(sizesToRemove); j++ {
						storedBytes += sizesToRemove[j]
					}
					// Unreserve space for the new entry
					storedBytes -= bodySize
					spaceReserved = false

					// Re-add entries to the heap to keep expiration tracking consistent
					var restored []evictionCandidate
					for j := i; j < len(candidates); j++ {
						candidate := candidates[j]
						candidate.heapIdx = heap.put(candidate.key, candidate.exp, candidate.size)
						if keyHeapIdx != nil {
							keyHeapIdx[candidate.key] = candidate.heapIdx
						}
						restored = append(restored, candidate)
					}
					mux.Unlock()

					var restoreErr error
					for _, candidate := range restored {
						if err := refreshHeapIndex(reqCtx, candidate); err != nil {
							restoreErr = errors.Join(restoreErr, err)
						}
					}

					if restoreErr != nil {
						return errors.Join(fmt.Errorf("cache: failed to delete key %q while evicting: %w", maskKey(keyToRemove), delErr), restoreErr)
					}

					return fmt.Errorf("cache: failed to delete key %q while evicting: %w", maskKey(keyToRemove), delErr)
				}
			}
		}

		e = manager.acquire()
		// Cache response
		e.body = utils.CopyBytes(c.Response().Body())
		e.status = c.Response().StatusCode()
		e.ctype = utils.CopyBytes(c.Response().Header.ContentType())
		e.cencoding = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderContentEncoding))
		e.private = false
		e.cacheControl = utils.CopyBytes(cacheControlBytes)
		e.expires = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderExpires))
		e.etag = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderETag))
		e.date = 0
		// Parse Last-Modified from response header
		if lm := c.Response().Header.Peek(fiber.HeaderLastModified); len(lm) > 0 {
			if t, parseErr := fasthttp.ParseHTTPDate(lm); parseErr == nil {
				e.lastModified = safeUnixSeconds(t)
			}
		}

		e.tags = entryTags

		ageVal := uint64(0)
		if b := c.Response().Header.Peek(fiber.HeaderAge); len(b) > 0 {
			if v, err := fasthttp.ParseUint(b); err == nil {
				if v >= 0 {
					ageVal = uint64(v)
				}
			}
		} else {
			c.Response().Header.Set(fiber.HeaderAge, "0")
		}
		e.age = ageVal
		e.shareable = isSharedCacheAllowed
		now := time.Now().UTC()
		nowUnix := safeUnixSeconds(now)
		dateHeader := c.Response().Header.Peek(fiber.HeaderDate)
		parsedDate, _ := parseHTTPDate(dateHeader)
		e.date = clampDateSeconds(parsedDate, nowUnix)
		dateBytes := fasthttp.AppendHTTPDate(nil, secondsToTime(e.date))
		c.Response().Header.SetBytesV(fiber.HeaderDate, dateBytes)

		// Store all response headers
		// (more: https://datatracker.ietf.org/doc/html/rfc2616#section-13.5.1)
		if cfg.StoreResponseHeaders {
			allHeaders := c.Response().Header.All()
			e.headers = e.headers[:0]
			for key, value := range allHeaders {
				keyStr := string(key)
				if _, ok := ignoreHeaders[keyStr]; ok {
					continue
				}

				e.headers = append(e.headers, cachedHeader{
					key:   utils.CopyBytes(utils.UnsafeBytes(keyStr)),
					value: utils.CopyBytes(value),
				})
			}
		}

		expirationSource := expirationSourceConfig
		expiresParseError := false
		mustRevalidate := respCacheControl.mustRevalidate || respCacheControl.proxyRevalidate
		// default cache expiration
		expiration := cfg.Expiration
		if sharedCacheMode && respCacheControl.sMaxAgeSet {
			expiration = secondsToDuration(respCacheControl.sMaxAge)
			expirationSource = expirationSourceSMaxAge
		}
		if expirationSource == expirationSourceConfig {
			if respCacheControl.maxAgeSet {
				expiration = secondsToDuration(respCacheControl.maxAge)
				expirationSource = expirationSourceMaxAge
			} else if expiresBytes := c.Response().Header.Peek(fiber.HeaderExpires); len(expiresBytes) > 0 {
				expiresAt, err := fasthttp.ParseHTTPDate(expiresBytes)
				if err != nil {
					expiration = time.Nanosecond
					expiresParseError = true
				} else {
					expiration = time.Until(expiresAt)
				}
				expirationSource = expirationSourceExpires
			}
		}
		// Calculate expiration by response header or other setting
		if cfg.ExpirationGenerator != nil {
			expiration = cfg.ExpirationGenerator(c, &cfg)
			expirationSource = expirationSourceGenerator
		}
		e.forceRevalidate = expiresParseError
		e.revalidate = mustRevalidate

		storageExpiration := expiration
		if expiresParseError || storageExpiration < cfg.Expiration {
			storageExpiration = cfg.Expiration
		}

		if expiration <= 0 && !expiresParseError {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		ts = atomic.LoadUint64(&timestamp)
		responseTS := max(ts, nowUnix)

		maxAgeSeconds := uint64(time.Duration(math.MaxInt64) / time.Second)
		var ageDuration time.Duration
		apparentAge := e.age
		if e.date > 0 && responseTS > e.date {
			dateAge := responseTS - e.date
			if dateAge > apparentAge {
				apparentAge = dateAge
			}
		}
		if expirationSource != expirationSourceExpires {
			if apparentAge > maxAgeSeconds {
				ageDuration = expiration + time.Second
			} else {
				ageDuration = time.Duration(apparentAge) * time.Second
			}
		}
		remainingExpiration := expiration - ageDuration
		if remainingExpiration <= 0 {
			if expirationSource != expirationSourceExpires {
				c.Set(cfg.CacheHeader, cacheUnreachable)
				return nil
			}
			remainingExpiration = 0
		}

		if shouldStoreVaryManifest {
			if err := storeVaryManifest(reqCtx, manager, manifestKey, varyNames, storageExpiration); err != nil {
				return err
			}
		}

		e.exp = responseTS + uint64(remainingExpiration.Seconds())
		e.ttl = uint64(expiration.Seconds())
		if expiresParseError {
			e.exp = ts + 1
		}

		// Store entry in heap (space already reserved in eviction phase)
		var heapIdx int
		if cfg.MaxBytes > 0 {
			mux.Lock()
			heapIdx = heap.put(key, e.exp, bodySize)
			e.heapidx = heapIdx
			if keyHeapIdx != nil {
				keyHeapIdx[key] = heapIdx
			}
			// Note: storedBytes was incremented during reservation, and evictions
			// have already been accounted for, so no additional increment is needed
			spaceReserved = false // Clear flag to prevent defer from unreserving
			mux.Unlock()
		}

		cleanupOnStoreError := func(ctx context.Context, releaseEntry, rawStored bool) error {
			var cleanupErr error
			if cfg.MaxBytes > 0 {
				mux.Lock()
				_, size := heap.remove(heapIdx)
				storedBytes -= size
				if keyHeapIdx != nil {
					delete(keyHeapIdx, key)
				}
				mux.Unlock()
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
			if err := manager.setRaw(reqCtx, key+"_body", e.body, storageExpiration); err != nil {
				if cleanupErr := cleanupOnStoreError(reqCtx, true, false); cleanupErr != nil {
					err = errors.Join(err, cleanupErr)
				}
				return err
			}
			// avoid body msgp encoding
			e.body = nil
			if err := manager.set(reqCtx, key, e, storageExpiration); err != nil {
				if cleanupErr := cleanupOnStoreError(reqCtx, false, true); cleanupErr != nil {
					err = errors.Join(err, cleanupErr)
				}
				return err
			}
		} else {
			// Store entry in memory
			if err := manager.set(reqCtx, key, e, storageExpiration); err != nil {
				if cleanupErr := cleanupOnStoreError(reqCtx, true, false); cleanupErr != nil {
					err = errors.Join(err, cleanupErr)
				}
				return err
			}
		}

		// If revalidating, remove old heap entry now that replacement is successfully stored
		if cfg.MaxBytes > 0 && revalidate && oldHeapIdx >= 0 {
			mux.Lock()
			removeHeapEntry(key, oldHeapIdx)
			mux.Unlock()
		}

		// Register tags for this cache entry
		if ti != nil && len(entryTags) > 0 {
			ti.add(key, entryTags)
		}

		// Generate Cache-Control on miss when the handler did not set one
		if !cfg.DisableCacheControl && len(c.Response().Header.Peek(fiber.HeaderCacheControl)) == 0 {
			c.Set(fiber.HeaderCacheControl, buildCacheControl(uint64(remainingExpiration.Seconds()), mustRevalidate))
		}

		c.Set(cfg.CacheHeader, cacheMiss)

		// Finish response
		return nil
	}
}

// hasDirective checks if a cache-control header contains a directive (case-insensitive)
func hasDirective(cc, directive string) bool {
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

func parseUintDirective(val []byte) (uint64, bool) {
	if len(val) == 0 {
		return 0, false
	}
	parsed, err := fasthttp.ParseUint(val)
	if err != nil || parsed < 0 {
		return 0, false
	}
	return uint64(parsed), true
}

func parseCacheControlDirectives(cc []byte, fn func(key, value []byte)) {
	for i := 0; i < len(cc); {
		// skip leading separators/spaces
		for i < len(cc) && (cc[i] == ' ' || cc[i] == ',') {
			i++
		}
		if i >= len(cc) {
			break
		}

		start := i
		for i < len(cc) && cc[i] != ',' {
			i++
		}
		partEnd := i
		for partEnd > start && cc[partEnd-1] == ' ' {
			partEnd--
		}

		keyStart := start
		for keyStart < partEnd && cc[keyStart] == ' ' {
			keyStart++
		}
		if keyStart >= partEnd {
			continue
		}

		keyEnd := keyStart
		for keyEnd < partEnd && cc[keyEnd] != '=' {
			keyEnd++
		}
		// Trim trailing spaces from key
		keyEndTrimmed := keyEnd
		for keyEndTrimmed > keyStart && cc[keyEndTrimmed-1] == ' ' {
			keyEndTrimmed--
		}
		key := cc[keyStart:keyEndTrimmed]

		var value []byte
		if keyEnd < partEnd && cc[keyEnd] == '=' {
			valueStart := keyEnd + 1
			for valueStart < partEnd && cc[valueStart] == ' ' {
				valueStart++
			}
			valueEnd := partEnd
			for valueEnd > valueStart && cc[valueEnd-1] == ' ' {
				valueEnd--
			}
			if valueStart <= valueEnd {
				value = cc[valueStart:valueEnd]
				// Handle quoted-string values per RFC 9111 Section 5.2
				if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
					value = unquoteCacheDirective(value)
				}
			}
		}

		fn(key, value)
		i++ // skip comma
	}
}

// unquoteCacheDirective removes quotes and handles escaped characters in quoted-string values.
// Per RFC 9111 Section 5.2, quoted-string values follow RFC 9110 Section 5.6.4.
func unquoteCacheDirective(quoted []byte) []byte {
	if len(quoted) < 2 {
		return quoted
	}

	// Remove surrounding quotes
	inner := quoted[1 : len(quoted)-1]

	// Check if there are any escaped characters (backslash followed by another character)
	hasEscapes := false
	for i := 0; i < len(inner)-1; i++ {
		if inner[i] == '\\' {
			hasEscapes = true
			break
		}
	}

	// If no escapes, return the inner content directly
	if !hasEscapes {
		return inner
	}

	// Process escaped characters
	result := make([]byte, 0, len(inner))
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) {
			// Skip the backslash and take the next character
			i++
			result = append(result, inner[i])
		} else {
			result = append(result, inner[i])
		}
	}

	return result
}

type responseCacheControl struct {
	maxAge          uint64
	sMaxAge         uint64
	maxAgeSet       bool
	sMaxAgeSet      bool
	hasNoCache      bool
	hasNoStore      bool
	hasPrivate      bool
	hasPublic       bool
	mustRevalidate  bool
	proxyRevalidate bool
}

func parseResponseCacheControl(cc []byte) responseCacheControl {
	parsed := responseCacheControl{}
	parseCacheControlDirectives(cc, func(key, value []byte) {
		switch {
		case utils.EqualFold(utils.UnsafeString(key), noStore):
			parsed.hasNoStore = true
		case utils.EqualFold(utils.UnsafeString(key), noCache):
			parsed.hasNoCache = true
		case utils.EqualFold(utils.UnsafeString(key), privateDirective):
			parsed.hasPrivate = true
		case utils.EqualFold(utils.UnsafeString(key), "public"):
			parsed.hasPublic = true
		case utils.EqualFold(utils.UnsafeString(key), "max-age"):
			if v, ok := parseUintDirective(value); ok {
				parsed.maxAgeSet = true
				parsed.maxAge = v
			}
		case utils.EqualFold(utils.UnsafeString(key), "s-maxage"):
			if v, ok := parseUintDirective(value); ok {
				parsed.sMaxAgeSet = true
				parsed.sMaxAge = v
			}
		case utils.EqualFold(utils.UnsafeString(key), "must-revalidate"):
			parsed.mustRevalidate = true
		case utils.EqualFold(utils.UnsafeString(key), "proxy-revalidate"):
			parsed.proxyRevalidate = true
		default:
			// ignore unknown directives
		}
	})
	return parsed
}

// parseMaxAge extracts the max-age directive from a Cache-Control header.
func parseMaxAge(cc string) (time.Duration, bool) {
	parsed := parseResponseCacheControl(utils.UnsafeBytes(cc))
	if !parsed.maxAgeSet {
		return 0, false
	}
	return secondsToDuration(parsed.maxAge), true
}

func parseRequestCacheControl(cc []byte) requestCacheDirectives {
	directives := requestCacheDirectives{}
	parseCacheControlDirectives(cc, func(key, value []byte) {
		switch {
		case utils.EqualFold(utils.UnsafeString(key), noStore):
			directives.noStore = true
		case utils.EqualFold(utils.UnsafeString(key), noCache):
			directives.noCache = true
		case utils.EqualFold(utils.UnsafeString(key), "only-if-cached"):
			directives.onlyIfCached = true
		case utils.EqualFold(utils.UnsafeString(key), "max-age"):
			if sec, ok := parseUintDirective(value); ok {
				directives.maxAgeSet = true
				directives.maxAge = sec
			}
		case utils.EqualFold(utils.UnsafeString(key), "max-stale"):
			directives.maxStaleSet = true
			directives.maxStaleAny = len(value) == 0
			if !directives.maxStaleAny {
				if sec, ok := parseUintDirective(value); ok {
					directives.maxStale = sec
				}
			}
		case utils.EqualFold(utils.UnsafeString(key), "min-fresh"):
			if sec, ok := parseUintDirective(value); ok {
				directives.minFreshSet = true
				directives.minFresh = sec
			}
		default:
			// ignore unknown directives
		}
	})
	return directives
}

func parseRequestCacheControlString(cc string) requestCacheDirectives {
	return parseRequestCacheControl(utils.UnsafeBytes(cc))
}

func cachedResponseAge(e *item, now uint64) uint64 {
	clampedDate := clampDateSeconds(e.date, now)

	resident := uint64(0)
	if e.exp != 0 {
		if e.exp <= now {
			resident = e.ttl + (now - e.exp)
		} else {
			resident = e.ttl - (e.exp - now)
		}
	}

	dateAge := uint64(0)
	if clampedDate != 0 && now > clampedDate {
		dateAge = now - clampedDate
	}

	currentAge := max(dateAge, max(resident, e.age))
	return currentAge
}

func appendWarningHeaders(h *fasthttp.ResponseHeader, servedStale, heuristicFreshness bool) { //nolint:revive // flags are intentional to represent Warning variants
	if servedStale {
		h.Add(fiber.HeaderWarning, `110 - "Response is stale"`)
	}
	if heuristicFreshness {
		h.Add(fiber.HeaderWarning, `113 - "Heuristic expiration"`)
	}
}

func remainingFreshness(e *item, now uint64) uint64 {
	if e == nil || e.exp == 0 || now >= e.exp {
		return 0
	}

	return e.exp - now
}

func isHeuristicFreshness(e *item, cfg *Config, entryAge uint64) bool {
	const heuristicAgeThresholdSeconds = uint64(24 * time.Hour / time.Second)
	if entryAge <= heuristicAgeThresholdSeconds {
		return false
	}

	if len(e.expires) > 0 {
		return false
	}

	cacheControl := utils.UnsafeString(e.cacheControl)
	if parsedCC := parseResponseCacheControl(utils.UnsafeBytes(cacheControl)); parsedCC.maxAgeSet || parsedCC.sMaxAgeSet {
		return false
	}

	return cfg.Expiration > 0
}

func lookupCachedHeader(headers []cachedHeader, name string) ([]byte, bool) {
	for i := range headers {
		if utils.EqualFold(utils.UnsafeString(headers[i].key), name) {
			return headers[i].value, true
		}
	}
	return nil, false
}

func parseHTTPDate(dateBytes []byte) (uint64, bool) {
	if len(dateBytes) == 0 {
		return 0, false
	}
	parsedDate, err := fasthttp.ParseHTTPDate(dateBytes)
	if err != nil {
		return 0, false
	}

	return safeUnixSeconds(parsedDate), true
}

func clampDateSeconds(dateSeconds, fallback uint64) uint64 {
	const maxUnixSeconds = uint64(math.MaxInt64)
	if dateSeconds == 0 || dateSeconds > maxUnixSeconds || dateSeconds > fallback {
		return fallback
	}

	return dateSeconds
}

func safeUnixSeconds(t time.Time) uint64 {
	sec := t.Unix()
	if sec < 0 {
		return 0
	}

	return uint64(sec)
}

func secondsToTime(sec uint64) time.Time {
	var clamped int64
	if sec > uint64(math.MaxInt64) {
		clamped = math.MaxInt64
	} else {
		clamped = int64(sec)
	}

	return time.Unix(clamped, 0).UTC()
}

func secondsToDuration(sec uint64) time.Duration {
	const maxSeconds = uint64(math.MaxInt64) / uint64(time.Second)
	if sec > maxSeconds {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(sec) * time.Second
}

func parseVary(vary string) ([]string, bool) {
	names := make([]string, 0, 8)
	for part := range strings.SplitSeq(vary, ",") {
		name := utils.TrimSpace(utils.ToLower(part))
		if name == "" {
			continue
		}
		if name == "*" {
			return nil, true
		}
		names = append(names, name)
	}

	if len(names) == 0 {
		return nil, false
	}

	sort.Strings(names)
	return names, false
}

func makeBuildVaryKeyFunc(hexBufPool *sync.Pool) func([]string, *fasthttp.RequestHeader) string {
	return func(names []string, hdr *fasthttp.RequestHeader) string {
		sum := sha256.New()
		for _, name := range names {
			_, _ = sum.Write(utils.UnsafeBytes(name)) //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write([]byte{0})               //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write(hdr.Peek(name))          //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write([]byte{0})               //nolint:errcheck // hash.Hash.Write for std hashes never errors
		}

		var hashBytes [sha256.Size]byte
		sum.Sum(hashBytes[:0])

		v := hexBufPool.Get()
		bufPtr, ok := v.(*[]byte)
		if !ok || bufPtr == nil {
			b := make([]byte, hexLen)
			bufPtr = &b
		}

		buf := *bufPtr
		// Defensive in case someone changed Pool.New or Put a different sized buffer.
		if cap(buf) < hexLen {
			buf = make([]byte, hexLen)
		} else {
			buf = buf[:hexLen]
		}
		*bufPtr = buf

		hex.Encode(buf, hashBytes[:])
		result := "|vary|" + string(buf)

		hexBufPool.Put(bufPtr)
		return result
	}
}

func storeVaryManifest(ctx context.Context, manager *manager, manifestKey string, names []string, exp time.Duration) error {
	if len(names) == 0 {
		return nil
	}
	data := strings.Join(names, ",")
	return manager.setRaw(ctx, manifestKey, utils.UnsafeBytes(data), exp)
}

//nolint:gocritic // returning explicit values keeps the signature concise while avoiding unnecessary named results
func loadVaryManifest(ctx context.Context, manager *manager, manifestKey string) ([]string, bool, error) {
	raw, err := manager.getRaw(ctx, manifestKey)
	if err != nil {
		if errors.Is(err, errCacheMiss) {
			return nil, false, nil
		}
		return nil, false, err
	}
	manifest := utils.UnsafeString(raw)
	names, hasStar := parseVary(manifest)
	if hasStar {
		return nil, false, nil
	}
	return names, len(names) > 0, nil
}

func allowsSharedCacheDirectives(cc responseCacheControl) bool {
	if cc.hasPrivate {
		return false
	}
	if cc.hasPublic || cc.sMaxAgeSet || cc.mustRevalidate || cc.proxyRevalidate {
		return true
	}

	// RFC 9111 §4.2.2 permits Expires as an absolute expiry for cacheable responses, but for
	// authenticated requests §3.6 requires an explicit shared-cache directive. Therefore,
	// an Expires header alone MUST NOT allow sharing when Authorization is present.
	return false
}

func allowsSharedCache(cc string) bool {
	return allowsSharedCacheDirectives(parseResponseCacheControl(utils.UnsafeBytes(cc)))
}

func makeHashAuthFunc(hexBufPool *sync.Pool) func([]byte) string {
	return func(authHeader []byte) string {
		sum := sha256.Sum256(authHeader)

		v := hexBufPool.Get()
		bufPtr, ok := v.(*[]byte)
		if !ok || bufPtr == nil {
			b := make([]byte, hexLen)
			bufPtr = &b
		}

		buf := *bufPtr
		if cap(buf) < hexLen {
			buf = make([]byte, hexLen)
		} else {
			buf = buf[:hexLen]
		}
		*bufPtr = buf

		hex.Encode(buf, sum[:])
		result := string(buf)

		hexBufPool.Put(bufPtr)
		return result
	}
}

// generateETag computes a strong ETag value from the response body.
// The result is a quoted SHA-256 hex digest per RFC 7232 §2.3.
func generateETag(body []byte) string {
	h := sha256.Sum256(body)
	return `"` + hex.EncodeToString(h[:]) + `"`
}

// stripETagWeakPrefix removes the W/ prefix from an ETag value if present.
func stripETagWeakPrefix(etag string) string {
	if len(etag) >= 2 && etag[0] == 'W' && etag[1] == '/' {
		return etag[2:]
	}
	return etag
}

// etagWeakMatch performs weak ETag comparison per RFC 7232 §2.3.2.
// It returns true if the If-None-Match header value matches the stored ETag.
// The If-None-Match value may be "*" (matches any ETag) or a comma-separated
// list of ETags; the W/ weak-validator prefix is stripped before comparison.
func etagWeakMatch(ifNoneMatch, storedETag []byte) bool {
	stored := stripETagWeakPrefix(utils.UnsafeString(storedETag))
	header := utils.UnsafeString(ifNoneMatch)
	if header == "*" {
		return true
	}

	for len(header) > 0 {
		// Skip separators
		for len(header) > 0 && (header[0] == ' ' || header[0] == ',') {
			header = header[1:]
		}
		if len(header) == 0 {
			break
		}

		end := strings.IndexByte(header, ',')
		var candidate string
		if end < 0 {
			candidate = header
			header = ""
		} else {
			candidate = header[:end]
			header = header[end+1:]
		}

		candidate = strings.TrimRight(candidate, " ")
		if stripETagWeakPrefix(candidate) == stored {
			return true
		}
	}

	return false
}

// buildCacheControl constructs a Cache-Control response header value.
// It always includes "public, max-age=<remainingSec>". When mustRevalidate
// is true it appends ", must-revalidate" per RFC 9111 §5.2.2.8.
func buildCacheControl(remainingSec uint64, mustRevalidate bool) string {
	s := "public, max-age=" + utils.FormatUint(remainingSec)
	if mustRevalidate {
		s += ", must-revalidate"
	}
	return s
}
