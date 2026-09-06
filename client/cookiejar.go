// The code was originally taken from https://github.com/valyala/fasthttp/pull/526.
package client

import (
	"bytes"
	"cmp"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	utilsbytes "github.com/gofiber/utils/v2/bytes"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/publicsuffix"
)

const (
	maxCookieJarHosts = 1024

	// maxCookiesPerHost bounds the cookies stored under one storage key.
	// Scoping path-less cookies to their default-path (RFC 6265 Section
	// 5.1.4) means one origin can mint a distinct entry per directory it
	// serves, so the per-host count needs its own ceiling — the host cap
	// alone no longer bounds the jar. RFC 6265 Section 5.3 suggests evicting
	// when a domain exceeds a limit; 64 is comfortably above what real sites
	// use.
	maxCookiesPerHost = 64

	// maxCookiesPerRequest bounds how many cookies one request may carry. The
	// per-key cap alone does not bound this: a host-only cookie and one
	// Domain= cookie per parent label are stored under different keys and all
	// domain-match the same request, so a host deep in a DNS tree can multiply
	// its allowance by its label count and inflate the Cookie header it makes
	// the client send. RFC 6265 Section 5.3 asks for at least 50 cookies per
	// domain; this sits above that floor, and the most specific cookies are
	// kept because the list is already sorted longest-path-first.
	//
	// It is applied in dumpCookiesToReq rather than in cookiesForRequest: the
	// limit is about what goes on the wire, and cookiesForRequest also backs the
	// exported Get, which promises every stored cookie matching the URI.
	maxCookiesPerRequest = 64

	// defaultCookiePathStr is the path assumed for a cookie that carries no
	// usable Path attribute and for a request with no path
	// (RFC 6265 Section 5.1.4).
	defaultCookiePathStr = "/"
)

// defaultCookiePath is the byte form of defaultCookiePathStr. Every use is
// read-only — it is only ever compared against or copied out of — so the one
// shared backing array is safe and avoids a per-call []byte conversion, which
// escapes to the heap on this path.
var defaultCookiePath = []byte(defaultCookiePathStr)

// Replacement pair for escapePercent, hoisted so it is not rebuilt per call.
var (
	percentByte    = []byte("%")
	percentEscaped = []byte("%25")
)

var cookieJarPool = sync.Pool{
	New: func() any {
		return &CookieJar{}
	},
}

// AcquireCookieJar returns an empty CookieJar object from the pool.
func AcquireCookieJar() *CookieJar {
	jar, ok := cookieJarPool.Get().(*CookieJar)
	if !ok {
		panic(errCookieJarTypeAssertion)
	}

	return jar
}

// ReleaseCookieJar returns a CookieJar object to the pool.
func ReleaseCookieJar(c *CookieJar) {
	c.Release()
	cookieJarPool.Put(c)
}

// CookieJar manages cookie storage for the client.
// CookieJar is safe for concurrent use, except Release. Release must not run
// concurrently with other methods, and the jar must not be used after Release.
type CookieJar struct {
	// hostCookies stores wrapped cookies keyed by storage scope:
	// host-only cookies use the request host, while domain cookies use the
	// accepted Domain attribute.
	// If release logic is re-enabled for these entries, iterate as storedCookie
	// values and call fasthttp.ReleaseCookie(stored.cookie) on the wrapped cookie.
	hostCookies map[string][]storedCookie
	seq         uint64
	mu          sync.Mutex
}

// nextSeqLocked returns the next write sequence number.
func (cj *CookieJar) nextSeqLocked() uint64 {
	cj.seq++
	return cj.seq
}

type storedCookie struct {
	cookie *fasthttp.Cookie
	// seq is the jar-wide sequence number of the write that last stored this
	// cookie. It orders equal-length paths deterministically in
	// cookiesForRequest (RFC 6265 Section 5.4 breaks such ties by creation
	// time; sorting by last write matches that for cookies that are never
	// rewritten, which is the common case) and picks the eviction victim in
	// enforceHostCookieLimitLocked. Refreshing it on every write is what keeps
	// a session cookie the server re-sends on each response from aging out
	// behind a flood of one-off cookies.
	seq        uint64
	isHostOnly bool
}

