package client

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func checkKeyValue(t *testing.T, cj *CookieJar, cookie *fasthttp.Cookie, uri *fasthttp.URI, n int) {
	t.Helper()

	cs := cj.Get(uri)
	require.GreaterOrEqual(t, len(cs), n)

	c := cs[n-1]
	require.NotNil(t, c)

	require.Equal(t, string(c.Key()), string(cookie.Key()))
	require.Equal(t, string(c.Value()), string(cookie.Value()))
}

func Test_CookieJarGet(t *testing.T) {
	t.Parallel()

	url := []byte("http://fasthttp.com/")
	url1 := []byte("http://fasthttp.com/make/")
	url11 := []byte("http://fasthttp.com/hola")
	url2 := []byte("http://fasthttp.com/make/fasthttp")
	url3 := []byte("http://fasthttp.com/make/fasthttp/great")
	cj := &CookieJar{}

	c1 := &fasthttp.Cookie{}
	c1.SetKey("k")
	c1.SetValue("v")
	c1.SetPath("/make/")

	c2 := &fasthttp.Cookie{}
	c2.SetKey("kk")
	c2.SetValue("vv")
	c2.SetPath("/make/fasthttp")

	c3 := &fasthttp.Cookie{}
	c3.SetKey("kkk")
	c3.SetValue("vvv")
	c3.SetPath("/make/fasthttp/great")

	uri := fasthttp.AcquireURI()
	require.NoError(t, uri.Parse(nil, url))

	uri1 := fasthttp.AcquireURI()
	require.NoError(t, uri1.Parse(nil, url1))

	uri11 := fasthttp.AcquireURI()
	require.NoError(t, uri11.Parse(nil, url11))

	uri2 := fasthttp.AcquireURI()
	require.NoError(t, uri2.Parse(nil, url2))

	uri3 := fasthttp.AcquireURI()
	require.NoError(t, uri3.Parse(nil, url3))

	cj.Set(uri1, c1, c2, c3)

	cookies := cj.Get(uri1)
	require.Len(t, cookies, 1)
	for _, cookie := range cookies {
		require.True(t, bytes.HasPrefix(uri1.Path(), cookie.Path()))
	}

	cookies = cj.Get(uri11)
	require.Empty(t, cookies)

	cookies = cj.Get(uri2)
	require.Len(t, cookies, 2)
	for _, cookie := range cookies {
		require.True(t, bytes.HasPrefix(uri2.Path(), cookie.Path()))
	}

	cookies = cj.Get(uri3)
	require.Len(t, cookies, 3)
	for _, cookie := range cookies {
		require.True(t, bytes.HasPrefix(uri3.Path(), cookie.Path()))
	}

	cookies = cj.Get(uri)
	require.Empty(t, cookies)
}

func Test_CookieJarGetExpired(t *testing.T) {
	t.Parallel()

	url1 := []byte("http://fasthttp.com/make/")
	uri1 := fasthttp.AcquireURI()
	require.NoError(t, uri1.Parse(nil, url1))

	c1 := &fasthttp.Cookie{}
	c1.SetKey("k")
	c1.SetValue("v")
	c1.SetExpire(time.Now().Add(-time.Hour))

	cj := &CookieJar{}
	cj.Set(uri1, c1)

	cookies := cj.Get(uri1)
	require.Empty(t, cookies)
}

func Test_CookieJarSet(t *testing.T) {
	t.Parallel()

	url := []byte("http://fasthttp.com/hello/world")
	cj := &CookieJar{}

	cookie := &fasthttp.Cookie{}
	cookie.SetKey("k")
	cookie.SetValue("v")

	uri := fasthttp.AcquireURI()
	require.NoError(t, uri.Parse(nil, url))

	cj.Set(uri, cookie)
	checkKeyValue(t, cj, cookie, uri, 1)
}

