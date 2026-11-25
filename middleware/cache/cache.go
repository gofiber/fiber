// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3/internal/redact"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/errorreplay"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/internal/personalheader"
	"github.com/gofiber/fiber/v3/internal/sizedstream"
	"github.com/gofiber/fiber/v3/log"
)

// hexLen is the hex-encoded length of a SHA-256 sum, shared by the auth and vary hashers.
const hexLen = sha256.Size * 2

var replayPlaceholder = []byte("\x00cache replay\x00")

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

// publicMaxAge and privateMaxAge are the Cache-Control this middleware writes
// on a hit when the entry carries none of its own, less the delta-seconds
// appended after it. The private form goes out when the entry was selected by
// a credential — a cookie, Authorization or another credential header, through Vary, KeyCookies or
// KeyHeaders — and so holds one user's response: a shared cache downstream that
// keys on the URL alone would otherwise store it and serve it to the next user.
const (
	publicMaxAge  = "public, max-age="
	privateMaxAge = "private, max-age="
)

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
// Bumped whenever the rules for what shares a partition change. v2: an earlier
// version detected Authorization byte-exactly, so under DisableHeaderNormalizing
// a request bearing a lower-case "authorization" was taken for anonymous and its
// response cached under the anonymous key. Only lookups that carry the header
// now move to a partition of their own — so on an external store that survived
// the upgrade, an anonymous request would still find that entry and be served an
// authenticated body. v3: the default key gained the request scheme and host.
// An entry written without them stands for every host at once, so on a store
// that survived the upgrade a request for one host would still find, and be
// served, what another host populated. v4: entries gained the response's Vary.
// One written without it is replayed with no Vary and, when it carries no
// Cache-Control of its own, under "public, max-age" — telling a shared cache
// downstream it may store one user's response for every user of the URL. v5:
// credential values in the key are hashed and a refusal ends the entry of the credential it
// refused; an entry cached for a credential since ended would still be served. It
// also retires v4 entries stored under the lookup's Vary list instead of their own. v6:
// the body slot and the Vary manifest moved from a suffix of the entry key, which
// a request can spell, to a namespace of their own (see bodyKeyPrefix), and a
// custom KeyGenerator's key is escaped so it cannot spell a variant's suffix. v7: a
// response that varies on a request header an inner middleware added, rewrote or
// removed is no longer stored, as its key held the value the client sent while the
// page was built from another. One the earlier rule stored would still be served,
// on a store that survived the upgrade, to every client sending that value. v8:
// the key holds the raw target and a bodyless Content-Encoding; entries carry keptHeaders and stored. Bump
// this whenever what a key stands for, or what its entry carries, changes; the
// cost is one cold cache after a deploy. Additive bookkeeping fields such as
// heapgen need no bump: old entries decode with generation zero, and older
// readers skip the new field without changing the response or key semantics.
const cacheKeyVersion = "v8"

// The keys derived from an entry key sit right after the version, where no entry
// key goes: those continue with their method.
const (
	bodyKeyPrefix     = cacheKeyVersion + "|body|"
	manifestKeyPrefix = cacheKeyVersion + "|manifest|"
)

// bodyKey names the slot an external Storage keeps the body of entryKey in.
func bodyKey(entryKey string) string {
	return bodyKeyPrefix + entryKey[len(cacheKeyVersion)+1:]
}

// MaxBytes also caps the entries tracked at once, one per trackedEntryOverhead bytes and at least
// minTrackedEntries: an entry holds a heap node and bookkeeping MaxBytes does not count.
const (
	trackedEntryOverhead = 256
	minTrackedEntries    = 1024
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
	"Age":                {},
	"Cache-Control":      {}, // already stored explicitly by the cache manager
	"Connection":         {},
	"Content-Encoding":   {}, // already stored explicitly by the cache manager
	"Content-Type":       {}, // already stored explicitly by the cache manager
	"Date":               {},
	"ETag":               {}, // already stored explicitly by the cache manager
	"Expires":            {}, // already stored explicitly by the cache manager
	"Keep-Alive":         {},
	"Proxy-Authenticate": {},
	// Proxy-specific, with the two beside it (RFC 9111 §3.1).
	"Proxy-Authentication-Info": {},
	"Proxy-Authorization":       {},
	// Hop-by-hop like Keep-Alive, whether or not Connection names it (RFC 9110 §7.6.1).
	"Proxy-Connection": {},
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
	"Vary":              {}, // already stored explicitly by the cache manager
}