type cookieDomainAcceptance struct {
	domain     string
	isHostOnly bool
	isOk       bool
}

// Get returns all cookies stored for a given URI. If there are no cookies for the
// provided host, the returned slice will be nil.
//
// The CookieJar keeps its own copies of cookies, so it is safe to release the returned
// cookies after use.
func (cj *CookieJar) Get(uri *fasthttp.URI) []*fasthttp.Cookie {
	if uri == nil {
		return nil
	}

	secure := bytes.Equal(uri.Scheme(), httpsScheme)
	return cj.getByHostAndPath(uri.Host(), uri.Path(), secure)
}

// getByHostAndPath returns cookies stored for a specific host and path.
func (cj *CookieJar) getByHostAndPath(host, path []byte, secure bool) []*fasthttp.Cookie {
	if cj.hostCookies == nil {
		return nil
	}

	var (
		err     error
		hostStr = utils.UnsafeString(host)
	)

	// port must not be included.
	hostStr, _, err = net.SplitHostPort(hostStr)
	if err != nil {
		hostStr = utils.UnsafeString(host)
	}
	return cj.cookiesForRequest(hostStr, path, secure)
}

// getCookiesByHost returns cookies stored for a specific host, removing any that have expired.
func (cj *CookieJar) getCookiesByHost(host string) []*fasthttp.Cookie {
	cj.mu.Lock()
	defer cj.mu.Unlock()

	now := time.Now()
	stored := cj.hostCookies[host]

	kept := stored[:0]
	for _, sc := range stored {
		c := sc.cookie
		// Remove expired cookies.
		if !c.Expire().Equal(fasthttp.CookieExpireUnlimited) && c.Expire().Before(now) {
			fasthttp.ReleaseCookie(c)
			continue
		}
		kept = append(kept, sc)
	}
	if len(kept) == 0 {
		delete(cj.hostCookies, host)
	} else {
		clearVacated(stored, kept)
		cj.hostCookies[host] = kept
	}

	out := make([]*fasthttp.Cookie, 0, len(kept))
	for _, sc := range kept {
		out = append(out, sc.cookie)
	}
	return out
}

// cookiesForRequest returns cookies that match the given host, path and security settings.
func (cj *CookieJar) cookiesForRequest(host string, path []byte, secure bool) []*fasthttp.Cookie { //nolint:revive // secure is a deliberate scheme filter, not a control-flow flag
	cj.mu.Lock()
	defer cj.mu.Unlock()

	host = utilsstrings.ToLower(host)
	now := time.Now()
	var matched []matchedCookie

	for domain, cookies := range cj.hostCookies {
		if len(cookies) == 0 {
			continue
		}
		if !domainMatch(host, domain) {
			continue
		}

		kept := cookies[:0]
		for _, sc := range cookies {
			c := sc.cookie
			if !c.Expire().Equal(fasthttp.CookieExpireUnlimited) && c.Expire().Before(now) {
				fasthttp.ReleaseCookie(c)
				continue
			}
			kept = append(kept, sc)

			if sc.isHostOnly && host != domain {
				continue
			}
			if !pathMatch(path, c.Path()) {
				continue
			}
			if c.Secure() && !secure {
				continue
			}
			nc := fasthttp.AcquireCookie()
			nc.CopyTo(c)
			matched = append(matched, matchedCookie{cookie: nc, seq: sc.seq})
		}
		if len(kept) == 0 {
			delete(cj.hostCookies, domain)
		} else {
			clearVacated(cookies, kept)
			cj.hostCookies[domain] = kept
		}
	}

	// RFC 6265 Section 5.4 step 2: cookies with a longer path sort first,
	// ties broken by creation order. Both halves matter — matched is assembled
	// by ranging over hostCookies, so without an explicit tiebreak two
	// equal-length paths stored under different keys (a host-only cookie and a
	// Domain= cookie of the same name) would order randomly and the value put
	// on the wire would change run to run.
	if len(matched) > 1 {
		slices.SortStableFunc(matched, func(a, b matchedCookie) int {
			if d := len(b.cookie.Path()) - len(a.cookie.Path()); d != 0 {
				return d
			}
			return cmp.Compare(a.seq, b.seq)
		})
	}

	if len(matched) == 0 {
		// Get documents a nil result when nothing matches, and a caller may be
		// testing for exactly that; make([]T, 0) is not nil.
		return nil
	}

	out := make([]*fasthttp.Cookie, len(matched))
	for i, m := range matched {
		out[i] = m.cookie
	}
	return out
}

