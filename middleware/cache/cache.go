// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
)

// buffer size for hexpool
// hexLen is the hex-encoded length of a SHA-256 sum, shared by the auth and vary hashers.
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

func sameCachedEntry(a, b *item) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.date != b.date ||
		a.status != b.status ||
		a.age != b.age ||
		a.exp != b.exp ||
		a.ttl != b.ttl ||
		a.forceRevalidate != b.forceRevalidate ||
		a.revalidate != b.revalidate ||
		a.shareable != b.shareable ||
		a.private != b.private ||
		a.heapidx != b.heapidx {
		return false
	}
	if !slices.Equal(a.body, b.body) ||
		!slices.Equal(a.ctype, b.ctype) ||
		!slices.Equal(a.cencoding, b.cencoding) ||
		!slices.Equal(a.cacheControl, b.cacheControl) ||
		!slices.Equal(a.expires, b.expires) ||
		!slices.Equal(a.etag, b.etag) {
		return false
	}
	if len(a.headers) != len(b.headers) {
		return false
	}
	for i := range a.headers {
		if !slices.Equal(a.headers[i].key, b.headers[i].key) ||
			!slices.Equal(a.headers[i].value, b.headers[i].value) {
			return false
		}
	}
	return true
}

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

type cacheLockGuard struct {
	mux    *sync.Mutex
	locked bool
}

func newCacheLockGuard(mux *sync.Mutex) *cacheLockGuard {
	mux.Lock()
	return &cacheLockGuard{
		mux:    mux,
		locked: true,
	}
}

func (g *cacheLockGuard) unlock() {
	if g.locked {
		g.mux.Unlock()
		g.locked = false
	}
}

func (g *cacheLockGuard) relock() {
	if !g.locked {
		g.mux.Lock()
		g.locked = true
	}
}