// keptHeaders restrict use of the body, so they are kept without StoreResponseHeaders; grants such as CORS are not.
var keptHeaders = map[string]struct{}{
	fiber.HeaderContentDisposition:              {},
	fiber.HeaderContentSecurityPolicy:           {},
	fiber.HeaderContentSecurityPolicyReportOnly: {},
	"Cross-Origin-Embedder-Policy":              {},
	"Cross-Origin-Embedder-Policy-Report-Only":  {},
	"Cross-Origin-Opener-Policy":                {},
	"Cross-Origin-Opener-Policy-Report-Only":    {},
	fiber.HeaderCrossOriginResourcePolicy:       {},
	"Origin-Agent-Cluster":                      {},
	fiber.HeaderPermissionsPolicy:               {},
	fiber.HeaderReferrerPolicy:                  {},
	fiber.HeaderStrictTransportSecurity:         {},
	fiber.HeaderXContentTypeOptions:             {},
	fiber.HeaderXDNSPrefetchControl:             {},
	fiber.HeaderXDownloadOptions:                {},
	fiber.HeaderXFrameOptions:                   {},
	fiber.HeaderXPermittedCrossDomainPolicies:   {},
	fiber.HeaderXRobotsTag:                      {},
	fiber.HeaderXXSSProtection:                  {},
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

// cacheLockGuard remembers whether mux is held, so Unlock and Relock are idempotent.
type cacheLockGuard struct {
	mux    *sync.Mutex
	locked bool
}

func newCacheLockGuard(mux *sync.Mutex) *cacheLockGuard {
	mux.Lock()
	return &cacheLockGuard{mux: mux, locked: true}
}

func (g *cacheLockGuard) Unlock() {
	if g.locked {
		g.mux.Unlock()
		g.locked = false
	}
}

func (g *cacheLockGuard) Relock() {
	if !g.locked {
		g.mux.Lock()
		g.locked = true
	}
}

// invalidates asks CacheInvalidator about the entry found, releasing it should the app panic.
func invalidates(invalidator func(fiber.Ctx) bool, c fiber.Ctx, m *manager, e *item) bool {
	returned := false
	defer func() {
		if !returned {
			m.release(e)
		}
	}()
	invalidated := invalidator(c)
	returned = true
	return invalidated
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	type evictionCandidate struct {
		key      string
		manifest string
		size     uint
		exp      uint64 // when the entry is due to be evicted
		expires  uint64 // when it stops being fresh
		oldGen   uint64
		tag      purgeTag // so an entry put back stays ended for a write that ended it
	}
	type poppedNode struct {
		key string
		gen uint64
	}
	// evictingKey counts the nodes of a key in-flight evictions popped, and whether a store for the key began since.
	type evictingKey struct {
		count      int
		superseded bool
	}

	redactKeys := !cfg.DisableValueRedaction

	maskKey := redact.Masker(redactKeys)

	// Nothing to cache
	if int(cfg.Expiration.Seconds()) < 0 {
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	// Cache settings
	mux := &sync.Mutex{}
	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage, redactKeys)
	// Keeps the refusal gates' utils.Timestamp reads an atomic load instead of a clock read.
	utils.StartTimeStampUpdater()
	refusals := newRefusalRecords(&cfg, manager)
	// Create indexed heap for tracking expirations ( see heap.go )
	heap := newIndexedHeap()
	// count stored bytes (each entry's body, key and stored headers)
	var storedBytes uint
	// Eviction counts at most Expiration past storing, so a long max-age cannot outlast newer entries.
	evictionHorizon := uint64(cfg.Expiration.Seconds())
	keyVary, bodyVary := cfg.keyVaryFields(false), cfg.keyVaryFields(true)
	keyVaryValue, bodyVaryValue := []byte(strings.Join(keyVary, ", ")), []byte(strings.Join(bodyVary, ", "))
	purges := cfg.purges
	if purges == nil {
		purges = newPurgeTable(cfg.now)
	}
	heap.ends = purges.ends
	maxEntries := max(minTrackedEntries, int(min(cfg.MaxBytes/trackedEntryOverhead, math.MaxInt32)))

	// Delete key from both manager and storage
	deleteKey := func(ctx context.Context, dkey string) error {
		if err := manager.del(ctx, dkey); err != nil {
			return err
		}
		// External storage saves body data with different key
		if cfg.Storage != nil {
			if err := manager.del(ctx, bodyKey(dkey)); err != nil {
				return err
			}
		}
		return nil
	}

	// evict deletes a popped entry, sparing in memory a replacement stored since: a Storage has no conditional delete.
	evict := func(ctx context.Context, candidate evictionCandidate) error {
		if cfg.Storage == nil {
			manager.memory.DeleteIf(candidate.key, func(current any) bool {
				entry, ok := current.(*item)
				return ok && entry.heapgen == candidate.oldGen
			})
			return nil
		}
		return deleteKey(ctx, candidate.key)
	}

	// removeHeapEntry drops a tracked entry and reports it, so a caller that
	// removes one before storing its replacement can put it back on failure.
	removeHeapEntry := func(entryKey string, heapIdx int, heapGen uint64) (evictionCandidate, bool) {
		if cfg.MaxBytes == 0 {
			return evictionCandidate{}, false
		}

		if !heap.matches(entryKey, heapIdx, heapGen) {
			// A failed eviction may restore the original generation into a
			// different heap slot after the old slot has been recycled. The
			// backend still carries the original index; resolve the live node
			// by key and generation only on this exceptional path.
			if currentIdx, ok := heap.findKey(entryKey); ok {
				heapIdx = currentIdx
			}
			if !heap.matches(entryKey, heapIdx, heapGen) {
				return evictionCandidate{}, false
			}
		}
		entry := heap.entries[heap.indices[heapIdx]]

		exp := entry.exp
		_, size := heap.remove(heapIdx)
		storedBytes -= size

		return evictionCandidate{
			key: entryKey, manifest: entry.manifest, size: size, exp: exp, expires: entry.expires, oldGen: heapGen, tag: entry.tag,
		}, true
	}

	// evicting maps popped nodes to whether their entry went since; evictingKeys counts them by key. Guarded by mux.
	evicting := make(map[poppedNode]bool)
	evictingKeys := make(map[string]evictingKey)

	// trackEvicting and untrackEvicting record and forget an eviction's popped nodes. The caller holds mux.
	trackEvicting := func(candidates []evictionCandidate) {
		for _, candidate := range candidates {
			evicting[poppedNode{key: candidate.key, gen: candidate.oldGen}] = false
			popped := evictingKeys[candidate.key]
			popped.count++
			evictingKeys[candidate.key] = popped
		}
	}
	untrackEvicting := func(candidates []evictionCandidate) {
		for _, candidate := range candidates {
			delete(evicting, poppedNode{key: candidate.key, gen: candidate.oldGen})
			if popped := evictingKeys[candidate.key]; popped.count > 1 {
				popped.count--
				evictingKeys[candidate.key] = popped
			} else {
				delete(evictingKeys, candidate.key)
			}
		}
	}

	// removeDeleted drops a gone entry's node, or marks it so its eviction won't restore it. The caller holds mux.
	removeDeleted := func(entryKey string, heapIdx int, heapGen uint64) {
		if _, removed := removeHeapEntry(entryKey, heapIdx, heapGen); removed {
			return
		}
		node := poppedNode{key: entryKey, gen: heapGen}
		if _, popped := evicting[node]; popped {
			evicting[node] = true
		}
	}

	retrack := func(candidate evictionCandidate) (int, bool) {
		if _, exists := heap.findKey(candidate.key); exists {
			return 0, false
		}
		idx := heap.putWithGeneration(candidate.key, candidate.exp, candidate.expires, candidate.size, candidate.oldGen, candidate.manifest, candidate.tag)
		storedBytes += candidate.size
		return idx, true
	}

	restoreHeapEntry := func(ctx context.Context, candidate evictionCandidate) error {
		// Storage I/O must stay outside the global heap lock: a slow backend
		// must not block requests for unrelated keys. The original generation
		// can be restored directly in the heap, so metadata needs no rewrite.
		mux.Lock()
		idx, restored := retrack(candidate)
		mux.Unlock()
		if !restored {
			return nil
		}

		entry, err := manager.get(ctx, candidate.key)
		if err != nil {
			if errors.Is(err, errCacheMiss) {
				if cfg.Storage != nil {
					// Metadata may have been deleted before the body deletion failed.
					// Keep its reservation so a later eviction can retry the body.
					_, bodyErr := manager.getRaw(ctx, bodyKey(candidate.key))
					if bodyErr == nil {
						return nil
					}
					if !errors.Is(bodyErr, errCacheMiss) {
						return fmt.Errorf("cache: failed to verify body for key %q after eviction failure: %w", maskKey(candidate.key), bodyErr)
					}
				}
				// Recorded, as an eviction may have popped the node again since it went back.
				mux.Lock()
				removeDeleted(candidate.key, idx, candidate.oldGen)
				mux.Unlock()
				return nil
			}
		} else {
			matches := entry.heapgen == candidate.oldGen
			if cfg.Storage != nil {
				manager.release(entry)
			}
			if !matches {
				mux.Lock()
				removeDeleted(candidate.key, idx, candidate.oldGen)
				mux.Unlock()
				return nil
			}
		}

		if err != nil {
			return fmt.Errorf("cache: failed to reload key %q after eviction failure: %w", maskKey(candidate.key), err)
		}
		return nil
	}

	// endPath ends the entries of the path a write succeeded at (RFC 9111 §4.4), and lets eviction find them
	// first: none can be served again, while the entries it would evict instead still can.
	endPath := func(slot uint64) {
		purges.mark(slot, purges.stamp())
		if cfg.MaxBytes > 0 {
			mux.Lock()
			heap.flag(slot)
			mux.Unlock()
		}
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

		requestMethod := c.Method()
		cached := slices.Contains(cfg.Methods, requestMethod)

		// pass answers without the cache; an unsafe request that succeeds ends its path's entries (RFC 9111 §4.4).
		pass := func() error {
			if cached || !unsafeMethod(requestMethod) || (cfg.Next != nil && cfg.Next(c)) {
				return c.Next()
			}
			slot := purges.slot(c.Host(), c.Path()) // before the handler, which may rewrite the path
			err := c.Next()
			// An abandoned write may still complete, so its entries end, its status unread as it may still be written.
			if c.IsAbandoned() && c.RequestCtx().LastTimeoutErrorResponse() != nil {
				endPath(slot)
				return err
			}
			if err != nil {
				return err
			}
			if status := c.Response().StatusCode(); status >= fiber.StatusOK && status < fiber.StatusBadRequest {
				endPath(slot)
			}
			return nil
		}

		// Refrain from caching; with only-if-cached, no-store still allows serving (RFC 9111 §5.2.1.5).
		if reqDirectives.noStore && !reqDirectives.onlyIfCached {
			return pass()
		}

		// Only cache methods listed in cfg.Methods (default: GET, HEAD).
		if !cached {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			// Nothing is stored for the method, and only-if-cached forbids the handler (RFC 9111 §5.2.1.7), unless the
			// error handler answers a refused request or Next skips the middleware. An empty Methods turns caching off.
			if reqDirectives.onlyIfCached && len(cfg.Methods) > 0 {
				if errorreplay.Active(c.RequestCtx()) || (cfg.Next != nil && cfg.Next(c)) {
					return c.Next()
				}
				return c.SendStatus(fiber.StatusGatewayTimeout)
			}
			return pass()
		}

		// A request the server refused runs the chain without its route, and the
		// error handler answers it: nothing here is the URL's response.
		if errorreplay.Active(c.RequestCtx()) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.Next()
		}

		if cfg.Next != nil && cfg.Next(c) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return c.Next()
		}

		// A key generator may read the body (QUERY), and only a declared length bounds a streamed one;
		// otherwise the stream is left to the handler, or answered 504 under only-if-cached.
		// MaxRequestBodySize is BodyLimit, read without copying Config.
		if req := c.Request(); req.IsBodyStream() {
			if n := req.Header.ContentLength(); n < 0 || n > c.App().Server().MaxRequestBodySize {
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					return c.SendStatus(fiber.StatusGatewayTimeout)
				}
				return c.Next()
			}
		}

		// The default key holds the body, and the body of a multipart form fasthttp parsed is not the one sent.
		if cfg.usesDefaultKeyGenerator && preParsedMultipart(c) {
			c.Set(cfg.CacheHeader, cacheUnreachable)
			if reqDirectives.onlyIfCached {
				return c.SendStatus(fiber.StatusGatewayTimeout)
			}
			return c.Next()
		}

		// Get key from request. Assembled in one pooled buffer so the whole key
		// costs a single allocation: the pieces used to be concatenated one at a
		// time, and every join copied the key built so far into a fresh string.
		keyBufPtr := acquireKeyBuffer()
		keyBuf := (*keyBufPtr)[:0]
		// The manifest key is the base key behind its prefix: one string holds both,
		// and slicing the base key off it shares its bytes.
		if !cfg.DisableVaryHeaders {
			keyBuf = append(keyBuf, manifestKeyPrefix...)
		}
		start := len(keyBuf)
		keyBuf = append(keyBuf, cacheKeyVersion...)
		keyBuf = append(keyBuf, '|')
		keyBuf = append(keyBuf, requestMethod...)
		keyBuf = append(keyBuf, '|')
		generatedAt := len(keyBuf) - start
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

		both := string(keyBuf)
		baseKey := both[start:]
		var manifestKey string
		if !cfg.DisableVaryHeaders {
			manifestKey = both
		}
		key := baseKey
		generatedKey := baseKey[generatedAt:] // baseKey without the version and method, for the refusal records

		reqCtx := c.Context()

		// unserved fails open on a Storage error (only-if-cached gets 504), unless the request's context is done.
		unserved := func(err error) error {
			if reqCtx.Err() != nil {
				return err
			}
			log.Warnf("%v; the request is answered without the cache", err)
			c.Set(cfg.CacheHeader, cacheUnreachable)
			if reqDirectives.onlyIfCached {
				return c.SendStatus(fiber.StatusGatewayTimeout)
			}
			return c.Next()
		}

		varyNames := []string(nil)
		hasVaryManifest := false
		var err error
		if !cfg.DisableVaryHeaders {
			varyNames, hasVaryManifest, err = loadVaryManifest(reqCtx, manager, manifestKey)
			if err != nil {
				releaseKeyBuffer(keyBufPtr, keyBuf)
				return unserved(err)
			}
			if len(varyNames) > 0 {
				keyBuf = appendVaryKey(keyBuf, varyNames, &c.Request().Header, canonical)
				key = string(keyBuf[start:])
			}
		}
		releaseKeyBuffer(keyBufPtr, keyBuf)

		// Get entry from pool
		e, err := manager.get(reqCtx, key)
		switch {
		case err == nil, errors.Is(err, errCacheMiss):
		case errors.Is(err, errUndecodable):
			log.Warnf("%v; it is treated as a miss", err)
		default:
			return unserved(err)
		}
		// Set aside, or deleted as expired or invalidated, the entry is still one the request found.
		foundAtLookup := e != nil
		entryAge := uint64(0)
		revalidate := false
		oldHeapIdx := -1 // Track old heap index for replacement during revalidation
		var oldHeapGen uint64
		var staleMemoryEntry *item

		// setAside parks the entry found for revalidation; the replacement takes over its heap node.
		setAside := func() {
			revalidate = true
			oldHeapIdx = e.heapidx
			oldHeapGen = e.heapgen
			if cfg.Storage != nil {
				manager.release(e)
			} else {
				staleMemoryEntry = e
			}
			e = nil
		}

		// deleteFound deletes the entry found unless a concurrent request replaced it. A Storage has no
		// conditional delete, so it reads first: the reads before the lock leave a replacement time to land.
		deleteFound := func(ctx context.Context) error {
			if cfg.Storage == nil {
				manager.delIf(key, e)
				return nil
			}
			current, err := manager.get(ctx, key)
			if err != nil {
				if errors.Is(err, errCacheMiss) {
					return nil
				}
				return err
			}
			replaced := current.heapidx != e.heapidx || current.heapgen != e.heapgen
			manager.release(current)
			if replaced {
				return nil
			}
			return deleteKey(ctx, key)
		}

		handleMinFresh := func(now uint64) {
			if e == nil || !reqDirectives.minFreshSet {
				return
			}
			remainingFreshness := remainingFreshness(e, now)
			if remainingFreshness < reqDirectives.minFresh {
				setAside()
			}
		}

		// settle sets the entry found aside when the request or the entry rules it out at now.
		settle := func(now uint64) {
			if e == nil {
				return
			}
			entryAge = cachedResponseAge(e, now)
			if reqDirectives.maxAgeSet && (reqDirectives.maxAge == 0 || entryAge > reqDirectives.maxAge) {
				setAside()
			}
			handleMinFresh(now)
			if e != nil && e.ttl == 0 && e.forceRevalidate {
				setAside()
			}
		}

		// What the request sends that ties an entry to its credential, found when first needed.
		var creds requestCredentials
		credsKnown := false
		requestCreds := func() requestCredentials {
			if !credsKnown {
				creds, credsKnown = cfg.credentialsOf(c, varyNames, hasAuthorization), true
			}
			return creds
		}

		// A credential the origin refused is revalidated, not served (see refusalRecords.key).
		refusalRecorded := false
		var foundExp uint64 // when the entry found stops being fresh, for the record
		if e != nil {
			foundExp = e.exp
			if cfg.mayKeyCredential(hasAuthorization, varyNames) && refusals.mayHold(reqCtx, c, generatedKey) &&
				refusals.refusedFor(reqCtx, c, generatedKey, varyNames, requestCreds()) {
				refusalRecorded = true
				setAside()
			}
		}

		// App code runs before the lock is taken and the clock read, so a slow call stalls no request, a
		// panic leaves the lock free, and freshness is judged as of one instant; the entry is immutable, so
		// a concurrent replacement is harmless. The app is not asked about an entry already ruled out.
		invalidated := e != nil && purges.supersedes(c.Host(), c.Path(), e)
		if e != nil && !invalidated && cfg.CacheInvalidator != nil {
			early := safeUnixSeconds(cfg.now())
			settle(early)
			expired := e != nil && e.ttl == 0 && e.exp != 0 && early >= e.exp
			if e != nil && !expired {
				invalidated = invalidates(cfg.CacheInvalidator, c, manager, e)
			}
		}

		// Lock entry before reading the current timestamp so freshness decisions
		// are based on the time the protected cache entry is evaluated.
		guard := newCacheLockGuard(mux)
		ts := safeUnixSeconds(cfg.now())

		// handled reports that the lookup phase already wrote the response.
		handled, lookupErr := func() (bool, error) {
			// Frees the lock on a panic; a no-op after the explicit unlocks below.
			defer guard.Unlock()

			// Cache Entry found
			settle(ts)

			if e != nil && e.ttl == 0 && e.exp != 0 && ts >= e.exp {
				guard.Unlock()
				if err := deleteFound(reqCtx); err != nil {
					log.Warnf("cache: failed to delete expired key %q: %v", maskKey(key), err)
				} else {
					guard.Relock()
					removeDeleted(key, e.heapidx, e.heapgen)
				}
				if cfg.Storage != nil {
					manager.release(e)
				}
				e = nil
				guard.Unlock()
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					return true, c.SendStatus(fiber.StatusGatewayTimeout)
				}
				return false, nil
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

				// Asked at most once, by whichever check needs it first.
				holds, holdsKnown := false, false
				holdsCredential := func() bool {
					if !holdsKnown {
						holds, holdsKnown = cfg.holdsCredentialResponse(c, e.vary), true
					}
					return holds
				}

				// Invalidate on a local copy: the in-memory item is shared with concurrent hits.
				entryExp := e.exp
				if invalidated {
					entryExp = ts - 1
				}

				entryHasExpiration := entryExp != 0
				entryExpired := entryHasExpiration && ts >= entryExp
				staleness := uint64(0)
				if entryExpired {
					staleness = ts - entryExp
				}
				// max-stale revives no invalidated entry, and no credential's response unless allowed.
				allowStale := !invalidated && entryExpired &&
					(reqDirectives.maxStaleAny || (reqDirectives.maxStaleSet && staleness <= reqDirectives.maxStale)) &&
					(cfg.AllowStaleCredentialResponses || (!hasAuthorization && !holdsCredential()))

				if entryExpired && e.revalidate {
					setAside()
				}

				handleMinFresh(ts)

				if revalidate {
					guard.Unlock()
					c.Set(cfg.CacheHeader, cacheUnreachable)
					if reqDirectives.onlyIfCached {
						return true, c.SendStatus(fiber.StatusGatewayTimeout)
					}
					return false, nil
				}

				servedStale := false

				switch {
				case entryExpired && !allowStale:
					guard.Unlock()
					idx := e.heapidx
					gen := e.heapgen
					if err := deleteFound(reqCtx); err != nil {
						log.Warnf("cache: failed to delete expired key %q: %v", maskKey(key), err)
						manager.release(e)
						e = nil
						break
					}
					guard.Relock()
					manager.release(e)
					removeDeleted(key, idx, gen)
					e = nil
				case entryHasPrivate:
					guard.Unlock()
					if err := deleteFound(reqCtx); err != nil {
						log.Warnf("cache: failed to delete private response for key %q: %v", maskKey(key), err)
					} else {
						guard.Relock()
						removeDeleted(key, e.heapidx, e.heapgen)
					}
					if cfg.Storage != nil && e != nil {
						manager.release(e)
					}
					e = nil
					guard.Unlock()
					c.Set(cfg.CacheHeader, cacheUnreachable)
					if reqDirectives.onlyIfCached {
						return true, c.SendStatus(fiber.StatusGatewayTimeout)
					}
					return true, c.Next()
				case entryHasExpiration && hasAuthorization && !e.shareable && reqDirectives.onlyIfCached:
					if cfg.Storage != nil {
						manager.release(e)
					}
					guard.Unlock()
					c.Set(cfg.CacheHeader, cacheUnreachable)
					return true, c.SendStatus(fiber.StatusGatewayTimeout)
				case entryHasExpiration && !requestNoCache:
					servedStale = entryExpired
					if hasAuthorization && !e.shareable {
						if cfg.Storage != nil {
							manager.release(e)
						}
						guard.Unlock()
						c.Set(cfg.CacheHeader, cacheUnreachable)
						return true, c.Next()
					}

					// Separate body value to avoid msgp serialization
					// We can store raw bytes with Storage 👍
					if cfg.Storage != nil {
						guard.Unlock()
						rawBody, err := manager.getRaw(reqCtx, bodyKey(key))
						if err != nil {
							// A request whose context ended gets the error, as for a failed lookup (see unserved).
							if !errors.Is(err, errCacheMiss) && reqCtx.Err() != nil {
								manager.release(e)
								e = nil
								return true, err
							}
							// Metadata without its body goes unless replaced meanwhile; a failed read keeps it.
							if errors.Is(err, errCacheMiss) {
								err = deleteFound(reqCtx)
							}
							if err != nil {
								log.Warnf("%v; the entry is left as it is", err)
							} else {
								guard.Relock()
								removeDeleted(key, e.heapidx, e.heapgen)
								guard.Unlock()
							}
							manager.release(e)
							e = nil
							if reqDirectives.onlyIfCached {
								c.Set(cfg.CacheHeader, cacheUnreachable)
								return true, c.SendStatus(fiber.StatusGatewayTimeout)
							}
							return false, nil
						}
						e.body = rawBody
					} else {
						guard.Unlock()
					}
					// Copied, so a middleware that edits the body in place cannot edit the entry.
					c.Response().SetBody(e.body)
					// Set response headers from cache
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
					// The stored response's Vary, whether or not headers are stored: a
					// cache that selected by it has to say so (RFC 9111 §4.1), or the
					// next cache downstream keys the hit on the URL alone.
					if len(e.vary) > 0 {
						setFieldLine(&c.Response().Header, fiber.HeaderVary, e.vary, canonical)
					}
					clampedDate := clampDateSeconds(e.date, ts)
					// Formatted into a local array rather than a fresh slice: the value
					// is copied into the header store, so nothing outlives this line and
					// the nil destination was an allocation on every cache hit.
					var dateBuf [httpDateLen]byte
					dateValue := utils.AppendHTTPDate(dateBuf[:0], secondsToTime(clampedDate))
					setFieldLine(&c.Response().Header, fiber.HeaderDate, dateValue, canonical)
					// Replayed per line; a name the response holds is Set to a placeholder first, keeping line order.
					var placedBuf [1]uint64
					placed := placedBuf[:] // the stored lines that write over a placeholder
					if len(e.headers) > 64 {
						placed = make([]uint64, (len(e.headers)+63)/64)
					}
					for i := range e.headers {
						if isIgnoredHeader(e.headers[i].key) {
							// An entry stored before Set-Cookie joined ignoreHeaders still
							// carries one, which is not replayed (see ignoreHeaders).
							continue
						}
						name := utils.UnsafeString(e.headers[i].key)
						if !canonical {
							// Set is byte-exact, so lines under other spellings are cleared first.
							fieldname.DelOthers(&c.Response().Header, name)
						}
						if held := c.Response().Header.PeekAll(name); len(held) > 0 && !bytes.Equal(held[0], replayPlaceholder) {
							c.Response().Header.SetBytesKV(e.headers[i].key, replayPlaceholder)
							placed[i/64] |= 1 << (i % 64)
						}
					}
					for i := range e.headers {
						switch {
						case isIgnoredHeader(e.headers[i].key):
						case placed[i/64]&(1<<(i%64)) != 0:
							c.Response().Header.SetBytesKV(e.headers[i].key, e.headers[i].value)
						default:
							c.Response().Header.AddBytesKV(e.headers[i].key, e.headers[i].value)
						}
					}
					// Set Cache-Control header if not disabled and not already set
					if !cfg.DisableCacheControl && len(fieldname.First(&c.Response().Header, fiber.HeaderCacheControl, canonical)) == 0 {
						remaining := uint64(0)
						if entryExp > ts {
							remaining = entryExp - ts
						}
						// private for an entry that holds one user's response: public, or
						// a bare max-age, lets a shared cache downstream store this hit
						// and serve it to whoever asks for the URL next.
						// So does one for a request with a body, a QUERY's aside (RFC 10008): the key may hold it.
						bodyKeyed := false
						length := c.Request().Header.ContentLength() // -1 is chunked, -2 no body framing at all
						carriesBody := length > 0 || length == -1
						if cfg.usesDefaultKeyGenerator {
							var body []byte
							body, bodyKeyed = bodySegment(c, canonical)
							carriesBody = len(body) > 0
						}
						directive := publicMaxAge
						if holdsCredential() || (carriesBody && requestMethod != fiber.MethodQuery) {
							directive = privateMaxAge
						}
						// Built in a local array: FormatUint and the join were two
						// allocations, and the header store copies the bytes anyway.
						var ccBuf [len(privateMaxAge) + maxUintDigits]byte
						cacheControlValue := utils.AppendUint(append(ccBuf[:0], directive...), remaining)
						setFieldLine(&c.Response().Header, fiber.HeaderCacheControl, cacheControlValue, canonical)

						// Vary on the key's headers, or a downstream cache serves this hit to all (RFC 9111 §4.1).
						vary, varyValue := keyVary, keyVaryValue
						if bodyKeyed {
							vary, varyValue = bodyVary, bodyVaryValue
						}
						switch {
						case len(vary) == 0:
						case len(fieldname.First(&c.Response().Header, fiber.HeaderVary, canonical)) == 0:
							setFieldLine(&c.Response().Header, fiber.HeaderVary, varyValue, canonical)
						default:
							var varyBuf [256]byte
							current := joinedHeader(&c.Response().Header, fiber.HeaderVary, canonical)
							if merged, added := appendMissingFields(varyBuf[:0], current, vary); added {
								setFieldLine(&c.Response().Header, fiber.HeaderVary, merged, canonical)
							}
						}
					}

					ageSeconds := min(entryAge, maxDeltaSeconds)

					// RFC-compliant Age header (RFC 9111)
					var ageBuf [maxUintDigits]byte
					setFieldLine(&c.Response().Header, fiber.HeaderAge, utils.AppendUint(ageBuf[:0], ageSeconds), canonical)
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
				guard.Unlock()
				c.Set(cfg.CacheHeader, cacheUnreachable)
				if reqDirectives.onlyIfCached {
					return true, c.SendStatus(fiber.StatusGatewayTimeout)
				}
				return false, nil
			}

			if e == nil && reqDirectives.onlyIfCached {
				guard.Unlock()
				c.Set(cfg.CacheHeader, cacheUnreachable)
				return true, c.SendStatus(fiber.StatusGatewayTimeout)
			}

			// An entry is still held here, so the switch found it unusable for this
			// request: the request said no-cache, which forbids serving a stored
			// response unvalidated, or the entry carries no expiry. Validating or
			// refreshing it means calling the handler, and only-if-cached forbids that
			// (RFC 9111 Section 5.2.1.7), so the answer is the same 504 as for a miss.
			// The entry stays stored for the requests that can use it.
			if e != nil && reqDirectives.onlyIfCached {
				manager.release(e)
				guard.Unlock()
				c.Set(cfg.CacheHeader, cacheUnreachable)
				return true, c.SendStatus(fiber.StatusGatewayTimeout)
			}

			// Remember the superseded entry's heap node so the replacement takes over its accounting.
			if e != nil {
				oldHeapIdx = e.heapidx
				oldHeapGen = e.heapgen
			}

			// make sure we're not blocking concurrent requests - do unlock
			guard.Unlock()
			return false, nil
		}()
		if handled || lookupErr != nil {
			return lookupErr
		}

		entryFound := e != nil || revalidate

		// Read before the handler can clear the credential or rewrite the path.
		urlGroup := refusals.group(c, generatedKey)
		// Also the headers, for a response that varies on other names than the lookup used: an inner
		// middleware may rewrite them in place, as encryptcookie does with the cookie it decrypts.
		var lookupHeader *fasthttp.RequestHeader
		if !cfg.DisableVaryHeaders {
			lookupHeader = acquireLookupHeader(&c.Request().Header)
			defer lookupHeaders.Put(lookupHeader)
		}
		var (
			scope     credentialScope
			recordKey string
		)
		if foundAtLookup {
			recordKey = refusals.key(c, generatedKey, varyNames, requestCreds())
			scope = requestCreds().scope()
		}

		// dropUnlessReplaced removes the entry found and its heap node unless a request replaced it first.
		dropUnlessReplaced := func() bool {
			replaced := false
			heapIdx, heapGen := oldHeapIdx, oldHeapGen
			if e != nil {
				heapIdx, heapGen = e.heapidx, e.heapgen
			}
			removeStaleHeap := true
			if cfg.Storage == nil {
				expected := e
				if expected == nil {
					expected = staleMemoryEntry
				}
				replaced = manager.delIf(key, expected) == memoryEntryReplaced
			} else {
				// Storage has no conditional delete: keep completed replacements; the read/delete race is intentional.
				current, err := manager.get(reqCtx, key)
				switch {
				case errors.Is(err, errCacheMiss):
					// No entry remains, but its Vary manifest still needs cleanup.
				case err != nil:
					log.Warnf("cache: failed to verify cached response for key %q: %v", maskKey(key), err)
					removeStaleHeap = false
				default:
					replaced = current.heapidx != heapIdx || current.heapgen != heapGen
					manager.release(current)
					if !replaced {
						if err := deleteKey(reqCtx, key); err != nil {
							log.Warnf("cache: failed to delete cached response for key %q: %v", maskKey(key), err)
							removeStaleHeap = false
						}
					}
				}
			}
			if removeStaleHeap {
				mux.Lock()
				removeDeleted(key, heapIdx, heapGen)
				mux.Unlock()
			}
			return replaced
		}

		dropManifest := func() {
			if err := manager.del(reqCtx, manifestKey); err != nil {
				log.Warnf("cache: failed to delete stale vary manifest %q: %v", maskKey(manifestKey), err)
			}
		}

		// manager.release is a no-op for in-memory storage, so no guard.
		releaseEntry := func() {
			if e != nil {
				manager.release(e)
				e = nil
			}
		}

		// dropReplacement removes what a concurrent request stored under the key meanwhile, and its heap node.
		dropReplacement := func() {
			current, err := manager.get(reqCtx, key)
			if err != nil {
				if !errors.Is(err, errCacheMiss) {
					log.Warnf("cache: failed to verify cached response for key %q: %v", maskKey(key), err)
				}
				return
			}
			heapIdx, heapGen := current.heapidx, current.heapgen
			manager.release(current)
			if cfg.Storage == nil {
				manager.delIf(key, current)
			} else if err := deleteKey(reqCtx, key); err != nil {
				log.Warnf("cache: failed to delete cached response for key %q: %v", maskKey(key), err)
				return
			}
			mux.Lock()
			removeDeleted(key, heapIdx, heapGen)
			mux.Unlock()
		}

		// dropRefused ends the refused entry and any replacement stored meanwhile, which may predate the
		// refusal. The Vary manifest goes too when the key holds the credential; otherwise it is the others'.
		dropRefused := func() {
			if dropUnlessReplaced() {
				dropReplacement()
			}
			if hasVaryManifest && scope.keyed {
				dropManifest()
			}
			if recordKey != "" {
				refusals.record(reqCtx, urlGroup, recordKey, foundExp)
			}
		}

		began := purges.stamp()
		// How a write finds the entry this request may store, its path read before the handler can rewrite it.
		var tag purgeTag
		if cfg.MaxBytes > 0 {
			tag = purgeTag{slot: purges.slot(c.Host(), c.Path()), stored: began}
		}

		// Continue stack, return err to Fiber if exist
		if err := c.Next(); err != nil {
			// A handler that returns the refusal (fiber.ErrUnauthorized) writes no status.
			if scope.entry && errorRefusesCredential(err) {
				dropRefused()
				// A handler timeout.New gave up on may still write the response; IsAbandoned orders the read.
				if !c.IsAbandoned() || c.RequestCtx().LastTimeoutErrorResponse() == nil {
					c.Set(cfg.CacheHeader, cacheUnreachable)
				}
			}
			releaseEntry()
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
			releaseEntry()
			c.Set(cfg.CacheHeader, cacheUnreachable)
		}

		// A status that is never stored is no verdict on the entry or its Vary
		// manifest: an error page carrying no-store must not purge a healthy one.
		// The exception is a refusal, when the request found an entry.
		status := c.Res().StatusCode()
		refused := foundAtLookup && refusesCredential(status)

		// The origin accepting a credential recorded as refused ends the record, stored or not.
		if refusalRecorded && acceptsCredential(status) {
			refusals.forget(reqCtx, recordKey)
		}

		if _, cacheable := cacheableStatusCodes[status]; !cacheable && !refused {
			markUnreachable()
			return nil
		}

		// The response's own spelling, not the app's: a proxy hands c.Response()
		// to an outbound fasthttp.Client, which stamps its own normalizing setting
		// on it. Reading a lower-case "cache-control: private" as absent cached
		// one client's body and replayed it to the next under "public, max-age".
		respCanonical := fieldname.Canonical(&c.Response().Header)

		cacheControlBytes := joinedHeader(&c.Response().Header, fiber.HeaderCacheControl, respCanonical)
		respCacheControl := parseResponseCacheControl(cacheControlBytes)
		varyHeader := utils.UnsafeString(joinedHeader(&c.Response().Header, fiber.HeaderVary, respCanonical))
		respVaryNames, varyHasStar := parseVary(varyHeader)
		hasCleanupDirective := respCacheControl.hasNoStore || respCacheControl.hasPrivate || respCacheControl.hasNoCache || varyHasStar

		// A refusal ends the entry found only within the credential's reach: a client turned away for its
		// own reasons must not end what others share.
		if refused {
			if scope.entry {
				dropRefused()
			}
			markUnreachable()
			return nil
		}

		// RFC 9111 forbids storing no-store responses and responses with Vary: *.
		// The latter remains true when response-driven Vary partitioning is disabled.
		if hasCleanupDirective {
			replaced := entryFound && dropUnlessReplaced()

			// A concurrent request that replaced the entry keeps its manifest too.
			// A plain no-store miss has no stale entry to invalidate. Preserve
			// the shared manifest so sibling variants remain reachable.
			if !replaced && !cfg.DisableVaryHeaders && hasVaryManifest && (!respCacheControl.hasNoStore || entryFound) {
				dropManifest()
			}

			markUnreachable()
			return nil
		}

		shouldStoreVaryManifest := !cfg.DisableVaryHeaders && len(respVaryNames) > 0
		entryManifest := "" // the manifest the entry is stored under, dropped with the last such entry
		if !cfg.DisableVaryHeaders && len(respVaryNames) > 0 {
			// The key is built from the request headers as the lookup saw them, which is
			// what a later lookup reproduces. A header an inner middleware added,
			// rewrote or removed since, as one deriving X-User-Id from the session
			// cookie does, gave the handler a value no client's key selects by, so the
			// first client's page would be served to every other sending the same
			// placeholder: stored nowhere instead (cache.md says to register the cache
			// behind such middleware).
			if varyHeadersChanged(respVaryNames, lookupHeader, &c.Request().Header, canonical) {
				markUnreachable()
				return nil
			}
			entryManifest = manifestKey
			// Keyed by the response's own Vary, not the manifest the lookup used, or a
			// Vary: Cookie page lands under a key no cookie is part of.
			if !slices.Equal(respVaryNames, varyNames) {
				key = varyKey(baseKey, respVaryNames, lookupHeader, canonical)
			}
		} else if !cfg.DisableVaryHeaders && hasVaryManifest {
			if err := manager.del(reqCtx, manifestKey); err != nil {
				// Lookups would follow the stale manifest, so this and every Storage failure below leave it unstored.
				log.Warnf("cache: failed to delete stale vary manifest %q: %v", maskKey(manifestKey), err)
				markUnreachable()
				return nil
			}
			key = baseKey
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

		// Don't cache response if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			markUnreachable()
			return nil
		}

		// A stream is judged by its declared length: one that declares none may never end, and one
		// that delivers other than it declares is passed on unstored.
		if c.Response().IsBodyStream() {
			if length := c.Response().Header.ContentLength(); length < 0 || (cfg.MaxBytes > 0 && uint(length) > cfg.MaxBytes) ||
				!sizedstream.Read(c.Response(), length) {
				markUnreachable()
				return nil
			}
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
		releaseEntry()
		e = manager.acquire()
		// Cache response
		e.body = utils.CopyBytes(c.Response().Body())
		e.status = status
		e.ctype = utils.CopyBytes(c.Response().Header.ContentType())
		e.cencoding = utils.CopyBytes(c.Response().Header.Peek(fiber.HeaderContentEncoding))
		e.private = false
		e.cacheControl = utils.CopyBytes(cacheControlBytes)
		e.expires = utils.CopyBytes(fieldname.First(&c.Response().Header, fiber.HeaderExpires, respCanonical))
		e.etag = utils.CopyBytes(fieldname.First(&c.Response().Header, fiber.HeaderETag, respCanonical))
		// Every field line, joined: the stored response has to be replayed with
		// the Vary it was selected by, and Peek would drop a second line.
		e.vary = utils.CopyBytes(joinedHeader(&c.Response().Header, fiber.HeaderVary, respCanonical))
		e.stored = began
		e.date = 0

		ageVal := uint64(0)
		if b := fieldname.First(&c.Response().Header, fiber.HeaderAge, respCanonical); len(b) > 0 {
			// Age is a delta-seconds too (RFC 9111 Section 5.1).
			if v, ok := parseUintDirective(b); ok {
				ageVal = v
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
		e.headers = e.headers[:0]
		// Fields named in Connection are hop-by-hop (RFC 9110 §7.6.1).
		connection := utils.UnsafeString(joinedHeader(&c.Response().Header, fiber.HeaderConnection, respCanonical))
		if !cfg.StoreResponseHeaders && (connection == "" || !headerlist.ContainsFold(connection, fiber.HeaderLocation)) {
			// Keep a redirect's destination even when headers are not stored: 300 and
			// 301 are cacheable, so without this the first client got
			// "301 Location: /new" and every one after it a bare 301.
			if location := fieldname.First(&c.Response().Header, fiber.HeaderLocation, respCanonical); len(location) > 0 {
				e.headers = append(e.headers, cachedHeader{
					key:   utils.CopyBytes(utils.UnsafeBytes(fiber.HeaderLocation)),
					value: utils.CopyBytes(location),
				})
			}
		}
		// A header that middleware inside this one wrote for the client of this request,
		// as the session middleware writes the session ID into one, is that client's:
		// the entry is served to every client that hits it, so it is not kept, as
		// Set-Cookie is not.
		personal := personalheader.Marked(c.RequestCtx())
		for key, value := range c.Response().Header.All() {
			if isIgnoredHeader(key) || personal.Has(key) || (!cfg.StoreResponseHeaders && !isKeptHeader(key)) ||
				(connection != "" && headerlist.ContainsFold(connection, utils.UnsafeString(key))) {
				continue
			}

			e.headers = append(e.headers, cachedHeader{
				key:   utils.CopyBytes(key),
				value: utils.CopyBytes(value),
			})
		}

		if cfg.Storage != nil && !e.decodable() {
			markUnreachable()
			return nil
		}

		// A short body can sit under a key or stored headers many times its size, so all three are charged.
		entrySize := bodySize + uint(len(key)) + e.metaSize()
		if cfg.MaxBytes > 0 && entrySize > cfg.MaxBytes {
			markUnreachable()
			return nil
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
		if cfg.holdsCredentialResponse(c, e.vary) {
			refusals.noteLifetime(urlGroup, storageExpiration)
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
		// 3. On deletion failure: restore storedBytes and leave the response unstored
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
				storedBytes -= entrySize
			}
			// A replacement body that landed overwrote the one the old entry
			// pointed at, and the store-error cleanup then deletes it, so that
			// entry can no longer be served: restoring its accounting would
			// count bytes nothing can read. The next request for the key finds
			// metadata without a body and drops it.
			restore := replacedOK && !stored && !rawWritten
			mux.Unlock()

			if restore {
				if err := restoreHeapEntry(reqCtx, replaced); err != nil {
					log.Warnf("%v", err)
				}
			}
		}()

		if cfg.MaxBytes > 0 {
			mux.Lock()
			// Remove the entry this request read. Another request may already
			// have replaced it, so also remove the current node for this key.
			if oldHeapIdx >= 0 {
				if candidate, ok := removeHeapEntry(key, oldHeapIdx, oldHeapGen); ok {
					replaced, replacedOK = candidate, true
				}
				oldHeapIdx = -1
			}
			if currentIdx, ok := heap.findKey(key); ok {
				currentGen := heap.generation(currentIdx)
				if candidate, removed := removeHeapEntry(key, currentIdx, currentGen); removed {
					replaced, replacedOK = candidate, true
				}
			}
			// Reserve space for the new entry first
			storedBytes += entrySize
			spaceReserved = true

			// Now evict entries until we're under the limit
			var candidates []evictionCandidate

			for storedBytes > cfg.MaxBytes || heap.Len() >= maxEntries {
				if heap.Len() == 0 {
					// The rest is reserved by stores still in flight. The popped entries are
					// still stored, so they go back, in reverse to regain their heap indices.
					for _, candidate := range slices.Backward(candidates) {
						retrack(candidate)
					}
					storedBytes -= entrySize
					// Set spaceReserved to false so the deferred cleanup does not unreserve again
					spaceReserved = false
					mux.Unlock()
					markUnreachable()
					return nil
				}
				// Entries a write ended go first, as no request can be served them, then expired ones:
				// only a request that accepts a stale response can use those.
				next, found := heap.removeEnded()
				if !found {
					next, found = heap.removeExpired(nowUnix)
				}
				if !found {
					next = heap.entries[0]
					heap.removeFirst()
				}
				candidates = append(candidates, evictionCandidate{
					key:      next.key,
					manifest: next.manifest,
					size:     next.bytes,
					exp:      next.exp,
					expires:  next.expires,
					oldGen:   next.gen,
					tag:      next.tag,
				})
				storedBytes -= next.bytes
			}
			trackEvicting(candidates)
			mux.Unlock()

			// Perform deletions outside the lock
			if len(candidates) > 0 {
				failed := -1
				var delErr error
				func() {
					// Done, or the Storage panicked: either way no popped node stays recorded.
					defer func() {
						if failed < 0 {
							mux.Lock()
							untrackEvicting(candidates)
							mux.Unlock()
						}
					}()
					for i := range candidates {
						if delErr = evict(reqCtx, candidates[i]); delErr != nil {
							failed = i
							return
						}
					}
				}()
				if failed >= 0 {
					// Deletion failed: restore only entries still present in storage.
					mux.Lock()
					// Unreserve space for the new entry
					storedBytes -= entrySize
					spaceReserved = false

					// Untouched candidates go back unread; the failed one and those a store took up are read back.
					var readBack []evictionCandidate
					if !evicting[poppedNode{key: candidates[failed].key, gen: candidates[failed].oldGen}] {
						readBack = append(readBack, candidates[failed])
					}
					for _, candidate := range slices.Backward(candidates[failed+1:]) {
						switch {
						case evicting[poppedNode{key: candidate.key, gen: candidate.oldGen}]:
						case evictingKeys[candidate.key].superseded:
							readBack = append(readBack, candidate)
						default:
							retrack(candidate)
						}
					}
					untrackEvicting(candidates)
					mux.Unlock()
					for _, candidate := range readBack {
						if err := restoreHeapEntry(reqCtx, candidate); err != nil {
							log.Warnf("%v", err)
						}
					}

					log.Warnf("cache: failed to delete key %q while evicting: %v", maskKey(candidates[failed].key), delErr)
					markUnreachable()
					return nil
				}
			}
		}

		if !cfg.DisableVaryHeaders && shouldStoreVaryManifest {
			if err := storeVaryManifest(reqCtx, manager, manifestKey, respVaryNames, storageExpiration); err != nil {
				log.Warnf("%v", err)
				markUnreachable()
				return nil
			}
		}

		e.exp = responseTS + uint64(remainingExpiration.Seconds())
		e.ttl = uint64(expiration.Seconds())
		if expiresParseError {
			e.exp = responseTS + 1
		}
		evictAt := min(e.exp, responseTS+evictionHorizon)
		entryExpires := e.exp // manager.set hands e back to the pool

		// Store entry in heap (space already reserved in eviction phase)
		var heapIdx int
		var heapGen uint64
		if cfg.MaxBytes > 0 {
			mux.Lock()
			// A concurrent store can complete after the reservation above.
			// Supersede its node atomically with this insertion.
			if currentIdx, ok := heap.findKey(key); ok {
				currentGen := heap.generation(currentIdx)
				if candidate, removed := removeHeapEntry(key, currentIdx, currentGen); removed {
					replaced, replacedOK = candidate, true
				}
			}
			heapIdx = heap.put(key, evictAt, entryExpires, entrySize, entryManifest, tag)
			e.heapidx = heapIdx
			heapGen = heap.generation(heapIdx)
			e.heapgen = heapGen
			// An eviction holding a popped node of this key must read it back.
			if popped, ok := evictingKeys[key]; ok {
				popped.superseded = true
				evictingKeys[key] = popped
			}
			// Note: storedBytes was incremented during reservation, and evictions
			// have already been accounted for, so no additional increment is needed
			spaceReserved = false // Clear flag to prevent defer from unreserving
			if cfg.Storage == nil {
				// Set with its node, so no eviction pops the node before the entry exists.
				manager.memory.Set(key, e, storageExpiration)
				stored = true
			}
			orphans := heap.takeOrphans()
			mux.Unlock()

			// MaxBytes counts no manifest, so one whose last entry was evicted or dropped goes too.
			for _, orphan := range orphans {
				if err := manager.del(reqCtx, orphan); err != nil {
					log.Warnf("cache: failed to delete stale vary manifest %q: %v", maskKey(orphan), err)
				}
			}
		} else if cfg.Storage != nil {
			// Persist an identity even without heap tracking so cleanup can tell
			// this entry from a same-key replacement.
			e.heapgen = heap.nextGeneration()
		}

		cleanupOnStoreError := func(ctx context.Context, releaseEntry, rawStored bool) error {
			var cleanupErr error
			if cfg.MaxBytes > 0 {
				mux.Lock()
				removeDeleted(key, heapIdx, heapGen)
				mux.Unlock()
			}
			if releaseEntry {
				manager.release(e)
			}
			if rawStored {
				rawKey := bodyKey(key)
				if err := manager.del(ctx, rawKey); err != nil {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cache: failed to delete raw key %q after store error: %w", maskKey(rawKey), err))
				}
			}
			return cleanupErr
		}

		notStored := func(err, cleanupErr error) error {
			log.Warnf("%v", err)
			if cleanupErr != nil {
				log.Warnf("%v", cleanupErr)
			}
			e = nil
			c.Set(cfg.CacheHeader, cacheUnreachable)
			return nil
		}

		// For external Storage we store raw body separated
		if cfg.Storage != nil {
			if err := manager.setRaw(reqCtx, bodyKey(key), e.body, storageExpiration); err != nil {
				return notStored(err, cleanupOnStoreError(reqCtx, true, false))
			}
			rawWritten = true
			// avoid body msgp encoding
			e.body = nil
			if err := manager.set(reqCtx, key, e, storageExpiration); err != nil {
				return notStored(err, cleanupOnStoreError(reqCtx, false, true))
			}

			// The write runs outside the lock, so an eviction may have popped the node and run
			// its delete before the entry existed. Take the entry back out then, or nothing counts it.
			if cfg.MaxBytes > 0 {
				mux.Lock()
				_, tracked := heap.findKey(key)
				mux.Unlock()
				if !tracked {
					delErr := deleteKey(reqCtx, key)
					if delErr == nil {
						// Gone now, so an eviction that popped the node and then fails must not put it back.
						mux.Lock()
						removeDeleted(key, heapIdx, heapGen)
						mux.Unlock()
						// Neither served from the cache nor stored; the replaced body was overwritten, so nothing is restored.
						c.Set(cfg.CacheHeader, cacheUnreachable)
						return nil
					}
					// Still stored, so track it again for a later eviction to retry.
					untracked := evictionCandidate{
						key: key, manifest: entryManifest, size: entrySize, exp: evictAt, expires: entryExpires, oldGen: heapGen, tag: tag,
					}
					log.Warnf("cache: failed to delete key %q evicted while it was stored: %v", maskKey(key), delErr)
					if err := restoreHeapEntry(reqCtx, untracked); err != nil {
						log.Warnf("%v", err)
					}
				}
			}
		} else if !stored {
			// Store entry in memory
			if err := manager.set(reqCtx, key, e, storageExpiration); err != nil {
				return notStored(err, cleanupOnStoreError(reqCtx, true, false))
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