// matchedCookie pairs a cookie copy with the write sequence of the entry it
// came from, so cookiesForRequest can order equal-length paths deterministically.
type matchedCookie struct {
	cookie *fasthttp.Cookie
	seq    uint64
}

// Set stores the given cookies for the specified URI host. A stored cookie is
// replaced only when it matches on both key and normalized path within the same
// storage scope; cookies sharing a key at different paths coexist, and the most
// specific one is sent first (RFC 6265 Section 5.4).
//
// A cookie with no usable Path attribute is scoped to the URI's default-path
// (RFC 6265 Section 5.1.4), the same rule applied to a Set-Cookie received for
// that URI: cookies set against "/a/b" are scoped to "/a", not to the whole
// host. Set an explicit Path on the cookie to widen it.
//
// CookieJar stores copies of the provided cookies, so they may be safely released after use.
func (cj *CookieJar) Set(uri *fasthttp.URI, cookies ...*fasthttp.Cookie) {
	if uri == nil {
		return
	}
	cj.setByHostAndPath(uri.Host(), uri.Path(), cookies...)
}

// SetByHost stores the given cookies for the specified host. A stored cookie is
// replaced only when it matches on both key and normalized path within the same
// storage scope; cookies sharing a key at different paths coexist.
//
// There is no request path to derive a default-path from, so a cookie with no
// usable Path attribute is scoped to "/". Use Set when a URI is available and
// the RFC 6265 Section 5.1.4 scoping is wanted.
//
// CookieJar stores copies of the provided cookies, so they may be safely released after use.
func (cj *CookieJar) SetByHost(host []byte, cookies ...*fasthttp.Cookie) {
	cj.setByHostAndPath(host, nil, cookies...)
}

