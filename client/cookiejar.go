// The code has been taken from https://github.com/valyala/fasthttp/pull/526 originally.
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

// AcquireCookieJar returns an empty CookieJar object from pool.
func AcquireCookieJar() *CookieJar {
	jar, ok := cookieJarPool.Get().(*CookieJar)
	if !ok {
		panic(errors.New("failed to type-assert to *CookieJar"))
	}

	return jar
}

// ReleaseCookieJar returns CookieJar to the pool.
func ReleaseCookieJar(c *CookieJar) {
	c.Release()
	cookieJarPool.Put(c)
}

// CookieJar manages cookie storage. It is used by the client to store cookies.
type CookieJar struct {
	mu          sync.Mutex
	hostCookies map[string][]*fasthttp.Cookie
}

// Get returns the cookies stored from a specific domain.
// If there were no cookies related with host returned slice will be nil.
//
// CookieJar keeps a copy of the cookies, so the returned cookies can be released safely.
func (cj *CookieJar) Get(uri *fasthttp.URI) []*fasthttp.Cookie {
	if uri == nil {
		return nil
	}

	return cj.getByHostAndPath(uri.Host(), uri.Path())
}

// get returns the cookies stored from a specific host and path.
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

// getCookiesByHost returns the cookies stored from a specific host.
// If cookies are expired they will be deleted.
func (cj *CookieJar) getCookiesByHost(host string) []*fasthttp.Cookie {
	cj.mu.Lock()
	defer cj.mu.Unlock()

	now := time.Now()
	cookies := cj.hostCookies[host]

	for i := 0; i < len(cookies); i++ {
		c := cookies[i]
		if !c.Expire().Equal(fasthttp.CookieExpireUnlimited) && c.Expire().Before(now) { // release cookie if expired
			cookies = append(cookies[:i], cookies[i+1:]...)
			fasthttp.ReleaseCookie(c)
			i--
		}
	}

	return cookies
}

// Set sets cookies for a specific host.
// The host is get from uri.Host().
// If the cookie key already exists it will be replaced by the new cookie value.
//
// CookieJar keeps a copy of the cookies, so the parsed cookies can be released safely.
func (cj *CookieJar) Set(uri *fasthttp.URI, cookies ...*fasthttp.Cookie) {
	if uri == nil {
		return
	}

	cj.SetByHost(uri.Host(), cookies...)
}

// SetByHost sets cookies for a specific host.
// If the cookie key already exists it will be replaced by the new cookie value.
//
// CookieJar keeps a copy of the cookies, so the parsed cookies can be released safely.
func (cj *CookieJar) SetByHost(host []byte, cookies ...*fasthttp.Cookie) {
	hostStr := utils.UnsafeString(host)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}

	hostCookies, ok := cj.hostCookies[hostStr]
	if !ok {
		// If the key does not exist in the map, then we must make a copy for the key to avoid unsafe usage.
		hostStr = string(host)
	}

	for _, cookie := range cookies {
		c := searchCookieByKeyAndPath(cookie.Key(), cookie.Path(), hostCookies)
		if c == nil {
			// If the cookie does not exist in the slice, let's acquire new cookie and store it.
			c = fasthttp.AcquireCookie()
			hostCookies = append(hostCookies, c)
		}
		c.CopyTo(cookie) // override cookie properties
	}
	cj.hostCookies[hostStr] = hostCookies
}

// SetKeyValue sets a cookie by key and value for a specific host.
//
// This function prevents extra allocations by making repeated cookies
// not being duplicated.
func (cj *CookieJar) SetKeyValue(host, key, value string) {
	cj.SetKeyValueBytes(host, utils.UnsafeBytes(key), utils.UnsafeBytes(value))
}

// SetKeyValueBytes sets a cookie by key and value for a specific host.
//
// This function prevents extra allocations by making repeated cookies
// not being duplicated.
func (cj *CookieJar) SetKeyValueBytes(host string, key, value []byte) {
	c := fasthttp.AcquireCookie()
	c.SetKeyBytes(key)
	c.SetValueBytes(value)

	cj.SetByHost(utils.UnsafeBytes(host), c)
}

// dumpCookiesToReq dumps the stored cookies to the request.
func (cj *CookieJar) dumpCookiesToReq(req *fasthttp.Request) {
	uri := req.URI()

	cookies := cj.getByHostAndPath(uri.Host(), uri.Path())
	for _, cookie := range cookies {
		req.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	}
}

// parseCookiesFromResp parses the response cookies and stores them.
func (cj *CookieJar) parseCookiesFromResp(host, path []byte, resp *fasthttp.Response) {
	hostStr := utils.UnsafeString(host)

	cj.mu.Lock()
	defer cj.mu.Unlock()

	if cj.hostCookies == nil {
		cj.hostCookies = make(map[string][]*fasthttp.Cookie)
	}
	cookies, ok := cj.hostCookies[hostStr]
	if !ok {
		// If the key does not exist in the map then
		// we must make a copy for the key to avoid unsafe usage.
		hostStr = string(host)
	}

	now := time.Now()
	resp.Header.VisitAllCookie(func(key, value []byte) {
		isCreated := false
		c := searchCookieByKeyAndPath(key, path, cookies)
		if c == nil {
			c, isCreated = fasthttp.AcquireCookie(), true
		}

		_ = c.ParseBytes(value) //nolint:errcheck // ignore error
		if c.Expire().Equal(fasthttp.CookieExpireUnlimited) || c.Expire().After(now) {
			cookies = append(cookies, c)
		} else if isCreated {
			fasthttp.ReleaseCookie(c)
		}
	})
	cj.hostCookies[hostStr] = cookies
}

// Release releases all cookie values.
func (cj *CookieJar) Release() {
	for _, v := range cj.hostCookies {
		for _, c := range v {
			fasthttp.ReleaseCookie(c)
		}
	}
	cj.hostCookies = nil
}

// searchCookieByKeyAndPath searches for a cookie by key and path.
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
