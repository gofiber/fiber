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
	"net/http"
	"slices"
	"sort"
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
	fiber.StatusPermanentRedirect:           {},
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
		timestamp = safeUnixSeconds(time.Now())
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
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		hasAuthorization := len(c.Request().Header.Peek(fiber.HeaderAuthorization)) > 0
		reqCacheControl := utils.UnsafeString(c.Request().Header.Peek(fiber.HeaderCacheControl))
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
		// TODO(allocation optimization): try to minimize the allocation from 2 to 1
		baseKey := cfg.KeyGenerator(c) + "_" + requestMethod
		if hasAuthorization {
			baseKey += "_auth_" + hashAuthorization(c.Request().Header.Peek(fiber.HeaderAuthorization))
		}
		manifestKey := baseKey + "|vary"
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
		entryAge := uint64(0)
		revalidate := false

		// Lock entry
		mux.Lock()
		// Get timestamp
		ts := atomic.LoadUint64(&timestamp)

		// Cache Entry found
		if e != nil {
			entryAge = cachedResponseAge(e, ts)
			if reqDirectives.maxAgeSet && (reqDirectives.maxAge == 0 || entryAge > reqDirectives.maxAge) {
				revalidate = true
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
			}

			remainingFreshness := uint64(0)
			if e != nil && e.exp != 0 && ts < e.exp {
				remainingFreshness = e.exp - ts
			}
			if e != nil && reqDirectives.minFreshSet && remainingFreshness < reqDirectives.minFresh {
				revalidate = true
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
			}
		}

		if e != nil && e.ttl == 0 && e.forceRevalidate {
			revalidate = true
			if cfg.Storage != nil {
				manager.release(e)
			}
			e = nil
		}

		if e != nil && e.ttl == 0 && e.exp != 0 && ts >= e.exp {
			if err := deleteKey(reqCtx, key); err != nil {
				if cfg.Storage != nil {
					manager.release(e)
				}
				mux.Unlock()
				return fmt.Errorf("cache: failed to delete expired key %q: %w", maskKey(key), err)
			}
			removeHeapEntry(key, e.heapidx)
			if cfg.Storage != nil {
				manager.release(e)
			}
			e = nil
			mux.Unlock()
			c.Set(cfg.CacheHeader, cacheUnreachable)
			goto continueRequest
		}

		if e != nil && e.forceRevalidate {
			revalidate = true
			if cfg.Storage != nil {
				manager.release(e)
			}
			e = nil
		}

		if e != nil {
			entryHasPrivate := e != nil && e.private
			if !entryHasPrivate && cfg.StoreResponseHeaders && len(e.headers) > 0 {
				if cc, ok := e.headers[fiber.HeaderCacheControl]; ok && hasDirective(utils.UnsafeString(cc), privateDirective) {
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
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
			}

			remainingFreshness := uint64(0)
			if e != nil && entryHasExpiration && ts < e.exp {
				remainingFreshness = e.exp - ts
			}
			if e != nil && reqDirectives.minFreshSet && remainingFreshness < reqDirectives.minFresh {
				revalidate = true
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
			}

			if revalidate {
				mux.Unlock()
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					return c.SendStatus(fiber.StatusGatewayTimeout)
				}
				goto continueRequest
			}

			servedStale := false

			switch {
			case entryExpired && !allowStale:
				if err := deleteKey(reqCtx, key); err != nil {
					if e != nil {
						manager.release(e)
					}
					mux.Unlock()
					return fmt.Errorf("cache: failed to delete expired key %q: %w", maskKey(key), err)
				}
				idx := e.heapidx
				manager.release(e)
				removeHeapEntry(key, idx)
				e = nil
			case entryHasPrivate:
				if err := deleteKey(reqCtx, key); err != nil {
					if e != nil {
						manager.release(e)
					}
					mux.Unlock()
					return fmt.Errorf("cache: failed to delete private response for key %q: %w", maskKey(key), err)
				}
				removeHeapEntry(key, e.heapidx)
				if cfg.Storage != nil && e != nil {
					manager.release(e)
				}
				e = nil
				mux.Unlock()
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
					mux.Unlock()
					c.Set(cfg.CacheHeader, cacheUnreachable)
					return c.Next()
				}

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
				if len(e.cacheControl) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderCacheControl, e.cacheControl)
				}
				if len(e.expires) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderExpires, e.expires)
				}
				if len(e.etag) > 0 {
					c.Response().Header.SetBytesV(fiber.HeaderETag, e.etag)
				}
				e.date = clampDateSeconds(e.date, ts)
				dateStr := secondsToTime(e.date).Format(http.TimeFormat)
				c.Response().Header.Set(fiber.HeaderDate, dateStr)
				for k, v := range e.headers {
					c.Response().Header.SetBytesV(k, v)
				}
				if len(c.Response().Header.Peek(fiber.HeaderCacheControl)) == 0 && !cfg.DisableCacheControl {
					remaining := uint64(0)
					if e.exp > ts {
						remaining = e.exp - ts
					}
					maxAge := strconv.FormatUint(remaining, 10)
					c.Set(fiber.HeaderCacheControl, "public, max-age="+maxAge)
				}

				const maxDeltaSeconds = uint64(math.MaxInt32)
				ageSeconds := min(entryAge, maxDeltaSeconds)

				age := strconv.FormatUint(ageSeconds, 10)
				c.Response().Header.Set(fiber.HeaderAge, age)
				appendWarningHeaders(&c.Response().Header, servedStale, isHeuristicFreshness(e, &cfg, entryAge))

				c.Set(cfg.CacheHeader, cacheHit)

				// release item allocated from storage
				if cfg.Storage != nil {
					manager.release(e)
				}

				mux.Unlock()

				// Return response
				return nil
			default:
				// no cached response to serve
			}
		}

		if e == nil && revalidate {
			mux.Unlock()
			c.Set(cfg.CacheHeader, cacheUnreachable)
			if reqDirectives.onlyIfCached {
				return c.SendStatus(fiber.StatusGatewayTimeout)
			}
			goto continueRequest
		}

		if e == nil && reqDirectives.onlyIfCached {
			mux.Unlock()
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.SendStatus(fiber.StatusGatewayTimeout)
		}

		// make sure we're not blocking concurrent requests - do unlock
		mux.Unlock()

	continueRequest:
		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
		}

		cacheControl := utils.UnsafeString(c.Response().Header.Peek(fiber.HeaderCacheControl))
		varyHeader := utils.UnsafeString(c.Response().Header.Peek(fiber.HeaderVary))
		hasExpires := len(c.Response().Header.Peek(fiber.HeaderExpires)) > 0
		hasPrivate := hasDirective(cacheControl, privateDirective)
		hasNoCache := hasDirective(cacheControl, noCache)
		varyNames, varyHasStar, releaseVaryNames := parseVary(varyHeader)
		defer releaseVaryNames()

		// Respect server cache-control: no-store
		if hasDirective(cacheControl, noStore) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		if hasPrivate || hasNoCache || varyHasStar {
			if e != nil {
				mux.Lock()
				if err := deleteKey(reqCtx, key); err != nil {
					if cfg.Storage != nil {
						manager.release(e)
					}
					mux.Unlock()
					return fmt.Errorf("cache: failed to delete cached response for key %q: %w", maskKey(key), err)
				}
				removeHeapEntry(key, e.heapidx)
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
				mux.Unlock()
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

		isSharedCacheAllowed := allowsSharedCache(cacheControl, hasExpires)
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
		e.private = false
		e.cacheControl = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderCacheControl))
		e.expires = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderExpires))
		e.etag = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderETag))
		e.date = 0

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
		dateStr := secondsToTime(e.date).Format(http.TimeFormat)
		c.Response().Header.Set(fiber.HeaderDate, dateStr)

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

		expirationSource := expirationSourceConfig
		expiresParseError := false
		mustRevalidate := false
		// default cache expiration
		expiration := cfg.Expiration
		if sharedCacheMode {
			if v, ok := parseSMaxAge(cacheControl); ok {
				expiration = v
				expirationSource = expirationSourceSMaxAge
			}
		}
		if expirationSource == expirationSourceConfig {
			if v, ok := parseMaxAge(cacheControl); ok {
				expiration = v
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
		mustRevalidate = hasDirective(cacheControl, "must-revalidate") || hasDirective(cacheControl, "proxy-revalidate")
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

// parseMaxAge extracts the max-age directive from a Cache-Control header.
func parseMaxAge(cc string) (time.Duration, bool) {
	for part := range strings.SplitSeq(cc, ",") {
		part = utils.TrimSpace(utils.ToLower(part))
		if after, ok := strings.CutPrefix(part, "max-age="); ok {
			if sec, err := strconv.Atoi(after); err == nil {
				return time.Duration(sec) * time.Second, true
			}
		}
	}
	return 0, false
}

func parseSMaxAge(cc string) (time.Duration, bool) {
	for part := range strings.SplitSeq(cc, ",") {
		part = utils.TrimSpace(utils.ToLower(part))
		if after, ok := strings.CutPrefix(part, "s-maxage="); ok {
			if sec, err := strconv.Atoi(after); err == nil {
				return time.Duration(sec) * time.Second, true
			}
		}
	}

	return 0, false
}

func parseRequestCacheControl(cc string) requestCacheDirectives {
	directives := requestCacheDirectives{}

	for part := range strings.SplitSeq(cc, ",") {
		part = utils.TrimSpace(utils.ToLower(part))
		switch {
		case part == "":
			continue
		case part == noStore:
			directives.noStore = true
		case part == noCache:
			directives.noCache = true
		case part == "only-if-cached":
			directives.onlyIfCached = true
		case strings.HasPrefix(part, "max-age="):
			if sec, err := strconv.Atoi(strings.TrimPrefix(part, "max-age=")); err == nil && sec >= 0 {
				directives.maxAgeSet = true
				directives.maxAge = uint64(sec)
			}
		case part == "max-stale":
			directives.maxStaleSet = true
			directives.maxStaleAny = true
		case strings.HasPrefix(part, "max-stale="):
			if sec, err := strconv.Atoi(strings.TrimPrefix(part, "max-stale=")); err == nil && sec >= 0 {
				directives.maxStaleSet = true
				directives.maxStale = uint64(sec)
			}
		case strings.HasPrefix(part, "min-fresh="):
			if sec, err := strconv.Atoi(strings.TrimPrefix(part, "min-fresh=")); err == nil && sec >= 0 {
				directives.minFreshSet = true
				directives.minFresh = uint64(sec)
			}
		default:
			continue
		}
	}

	return directives
}

func cachedResponseAge(e *item, now uint64) uint64 {
	e.date = clampDateSeconds(e.date, now)

	resident := uint64(0)
	if e.exp != 0 {
		if e.exp <= now {
			resident = e.ttl + (now - e.exp)
		} else {
			resident = e.ttl - (e.exp - now)
		}
	}

	dateAge := uint64(0)
	if e.date != 0 && now > e.date {
		dateAge = now - e.date
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

func isHeuristicFreshness(e *item, cfg *Config, entryAge uint64) bool {
	const heuristicAgeThresholdSeconds = uint64(24 * time.Hour / time.Second)
	if entryAge <= heuristicAgeThresholdSeconds {
		return false
	}

	if len(e.expires) > 0 {
		return false
	}

	cacheControl := utils.UnsafeString(e.cacheControl)
	if hasDirective(cacheControl, "max-age") || hasDirective(cacheControl, "s-maxage") {
		return false
	}

	return cfg.Expiration > 0
}

func parseHTTPDate(dateBytes []byte) (uint64, bool) {
	parsedDate, err := http.ParseTime(utils.UnsafeString(dateBytes))
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

var varyNamesPool = sync.Pool{
	New: func() any {
		names := make([]string, 0, 8)
		return &names
	},
}

//nolint:nonamedreturns // gocritic unnamedResult prefers naming vary parsing results for clarity
func parseVary(vary string) (names []string, hasStar bool, release func()) {
	namesPtr, ok := varyNamesPool.Get().(*[]string)
	if !ok {
		fresh := make([]string, 0, 8)
		namesPtr = &fresh
	}
	names = (*namesPtr)[:0]
	release = func() {
		*namesPtr = (*namesPtr)[:0]
		varyNamesPool.Put(namesPtr)
	}
	for part := range strings.SplitSeq(vary, ",") {
		name := utils.TrimSpace(utils.ToLower(part))
		if name == "" {
			continue
		}
		if name == "*" {
			return nil, true, release
		}
		names = append(names, name)
	}

	if len(names) == 0 {
		return nil, false, release
	}

	sort.Strings(names)
	return names, false, release
}

func buildVaryKey(names []string, hdr *fasthttp.RequestHeader) string {
	sum := sha256.New()
	for _, name := range names {
		if _, err := sum.Write(utils.UnsafeBytes(name)); err != nil {
			return ""
		}
		if _, err := sum.Write([]byte{0}); err != nil {
			return ""
		}
		if _, err := sum.Write(hdr.Peek(name)); err != nil {
			return ""
		}
		if _, err := sum.Write([]byte{0}); err != nil {
			return ""
		}
	}
	return "|vary|" + hex.EncodeToString(sum.Sum(nil))
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
	names, hasStar, releaseNames := parseVary(manifest)
	defer releaseNames()
	if hasStar {
		return nil, false, nil
	}
	return names, len(names) > 0, nil
}

func allowsSharedCache(cc string, _ bool) bool {
	shareable := false

	for part := range strings.SplitSeq(cc, ",") {
		part = utils.TrimSpace(utils.ToLower(part))
		switch {
		case part == "":
			continue
		case part == "private":
			return false
		case part == "public":
			shareable = true
		case strings.HasPrefix(part, "s-maxage="):
			shareable = true
		case part == "must-revalidate":
			shareable = true
		case part == "proxy-revalidate":
			shareable = true
		default:
			continue
		}
	}

	if shareable {
		return true
	}

	// RFC 9111 §4.2.2 permits Expires as an absolute expiry for cacheable responses, but for
	// authenticated requests §3.6 requires an explicit shared-cache directive. Therefore,
	// an Expires header alone MUST NOT allow sharing when Authorization is present.
	return false
}

func hashAuthorization(authHeader []byte) string {
	sum := sha256.Sum256(authHeader)
	return hex.EncodeToString(sum[:])
}