// setByHostAndPath backs Set and SetByHost. requestPath is the path of the URI
// the cookies were set against, used to derive the default-path for cookies
// that carry no usable Path attribute; a nil requestPath yields "/".
func (cj *CookieJar) setByHostAndPath(host, requestPath []byte, cookies ...*fasthttp.Cookie) {
	hostStr := utils.UnsafeString(host)
	if h, _, err := net.SplitHostPort(hostStr); err == nil {
		hostStr = h
	}
	hostStr = utilsstrings.ToLower(hostStr)
	hostKey := utils.CopyString(hostStr)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]storedCookie)
	}

	// One scratch cookie for the whole call, used only to learn what path
	// fasthttp will actually persist. Acquired outside the loop so a caller
	// passing many cookies still pays a single pooled get.
	scratch := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(scratch)

	defaultPath := defaultCookiePathFor(requestPath)

	// One clock reading for the whole call, as parseCookiesFromResp already does.
	// The capacity sweep below runs per cookie, and a fresh time.Now() inside it
	// bought nothing but a syscall per cookie while cj.mu is held.
	now := time.Now()

	for _, cookie := range cookies {
		// Fold with utilsstrings.ToLower rather than in place: the cookie
		// belongs to the caller, and the jar documents that it only stores
		// copies. ToLower returns its input unchanged when there is nothing to
		// fold, so `domain` may alias the caller's cookie buffer — every
		// retained use below copies (utils.CopyString for the map key,
		// Cookie.SetDomain for the stored value), and it must stay that way.
		domain := utilsstrings.ToLower(utils.UnsafeString(utils.TrimLeft(cookie.Domain(), '.')))
		key := hostKey
		storedDomain := hostStr
		isHostOnly := domain == ""
		if !isHostOnly {
			acceptance := acceptCookieDomain(hostStr, domain)
			if !acceptance.isOk {
				continue
			}
			isHostOnly = acceptance.isHostOnly
			if !isHostOnly {
				key = utils.CopyString(acceptance.domain)
				storedDomain = acceptance.domain
			}
		}

		cj.ensureHostCapacityLocked(key, now)
		hostCookies := cj.hostCookies[key]

		// Normalize the path up front so an entry stored through this API is
		// identified — and ordered — exactly like one parsed from a response.
		// Storing "" verbatim would make searchCookieByKeyAndPath miss the
		// response-stored twin (breaking this method's documented replace
		// semantics) and would always lose the specificity sort.
		//
		// A missing Path — or one that does not begin with '/', which fasthttp's
		// ParseBytes stores verbatim — falls back to the caller's default-path,
		// matching parseCookiesFromResp (RFC 6265 Sections 5.1.4 and 5.2.4).
		rawPath := cookie.Path()
		if len(rawPath) == 0 || rawPath[0] != '/' {
			rawPath = defaultPath
		}

		// Search on the path the entry will actually carry, not the one the
		// caller handed us. SetPathBytes runs the value through normalizePath,
		// which percent-decodes and rewrites ';', so for a cookie built by
		// ParseBytes the two can differ. Looking up the raw form would miss the
		// entry an earlier identical call created and append a duplicate
		// instead of replacing it, consuming per-host slots until eviction drops
		// a legitimate cookie. setDefaultCookiePath applies the same
		// escape-and-verify round trip used for response-parsed cookies, so
		// both entry points agree on the stored form.
		setDefaultCookiePath(scratch, rawPath)
		lookupPath := scratch.Path()

		seq := cj.nextSeqLocked()
		existing := searchCookieByKeyAndPath(cookie.Key(), lookupPath, hostCookies)
		if existing == nil {
			existing = fasthttp.AcquireCookie()
			hostCookies = append(hostCookies, storedCookie{cookie: existing, seq: seq, isHostOnly: isHostOnly})
		} else {
			for i := range hostCookies {
				if hostCookies[i].cookie == existing {
					hostCookies[i].isHostOnly = isHostOnly
					hostCookies[i].seq = seq
					break
				}
			}
		}
		existing.CopyTo(cookie)
		// Store through the same setter the lookup path came out of, so the
		// entry carries exactly the bytes the next lookup will search for.
		setDefaultCookiePath(existing, lookupPath)
		existing.SetDomain(storedDomain)
		cj.hostCookies[key] = hostCookies
		cj.enforceHostCookieLimitLocked(key)
	}
}

// SetKeyValue sets a cookie for the specified host with the given key and value.
//
// This function helps prevent extra allocations by avoiding duplication of repeated cookies.
func (cj *CookieJar) SetKeyValue(host, key, value string) {
	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	c.SetKey(key)
	c.SetValue(value)

	cj.SetByHost(utils.UnsafeBytes(host), c)
}

// SetKeyValueBytes sets a cookie for the specified host using byte slices for the key and value.
//
// This function helps prevent extra allocations by avoiding duplication of repeated cookies.
func (cj *CookieJar) SetKeyValueBytes(host string, key, value []byte) {
	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	c.SetKeyBytes(key)
	c.SetValueBytes(value)

	cj.SetByHost(utils.UnsafeBytes(host), c)
}