func Test_CookieJarSetRepeatedCookieKeys(t *testing.T) {
	t.Parallel()
	host := "fast.http"
	cj := &CookieJar{}

	uri := fasthttp.AcquireURI()
	uri.SetHost(host)

	cookie := &fasthttp.Cookie{}
	cookie.SetKey("k")
	cookie.SetValue("v")

	cookie2 := &fasthttp.Cookie{}
	cookie2.SetKey("k")
	cookie2.SetValue("v2")

	cookie3 := &fasthttp.Cookie{}
	cookie3.SetKey("key")
	cookie3.SetValue("value")

	cj.Set(uri, cookie, cookie2, cookie3)

	cookies := cj.Get(uri)
	require.Len(t, cookies, 2)
	require.Equal(t, cookies[0].String(), cookie2.String())
	require.True(t, bytes.Equal(cookies[0].Value(), cookie2.Value()))
}

func Test_CookieJarSetKeyValue(t *testing.T) {
	t.Parallel()

	host := "fast.http"
	cj := &CookieJar{}

	uri := fasthttp.AcquireURI()
	uri.SetHost(host)

	cj.SetKeyValue(host, "k", "v")
	cj.SetKeyValue(host, "key", "value")
	cj.SetKeyValue(host, "k", "vv")
	cj.SetKeyValue(host, "key", "value2")

	cookies := cj.Get(uri)
	require.Len(t, cookies, 2)
}

func Test_CookieJarGetFromResponse(t *testing.T) {
	t.Parallel()

	res := fasthttp.AcquireResponse()
	host := []byte("fast.http")
	uri := fasthttp.AcquireURI()
	uri.SetHostBytes(host)

	c := &fasthttp.Cookie{}
	c.SetKey("key")
	c.SetValue("val")

	c2 := &fasthttp.Cookie{}
	c2.SetKey("k")
	c2.SetValue("v")

	c3 := &fasthttp.Cookie{}
	c3.SetKey("kk")
	c3.SetValue("vv")

	res.Header.SetStatusCode(200)
	res.Header.SetCookie(c)
	res.Header.SetCookie(c2)
	res.Header.SetCookie(c3)

	cj := &CookieJar{}
	cj.parseCookiesFromResp(host, nil, res)

	cookies := cj.Get(uri)
	require.Len(t, cookies, 3)
	values := map[string]string{"key": "val", "k": "v", "kk": "vv"}
	for _, c := range cookies {
		k := string(c.Key())
		v, ok := values[k]
		require.True(t, ok)
		require.Equal(t, v, string(c.Value()))
		delete(values, k)
	}
	require.Empty(t, values)
}

func Test_CookieJar_HostPort(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	uriSet := fasthttp.AcquireURI()
	require.NoError(t, uriSet.Parse(nil, []byte("http://fasthttp.com:80/path")))

	c := &fasthttp.Cookie{}
	c.SetKey("k")
	c.SetValue("v")
	jar.Set(uriSet, c)

	// retrieve using a different port to ensure port is ignored
	uriGet := fasthttp.AcquireURI()
	require.NoError(t, uriGet.Parse(nil, []byte("http://fasthttp.com:8080/path")))

	cookies := jar.Get(uriGet)
	require.Len(t, cookies, 1)
	require.Equal(t, "k", string(cookies[0].Key()))
	require.Equal(t, "v", string(cookies[0].Value()))
	require.Equal(t, "fasthttp.com", string(cookies[0].Domain()))
}

func Test_CookieJar_Domain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}

	uri := fasthttp.AcquireURI()
	require.NoError(t, uri.Parse(nil, []byte("http://sub.example.com/")))

	c := &fasthttp.Cookie{}
	c.SetKey("k")
	c.SetValue("v")
	c.SetDomain("example.com")

	jar.Set(uri, c)

	uri2 := fasthttp.AcquireURI()
	require.NoError(t, uri2.Parse(nil, []byte("http://other.example.com/")))

	cookies := jar.Get(uri2)
	require.Len(t, cookies, 1)
	require.Equal(t, "k", string(cookies[0].Key()))
	require.Equal(t, "v", string(cookies[0].Value()))
}

func Test_CookieJar_Secure(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}

	uriHTTP := fasthttp.AcquireURI()
	require.NoError(t, uriHTTP.Parse(nil, []byte("http://example.com/")))

	c := &fasthttp.Cookie{}
	c.SetKey("k")
	c.SetValue("v")
	c.SetSecure(true)

	jar.Set(uriHTTP, c)

	cookies := jar.Get(uriHTTP)
	require.Empty(t, cookies)

	uriHTTPS := fasthttp.AcquireURI()
	require.NoError(t, uriHTTPS.Parse(nil, []byte("https://example.com/")))

	cookies = jar.Get(uriHTTPS)
	require.Len(t, cookies, 1)
	require.Equal(t, "k", string(cookies[0].Key()))
	require.Equal(t, "v", string(cookies[0].Value()))
}

