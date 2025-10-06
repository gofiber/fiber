// The code was originally taken from https://github.com/valyala/fasthttp/pull/526.
package client

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	utils "github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const (
	cookieJarHostDefaultCap = 8
	cookieJarHostMaxEntries = 64
)

const (
	cookieJarMatchDefaultCap = 4
	cookieJarMatchMaxCap     = 128
)

var cookieJarPool = sync.Pool{
	New: func() any {
		return &CookieJar{}
	},
}

var cookieJarMatchPool = sync.Pool{
	New: func() any {
		slice := make([]*fasthttp.Cookie, 0, cookieJarMatchDefaultCap)
		return &slice
	},
}

// AcquireCookieJar returns an empty CookieJar object from the pool.
func AcquireCookieJar() *CookieJar {
	jar, ok := cookieJarPool.Get().(*CookieJar)
	if !ok {
		panic(errors.New("failed to type-assert to *CookieJar"))
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
	hostCookies map[string][]*fasthttp.Cookie
	mu          sync.Mutex
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

	secure := bytes.Equal(uri.Scheme(), []byte("https"))
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
	matches, _ := cj.collectCookiesForRequest(nil, hostStr, path, secure)
	return matches
}

func (cj *CookieJar) borrowCookiesByHostAndPath(host, path []byte, secure bool) ([]*fasthttp.Cookie, *[]*fasthttp.Cookie) {
	if cj.hostCookies == nil {
		return nil, nil
	}

	var (
		err     error
		hostStr = utils.UnsafeString(host)
	)

	hostStr, _, err = net.SplitHostPort(hostStr)
	if err != nil {
		hostStr = utils.UnsafeString(host)
	}

	return cj.borrowCookiesForRequest(hostStr, path, secure)
}

// getCookiesByHost returns cookies stored for a specific host, removing any that have expired.
func (cj *CookieJar) getCookiesByHost(host string) []*fasthttp.Cookie {
	cj.mu.Lock()
	defer cj.mu.Unlock()

	now := time.Now()
	cookies := cj.hostCookies[host]

	kept := cookies[:0]
	for _, c := range cookies {
		// Remove expired cookies.
		if !c.Expire().Equal(fasthttp.CookieExpireUnlimited) && c.Expire().Before(now) {
			fasthttp.ReleaseCookie(c)
			continue
		}
		kept = append(kept, c)
	}
	cj.hostCookies[host] = kept

	return kept
}

// cookiesForRequest returns cookies that match the given host, path and security settings.
func acquireCookieMatches() *[]*fasthttp.Cookie {
	sliceAny := cookieJarMatchPool.Get()
	matchesPtr, ok := sliceAny.(*[]*fasthttp.Cookie)
	if !ok {
		panic(errors.New("failed to type-assert to *[]*fasthttp.Cookie"))
	}

	matches := *matchesPtr
	if len(matches) > 0 {
		matches = matches[:0]
	}
	*matchesPtr = matches

	return matchesPtr
}

func releaseCookieMatches(matchesPtr *[]*fasthttp.Cookie) {
	if matchesPtr == nil {
		return
	}

	matches := *matchesPtr
	for i := range matches {
		matches[i] = nil
	}

	if cap(matches) > cookieJarMatchMaxCap {
		*matchesPtr = make([]*fasthttp.Cookie, 0, cookieJarMatchDefaultCap)
	} else {
		*matchesPtr = matches[:0]
	}

	cookieJarMatchPool.Put(matchesPtr)
}

func (cj *CookieJar) borrowCookiesForRequest(host string, path []byte, secure bool) ([]*fasthttp.Cookie, *[]*fasthttp.Cookie) {
	matchesPtr := acquireCookieMatches()
	matches, ptr := cj.collectCookiesForRequest(matchesPtr, host, path, secure)
	return matches, ptr
}

func (cj *CookieJar) collectCookiesForRequest( //nolint:revive // Accepting a bool param is fine here
	matchesPtr *[]*fasthttp.Cookie,
	host string,
	path []byte,
	secure bool,
) ([]*fasthttp.Cookie, *[]*fasthttp.Cookie) {
	cj.mu.Lock()
	defer cj.mu.Unlock()

	var matches []*fasthttp.Cookie
	if matchesPtr != nil {
		matches = *matchesPtr
	}

	matches = matches[:0]

	now := time.Now()

	for domain, cookies := range cj.hostCookies {
		if !domainMatch(host, domain) {
			continue
		}

		kept := cookies[:0]
		for _, c := range cookies {
			if !c.Expire().Equal(fasthttp.CookieExpireUnlimited) && c.Expire().Before(now) {
				fasthttp.ReleaseCookie(c)
				continue
			}
			kept = append(kept, c)

			if !pathMatch(path, c.Path()) {
				continue
			}
			if c.Secure() && !secure {
				continue
			}
			nc := fasthttp.AcquireCookie()
			nc.CopyTo(c)
			matches = append(matches, nc)
		}
		cj.hostCookies[domain] = kept
	}

	if matchesPtr != nil {
		*matchesPtr = matches
	}

	return matches, matchesPtr
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
	hostStr = utils.ToLower(hostStr)
	hostKey := utils.CopyString(hostStr)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}

	for _, cookie := range cookies {
		domain := utils.TrimLeft(cookie.Domain(), '.')
		utils.ToLowerBytes(domain)
		key := hostKey
		if len(domain) == 0 {
			cookie.SetDomain(hostStr)
		} else {
			key = utils.CopyString(utils.UnsafeString(domain))
			cookie.SetDomainBytes(domain)
		}

		hostCookies := cj.hostCookies[key]

		existing := searchCookieByKeyAndPath(cookie.Key(), cookie.Path(), hostCookies)
		if existing == nil {
			existing = fasthttp.AcquireCookie()
			hostCookies = append(hostCookies, existing)
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
	secure := bytes.Equal(uri.Scheme(), []byte("https"))
	cookies, matchesPtr := cj.borrowCookiesByHostAndPath(uri.Host(), uri.Path(), secure)
	defer releaseCookieMatches(matchesPtr)

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
	hostStr = utils.ToLower(hostStr)
	hostKey := utils.CopyString(hostStr)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}

	now := time.Now()
	for _, value := range resp.Header.Cookies() {
		tmp := fasthttp.AcquireCookie()
		_ = tmp.ParseBytes(value) //nolint:errcheck // ignore error

		domainBytes := utils.TrimLeft(tmp.Domain(), '.')
		utils.ToLowerBytes(domainBytes)
		key := hostKey
		if len(domainBytes) == 0 {
			tmp.SetDomain(hostStr)
		} else {
			key = utils.CopyString(utils.UnsafeString(domainBytes))
			tmp.SetDomainBytes(domainBytes)
		}

		cookies := cj.hostCookies[key]
		c := searchCookieByKeyAndPath(tmp.Key(), tmp.Path(), cookies)
		if c == nil {
			c = fasthttp.AcquireCookie()
			cookies = append(cookies, c)
		}

		c.CopyTo(tmp)
		if c.Expire().Equal(fasthttp.CookieExpireUnlimited) || c.Expire().After(now) {
			cj.hostCookies[key] = cookies
		} else {
			kept := cookies[:0]
			for _, v := range cookies {
				if v != c {
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
	cj.mu.Lock()
	defer cj.mu.Unlock()

	hostCount := len(cj.hostCookies)
	if hostCount == 0 {
		return
	}

	for _, cookies := range cj.hostCookies {
		for i, c := range cookies {
			if c == nil {
				continue
			}
			fasthttp.ReleaseCookie(c)
			cookies[i] = nil
		}
	}

	if hostCount > cookieJarHostMaxEntries {
		cj.hostCookies = nil
		return
	}

	if hostCount > cookieJarHostDefaultCap {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie, cookieJarHostDefaultCap)
		return
	}

	clear(cj.hostCookies)
}

// searchCookieByKeyAndPath looks up a cookie by its key and path from the provided slice of cookies.
func searchCookieByKeyAndPath(key, path []byte, cookies []*fasthttp.Cookie) *fasthttp.Cookie {
	for _, c := range cookies {
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
	host = utils.ToLower(host)

	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}