// dumpCookiesToReq writes the stored cookies to the given request.
//
// cookiesForRequest returns them in RFC 6265 Section 5.4 order (longest path
// first). fasthttp keys request cookies by name and cannot represent the same
// name twice, so where several stored cookies share a name only the first —
// the most specific — is written; writing them all would let the least
// specific overwrite the most specific.
func (cj *CookieJar) dumpCookiesToReq(req *fasthttp.Request) {
	uri := req.URI()
	secure := bytes.Equal(uri.Scheme(), httpsScheme)
	cookies := cj.getByHostAndPath(uri.Host(), uri.Path(), secure)

	// Bound what reaches the wire (see maxCookiesPerRequest). Truncating here
	// rather than in cookiesForRequest keeps the cap off Get, which promises
	// every stored cookie matching the URI. The list is already sorted
	// longest-path-first, so the cookies dropped are the least specific.
	writable := cookies
	if len(writable) > maxCookiesPerRequest {
		writable = writable[:maxCookiesPerRequest]
	}

	for i, cookie := range writable {
		// Linear scan rather than a set: the list is bounded by
		// maxCookiesPerRequest and is typically a handful, so this stays
		// allocation-free where a map would cost one alloc per request.
		if !containsCookieName(writable[:i], cookie.Key()) {
			req.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
		}
	}

	// Release only after the scan: ReleaseCookie resets the cookie, so
	// freeing as we go would blank the names the dedupe compares against and
	// let a less specific cookie overwrite a more specific one.
	for _, cookie := range cookies {
		fasthttp.ReleaseCookie(cookie)
	}
}

// containsCookieName reports whether any cookie in cookies carries name.
func containsCookieName(cookies []*fasthttp.Cookie, name []byte) bool {
	for _, c := range cookies {
		if bytes.Equal(c.Key(), name) {
			return true
		}
	}
	return false
}

// defaultCookiePathFor implements the RFC 6265 Section 5.1.4 default-path
// algorithm. A cookie that arrives without a Path attribute is scoped to the
// directory of the request that set it, not to the whole host: a Set-Cookie on
// "/a/b" defaults to "/a", so the cookie is not returned for "/".
func defaultCookiePathFor(requestPath []byte) []byte {
	if len(requestPath) == 0 || requestPath[0] != '/' {
		return defaultCookiePath
	}
	i := bytes.LastIndexByte(requestPath, '/')
	if i <= 0 {
		return defaultCookiePath
	}
	return requestPath[:i]
}

// setDefaultCookiePath stores path as c's Path attribute.
//
// SetPathBytes percent-decodes the value, and the path came from URI.Path()
// already decoded once, so setting it naively decodes twice — "/a%2541b/c"
// stored "/aAb". A path holding ';' cannot round-trip and falls back to "/".
func setDefaultCookiePath(c *fasthttp.Cookie, path []byte) {
	c.SetPathBytes(escapePercent(path))
	if !bytes.Equal(c.Path(), path) {
		c.SetPathBytes(defaultCookiePath)
	}
}

// escapePercent returns p with every '%' rewritten as "%25", so one round of
// percent-decoding reproduces p exactly. It returns p unchanged when there is
// nothing to escape, keeping the common path allocation-free (bytes.ReplaceAll
// copies even when it replaces nothing).
func escapePercent(p []byte) []byte {
	if bytes.IndexByte(p, '%') == -1 {
		return p
	}
	return bytes.ReplaceAll(p, percentByte, percentEscaped)
}

