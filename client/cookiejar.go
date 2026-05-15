// The code was originally taken from https://github.com/valyala/fasthttp/pull/526.
package client

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	utilsbytes "github.com/gofiber/utils/v2/bytes"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/publicsuffix"
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

// CookieJar manages cookie storage for the client. It stores cookies keyed by host.
type CookieJar struct {
	// hostCookies stores wrapped cookies keyed by host.
	// If release logic is re-enabled for these entries, iterate as storedCookie
	// values and call fasthttp.ReleaseCookie(stored.cookie) on the wrapped cookie.
	hostCookies map[string][]storedCookie
	mu          sync.Mutex
}

type storedCookie struct {
	cookie   *fasthttp.Cookie
	hostOnly bool
}

type cookieDomainAcceptance struct {
	domain   string
	hostOnly bool
	ok       bool
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
	cj.hostCookies[host] = kept

	out := make([]*fasthttp.Cookie, 0, len(kept))
	for _, sc := range kept {
		out = append(out, sc.cookie)
	}
	return out
}

// cookiesForRequest returns cookies that match the given host, path and security settings.
//
//nolint:revive // secure is required to filter Secure cookies based on scheme
func (cj *CookieJar) cookiesForRequest(host string, path []byte, secure bool) []*fasthttp.Cookie {
	cj.mu.Lock()
	defer cj.mu.Unlock()

	host = utilsstrings.ToLower(host)
	now := time.Now()
	var matched []*fasthttp.Cookie

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

			if sc.hostOnly && host != domain {
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
			matched = append(matched, nc)
		}
		cj.hostCookies[domain] = kept
	}

	return matched
}

// Set stores the given cookies for the specified URI host. If a cookie key already exists,
// it will be replaced by the new cookie value.
//
// CookieJar stores copies of the provided cookies, so they may be safely released after use.
func (cj *CookieJar) Set(uri *fasthttp.URI, cookies ...*fasthttp.Cookie) {
	if uri == nil {
		return
	}
	cj.SetByHost(uri.Host(), cookies...)
}

// SetByHost stores the given cookies for the specified host. If a cookie key already exists,
// it will be replaced by the new cookie value.
//
// CookieJar stores copies of the provided cookies, so they may be safely released after use.
func (cj *CookieJar) SetByHost(host []byte, cookies ...*fasthttp.Cookie) {
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

	for _, cookie := range cookies {
		domain := utils.TrimLeft(cookie.Domain(), '.')
		utilsbytes.UnsafeToLower(domain)
		key := hostKey
		hostOnly := len(domain) == 0
		if hostOnly {
			cookie.SetDomain(hostStr)
		} else {
			acceptance := acceptCookieDomain(hostStr, utils.UnsafeString(domain))
			if !acceptance.ok {
				continue
			}
			hostOnly = acceptance.hostOnly
			if hostOnly {
				cookie.SetDomain(hostStr)
			} else {
				key = utils.CopyString(acceptance.domain)
				cookie.SetDomain(acceptance.domain)
			}
		}

		hostCookies := cj.hostCookies[key]

		existing := searchCookieByKeyAndPath(cookie.Key(), cookie.Path(), hostCookies)
		if existing == nil {
			existing = fasthttp.AcquireCookie()
			hostCookies = append(hostCookies, storedCookie{cookie: existing, hostOnly: hostOnly})
		} else {
			for i := range hostCookies {
				if hostCookies[i].cookie == existing {
					hostCookies[i].hostOnly = hostOnly
					break
				}
			}
		}
		existing.CopyTo(cookie)
		cj.hostCookies[key] = hostCookies
	}
}

// SetKeyValue sets a cookie for the specified host with the given key and value.
//
// This function helps prevent extra allocations by avoiding duplication of repeated cookies.
func (cj *CookieJar) SetKeyValue(host, key, value string) {
	c := fasthttp.AcquireCookie()
	c.SetKey(key)
	c.SetValue(value)

	cj.SetByHost(utils.UnsafeBytes(host), c)
}

// SetKeyValueBytes sets a cookie for the specified host using byte slices for the key and value.
//
// This function helps prevent extra allocations by avoiding duplication of repeated cookies.
func (cj *CookieJar) SetKeyValueBytes(host string, key, value []byte) {
	c := fasthttp.AcquireCookie()
	c.SetKeyBytes(key)
	c.SetValueBytes(value)

	cj.SetByHost(utils.UnsafeBytes(host), c)
}

