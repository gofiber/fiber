// The code was originally taken from https://github.com/valyala/fasthttp/pull/526.
package client

import (
	"bytes"
	"errors"
	"net"
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

	return cj.getByHostAndPath(uri.Host(), uri.Path())
}

// getByHostAndPath returns cookies stored for a specific host and path.
func (cj *CookieJar) getByHostAndPath(host, path []byte) []*fasthttp.Cookie {
	if cj.hostCookies == nil {
		return nil
	}

	var (
		err     error
		cookies []*fasthttp.Cookie
		hostStr = utils.UnsafeString(host)
	)

	// port must not be included.
	hostStr, _, err = net.SplitHostPort(hostStr)
	if err != nil {
		hostStr = utils.UnsafeString(host)
	}
	// get cookies deleting expired ones
	cookies = cj.getCookiesByHost(hostStr)

	newCookies := make([]*fasthttp.Cookie, 0, len(cookies))
	for i := 0; i < len(cookies); i++ {
		cookie := cookies[i]
		if len(path) > 1 && len(cookie.Path()) > 1 && !bytes.HasPrefix(cookie.Path(), path) {
			continue
		}
		newCookies = append(newCookies, cookie)
	}

	return newCookies
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

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}

	hostCookies, ok := cj.hostCookies[hostStr]
	if !ok {
		// If the key does not exist in the map, make a copy to avoid unsafe usage.
		hostStr = string(host)
	}

	for _, cookie := range cookies {
		existing := searchCookieByKeyAndPath(cookie.Key(), cookie.Path(), hostCookies)
		if existing == nil {
			// If the cookie does not exist, acquire a new one.
			existing = fasthttp.AcquireCookie()
			hostCookies = append(hostCookies, existing)
		}
		existing.CopyTo(cookie) // Override cookie properties.
	}
	cj.hostCookies[hostStr] = hostCookies
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
	cookies := cj.getByHostAndPath(uri.Host(), uri.Path())
	for _, cookie := range cookies {
		req.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	}
}

// parseCookiesFromResp parses the cookies from the response and stores them for the specified host and path.
func (cj *CookieJar) parseCookiesFromResp(host, path []byte, resp *fasthttp.Response) {
	hostStr := utils.UnsafeString(host)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}

	cookies, ok := cj.hostCookies[hostStr]
	if !ok {
		// If the key does not exist in the map, make a copy to avoid unsafe usage.
		hostStr = string(host)
	}

	now := time.Now()
	resp.Header.Cookies()(func(key, value []byte) bool {
		created := false
		c := searchCookieByKeyAndPath(key, path, cookies)
		if c == nil {
			c, created = fasthttp.AcquireCookie(), true
		}

		_ = c.ParseBytes(value) //nolint:errcheck // ignore error
		if c.Expire().Equal(fasthttp.CookieExpireUnlimited) || c.Expire().After(now) {
			cookies = append(cookies, c)
		} else if created {
			fasthttp.ReleaseCookie(c)
		}
		return true
	})
	cj.hostCookies[hostStr] = cookies
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
			if len(path) <= 1 || bytes.HasPrefix(c.Path(), path) {
				return c
			}
		}
	}
	return nil
}
