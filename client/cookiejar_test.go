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

func cookieKeys(cookies []*fasthttp.Cookie) []string {
	keys := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		keys = append(keys, string(cookie.Key()))
	}

	return keys
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

func Test_CookieJar_HostOnlyCookieNotSentToSubdomain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	origin := fasthttp.AcquireURI()
	require.NoError(t, origin.Parse(nil, []byte("http://example.com/")))

	c := &fasthttp.Cookie{}
	c.SetKey("sid")
	c.SetValue("123")
	jar.Set(origin, c)

	subdomain := fasthttp.AcquireURI()
	require.NoError(t, subdomain.Parse(nil, []byte("http://attacker.example.com/")))
	require.Empty(t, jar.Get(subdomain))
}

func Test_CookieJar_ResponseHostOnlyCookieNotSentToSubdomain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sid")
	c.SetValue("123")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("example.com"), nil, resp)

	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://example.com/")))
	require.Equal(t, []string{"sid"}, cookieKeys(jar.Get(origin)))

	subdomain := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(subdomain)
	require.NoError(t, subdomain.Parse(nil, []byte("http://attacker.example.com/")))
	require.Empty(t, jar.Get(subdomain))
}

func Test_CookieJar_HostOnlyCookieMatchesMixedCaseHost(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}

	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://example.com/")))

	c := &fasthttp.Cookie{}
	c.SetKey("sid")
	c.SetValue("123")
	jar.Set(origin, c)

	mixedCaseHost := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(mixedCaseHost)
	require.NoError(t, mixedCaseHost.Parse(nil, []byte("http://Example.com/")))

	require.Equal(t, []string{"sid"}, cookieKeys(jar.Get(mixedCaseHost)))
}

func Test_CookieJar_RejectUnrelatedResponseDomain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	host := []byte("attacker.invalid")

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("evil")
	c.SetDomain("victim.example")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp(host, nil, resp)

	uri := fasthttp.AcquireURI()
	require.NoError(t, uri.Parse(nil, []byte("http://victim.example/")))
	require.Empty(t, jar.Get(uri))
}

func Test_CookieJar_SetRejectUnrelatedDomain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://attacker.example/")))

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("evil")
	c.SetDomain("victim.example")

	jar.Set(origin, c)

	target := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(target)
	require.NoError(t, target.Parse(nil, []byte("http://victim.example/")))
	require.Empty(t, jar.Get(target))
}

func Test_CookieJar_RejectPublicSuffixResponseDomain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("evil")
	c.SetDomain("com")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("attacker.com"), nil, resp)

	require.Empty(t, jar.hostCookies)
}

func Test_CookieJar_RejectIPAddressSuffixResponseDomain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("evil")
	c.SetDomain("2.3.4")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("1.2.3.4"), nil, resp)

	require.Empty(t, jar.hostCookies)
}

func Test_CookieJar_MixedHostOnlyAndDomainCookies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		order []string
	}{
		{
			name:  "host-only first",
			order: []string{"host-only", "domain"},
		},
		{
			name:  "domain first",
			order: []string{"domain", "host-only"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			jar := &CookieJar{}

			hostOnlyOrigin := fasthttp.AcquireURI()
			defer fasthttp.ReleaseURI(hostOnlyOrigin)
			require.NoError(t, hostOnlyOrigin.Parse(nil, []byte("http://example.com/")))

			domainOrigin := fasthttp.AcquireURI()
			defer fasthttp.ReleaseURI(domainOrigin)
			require.NoError(t, domainOrigin.Parse(nil, []byte("http://sub.example.com/")))

			hostOnlyCookie := &fasthttp.Cookie{}
			hostOnlyCookie.SetKey("host-only")
			hostOnlyCookie.SetValue("123")

			domainCookie := &fasthttp.Cookie{}
			domainCookie.SetKey("domain")
			domainCookie.SetValue("456")
			domainCookie.SetDomain("example.com")

			for _, cookieType := range testCase.order {
				switch cookieType {
				case "host-only":
					jar.Set(hostOnlyOrigin, hostOnlyCookie)
				case "domain":
					jar.Set(domainOrigin, domainCookie)
				default:
					t.Fatalf("unexpected cookie type %q", cookieType)
				}
			}

			anotherSubdomain := fasthttp.AcquireURI()
			defer fasthttp.ReleaseURI(anotherSubdomain)
			require.NoError(t, anotherSubdomain.Parse(nil, []byte("http://child.example.com/")))
			require.Equal(t, []string{"domain"}, cookieKeys(jar.Get(anotherSubdomain)))

			require.ElementsMatch(t, []string{"domain", "host-only"}, cookieKeys(jar.Get(hostOnlyOrigin)))
		})
	}
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