// dumpCookiesToReq writes the stored cookies to the given request.
func (cj *CookieJar) dumpCookiesToReq(req *fasthttp.Request) {
	uri := req.URI()
	secure := bytes.Equal(uri.Scheme(), httpsScheme)
	cookies := cj.getByHostAndPath(uri.Host(), uri.Path(), secure)
	for _, cookie := range cookies {
		req.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
		fasthttp.ReleaseCookie(cookie)
	}
}

// parseCookiesFromResp parses the cookies from the response and stores them for the specified host and path.
func (cj *CookieJar) parseCookiesFromResp(host, _ []byte, resp *fasthttp.Response) {
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
	for _, value := range resp.Header.Cookies() {
		tmp := fasthttp.AcquireCookie()
		_ = tmp.ParseBytes(value) //nolint:errcheck // ignore error

		domainBytes := utils.TrimLeft(tmp.Domain(), '.')
		utilsbytes.UnsafeToLower(domainBytes)
		key := hostKey
		hostOnly := len(domainBytes) == 0
		if hostOnly {
			tmp.SetDomain(hostStr)
		} else {
			domain := utils.UnsafeString(domainBytes)
			acceptance := acceptCookieDomain(hostStr, domain)
			if !acceptance.ok {
				fasthttp.ReleaseCookie(tmp)
				continue
			}
			hostOnly = acceptance.hostOnly
			if hostOnly {
				tmp.SetDomain(hostStr)
			} else {
				key = utils.CopyString(acceptance.domain)
				tmp.SetDomain(acceptance.domain)
			}
		}

		cookies := cj.hostCookies[key]
		c := searchCookieByKeyAndPath(tmp.Key(), tmp.Path(), cookies)
		if c == nil {
			c = fasthttp.AcquireCookie()
			cookies = append(cookies, storedCookie{cookie: c, hostOnly: hostOnly})
		} else {
			for i := range cookies {
				if cookies[i].cookie == c {
					cookies[i].hostOnly = hostOnly
					break
				}
			}
		}

		c.CopyTo(tmp)
		if c.Expire().Equal(fasthttp.CookieExpireUnlimited) || c.Expire().After(now) {
			cj.hostCookies[key] = cookies
		} else {
			kept := cookies[:0]
			for _, v := range cookies {
				if v.cookie != c {
					kept = append(kept, v)
				}
			}
			cj.hostCookies[key] = kept
			fasthttp.ReleaseCookie(c)
		}
		fasthttp.ReleaseCookie(tmp)
	}
}

// Release releases all stored cookies. After this, the CookieJar is empty.
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

// searchCookieByKeyAndPath looks up a cookie by its key and path from the provided slice of cookies.
func searchCookieByKeyAndPath(key, path []byte, cookies []storedCookie) *fasthttp.Cookie {
	for _, sc := range cookies {
		c := sc.cookie
		if bytes.Equal(key, c.Key()) {
			if pathMatch(path, c.Path()) {
				return c
			}
		}
	}
	return nil
}

// pathMatch determines whether the request path matches the cookie path
// according to RFC 6265 section 5.1.4.
func pathMatch(reqPath, cookiePath []byte) bool {
	if len(reqPath) == 0 {
		reqPath = []byte("/")
	}
	if len(cookiePath) == 0 {
		cookiePath = []byte("/")
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

// domainMatch reports whether host domain-matches the given cookie domain.
func domainMatch(host, domain string) bool {
	host = utilsstrings.UnsafeToLower(host)

	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

// acceptCookieDomain enforces RFC 6265 response-domain acceptance. Exact-match
// public-suffix and exact-match IP-literal Domain attributes are downgraded to
// host-only so same-host behavior is preserved without storing cookies under
// shared suffixes or allowing IP suffix matching across unrelated hosts.
func acceptCookieDomain(host, domain string) cookieDomainAcceptance {
	if host == domain {
		if isIPLiteral(domain) || isPublicSuffixDomain(domain) {
			return cookieDomainAcceptance{domain: host, hostOnly: true, ok: true}
		}
		return cookieDomainAcceptance{domain: domain, ok: true}
	}

	if isIPLiteral(host) || isIPLiteral(domain) || isPublicSuffixDomain(domain) || !domainMatch(host, domain) {
		return cookieDomainAcceptance{}
	}

	return cookieDomainAcceptance{domain: domain, ok: true}
}

func isIPLiteral(host string) bool {
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}

	return net.ParseIP(host) != nil
}

func isPublicSuffixDomain(domain string) bool {
	suffix, _ := publicsuffix.PublicSuffix(domain)

	return suffix == domain
}
