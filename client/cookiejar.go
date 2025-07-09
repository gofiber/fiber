// The code was originally taken from https://github.com/valyala/fasthttp/pull/526.
package client

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
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
	return cj.cookiesForRequest(hostStr, path, secure)
}

// getCookiesByHost returns cookies stored for a specific host, removing any that have expired.
func (cj *CookieJar) getCookiesByHost(host string) []*fasthttp.Cookie {
	cj.mu.Lock()
	defer cj.mu.Unlock()

	now := time.Now()
	cookies := cj.hostCookies[host]

	for i := 0; i < len(cookies); i++ {
		c := cookies[i]
		// Remove expired cookies.
		if !c.Expire().Equal(fasthttp.CookieExpireUnlimited) && c.Expire().Before(now) {
			cookies = append(cookies[:i], cookies[i+1:]...)
			fasthttp.ReleaseCookie(c)
			i--
		}
	}

	return cookies
}

// cookiesForRequest returns cookies that match the given host, path and security settings.
func (cj *CookieJar) cookiesForRequest(host string, path []byte, secure bool) []*fasthttp.Cookie {
	cj.mu.Lock()
	defer cj.mu.Unlock()

	now := time.Now()
	var matched []*fasthttp.Cookie

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

			reqPath := path
			if len(reqPath) == 0 {
				reqPath = []byte("/")
			}
			cookiePath := c.Path()
			if len(cookiePath) == 0 {
				cookiePath = []byte("/")
			}
			if !bytes.HasPrefix(reqPath, cookiePath) {
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
	hostStr = utils.ToLower(hostStr)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}

	for _, cookie := range cookies {
		domain := utils.TrimLeft(cookie.Domain(), '.')
		utils.ToLowerBytes(domain)
		key := hostStr
		if len(domain) == 0 {
			cookie.SetDomain(hostStr)
		} else {
			key = utils.UnsafeString(domain)
			cookie.SetDomainBytes(domain)
		}

		hostCookies, ok := cj.hostCookies[key]
		if !ok {
			key = string([]byte(key))
		}

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
	cookies := cj.getByHostAndPath(uri.Host(), uri.Path(), secure)
	for _, cookie := range cookies {
		req.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
		fasthttp.ReleaseCookie(cookie)
	}
}

// parseCookiesFromResp parses the cookies from the response and stores them for the specified host and path.
func (cj *CookieJar) parseCookiesFromResp(host, path []byte, resp *fasthttp.Response) {
	hostStr := utils.UnsafeString(host)
	if h, _, err := net.SplitHostPort(hostStr); err == nil {
		hostStr = h
	}
	hostStr = utils.ToLower(hostStr)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}

	if _, ok := cj.hostCookies[hostStr]; !ok {
		hostStr = string([]byte(hostStr))
	}

	now := time.Now()
	for _, value := range resp.Header.Cookies() {
		tmp := fasthttp.AcquireCookie()
		_ = tmp.ParseBytes(value) //nolint:errcheck // ignore error

		domainBytes := utils.TrimLeft(tmp.Domain(), '.')
		utils.ToLowerBytes(domainBytes)
		key := hostStr
		if len(domainBytes) == 0 {
			tmp.SetDomain(hostStr)
		} else {
			key = utils.UnsafeString(domainBytes)
			tmp.SetDomainBytes(domainBytes)
		}

		if _, ok := cj.hostCookies[key]; !ok {
			key = string([]byte(key))
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
			// remove expired cookie from slice
			for i := 0; i < len(cookies); i++ {
				if cookies[i] == c {
					cookies = append(cookies[:i], cookies[i+1:]...)
					break
				}
			}
			cj.hostCookies[key] = cookies
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
func searchCookieByKeyAndPath(key, path []byte, cookies []*fasthttp.Cookie) *fasthttp.Cookie {
	for _, c := range cookies {
		if bytes.Equal(key, c.Key()) {
			if len(path) <= 1 || bytes.Equal(c.Path(), path) {
				return c
			}
		}
	}
	return nil
}

// domainMatch reports whether host domain-matches the given cookie domain.
func domainMatch(host, domain string) bool {
	host = utils.ToLower(host)
	domain = utils.UnsafeString(utils.TrimLeft(utils.UnsafeBytes(domain), '.'))
	domain = utils.ToLower(domain)
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}