// parseCookiesFromResp parses the cookies from the response and stores them for the specified host and path.
func (cj *CookieJar) parseCookiesFromResp(host, path []byte, resp *fasthttp.Response) {
	hostStr := utils.UnsafeString(host)
	if h, _, err := net.SplitHostPort(hostStr); err == nil {
		hostStr = h
	}
	hostStr = utilsstrings.ToLower(hostStr)
	hostKey := utils.CopyString(hostStr)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]storedCookie)
	}

	now := time.Now()
	defaultPath := defaultCookiePathFor(path)
	for _, value := range resp.Header.Cookies() {
		tmp := fasthttp.AcquireCookie()
		_ = tmp.ParseBytes(value) //nolint:errcheck // ignore error
		applyMaxAge(tmp, now, value)

		// A Set-Cookie whose Path attribute is missing — or does not begin
		// with '/', which fasthttp's ParseBytes stores verbatim — is scoped to
		// the request's directory, not to the whole host
		// (RFC 6265 Sections 5.1.4 and 5.2.4). Without the second half a
		// relative "Path=admin" would be stored as-is and could never match a
		// request path, permanently occupying one of the per-host slots.
		if p := tmp.Path(); len(p) == 0 || p[0] != '/' {
			setDefaultCookiePath(tmp, defaultPath)
		}

		domainBytes := utils.TrimLeft(tmp.Domain(), '.')
		utilsbytes.UnsafeToLower(domainBytes)
		key := hostKey
		isHostOnly := len(domainBytes) == 0
		if isHostOnly {
			tmp.SetDomain(hostStr)
		} else {
			domain := utils.UnsafeString(domainBytes)
			acceptance := acceptCookieDomain(hostStr, domain)
			if !acceptance.isOk {
				fasthttp.ReleaseCookie(tmp)
				continue
			}
			isHostOnly = acceptance.isHostOnly
			if isHostOnly {
				tmp.SetDomain(hostStr)
			} else {
				key = utils.CopyString(acceptance.domain)
				tmp.SetDomain(acceptance.domain)
			}
		}

		cj.ensureHostCapacityLocked(key, now)
		cookies := cj.hostCookies[key]
		seq := cj.nextSeqLocked()
		c := searchCookieByKeyAndPath(tmp.Key(), tmp.Path(), cookies)
		if c == nil {
			c = fasthttp.AcquireCookie()
			cookies = append(cookies, storedCookie{cookie: c, seq: seq, isHostOnly: isHostOnly})
		} else {
			for i := range cookies {
				if cookies[i].cookie == c {
					cookies[i].isHostOnly = isHostOnly
					cookies[i].seq = seq
					break
				}
			}
		}

		c.CopyTo(tmp)
		if c.Expire().Equal(fasthttp.CookieExpireUnlimited) || c.Expire().After(now) {
			cj.hostCookies[key] = cookies
			cj.enforceHostCookieLimitLocked(key)
		} else {
			kept := cookies[:0]
			for _, v := range cookies {
				if v.cookie != c {
					kept = append(kept, v)
				}
			}
			if len(kept) == 0 {
				delete(cj.hostCookies, key)
			} else {
				clearVacated(cookies, kept)
				cj.hostCookies[key] = kept
			}
			fasthttp.ReleaseCookie(c)
		}
		fasthttp.ReleaseCookie(tmp)
	}
}

// enforceHostCookieLimitLocked bounds the cookies stored under one key. It
// drops expired entries first and then the least recently written ones.
//
// Recency, not creation order, is the eviction key (RFC 6265 Section 5.3 step
// 12 asks for least-recently-used): storedCookie.seq is refreshed on every
// write, so a session cookie the server re-sends on each response survives a
// flood of one-off cookies from other directories, which is exactly the
// pressure default-path scoping creates.
func (cj *CookieJar) enforceHostCookieLimitLocked(key string) {
	cookies := cj.hostCookies[key]
	if len(cookies) <= maxCookiesPerHost {
		return
	}

	now := time.Now()
	kept := cookies[:0]
	for _, sc := range cookies {
		if !sc.cookie.Expire().Equal(fasthttp.CookieExpireUnlimited) && sc.cookie.Expire().Before(now) {
			fasthttp.ReleaseCookie(sc.cookie)
			continue
		}
		kept = append(kept, sc)
	}

	if overflow := len(kept) - maxCookiesPerHost; overflow > 0 {
		// Order by seq only to pick victims, then restore insertion order so
		// nothing else observes a reshuffle.
		byRecency := slices.Clone(kept)
		slices.SortFunc(byRecency, func(a, b storedCookie) int { return cmp.Compare(a.seq, b.seq) })
		evicted := byRecency[:overflow]
		releaseStoredCookies(evicted)
		kept = slices.DeleteFunc(kept, func(sc storedCookie) bool {
			return slices.ContainsFunc(evicted, func(e storedCookie) bool { return e.cookie == sc.cookie })
		})
	}

	if len(kept) == 0 {
		delete(cj.hostCookies, key)
		return
	}
	clearVacated(cookies, kept)
	cj.hostCookies[key] = kept
}

// clearVacated zeroes the slots compaction left behind. kept aliases the front
// of original, so without this the tail still holds pointers to cookies that
// were handed back to fasthttp's pool — reachable from the jar's map, which
// keeps them alive and defeats the pool's GC handoff.
func clearVacated(original, kept []storedCookie) {
	clear(original[len(kept):])
}

