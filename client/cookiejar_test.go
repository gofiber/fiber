package client

import (
	"bytes"
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
	url1 := []byte("http://fasthttp.com/make")
	url11 := []byte("http://fasthttp.com/hola")
	url2 := []byte("http://fasthttp.com/make/fasthttp")
	url3 := []byte("http://fasthttp.com/make/fasthttp/great")
	prefix := []byte("/")
	prefix1 := []byte("/make")
	prefix2 := []byte("/make/fasthttp")
	prefix3 := []byte("/make/fasthttp/great")
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
	require.Len(t, cookies, 3)
	for _, cookie := range cookies {
		require.True(t, bytes.HasPrefix(cookie.Path(), prefix1))
	}

	cookies = cj.Get(uri11)
	require.Empty(t, cookies)

	cookies = cj.Get(uri2)
	require.Len(t, cookies, 2)
	for _, cookie := range cookies {
		require.True(t, bytes.HasPrefix(cookie.Path(), prefix2))
	}

	cookies = cj.Get(uri3)
	require.Len(t, cookies, 1)

	for _, cookie := range cookies {
		require.True(t, bytes.HasPrefix(cookie.Path(), prefix3))
	}

	cookies = cj.Get(uri)
	require.Len(t, cookies, 3)
	for _, cookie := range cookies {
		require.True(t, bytes.HasPrefix(cookie.Path(), prefix))
	}
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
