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
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/log"
)

// hexLen is the hex-encoded length of a SHA-256 sum, shared by the auth and vary hashers.
const hexLen = sha256.Size * 2

// Sizes for the local arrays the response headers are formatted into, so the
// values reach the header store — which copies them — without a heap
// allocation each. Both are upper bounds; an append past one still produces the
// same bytes, only more slowly.
const (
	// httpDateLen holds an IMF-fixdate, "Mon, 02 Jan 2006 15:04:05 GMT".
	httpDateLen = 32
	// maxUintDigits holds the decimal form of any uint64.
	maxUintDigits = 20
)

// publicMaxAge is the Cache-Control this middleware writes on a hit when the
// entry carries none of its own, less the delta-seconds appended after it.
const publicMaxAge = "public, max-age="

// cache status
// unreachable: when cache is bypass, or invalid
// hit: cache is served
// miss: do not have cache record
const (
	cacheUnreachable = "unreachable"
	cacheHit         = "hit"
	cacheMiss        = "miss"
)

// cacheKeyVersion namespaces every key this version writes, so an entry a
// previous one stored is never read rather than being reinterpreted.
//
// Bumped because the rules for what shares a partition changed. An earlier
// version detected Authorization byte-exactly, so under DisableHeaderNormalizing
// a request bearing a lower-case "authorization" was taken for anonymous and its
// response cached under the anonymous key. Only lookups that carry the header
// now move to a partition of their own — so on an external store that survived
// the upgrade, an anonymous request would still find that entry and be served an
// authenticated body. Bump this whenever what a key stands for changes; the cost
// is one cold cache after a deploy.
const cacheKeyVersion = "v2"

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
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	// Served to every client matching the key, so a captured Set-Cookie hands one
	// client's session to all the others. It still reaches the client that caused
	// the miss. Set-Cookie2 is the obsolete RFC 2965 spelling and leaks the same.
	"Set-Cookie":  {},
	"Set-Cookie2": {},
	"TE":          {},
	// "Trailer", not "Trailers": the hop-by-hop header is Trailer (RFC 9110
	// §6.6.2), so the old spelling matched nothing and replayed one connection's
	// chunked framing to every later hit.
	"Trailer":           {},
	"Transfer-Encoding": {},
	"Upgrade":           {},
}