// ensureHostCapacityLocked bounds the number of stored hosts by evicting
// expired entries first and then one remaining host if the jar is still full.
func (cj *CookieJar) ensureHostCapacityLocked(key string, now time.Time) {
	if _, ok := cj.hostCookies[key]; ok || len(cj.hostCookies) < maxCookieJarHosts {
		return
	}

	for host, cookies := range cj.hostCookies {
		kept := cookies[:0]
		for _, sc := range cookies {
			if !sc.cookie.Expire().Equal(fasthttp.CookieExpireUnlimited) && sc.cookie.Expire().Before(now) {
				fasthttp.ReleaseCookie(sc.cookie)
				continue
			}
			kept = append(kept, sc)
		}
		if len(kept) == 0 {
			delete(cj.hostCookies, host)
			if len(cj.hostCookies) < maxCookieJarHosts {
				return
			}
			continue
		}
		clearVacated(cookies, kept)
		cj.hostCookies[host] = kept
	}

	var evictHost string
	for host := range cj.hostCookies {
		if evictHost == "" || host < evictHost {
			evictHost = host
		}
	}
	if evictHost != "" {
		releaseStoredCookies(cj.hostCookies[evictHost])
		delete(cj.hostCookies, evictHost)
	}
}

// releaseStoredCookies releases pooled cookies for an evicted host entry.
func releaseStoredCookies(cookies []storedCookie) {
	for _, sc := range cookies {
		fasthttp.ReleaseCookie(sc.cookie)
	}
}

// Release releases all stored cookies. After this, the CookieJar is empty and
// must not be used again.
func (cj *CookieJar) Release() {
	// FOLLOW-UP performance optimization:
	// Currently, a race condition is found because the reset method modifies a value
	// that is not a copy but a reference. A solution would be to make a copy.
	// for _, v := range cj.hostCookies {
	//	  for _, c := range v {
	//		fasthttp.ReleaseCookie(c)
	//	  }
	// }
	cj.hostCookies = nil
}

// searchCookieByKeyAndPath looks up the stored cookie that a newly received
// cookie replaces. RFC 6265 Section 5.3 step 11 identifies a cookie by the
// triple (name, domain, path), and the caller has already selected the entry
// list for the domain — so the path must be *equal*, not merely path-matching.
// Using pathMatch here would let "a=2; Path=/admin" overwrite an existing
// "a=1; Path=/" instead of storing both.
func searchCookieByKeyAndPath(key, path []byte, cookies []storedCookie) *fasthttp.Cookie {
	for _, sc := range cookies {
		c := sc.cookie
		if bytes.Equal(key, c.Key()) && samePath(path, c.Path()) {
			return c
		}
	}
	return nil
}

// samePath compares two cookie paths, treating an empty path as the default
// "/" the same way pathMatch does.
func samePath(a, b []byte) bool {
	if len(a) == 0 {
		a = defaultCookiePath
	}
	if len(b) == 0 {
		b = defaultCookiePath
	}
	return bytes.Equal(a, b)
}

// pathMatch determines whether the request path matches the cookie path
// according to RFC 6265 section 5.1.4.
func pathMatch(reqPath, cookiePath []byte) bool {
	if len(reqPath) == 0 {
		reqPath = defaultCookiePath
	}
	if len(cookiePath) == 0 {
		cookiePath = defaultCookiePath
	}
	if bytes.Equal(reqPath, cookiePath) {
		return true
	}
	if !bytes.HasPrefix(reqPath, cookiePath) {
		return false
	}
	if cookiePath[len(cookiePath)-1] == '/' {
		return true
	}
	return len(reqPath) > len(cookiePath) && reqPath[len(cookiePath)] == '/'
}

// domainMatch reports whether host domain-matches the given cookie domain
// (RFC 6265 Section 5.1.3). The comparison itself is ASCII case-insensitive
// and allocation-free, but callers still normalize hosts and domains to
// lowercase: the jar's map keys and its exact-match checks (e.g. the
// host-only comparison in cookiesForRequest) rely on it.
func domainMatch(host, domain string) bool {
	if utils.EqualFold(host, domain) {
		return true
	}
	return len(host) > len(domain) &&
		host[len(host)-len(domain)-1] == '.' &&
		utils.HasSuffixFold(host, domain)
}