func withCacheLock(mux *sync.Mutex, fn func()) {
	guard := newCacheLockGuard(mux)
	defer guard.unlock()
	fn()
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

	// Cache settings
	mux := &sync.Mutex{}
	type keyedCacheLock struct {
		mu   sync.Mutex
		refs int
	}
	keyLocks := make(map[string]*keyedCacheLock)
	withKeyLock := func(entryKey string, fn func() error) error {
		var entryLock *keyedCacheLock
		withCacheLock(mux, func() {
			entryLock = keyLocks[entryKey]
			if entryLock == nil {
				entryLock = &keyedCacheLock{}
				keyLocks[entryKey] = entryLock
			}
			entryLock.refs++
		})
		entryLock.mu.Lock()
		defer func() {
			entryLock.mu.Unlock()
			withCacheLock(mux, func() {
				entryLock.refs--
				if entryLock.refs == 0 {
					delete(keyLocks, entryKey)
				}
			})
		}()
		return fn()
	}
	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage, redactKeys)
	// Create indexed heap for tracking expirations ( see heap.go )
	heap := &indexedHeap{}
	// count stored bytes (sizes of response bodies)
	var storedBytes uint
	// Pool for hex encoding buffers
	hexBufPool := &sync.Pool{
		New: func() any {
			buf := make([]byte, hexLen)
			return &buf
		},
	}
	hashAuthorization := makeHashAuthFunc(hexBufPool)
	buildVaryKey := makeBuildVaryKeyFunc(hexBufPool)

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
	deleteLockedKey := func(ctx context.Context, dkey string) error {
		return withKeyLock(dkey, func() error {
			return deleteKey(ctx, dkey)
		})
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

	refreshHeapIndex := func(ctx context.Context, candidate evictionCandidate) error {
		entry, err := manager.get(ctx, candidate.key)
		if err != nil {
			if errors.Is(err, errCacheMiss) {
				return nil
			}
			return fmt.Errorf("cache: failed to reload key %q after eviction failure: %w", maskKey(candidate.key), err)
		}

		entry.heapidx = candidate.heapIdx

		remainingTTL := max(secondsToTime(entry.exp).Sub(cfg.now()), 0)

		if err := manager.set(ctx, candidate.key, entry, remainingTTL); err != nil {
			return fmt.Errorf("cache: failed to restore heap index for key %q: %w", maskKey(candidate.key), err)
		}

		return nil
	}

	// Return new handler
	return func(c fiber.Ctx) error {
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

		// Only cache methods listed in cfg.Methods (default: GET, HEAD).
		if !slices.Contains(cfg.Methods, requestMethod) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.Next()
		}

		// Get key from request
		baseKey := requestMethod + "|" + cfg.KeyGenerator(c)
		manifestKey := baseKey + "|vary"
		if hasAuthorization {
			authHash := hashAuthorization(c.Request().Header.Peek(fiber.HeaderAuthorization))
			baseKey += "|auth=" + authHash
			manifestKey = baseKey + "|vary"
		}
		key := baseKey

		reqCtx := c.Context()

		varyNames := []string(nil)
		hasVaryManifest := false
		var err error
		if !cfg.DisableVaryHeaders {
			varyNames, hasVaryManifest, err = loadVaryManifest(reqCtx, manager, manifestKey)
			if err != nil {
				return err
			}
			if len(varyNames) > 0 {
				key += buildVaryKey(varyNames, &c.Request().Header)
			}
		}

		// Get entry from pool
		e, err := manager.get(reqCtx, key)
		if err != nil && !errors.Is(err, errCacheMiss) {
			return err
		}
		entryAge := uint64(0)
		revalidate := false
		oldHeapIdx := -1 // Track old heap index for replacement during revalidation
		revalidationBodyMatches := cfg.Storage == nil
		var revalidationEntry *item
		var revalidationBody []byte

		markRevalidate := func() {
			revalidate = true
			oldHeapIdx = e.heapidx
			if revalidationEntry == nil {
				snapshot := *e
				revalidationEntry = &snapshot
			}
			if cfg.Storage != nil {
				manager.release(e)
			}
			e = nil
		}

		handleMinFresh := func(now uint64) {
			if e == nil || !reqDirectives.minFreshSet {
				return
			}
			remainingFreshness := remainingFreshness(e, now)
			if remainingFreshness < reqDirectives.minFresh {
				markRevalidate()
			}
		}

		deleteCurrentEntry := func(guard *cacheLockGuard, wrapErr func(error) error) error {
			if guard != nil {
				guard.unlock()
			}
			if delErr := deleteLockedKey(reqCtx, key); delErr != nil {
				manager.release(e)
				return wrapErr(delErr)
			}

			removeEntry := func() {
				removeHeapEntry(key, e.heapidx)
				manager.release(e)
				e = nil
			}
			if guard != nil {
				guard.relock()
				removeEntry()
				return nil
			}
			withCacheLock(mux, removeEntry)
			return nil
		}

		loadRevalidationBody := func() error {
			if cfg.Storage == nil || revalidationEntry == nil || revalidationBodyMatches {
				return nil
			}
			return withKeyLock(key, func() error {
				body, bodyErr := manager.getRaw(reqCtx, key+"_body")
				if bodyErr != nil {
					if errors.Is(bodyErr, errCacheMiss) {
						return nil
					}
					return cacheBodyFetchError(maskKey, key, bodyErr)
				}
				revalidationBody = utils.CopyBytes(body)
				revalidationBodyMatches = true
				return nil
			})
		}

		deleteRevalidatedEntry := func() error {
			if revalidationEntry == nil {
				return nil
			}

			return withKeyLock(key, func() error {
				current, getErr := manager.get(reqCtx, key)
				if getErr != nil {
					if errors.Is(getErr, errCacheMiss) {
						return nil
					}
					return fmt.Errorf("cache: failed to reload cached response for key %q before deletion: %w", maskKey(key), getErr)
				}

				matchesStaleEntry := sameCachedEntry(current, revalidationEntry)
				if cfg.Storage != nil {
					manager.release(current)
				}
				if !matchesStaleEntry {
					return nil
				}
				if !revalidationBodyMatches {
					return nil
				}
				if cfg.Storage != nil {
					currentBody, bodyErr := manager.getRaw(reqCtx, key+"_body")
					if bodyErr != nil {
						if errors.Is(bodyErr, errCacheMiss) {
							return nil
						}
						return cacheBodyFetchError(maskKey, key, bodyErr)
					}
					if !slices.Equal(currentBody, revalidationBody) {
						return nil
					}
				}

				if delErr := deleteKey(reqCtx, key); delErr != nil {
					return fmt.Errorf("cache: failed to delete cached response for key %q: %w", maskKey(key), delErr)
				}
				if cfg.MaxBytes > 0 && oldHeapIdx >= 0 {
					withCacheLock(mux, func() {
						removeHeapEntry(key, oldHeapIdx)
					})
				}
				return nil
			})
		}

		handledCacheRequest, err := func() (bool, error) {
			// Lock entry before reading the current timestamp so freshness decisions
			// are based on the time the protected cache entry is evaluated.
			guard := newCacheLockGuard(mux)
			defer guard.unlock()
			ts := safeUnixSeconds(cfg.now())

			// Cache Entry found
			if e != nil {
				entryAge = cachedResponseAge(e, ts)
				if reqDirectives.maxAgeSet && (reqDirectives.maxAge == 0 || entryAge > reqDirectives.maxAge) {
					markRevalidate()
				}

				handleMinFresh(ts)
			}

			if e != nil && e.ttl == 0 && e.forceRevalidate {
				markRevalidate()
			}

			if e != nil && e.ttl == 0 && e.exp != 0 && ts >= e.exp {
				if deleteErr := deleteCurrentEntry(guard, func(delErr error) error {
					return fmt.Errorf("cache: failed to delete expired key %q: %w", maskKey(key), delErr)
				}); deleteErr != nil {
					return false, deleteErr
				}
				c.Set(cfg.CacheHeader, cacheUnreachable)
				return false, nil
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
					markRevalidate()
				}

				handleMinFresh(ts)

				if revalidate {
					c.Set(cfg.CacheHeader, cacheUnreachable)
					if reqDirectives.onlyIfCached {
						if statusErr := c.SendStatus(fiber.StatusGatewayTimeout); statusErr != nil {
							return false, statusErr
						}
						return true, nil
					}
					return false, nil
				}

				servedStale := false

				switch {
				case entryExpired && !allowStale:
					if deleteErr := deleteCurrentEntry(guard, func(delErr error) error {
						return fmt.Errorf("cache: failed to delete expired key %q: %w", maskKey(key), delErr)
					}); deleteErr != nil {
						return false, deleteErr
					}
				case entryHasPrivate:
					if deleteErr := deleteCurrentEntry(guard, func(delErr error) error {
						return fmt.Errorf("cache: failed to delete private response for key %q: %w", maskKey(key), delErr)
					}); deleteErr != nil {
						return false, deleteErr
					}
					c.Set(cfg.CacheHeader, cacheUnreachable)
					if reqDirectives.onlyIfCached {
						if statusErr := c.SendStatus(fiber.StatusGatewayTimeout); statusErr != nil {
							return false, statusErr
						}
						return true, nil
					}
					return false, nil
				case entryHasExpiration && !requestNoCache:
					servedStale = entryExpired
					if hasAuthorization && !e.shareable {
						c.Set(cfg.CacheHeader, cacheUnreachable)
						if reqDirectives.onlyIfCached {
							if statusErr := c.SendStatus(fiber.StatusGatewayTimeout); statusErr != nil {
								return false, statusErr
							}
							return true, nil
						}
						markRevalidate()
						return false, nil
					}

					// Separate body value to avoid msgp serialization
					// We can store raw bytes with Storage 👍
					if cfg.Storage != nil {
						guard.unlock()
						rawBody, bodyErr := manager.getRaw(reqCtx, key+"_body")
						if bodyErr != nil {
							manager.release(e)
							return false, cacheBodyFetchError(maskKey, key, bodyErr)
						}
						e.body = rawBody
					} else {
						guard.unlock()
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
						maxAge := utils.FormatUint(remaining)
						c.Set(fiber.HeaderCacheControl, "public, max-age="+maxAge)
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
					return true, nil
				default:
					// no cached response to serve
				}
			}

			if e == nil && revalidate {
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					if statusErr := c.SendStatus(fiber.StatusGatewayTimeout); statusErr != nil {
						return false, statusErr
					}
					return true, nil
				}
				return false, nil
			}

			if e == nil && reqDirectives.onlyIfCached {
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if statusErr := c.SendStatus(fiber.StatusGatewayTimeout); statusErr != nil {
					return false, statusErr
				}
				return true, nil
			}

			return false, nil
		}()
		if err != nil {
			return err
		}
		if handledCacheRequest {
			return nil
		}
		if err := loadRevalidationBody(); err != nil {
			return err
		}

		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
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

		// RFC 9111 requires responses with Vary: * to remain uncacheable even when
		// response-driven Vary partitioning is otherwise disabled.
		if hasPrivate || hasNoCache || varyHasStar {
			switch {
			case e != nil:
				if err := deleteCurrentEntry(nil, func(delErr error) error {
					return fmt.Errorf("cache: failed to delete cached response for key %q: %w", maskKey(key), delErr)
				}); err != nil {
					return err
				}
			case revalidate:
				if err := deleteRevalidatedEntry(); err != nil {
					return err
				}
			}

			if !cfg.DisableVaryHeaders && hasVaryManifest {
				if err := manager.del(reqCtx, manifestKey); err != nil {
					return fmt.Errorf("cache: failed to delete stale vary manifest %q: %w", maskKey(manifestKey), err)
				}
			}

			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		shouldStoreVaryManifest := !cfg.DisableVaryHeaders && len(varyNames) > 0
		if !cfg.DisableVaryHeaders && len(varyNames) > 0 {
			if key == baseKey {
				key += buildVaryKey(varyNames, &c.Request().Header)
			}
		} else if !cfg.DisableVaryHeaders && hasVaryManifest {
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
				withCacheLock(mux, func() {
					storedBytes -= bodySize
				})
			}
		}()

		if cfg.MaxBytes > 0 {
			var keysToRemove []string
			var sizesToRemove []uint
			var candidates []evictionCandidate
			var reserveErr error
			withCacheLock(mux, func() {
				// Reserve space for the new entry first
				storedBytes += bodySize
				spaceReserved = true

				// Now evict entries until we're under the limit
				for storedBytes > cfg.MaxBytes {
					if heap.Len() == 0 {
						// Can't evict more, unreserve space and fail
						storedBytes -= bodySize
						// Set spaceReserved to false so the deferred cleanup does not unreserve again
						spaceReserved = false
						reserveErr = errors.New("cache: insufficient space and no entries to evict")
						return
					}
					next := heap.entries[0]
					keyToRemove, size := heap.removeFirst()
					keysToRemove = append(keysToRemove, keyToRemove)
					sizesToRemove = append(sizesToRemove, size)
					candidates = append(candidates, evictionCandidate{
						key:  keyToRemove,
						size: size,
						exp:  next.exp,
					})
					storedBytes -= size
				}
			})
			if reserveErr != nil {
				return reserveErr
			}

			// Perform deletions outside the lock
			if len(keysToRemove) > 0 {
				for i, keyToRemove := range keysToRemove {
					delErr := deleteLockedKey(reqCtx, keyToRemove)
					if delErr == nil {
						continue
					}

					// Deletion failed: restore storedBytes for failed deletions
					var restored []evictionCandidate
					withCacheLock(mux, func() {
						// Restore sizes of entries we failed to delete
						for j := i; j < len(sizesToRemove); j++ {
							storedBytes += sizesToRemove[j]
						}
						// Unreserve space for the new entry
						storedBytes -= bodySize
						spaceReserved = false

						// Re-add entries to the heap to keep expiration tracking consistent
						for j := i; j < len(candidates); j++ {
							candidate := candidates[j]
							candidate.heapIdx = heap.put(candidate.key, candidate.exp, candidate.size)
							restored = append(restored, candidate)
						}
					})

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
		now := cfg.now().UTC()
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
					expiration = expiresAt.Sub(cfg.now())
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

		storeTS := safeUnixSeconds(cfg.now())
		responseTS := max(storeTS, nowUnix)

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

		if !cfg.DisableVaryHeaders && shouldStoreVaryManifest {
			if err := storeVaryManifest(reqCtx, manager, manifestKey, varyNames, storageExpiration); err != nil {
				return err
			}
		}

		e.exp = responseTS + uint64(remainingExpiration.Seconds())
		e.ttl = uint64(expiration.Seconds())
		if expiresParseError {
			e.exp = storeTS + 1
		}

		// Store entry in heap (space already reserved in eviction phase)
		var heapIdx int
		if cfg.MaxBytes > 0 {
			withCacheLock(mux, func() {
				heapIdx = heap.put(key, e.exp, bodySize)
				e.heapidx = heapIdx
				// Note: storedBytes was incremented during reservation, and evictions
				// have already been accounted for, so no additional increment is needed
				spaceReserved = false // Clear flag to prevent defer from unreserving
			})
		}

		cleanupOnStoreError := func(ctx context.Context, releaseEntry, rawStored bool) error {
			var cleanupErr error
			if cfg.MaxBytes > 0 {
				withCacheLock(mux, func() {
					_, size := heap.remove(heapIdx)
					storedBytes -= size
				})
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
			if err := withKeyLock(key, func() error {
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
				return nil
			}); err != nil {
				return err
			}
		} else {
			// Store entry in memory
			if err := withKeyLock(key, func() error {
				if err := manager.set(reqCtx, key, e, storageExpiration); err != nil {
					if cleanupErr := cleanupOnStoreError(reqCtx, true, false); cleanupErr != nil {
						err = errors.Join(err, cleanupErr)
					}
					return err
				}
				return nil
			}); err != nil {
				return err
			}
		}

		// If revalidating, remove old heap entry now that replacement is successfully stored
		if cfg.MaxBytes > 0 && revalidate && oldHeapIdx >= 0 {
			withCacheLock(mux, func() {
				removeHeapEntry(key, oldHeapIdx)
			})
		}

		c.Set(cfg.CacheHeader, cacheMiss)

		// Finish response
		return nil
	}
}