func Test_CookieJar_PathMatch(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}

	setURI := fasthttp.AcquireURI()
	require.NoError(t, setURI.Parse(nil, []byte("http://example.com/api")))

	c := &fasthttp.Cookie{}
	c.SetKey("k")
	c.SetValue("v")
	c.SetPath("/api")

	jar.Set(setURI, c)

	uriExact := fasthttp.AcquireURI()
	require.NoError(t, uriExact.Parse(nil, []byte("http://example.com/api")))
	require.Len(t, jar.Get(uriExact), 1)

	uriChild := fasthttp.AcquireURI()
	require.NoError(t, uriChild.Parse(nil, []byte("http://example.com/api/v1")))
	require.Len(t, jar.Get(uriChild), 1)

	uriNoMatch := fasthttp.AcquireURI()
	require.NoError(t, uriNoMatch.Parse(nil, []byte("http://example.com/apiv1")))
	require.Empty(t, jar.Get(uriNoMatch))
}

func Test_CookieJar_ReleaseClearsHosts(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{
		hostCookies: make(map[string][]*fasthttp.Cookie, 2),
	}

	for i := 0; i < 2; i++ {
		cookie := fasthttp.AcquireCookie()
		cookie.SetKey("k")
		cookie.SetValue("v")
		host := fmt.Sprintf("host-%d", i)
		jar.hostCookies[host] = append(jar.hostCookies[host], cookie)
	}

	jar.Release()

	require.NotNil(t, jar.hostCookies)
	require.Empty(t, jar.hostCookies)
}

func Test_CookieJar_ReleaseDropsOversizedMaps(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{
		hostCookies: make(map[string][]*fasthttp.Cookie, cookieJarHostMaxEntries+1),
	}

	for i := 0; i < cookieJarHostMaxEntries+1; i++ {
		cookie := fasthttp.AcquireCookie()
		cookie.SetKey("k")
		cookie.SetValue("v")
		host := fmt.Sprintf("oversize-%d", i)
		jar.hostCookies[host] = append(jar.hostCookies[host], cookie)
	}

	jar.Release()

	require.Nil(t, jar.hostCookies)
}

func Test_releaseCookieMatchesShrinksOversizedSlices(t *testing.T) {
	t.Parallel()

	matchesPtr := acquireCookieMatches()
	require.NotNil(t, matchesPtr)

	// Expand the slice beyond the max capacity and populate it with placeholders.
	oversized := make([]*fasthttp.Cookie, cookieJarMatchMaxCap+8)
	copy(oversized, []*fasthttp.Cookie{{}, {}})
	*matchesPtr = oversized

	releaseCookieMatches(matchesPtr)

	pooledPtr := acquireCookieMatches()
	require.NotNil(t, pooledPtr)
	require.Empty(t, *pooledPtr)
	require.LessOrEqual(t, cap(*pooledPtr), cookieJarMatchMaxCap)

	releaseCookieMatches(pooledPtr)
}

func Test_CookieJar_BorrowCookiesUsesPool(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	uri := fasthttp.AcquireURI()
	require.NoError(t, uri.Parse(nil, []byte("http://example.com/path")))

	cookie := &fasthttp.Cookie{}
	cookie.SetKey("k")
	cookie.SetValue("v")
	jar.Set(uri, cookie)

	matches, matchesPtr := jar.borrowCookiesByHostAndPath(uri.Host(), uri.Path(), false)
	require.NotNil(t, matchesPtr)
	require.Len(t, matches, 1)

	for _, c := range matches {
		fasthttp.ReleaseCookie(c)
	}

	releaseCookieMatches(matchesPtr)

	pooledPtr := acquireCookieMatches()
	require.Empty(t, *pooledPtr)
	releaseCookieMatches(pooledPtr)

	fasthttp.ReleaseURI(uri)
}