var cacheableStatusCodes = map[int]struct{}{
	fiber.StatusOK:                          {},
	fiber.StatusNonAuthoritativeInformation: {},
	fiber.StatusNoContent:                   {},
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
	mux := &sync.RWMutex{}
	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage, redactKeys)
	// Create indexed heap for tracking expirations ( see heap.go )
	heap := &indexedHeap{}
	// count stored bytes (sizes of response bodies)
	var storedBytes uint

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

	// removeHeapEntry drops a tracked entry and reports it, so a caller that
	// removes one before storing its replacement can put it back on failure.
	removeHeapEntry := func(entryKey string, heapIdx int) (evictionCandidate, bool) {
		if cfg.MaxBytes == 0 {
			return evictionCandidate{}, false
		}

		if heapIdx < 0 || heapIdx >= len(heap.indices) {
			return evictionCandidate{}, false
		}

		indexedIdx := heap.indices[heapIdx]
		if indexedIdx < 0 || indexedIdx >= len(heap.entries) {
			return evictionCandidate{}, false
		}

		entry := heap.entries[indexedIdx]
		if entry.idx != heapIdx || entry.key != entryKey {
			return evictionCandidate{}, false
		}

		exp := entry.exp
		_, size := heap.remove(heapIdx)
		storedBytes -= size

		return evictionCandidate{key: entryKey, size: size, exp: exp, heapIdx: heapIdx}, true
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
		// Every field line, like the hash below: an empty first Authorization line
		// would otherwise read as anonymous. canonical is app config, fixed for the
		// life of the handler, so read it once.
		canonical := headerlookup.Canonical(c)

		hasAuthorization := false
		for _, v := range fieldname.Lines(&c.Request().Header, fiber.HeaderAuthorization, canonical) {
			if len(v) > 0 {
				hasAuthorization = true
				break
			}
		}
		reqCacheControl := joinedHeader(&c.Request().Header, fiber.HeaderCacheControl, canonical)
		reqDirectives := parseRequestCacheControl(reqCacheControl)
		if !reqDirectives.noCache {
			reqPragma := utils.UnsafeString(joinedHeader(&c.Request().Header, fiber.HeaderPragma, canonical))
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

		// Get key from request. Assembled in one pooled buffer so the whole key
		// costs a single allocation: the pieces used to be concatenated one at a
		// time, and every join copied the key built so far into a fresh string.
		keyBufPtr := acquireKeyBuffer()
		keyBuf := (*keyBufPtr)[:0]
		keyBuf = append(keyBuf, cacheKeyVersion...)
		keyBuf = append(keyBuf, '|')
		keyBuf = append(keyBuf, requestMethod...)
		keyBuf = append(keyBuf, '|')
		keyBuf = appendGeneratedKey(keyBuf, c, &cfg)
		if hasAuthorization {
			// Read at the point of use: the result aliases the header's storage, and
			// cfg.KeyGenerator is user code that may recycle the slot. Every field
			// line, or two credentials differing past the first would share a key.
			//
			// Length-prefixed, not comma-joined: a lone "Bearer a,Bearer b" renders
			// the same as two lines carrying one each, and the two are different
			// principals, so a comma would put them in one partition.
			authLines := fieldname.Lines(&c.Request().Header, fiber.HeaderAuthorization, canonical)
			keyBuf = append(keyBuf, "|auth="...)
			keyBuf = appendAuthHash(keyBuf, authLines)
		}
		baseLen := len(keyBuf)

		// baseKey and manifestKey differ only by the suffix, so one string holds
		// both: slicing a string off the front of another shares its bytes rather
		// than copying them, which the separate concatenation could not do.
		var baseKey, manifestKey string
		if cfg.DisableVaryHeaders {
			baseKey = string(keyBuf)
		} else {
			keyBuf = append(keyBuf, "|vary"...)
			manifestKey = string(keyBuf)
			baseKey = manifestKey[:baseLen]
		}
		key := baseKey

		reqCtx := c.Context()

		varyNames := []string(nil)
		hasVaryManifest := false
		var err error
		if !cfg.DisableVaryHeaders {
			varyNames, hasVaryManifest, err = loadVaryManifest(reqCtx, manager, manifestKey)
			if err != nil {
				releaseKeyBuffer(keyBufPtr, keyBuf)
				return err
			}
			if len(varyNames) > 0 {
				keyBuf = appendVaryKey(keyBuf[:baseLen], varyNames, &c.Request().Header, canonical)
				key = string(keyBuf)
			}
		}
		releaseKeyBuffer(keyBufPtr, keyBuf)

		// Get entry from pool
		e, err := manager.get(reqCtx, key)
		if err != nil && !errors.Is(err, errCacheMiss) {
			return err
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

		// Lock entry before reading the current timestamp so freshness decisions
		// are based on the time the protected cache entry is evaluated.
		mux.Lock()
		ts := safeUnixSeconds(cfg.now())
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
			// An entry written before Set-Cookie joined ignoreHeaders carries one
			// beside a body that is personalized for whoever caused that miss.
			// Dropping the cookie on replay leaves that body, so the entry goes.
			if !entryHasPrivate && len(e.headers) > 0 && cachedHeadersSetCookie(e.headers) {
				entryHasPrivate = true
			}
			requestNoCache := reqDirectives.noCache

			// Invalidate on a local copy: the in-memory item is shared with concurrent hits.
			entryExp := e.exp
			if cfg.CacheInvalidator != nil && cfg.CacheInvalidator(c) {
				entryExp = ts - 1
			}

			entryHasExpiration := entryExp != 0
			entryExpired := entryHasExpiration && ts >= entryExp
			staleness := uint64(0)
			if entryExpired {
				staleness = ts - entryExp
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
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					return c.SendStatus(fiber.StatusGatewayTimeout)
				}
				return c.Next()
			case entryHasExpiration && hasAuthorization && !e.shareable && reqDirectives.onlyIfCached:
				if cfg.Storage != nil {
					manager.release(e)
				}
				unlock()
				c.Set(cfg.CacheHeader, cacheUnreachable)
				return c.SendStatus(fiber.StatusGatewayTimeout)
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

				// Separate body value to avoid msgp serialization
				// We can store raw bytes with Storage 👍
				if cfg.Storage != nil {
					unlock()
					rawBody, err := manager.getRaw(reqCtx, key+"_body")
					if err != nil {
						if !errors.Is(err, errCacheMiss) {
							manager.release(e)
							return cacheBodyFetchError(maskKey, key, err)
						}
						// The backend dropped the body but kept the metadata: a miss, so
						// drop the orphaned metadata.
						if err := deleteKey(reqCtx, key); err != nil {
							manager.release(e)
							return fmt.Errorf("cache: failed to delete key %q without a body: %w", maskKey(key), err)
						}
						relock()
						removeHeapEntry(key, e.heapidx)
						unlock()
						manager.release(e)
						e = nil
						goto continueRequest
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
					setFieldLine(&c.Response().Header, fiber.HeaderCacheControl, e.cacheControl, canonical)
				}
				if len(e.expires) > 0 {
					setFieldLine(&c.Response().Header, fiber.HeaderExpires, e.expires, canonical)
				}
				if len(e.etag) > 0 {
					setFieldLine(&c.Response().Header, fiber.HeaderETag, e.etag, canonical)
				}
				clampedDate := clampDateSeconds(e.date, ts)
				// Formatted into a local array rather than a fresh slice: the value
				// is copied into the header store, so nothing outlives this line and
				// the nil destination was an allocation on every cache hit.
				var dateBuf [httpDateLen]byte
				dateValue := utils.AppendHTTPDate(dateBuf[:0], secondsToTime(clampedDate))
				setFieldLine(&c.Response().Header, fiber.HeaderDate, dateValue, canonical)
				// One entry per field line, so Set collapsed a repeated name — a Vary
				// sent twice came back varying only on the second. Clear every stored
				// name first, then append; a per-entry scan would be quadratic here.
				for i := range e.headers {
					if isIgnoredHeader(e.headers[i].key) {
						// An entry stored before Set-Cookie joined ignoreHeaders still
						// carries one, and DelBytes on it clears fasthttp's whole cookie
						// store — wiping the session an outer middleware just set.
						continue
					}
					if !canonical {
						// DelBytes is byte-exact, so a line under another spelling survives
						// it and the append lands beside it — the pair this clear prevents.
						fieldname.DelOthers(&c.Response().Header, utils.UnsafeString(e.headers[i].key))
					}
					c.Response().Header.DelBytes(e.headers[i].key)
				}
				for i := range e.headers {
					if isIgnoredHeader(e.headers[i].key) {
						continue
					}
					c.Response().Header.AddBytesKV(e.headers[i].key, e.headers[i].value)
				}
				// Set Cache-Control header if not disabled and not already set
				if !cfg.DisableCacheControl && len(fieldname.First(&c.Response().Header, fiber.HeaderCacheControl, canonical)) == 0 {
					remaining := uint64(0)
					if entryExp > ts {
						remaining = entryExp - ts
					}
					// Built in a local array: FormatUint and the join were two
					// allocations, and the header store copies the bytes anyway.
					var ccBuf [len(publicMaxAge) + maxUintDigits]byte
					cacheControlValue := strconv.AppendUint(append(ccBuf[:0], publicMaxAge...), remaining, 10)
					setFieldLine(&c.Response().Header, fiber.HeaderCacheControl, cacheControlValue, canonical)
				}

				const maxDeltaSeconds = uint64(math.MaxInt32)
				ageSeconds := min(entryAge, maxDeltaSeconds)

				// RFC-compliant Age header (RFC 9111)
				var ageBuf [maxUintDigits]byte
				setFieldLine(&c.Response().Header, fiber.HeaderAge, strconv.AppendUint(ageBuf[:0], ageSeconds, 10), canonical)
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

		// Remember the superseded entry's heap node so the replacement takes over its accounting.
		if e != nil {
			oldHeapIdx = e.heapidx
		}

		// make sure we're not blocking concurrent requests - do unlock
		unlock()

	continueRequest:
		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			return err
		}

		if cfg.accounting != nil {
			defer func() {
				mux.Lock()
				cfg.accounting(storedBytes, heap.Len())
				mux.Unlock()
			}()
		}

		markUnreachable := func() {
			// manager.release is a no-op for in-memory storage, so no guard.
			if e != nil {
				manager.release(e)
				e = nil
			}
			c.Set(cfg.CacheHeader, cacheUnreachable)
		}

		// The response's own spelling, not the app's: a proxy hands c.Response()
		// to an outbound fasthttp.Client, which stamps its own normalizing setting
		// on it. Reading a lower-case "cache-control: private" as absent cached
		// one client's body and replayed it to the next under "public, max-age".
		respCanonical := fieldname.Canonical(&c.Response().Header)

		cacheControlBytes := joinedHeader(&c.Response().Header, fiber.HeaderCacheControl, respCanonical)
		respCacheControl := parseResponseCacheControl(cacheControlBytes)
		varyHeader := utils.UnsafeString(joinedHeader(&c.Response().Header, fiber.HeaderVary, respCanonical))
		hasPrivate := respCacheControl.hasPrivate
		hasNoCache := respCacheControl.hasNoCache
		varyNames, varyHasStar := parseVary(varyHeader)

		// Respect server cache-control: no-store
		if respCacheControl.hasNoStore {
			markUnreachable()
			return nil
		}

		// RFC 9111 requires responses with Vary: * to remain uncacheable even when
		// response-driven Vary partitioning is otherwise disabled.
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
				key = varyKey(baseKey, varyNames, &c.Request().Header, canonical)
			}
		} else if !cfg.DisableVaryHeaders && hasVaryManifest {
			if err := manager.del(reqCtx, manifestKey); err != nil {
				return fmt.Errorf("cache: failed to delete stale vary manifest %q: %w", maskKey(manifestKey), err)
			}
		}

		// unreachable marks the response uncacheable and hands back any entry still
		// held from the lookup: each decision below is a decision not to store, so
		// the unmarshalled copy would otherwise be dropped rather than pooled.
		isSharedCacheAllowed := allowsSharedCacheDirectives(respCacheControl)
		if hasAuthorization && !isSharedCacheAllowed {
			markUnreachable()
			return nil
		}

		// A cookie personalizes the response for the one client that caused this
		// miss, and the body is the payload. Stricter than the test above: RFC 9111
		// §3.5 allows must-revalidate on a cache that re-checks, and this never does.
		if !allowsSharedCacheStorage(respCacheControl) && responseSetsCookie(&c.Response().Header, respCanonical) {
			markUnreachable()
			return nil
		}

		sharedCacheMode := !hasAuthorization || isSharedCacheAllowed

		// Don't cache response if status code is not cacheable
		if _, ok := cacheableStatusCodes[c.Res().StatusCode()]; !ok {
			markUnreachable()
			return nil
		}

		// Don't cache response if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			markUnreachable()
			return nil
		}

		// Don't try to cache if body won't fit into cache
		bodySize := uint(len(c.Response().Body()))
		if cfg.MaxBytes > 0 && bodySize > cfg.MaxBytes {
			markUnreachable()
			return nil
		}

		// Hand back whatever the lookup left holding. A stale entry survives to here
		// whenever the request said no-cache or it carried no expiry, and dropping
		// it would leak one pooled item per such request.
		if e != nil {
			manager.release(e)
		}
		e = manager.acquire()
		// Cache response
		e.body = utils.CopyBytes(c.Response().Body())
		e.status = c.Res().StatusCode()
		e.ctype = utils.CopyBytes(c.Response().Header.ContentType())
		e.cencoding = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderContentEncoding))
		e.private = false
		e.cacheControl = utils.CopyBytes(cacheControlBytes)
		e.expires = utils.CopyBytes(fieldname.First(&c.Response().Header, fiber.HeaderExpires, respCanonical))
		e.etag = utils.CopyBytes(fieldname.First(&c.Response().Header, fiber.HeaderETag, respCanonical))
		e.date = 0

		ageVal := uint64(0)
		if b := fieldname.First(&c.Response().Header, fiber.HeaderAge, respCanonical); len(b) > 0 {
			if v, err := fasthttp.ParseUint(b); err == nil {
				if v >= 0 {
					ageVal = uint64(v)
				}
			}
		} else {
			setFieldLine(&c.Response().Header, fiber.HeaderAge, []byte("0"), respCanonical)
		}
		e.age = ageVal
		e.shareable = isSharedCacheAllowed
		now := cfg.now().UTC()
		nowUnix := safeUnixSeconds(now)
		dateHeader := fieldname.First(&c.Response().Header, fiber.HeaderDate, respCanonical)
		// The second result says whether the date parsed, which is not asked: an
		// absent or unparsable Date leaves parsedDate zero, which clampDateSeconds
		// resolves to the receipt timestamp — the same answer either way.
		parsedDate, _ := parseHTTPDate(dateHeader)
		e.date = clampDateSeconds(parsedDate, nowUnix)
		var dateBuf [httpDateLen]byte
		dateBytes := utils.AppendHTTPDate(dateBuf[:0], secondsToTime(e.date))
		setFieldLine(&c.Response().Header, fiber.HeaderDate, dateBytes, respCanonical)

		// Store all response headers
		// (more: https://datatracker.ietf.org/doc/html/rfc2616#section-13.5.1)
		if cfg.StoreResponseHeaders {
			allHeaders := c.Response().Header.All()
			e.headers = e.headers[:0]
			for key, value := range allHeaders {
				if isIgnoredHeader(key) {
					continue
				}

				e.headers = append(e.headers, cachedHeader{
					key:   utils.CopyBytes(key),
					value: utils.CopyBytes(value),
				})
			}
		} else {
			// Keep a redirect's destination even when headers are not stored: 300 and
			// 301 are cacheable, so without this the first client got
			// "301 Location: /new" and every one after it a bare 301.
			e.headers = e.headers[:0]
			if location := fieldname.First(&c.Response().Header, fiber.HeaderLocation, respCanonical); len(location) > 0 {
				e.headers = append(e.headers, cachedHeader{
					key:   utils.CopyBytes(utils.UnsafeBytes(fiber.HeaderLocation)),
					value: utils.CopyBytes(location),
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
			} else if expiresBytes := fieldname.First(&c.Response().Header, fiber.HeaderExpires, respCanonical); len(expiresBytes) > 0 {
				// Same parser as the Date header (utils.go parseHTTPDate) so
				// both share one acceptance set: IMF-fixdate plus the obsolete
				// RFC 850 and asctime forms RFC 9110 §5.6.7 requires.
				expiresAt, err := utils.ParseHTTPDate(expiresBytes)
				if err != nil {
					expiration = time.Nanosecond
					expiresParseError = true
				} else {
					// Measured from the receipt timestamp, not a fresh read, so the
					// lifetime stays anchored to the same instant as Date and e.exp
					expiration = expiresAt.Sub(now)
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
			markUnreachable()
			return nil
		}

		// Reuse the receipt timestamp the Date header was clamped against: a
		// fresh read here charges Fiber's own processing time to the response as age
		responseTS := nowUnix

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
				markUnreachable()
				return nil
			}
			remainingExpiration = 0
		}

		// Eviction loop: atomically reserve space for the new entry and evict old entries.
		// It runs only once the response is known to be stored.
		// Strategy:
		// 1. Under lock: reserve space by pre-incrementing storedBytes, then collect entries to evict
		// 2. Outside lock: perform I/O deletions
		// 3. On deletion failure: restore storedBytes and return error
		// 4. Track reservation with a flag; unreserve on early return via defer
		var spaceReserved bool
		// The entry being replaced is untracked before its replacement is
		// stored, so a later failure has to put it back: otherwise it can stay
		// in the backend counting toward nothing and never expiring.
		var (
			replaced   evictionCandidate
			replacedOK bool
			rawWritten bool
			stored     bool
		)
		defer func() {
			if cfg.MaxBytes == 0 {
				return
			}

			mux.Lock()
			// If we reserved space but the entry was not successfully added to heap, unreserve it
			if spaceReserved {
				storedBytes -= bodySize
			}
			// A replacement body that landed overwrote the one the old entry
			// pointed at, and the store-error cleanup then deletes it, so that
			// entry can no longer be served: restoring its accounting would
			// count bytes nothing can read. The next request for the key finds
			// metadata without a body and drops it.
			restore := replacedOK && !stored && !rawWritten
			if restore {
				replaced.heapIdx = heap.put(replaced.key, replaced.exp, replaced.size)
				storedBytes += replaced.size
			}
			mux.Unlock()

			if restore {
				if err := refreshHeapIndex(reqCtx, replaced); err != nil {
					log.Warnf("cache: %v", err)
				}
			}
		}()

		if cfg.MaxBytes > 0 {
			mux.Lock()
			// The replaced entry hands its bookkeeping over first, so a key is never tracked twice.
			if oldHeapIdx >= 0 {
				replaced, replacedOK = removeHeapEntry(key, oldHeapIdx)
				oldHeapIdx = -1
			}
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

		if !cfg.DisableVaryHeaders && shouldStoreVaryManifest {
			if err := storeVaryManifest(reqCtx, manager, manifestKey, varyNames, storageExpiration); err != nil {
				return err
			}
		}

		e.exp = responseTS + uint64(remainingExpiration.Seconds())
		e.ttl = uint64(expiration.Seconds())
		if expiresParseError {
			e.exp = responseTS + 1
		}

		// Store entry in heap (space already reserved in eviction phase)
		var heapIdx int
		if cfg.MaxBytes > 0 {
			mux.Lock()
			heapIdx = heap.put(key, e.exp, bodySize)
			e.heapidx = heapIdx
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
			rawWritten = true
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

		// The replacement is in the backend now, so the entry it superseded
		// must not be restored by the deferred cleanup.
		stored = true

		c.Set(cfg.CacheHeader, cacheMiss)

		// Finish response
		return nil
	}
}