// acceptCookieDomain enforces RFC 6265 response-domain acceptance. Trailing-dot,
// exact-match public-suffix, and exact-match IP-literal Domain attributes are
// downgraded to host-only so same-host behavior is preserved without storing
// cookies under shared suffixes or allowing IP suffix matching across
// unrelated hosts.
func acceptCookieDomain(host, domain string) cookieDomainAcceptance {
	if strings.HasSuffix(domain, ".") {
		return cookieDomainAcceptance{domain: host, isHostOnly: true, isOk: true}
	}

	if host == domain {
		if isIPLiteral(domain) || isPublicSuffixDomain(domain) {
			return cookieDomainAcceptance{domain: host, isHostOnly: true, isOk: true}
		}
		return cookieDomainAcceptance{domain: domain, isOk: true}
	}

	if isIPLiteral(host) || isIPLiteral(domain) || isPublicSuffixDomain(domain) || !domainMatch(host, domain) {
		return cookieDomainAcceptance{}
	}

	return cookieDomainAcceptance{domain: domain, isOk: true}
}

func isIPLiteral(host string) bool {
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}

	// Equivalent to net.ParseIP(host) != nil: utils.ParseIPv4/ParseIPv6
	// accept the same strings once zoned addresses (which net.ParseIP
	// rejects) are screened out, without allocating on either outcome.
	if strings.IndexByte(host, '%') >= 0 {
		return false
	}
	if _, ok := utils.ParseIPv4(host); ok {
		return true
	}
	_, ok := utils.ParseIPv6(host)
	return ok
}

func isPublicSuffixDomain(domain string) bool {
	suffix, _ := publicsuffix.PublicSuffix(domain)

	return suffix == domain
}

// lastMaxAge returns the Max-Age a Set-Cookie value asks for: the last
// occurrence that is an integer, since a repeated attribute is resolved by the
// last one and a value that is not an integer is ignored (RFC 6265 §5.2.2).
// The raw value has to be read because fasthttp cannot answer this — it parses
// MaxAge as 0 whether the attribute is absent, zero or negative.
func lastMaxAge(value []byte) (int64, bool) {
	_, rest, found := bytes.Cut(value, []byte{';'})
	if !found {
		return 0, false
	}

	// Parsed at 64 bits: int is 32 bits on 386 and arm, where a Max-Age past
	// two billion would fail to parse and silently downgrade a persistent
	// cookie to a session one.
	seconds, ok := int64(0), false
	for len(rest) > 0 {
		var part []byte
		part, rest, _ = bytes.Cut(rest, []byte{';'})
		name, raw, hasValue := bytes.Cut(part, []byte{'='})
		if !hasValue || !utils.EqualFold(utils.UnsafeString(utils.TrimSpace(name)), "max-age") {
			continue
		}
		if n, err := strconv.ParseInt(utils.UnsafeString(utils.TrimSpace(raw)), 10, 64); err == nil {
			seconds, ok = n, true
		}
	}

	return seconds, ok
}

// applyMaxAge turns Max-Age into the absolute expiry the jar keeps. It takes
// precedence over Expires, and zero or less expires the cookie at once (RFC 6265 §5.2.2).
func applyMaxAge(c *fasthttp.Cookie, now time.Time, value []byte) {
	seconds, ok := lastMaxAge(value)
	switch {
	case !ok:
	case seconds <= 0:
		c.SetExpire(now.Add(-time.Second))
	default:
		c.SetExpire(now.Add(maxAgeDuration(seconds)))
	}
}

// maxAgeDuration converts Max-Age seconds to a duration, saturating at the
// longest one a time.Duration holds: a lifetime too far out to express is still
// a lifetime, not the expiry in the past that overflowing would produce.
func maxAgeDuration(seconds int64) time.Duration {
	if seconds > int64(math.MaxInt64/time.Second) {
		return math.MaxInt64
	}
	return time.Duration(seconds) * time.Second
}
