package client

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	neturl "net/url"
	"sort"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/publicsuffix"
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
	require.Equal(t, "k", string(cookies[0].Key()))
	require.Equal(t, "v2", string(cookies[0].Value()))
	require.Equal(t, host, string(cookies[0].Domain()))
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
	cj.SetKeyValueBytes(host, []byte("kb"), []byte("vb"))

	cookies := cj.Get(uri)
	require.Len(t, cookies, 3)

	// Verify the entry written via SetKeyValueBytes has the exact key and value.
	var foundBytes bool
	for _, c := range cookies {
		if string(c.Key()) == "kb" {
			foundBytes = true
			require.Equal(t, "vb", string(c.Value()))
		}
	}
	require.True(t, foundBytes, "expected cookie kb=vb written by SetKeyValueBytes")
}

func Test_CookieJarHostStorageIsBounded(t *testing.T) {
	t.Parallel()

	cj := &CookieJar{}

	for i := range maxCookieJarHosts + 32 {
		host := fmt.Sprintf("host-%d.example.com", i)
		cookie := &fasthttp.Cookie{}
		cookie.SetKey("k")
		cookie.SetValue("v")
		cj.SetByHost([]byte(host), cookie)
	}

	require.LessOrEqual(t, len(cj.hostCookies), maxCookieJarHosts)
}

func Test_CookieJarHostEvictionIsDeterministic(t *testing.T) {
	t.Parallel()

	cj := &CookieJar{hostCookies: make(map[string][]storedCookie, maxCookieJarHosts)}
	for i := range maxCookieJarHosts {
		host := fmt.Sprintf("host-%04d.example.com", i+1)
		cookie := fasthttp.AcquireCookie()
		cookie.SetKey("k")
		cookie.SetValue("v")
		cj.hostCookies[host] = []storedCookie{{cookie: cookie, isHostOnly: true}}
	}

	cj.ensureHostCapacityLocked("zzz.example.com", time.Now())

	_, ok := cj.hostCookies["host-0001.example.com"]
	require.False(t, ok)
	require.Len(t, cj.hostCookies, maxCookieJarHosts-1)
}

func Test_CookieJarHostCapacityPrefersExpiredEntries(t *testing.T) {
	t.Parallel()

	cj := &CookieJar{hostCookies: make(map[string][]storedCookie, maxCookieJarHosts)}
	now := time.Now()

	expired := fasthttp.AcquireCookie()
	expired.SetKey("expired")
	expired.SetValue("v")
	expired.SetExpire(now.Add(-time.Minute))
	cj.hostCookies["expired.example.com"] = []storedCookie{{cookie: expired, isHostOnly: true}}

	for i := 1; i < maxCookieJarHosts; i++ {
		host := fmt.Sprintf("host-%04d.example.com", i)
		cookie := fasthttp.AcquireCookie()
		cookie.SetKey("k")
		cookie.SetValue("v")
		cj.hostCookies[host] = []storedCookie{{cookie: cookie, isHostOnly: true}}
	}

	cj.ensureHostCapacityLocked("new.example.com", now)

	_, ok := cj.hostCookies["expired.example.com"]
	require.False(t, ok)
	require.Len(t, cj.hostCookies, maxCookieJarHosts-1)
	_, ok = cj.hostCookies["host-0001.example.com"]
	require.True(t, ok)
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

func Test_CookieJar_isIPLiteral(t *testing.T) {
	t.Parallel()

	// Pinned to net.ParseIP semantics: zoned addresses are rejected.
	tests := []struct {
		host string
		want bool
	}{
		{"192.0.2.1", true},
		{"::1", true},
		{"[::1]", true},
		{"::ffff:192.0.2.1", true},
		{"fe80::1%eth0", false},
		{"[fe80::1%eth0]", false},
		{"example.com", false},
		{"192.0.2.256", false},
		{"", false},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, isIPLiteral(tt.host), "isIPLiteral(%q)", tt.host)
	}
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

func Test_CookieJar_SetByHostDoesNotMutateHostOnlyCookieToDomainCookie(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	c := &fasthttp.Cookie{}
	c.SetKey("sid")
	c.SetValue("123")

	jar.SetByHost([]byte("example.com"), c)
	require.Empty(t, c.Domain())

	subOrigin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(subOrigin)
	require.NoError(t, subOrigin.Parse(nil, []byte("http://sub.example.com/")))
	jar.Set(subOrigin, c)
	require.Empty(t, c.Domain())

	sibling := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(sibling)
	require.NoError(t, sibling.Parse(nil, []byte("http://other.example.com/")))
	require.Empty(t, jar.Get(sibling))
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

func Test_CookieJar_ExactPublicSuffixDomainDowngradedToHostOnly(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("ok")
	c.SetDomain("com")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("com"), nil, resp)
	require.Len(t, jar.hostCookies["com"], 1)
	require.True(t, jar.hostCookies["com"][0].isHostOnly)

	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://com/")))
	require.Equal(t, []string{"sess"}, cookieKeys(jar.Get(origin)))

	other := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(other)
	require.NoError(t, other.Parse(nil, []byte("http://example.com/")))
	require.Empty(t, jar.Get(other))
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

func Test_CookieJar_ExactIPAddressDomainDowngradedToHostOnly(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("ok")
	c.SetDomain("127.0.0.1")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("127.0.0.1"), nil, resp)
	require.Len(t, jar.hostCookies["127.0.0.1"], 1)
	require.True(t, jar.hostCookies["127.0.0.1"][0].isHostOnly)

	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://127.0.0.1/")))
	require.Equal(t, []string{"sess"}, cookieKeys(jar.Get(origin)))

	other := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(other)
	require.NoError(t, other.Parse(nil, []byte("http://evil.127.0.0.1/")))
	require.Empty(t, jar.Get(other))
}

func Test_CookieJar_RejectIPAddressResponseDomainFromHostname(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("evil")
	c.SetDomain("127.0.0.1")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("evil.127.0.0.1"), nil, resp)

	require.Empty(t, jar.hostCookies)

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	require.NoError(t, uri.Parse(nil, []byte("http://127.0.0.1/")))
	require.Empty(t, jar.Get(uri))
}

func Test_CookieJar_SetRejectIPAddressDomainFromHostname(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://evil.127.0.0.1/")))

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("evil")
	c.SetDomain("127.0.0.1")

	jar.Set(origin, c)

	require.Empty(t, jar.hostCookies)

	target := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(target)
	require.NoError(t, target.Parse(nil, []byte("http://127.0.0.1/")))
	require.Empty(t, jar.Get(target))
}

func Test_CookieJar_ResponseDomainCookieSentToMatchingSiblingSubdomain(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("shared")
	c.SetDomain("example.com")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("sub.example.com"), nil, resp)

	other := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(other)
	require.NoError(t, other.Parse(nil, []byte("http://other.example.com/")))
	require.Equal(t, []string{"sess"}, cookieKeys(jar.Get(other)))
}

func Test_CookieJar_TrailingDotDomainDowngradedToHostOnly(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("123")
	c.SetDomain("example.com.")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("sub.example.com."), nil, resp)
	require.Len(t, jar.hostCookies["sub.example.com."], 1)
	require.True(t, jar.hostCookies["sub.example.com."][0].isHostOnly)

	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://sub.example.com./")))
	require.Equal(t, []string{"sess"}, cookieKeys(jar.Get(origin)))

	other := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(other)
	require.NoError(t, other.Parse(nil, []byte("http://other.example.com./")))
	require.Empty(t, jar.Get(other))
}

func Test_CookieJar_TrailingDotDomainDowngradedToHostOnlyOnPlainHost(t *testing.T) {
	t.Parallel()

	jar := &CookieJar{}
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c := &fasthttp.Cookie{}
	c.SetKey("sess")
	c.SetValue("123")
	c.SetDomain("example.com.")
	resp.Header.SetCookie(c)

	jar.parseCookiesFromResp([]byte("sub.example.com"), nil, resp)
	require.Len(t, jar.hostCookies["sub.example.com"], 1)
	require.True(t, jar.hostCookies["sub.example.com"][0].isHostOnly)

	origin := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(origin)
	require.NoError(t, origin.Parse(nil, []byte("http://sub.example.com/")))
	require.Equal(t, []string{"sess"}, cookieKeys(jar.Get(origin)))

	other := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(other)
	require.NoError(t, other.Parse(nil, []byte("http://other.example.com/")))
	require.Empty(t, jar.Get(other))
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

// Test_CookieJar_SamePathAndPathMatchEdges exercises samePath and pathMatch
// directly. Both treat an empty path as the default "/" (RFC 6265 §5.1.4), and
// pathMatch's prefix rule only extends across a '/' boundary — reachable only
// with inputs the higher-level tests do not produce.
func Test_CookieJar_SamePathAndPathMatchEdges(t *testing.T) {
	t.Parallel()

	t.Run("samePath", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			a, b string
			want bool
		}{
			{"/a", "/a", true},
			{"", "/", true}, // empty defaults to "/"
			{"/", "", true}, // and in the other direction
			{"", "", true},  // both empty
			{"/a", "/b", false},
			{"/a", "/a/b", false}, // exact, not prefix
			{"", "/a", false},
		}
		for _, tc := range testCases {
			require.Equal(t, tc.want, samePath([]byte(tc.a), []byte(tc.b)),
				"samePath(%q, %q)", tc.a, tc.b)
		}
	})

	t.Run("pathMatch", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			reqPath, cookiePath string
			want                bool
		}{
			{"/a", "/a", true},
			{"", "/", true},       // empty request path defaults to "/"
			{"/a", "", true},      // empty cookie path defaults to "/"
			{"/a/b", "/a", true},  // prefix across a '/' boundary
			{"/a/b", "/a/", true}, // cookie path already ends in '/'
			{"/ab", "/a", false},  // prefix but not on a boundary
			{"/b", "/a", false},   // no prefix at all
			{"/a", "/a/b", false}, // request shorter than the cookie path
		}
		for _, tc := range testCases {
			require.Equal(t, tc.want, pathMatch([]byte(tc.reqPath), []byte(tc.cookiePath)),
				"pathMatch(%q, %q)", tc.reqPath, tc.cookiePath)
		}
	})
}

// Test_CookieJar_NilURI pins the nil guards on the exported URI-taking methods:
// they must be no-ops rather than panicking.
func Test_CookieJar_NilURI(t *testing.T) {
	t.Parallel()

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	require.Nil(t, jar.Get(nil))

	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	c.SetKey("k")
	c.SetValue("v")
	require.NotPanics(t, func() { jar.Set(nil, c) })
	require.Empty(t, jar.hostCookies, "a nil URI must store nothing")
}

// Test_CookieJar_DomainMatchBoundary pins the RFC 6265 §5.1.3 label-boundary
// semantics of domainMatch: a bare string suffix without a '.' separator must
// never match, and the comparison is ASCII case-insensitive.
func Test_CookieJar_DomainMatchBoundary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		host, domain string
		want         bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", "example.com", true},
		{"deep.sub.example.com", "example.com", true},
		// Suffix overlap without a label boundary must not match.
		{"evilexample.com", "example.com", false},
		{"xample.com", "example.com", false},
		// Domain longer than host never matches.
		{"example.com", "sub.example.com", false},
		{"com", "example.com", false},
		// ASCII case-insensitive on both sides.
		{"EXAMPLE.com", "example.com", true},
		{"sub.EXAMPLE.com", "example.COM", true},
		{"evilEXAMPLE.com", "example.com", false},
	}
	for _, tc := range testCases {
		require.Equal(t, tc.want, domainMatch(tc.host, tc.domain), "domainMatch(%q, %q)", tc.host, tc.domain)
	}
}

// Test_CookieJar_DistinctPathsCoexist covers RFC 6265 Section 5.3 step 11:
// a stored cookie is only replaced when name, domain and path all match. A
// cookie set for a deeper path must not evict the same-named cookie stored
// for a shallower one.
func Test_CookieJar_DistinctPathsCoexist(t *testing.T) {
	t.Parallel()

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	root := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(root)
	root.SetKey("a")
	root.SetValue("root")
	root.SetPath("/")

	admin := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(admin)
	admin.SetKey("a")
	admin.SetValue("admin")
	admin.SetPath("/admin")

	jar.SetByHost([]byte("example.com"), root)
	jar.SetByHost([]byte("example.com"), admin)

	collect := func(path string) map[string]string {
		got := jar.getByHostAndPath([]byte("example.com"), []byte(path), false)
		out := make(map[string]string, len(got))
		for _, c := range got {
			out[string(c.Path())] = string(c.Value())
			fasthttp.ReleaseCookie(c)
		}
		return out
	}

	require.Equal(t, map[string]string{"/": "root"}, collect("/"))
	require.Equal(t, map[string]string{"/": "root", "/admin": "admin"}, collect("/admin"))
	require.Equal(t, map[string]string{"/": "root"}, collect("/other"))

	// Re-setting the same (name, path) replaces in place rather than appending.
	updated := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(updated)
	updated.SetKey("a")
	updated.SetValue("root2")
	updated.SetPath("/")
	jar.SetByHost([]byte("example.com"), updated)

	require.Equal(t, map[string]string{"/": "root2"}, collect("/"))
	require.Equal(t, map[string]string{"/": "root2", "/admin": "admin"}, collect("/admin"))
}

// Test_CookieJar_SetByHost_DoesNotMutateArgument checks the jar's documented
// contract that it only stores copies: normalizing the Domain attribute used
// to case-fold the caller's cookie in place.
func Test_CookieJar_SetByHost_DoesNotMutateArgument(t *testing.T) {
	t.Parallel()

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	c.SetKey("a")
	c.SetValue("1")
	c.SetDomain("Example.COM")

	jar.SetByHost([]byte("sub.example.com"), c)

	require.Equal(t, "Example.COM", string(c.Domain()))

	// The stored copy is still normalized, so lookups keep working.
	got := jar.getByHostAndPath([]byte("sub.example.com"), []byte("/"), false)
	require.Len(t, got, 1)
	require.Equal(t, "1", string(got[0].Value()))
	fasthttp.ReleaseCookie(got[0])
}

// Test_CookieJar_SetByHost_ReplacesOnNormalizedPath pins SetByHost's documented
// replace semantics against fasthttp's path normalization. SetPathBytes runs the
// value through normalizePath, so a cookie built by ParseBytes can carry a path
// that differs from the one the entry ends up storing. Searching on the raw form
// missed the entry a previous identical call had created and appended a
// duplicate, quietly consuming per-host slots until eviction dropped a
// legitimate cookie.
func Test_CookieJar_SetByHost_ReplacesOnNormalizedPath(t *testing.T) {
	t.Parallel()

	// Each raw path is one fasthttp's normalizePath rewrites: collapsed slashes,
	// a percent escape, a dot segment, and a ';' that gets stripped.
	for _, raw := range []string{"/x//y", "/x/%41", "/x/./y", "/x;y"} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			jar := AcquireCookieJar()
			defer ReleaseCookieJar(jar)

			const host = "example.com"
			for i := range 3 {
				c := fasthttp.AcquireCookie()
				require.NoError(t, c.ParseBytes(
					fmt.Appendf(nil, "a=v%d; path=%s", i, raw)))
				jar.SetByHost([]byte(host), c)
				fasthttp.ReleaseCookie(c)
			}

			// Three writes of the same name and path must leave one entry.
			require.Len(t, jar.hostCookies[host], 1,
				"repeated SetByHost calls must replace, not accumulate")

			// And it must be the last value written.
			require.Equal(t, "v2", string(jar.hostCookies[host][0].cookie.Value()))
		})
	}
}

// Test_CookieJar_SetUsesURIDefaultPath pins that Set derives the RFC 6265
// Section 5.1.4 default-path from the URI it is given, the same rule
// parseCookiesFromResp applies to a Set-Cookie received for that URI. A cookie
// with no Path set against "/a/b" is scoped to "/a", not host-wide.
//
// SetByHost has no URI to derive a path from and keeps scoping such cookies to
// "/", so the two entry points stay distinguishable.
func Test_CookieJar_SetUsesURIDefaultPath(t *testing.T) {
	t.Parallel()

	t.Run("Set scopes to the URI default-path", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		setURI := fasthttp.AcquireURI()
		defer fasthttp.ReleaseURI(setURI)
		require.NoError(t, setURI.Parse(nil, []byte("http://example.com/a/b")))

		c := fasthttp.AcquireCookie()
		defer fasthttp.ReleaseCookie(c)
		c.SetKey("k")
		c.SetValue("v")

		jar.Set(setURI, c)

		require.Len(t, jar.hostCookies["example.com"], 1)
		require.Equal(t, "/a", string(jar.hostCookies["example.com"][0].cookie.Path()),
			"a path-less cookie must take the URI's default-path")

		// Sent to a sibling under the same directory...
		sibling := fasthttp.AcquireURI()
		defer fasthttp.ReleaseURI(sibling)
		require.NoError(t, sibling.Parse(nil, []byte("http://example.com/a/other")))
		got := jar.Get(sibling)
		require.Len(t, got, 1)
		for _, gc := range got {
			fasthttp.ReleaseCookie(gc)
		}

		// ...but not host-wide.
		root := fasthttp.AcquireURI()
		defer fasthttp.ReleaseURI(root)
		require.NoError(t, root.Parse(nil, []byte("http://example.com/")))
		require.Empty(t, jar.Get(root), "the cookie must not escape its default-path")
	})

	t.Run("an explicit Path still wins", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		setURI := fasthttp.AcquireURI()
		defer fasthttp.ReleaseURI(setURI)
		require.NoError(t, setURI.Parse(nil, []byte("http://example.com/a/b")))

		c := fasthttp.AcquireCookie()
		defer fasthttp.ReleaseCookie(c)
		c.SetKey("k")
		c.SetValue("v")
		c.SetPath("/")

		jar.Set(setURI, c)

		require.Equal(t, "/", string(jar.hostCookies["example.com"][0].cookie.Path()))
	})

	t.Run("SetByHost still scopes to root", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		c := fasthttp.AcquireCookie()
		defer fasthttp.ReleaseCookie(c)
		c.SetKey("k")
		c.SetValue("v")

		jar.SetByHost([]byte("example.com"), c)

		require.Equal(t, "/", string(jar.hostCookies["example.com"][0].cookie.Path()),
			"SetByHost has no request path, so the scope stays at the root")
	})
}

// Test_CookieJar_MatchesStdlibJar cross-checks storage and retrieval against
// net/http/cookiejar, which implements RFC 6265 with the same public-suffix
// list. It caught path-less cookies being stored at "/" instead of the
// request's default-path.
func Test_CookieJar_MatchesStdlibJar(t *testing.T) {
	t.Parallel()

	type setStep struct {
		url    string
		cookie string
	}
	tests := []struct {
		name string
		sets []setStep
		gets []string
	}{
		{
			name: "host only",
			sets: []setStep{{"http://example.com/", "a=1"}},
			gets: []string{"http://example.com/", "http://sub.example.com/", "http://other.com/"},
		},
		{
			name: "domain attribute",
			sets: []setStep{{"http://example.com/", "a=1; Domain=example.com"}},
			gets: []string{"http://example.com/", "http://sub.example.com/"},
		},
		{
			name: "leading dot domain",
			sets: []setStep{{"http://example.com/", "a=1; Domain=.example.com"}},
			gets: []string{"http://example.com/", "http://sub.example.com/"},
		},
		{
			name: "subdomain sets parent",
			sets: []setStep{{"http://sub.example.com/", "a=1; Domain=example.com"}},
			gets: []string{"http://example.com/", "http://sub.example.com/", "http://x.example.com/"},
		},
		{
			name: "public suffix rejected",
			sets: []setStep{{"http://example.com/", "a=1; Domain=com"}},
			gets: []string{"http://example.com/", "http://other.com/"},
		},
		{
			name: "unrelated domain rejected",
			sets: []setStep{{"http://example.com/", "a=1; Domain=evil.com"}},
			gets: []string{"http://example.com/", "http://evil.com/"},
		},
		{
			name: "explicit paths",
			sets: []setStep{{"http://example.com/", "a=1; Path=/"}, {"http://example.com/admin", "b=2; Path=/admin"}},
			gets: []string{"http://example.com/", "http://example.com/admin", "http://example.com/admin/x", "http://example.com/adminx"},
		},
		{
			name: "secure",
			sets: []setStep{{"https://example.com/", "a=1; Secure"}},
			gets: []string{"https://example.com/", "http://example.com/"},
		},
		{
			name: "overwrite",
			sets: []setStep{{"http://example.com/", "a=1"}, {"http://example.com/", "a=2"}},
			gets: []string{"http://example.com/"},
		},
		{
			name: "ip host",
			sets: []setStep{{"http://127.0.0.1/", "a=1"}},
			gets: []string{"http://127.0.0.1/"},
		},
		{
			name: "ip domain",
			sets: []setStep{{"http://127.0.0.1/", "a=1; Domain=127.0.0.1"}},
			gets: []string{"http://127.0.0.1/"},
		},
		{
			name: "default path",
			sets: []setStep{{"http://example.com/a/b", "a=1"}},
			gets: []string{"http://example.com/a/b", "http://example.com/a/", "http://example.com/a", "http://example.com/"},
		},
		{
			name: "default path at root",
			sets: []setStep{{"http://example.com/b", "a=1"}},
			gets: []string{"http://example.com/b", "http://example.com/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			std, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
			require.NoError(t, err)
			jar := AcquireCookieJar()
			defer ReleaseCookieJar(jar)

			for _, s := range tt.sets {
				u, err := neturl.Parse(s.url)
				require.NoError(t, err)

				header := http.Header{}
				header.Add("Set-Cookie", s.cookie)
				std.SetCookies(u, (&http.Response{Header: header}).Cookies())

				resp := fasthttp.AcquireResponse()
				resp.Header.Add("Set-Cookie", s.cookie)
				jar.parseCookiesFromResp([]byte(u.Host), []byte(u.Path), resp)
				fasthttp.ReleaseResponse(resp)
			}

			for _, g := range tt.gets {
				u, err := neturl.Parse(g)
				require.NoError(t, err)

				want := make([]string, 0, 2)
				for _, c := range std.Cookies(u) {
					want = append(want, c.Name+"="+c.Value)
				}
				sort.Strings(want)

				fURI := fasthttp.AcquireURI()
				require.NoError(t, fURI.Parse(nil, []byte(g)))
				got := make([]string, 0, 2)
				for _, c := range jar.Get(fURI) {
					got = append(got, string(c.Key())+"="+string(c.Value()))
					fasthttp.ReleaseCookie(c)
				}
				fasthttp.ReleaseURI(fURI)
				sort.Strings(got)

				require.Equal(t, want, got, "request %s", g)
			}
		})
	}
}

// Test_CookieJar_SendsMostSpecificCookie pins RFC 6265 Section 5.4 ordering:
// longer path first, ties broken by write order.
func Test_CookieJar_SendsMostSpecificCookie(t *testing.T) {
	t.Parallel()

	t.Run("longer path wins", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		adminResp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(adminResp)
		adminResp.Header.Add("Set-Cookie", "sess=ADMINVAL")
		jar.parseCookiesFromResp([]byte("example.com"), []byte("/admin/login"), adminResp)

		rootResp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(rootResp)
		rootResp.Header.Add("Set-Cookie", "sess=ROOTVAL; Path=/")
		jar.parseCookiesFromResp([]byte("example.com"), []byte("/"), rootResp)

		require.Equal(t, "sess=ADMINVAL", cookieHeaderFor(jar, "http://example.com/admin/dashboard"))
		require.Equal(t, "sess=ROOTVAL", cookieHeaderFor(jar, "http://example.com/other"))
	})

	// Equal-length paths stored under *different* keys is the shape where map
	// iteration order decides the winner, so it is the one that needs the
	// explicit tiebreak. Repeat it: Go randomizes map order per run.
	t.Run("equal paths across storage keys are deterministic", func(t *testing.T) {
		t.Parallel()

		for range 50 {
			jar := AcquireCookieJar()

			hostOnly := fasthttp.AcquireResponse()
			hostOnly.Header.Add("Set-Cookie", "sess=HOSTONLY; Path=/ab")
			jar.parseCookiesFromResp([]byte("sub.example.com"), []byte("/ab"), hostOnly)
			fasthttp.ReleaseResponse(hostOnly)

			domainWide := fasthttp.AcquireResponse()
			domainWide.Header.Add("Set-Cookie", "sess=DOMAINWIDE; Path=/ab; Domain=example.com")
			jar.parseCookiesFromResp([]byte("sub.example.com"), []byte("/ab"), domainWide)
			fasthttp.ReleaseResponse(domainWide)

			// Same path length, so the write-order tiebreak decides: the
			// host-only cookie was stored first and wins every run.
			require.Equal(t, "sess=HOSTONLY", cookieHeaderFor(jar, "http://sub.example.com/ab/c"))

			ReleaseCookieJar(jar)
		}
	})
}

// cookieHeaderFor renders the Cookie header the jar would put on a request.
func cookieHeaderFor(jar *CookieJar, rawURL string) string {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(rawURL)
	jar.dumpCookiesToReq(req)
	return string(req.Header.Peek("Cookie"))
}

// Test_CookieJar_DefaultPathStaysReachable guards the default-path against
// fasthttp's Cookie.SetPathBytes, which percent-decodes the value it is given.
// The path already came out of URI.Path() decoded once, so a naive set decoded
// it twice and stored a scope the setting URL itself could never match.
//
// The invariant is reachability, not exact scope: a path containing ';' cannot
// round-trip at all (SetPathBytes rewrites it unconditionally), so those fall
// back to "/" — the same scope every path-less cookie had before default-path
// scoping existed. Broader than the RFC prescribes, but never silently lost.
func Test_CookieJar_DefaultPathStaysReachable(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"http://example.com/a%2541b/c",
		"http://example.com/plain/c",
		"http://example.com/matrix;v=1/c",
	} {
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()

			jar := AcquireCookieJar()
			defer ReleaseCookieJar(jar)

			uri := fasthttp.AcquireURI()
			defer fasthttp.ReleaseURI(uri)
			require.NoError(t, uri.Parse(nil, []byte(rawURL)))

			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)
			resp.Header.Add("Set-Cookie", "k=v")
			jar.parseCookiesFromResp(uri.Host(), uri.Path(), resp)

			got := jar.Get(uri)
			require.Len(t, got, 1, "cookie must still be sent to the URL that set it")
			require.Equal(t, "v", string(got[0].Value()))
			fasthttp.ReleaseCookie(got[0])
		})
	}
}

// Test_CookieJar_BoundsCookiesPerHost checks the per-host ceiling. Scoping
// path-less cookies to their default-path lets one origin mint an entry per
// directory it serves, so the host cap alone no longer bounds the jar.
func Test_CookieJar_BoundsCookiesPerHost(t *testing.T) {
	t.Parallel()

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	for i := range 500 {
		resp := fasthttp.AcquireResponse()
		resp.Header.Add("Set-Cookie", "a=1")
		jar.parseCookiesFromResp([]byte("example.com"), fmt.Appendf(nil, "/u/%d/x", i), resp)
		fasthttp.ReleaseResponse(resp)
	}

	require.LessOrEqual(t, len(jar.hostCookies["example.com"]), maxCookiesPerHost)
}

// Test_CookieJar_PerHostEvictionPrefersExpired pins the first half of
// enforceHostCookieLimitLocked: when a host is over its ceiling, expired entries
// are dropped before any live one is considered, and the remaining eviction is
// least-recently-written rather than oldest-created. A cookie the server keeps
// re-sending must therefore survive a flood of one-off cookies, which is exactly
// the pressure default-path scoping creates.
func Test_CookieJar_PerHostEvictionPrefersExpired(t *testing.T) {
	t.Parallel()

	const host = "example.com"
	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)
	jar.hostCookies = make(map[string][]storedCookie)

	now := time.Now()

	// One live cookie written first, so it is the least-recently-written of the
	// live entries and would be the eviction victim if expiry were not checked.
	oldest := fasthttp.AcquireCookie()
	oldest.SetKey("oldest")
	oldest.SetValue("v")
	oldest.SetExpire(fasthttp.CookieExpireUnlimited)
	jar.hostCookies[host] = append(jar.hostCookies[host],
		storedCookie{cookie: oldest, seq: jar.nextSeqLocked(), isHostOnly: true})

	// Fill past the ceiling with already-expired cookies.
	for i := range maxCookiesPerHost {
		c := fasthttp.AcquireCookie()
		c.SetKey(fmt.Sprintf("dead%d", i))
		c.SetValue("v")
		c.SetExpire(now.Add(-time.Hour))
		jar.hostCookies[host] = append(jar.hostCookies[host],
			storedCookie{cookie: c, seq: jar.nextSeqLocked(), isHostOnly: true})
	}
	require.Greater(t, len(jar.hostCookies[host]), maxCookiesPerHost)

	jar.enforceHostCookieLimitLocked(host)

	// Every expired entry is gone and the single live one survived, even though
	// it was written first.
	require.Len(t, jar.hostCookies[host], 1)
	require.Equal(t, "oldest", string(jar.hostCookies[host][0].cookie.Key()))
}

// Test_CookieJar_PerHostEvictionDropsEmptyKey covers the case where dropping
// expired entries leaves nothing behind: the storage key must be deleted rather
// than left mapped to an empty slice, which would keep an entry in hostCookies
// forever and count against the jar's host ceiling.
func Test_CookieJar_PerHostEvictionDropsEmptyKey(t *testing.T) {
	t.Parallel()

	const host = "example.com"
	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)
	jar.hostCookies = make(map[string][]storedCookie)

	past := time.Now().Add(-time.Hour)
	for i := range maxCookiesPerHost + 1 {
		c := fasthttp.AcquireCookie()
		c.SetKey(fmt.Sprintf("dead%d", i))
		c.SetValue("v")
		c.SetExpire(past)
		jar.hostCookies[host] = append(jar.hostCookies[host],
			storedCookie{cookie: c, seq: jar.nextSeqLocked(), isHostOnly: true})
	}

	jar.enforceHostCookieLimitLocked(host)

	_, ok := jar.hostCookies[host]
	require.False(t, ok, "a host left with no live cookies must be removed from the map")
}

// Test_CookieJar_PerHostEvictionDropsLeastRecentlyWritten covers the other half:
// with nothing expired, the entries evicted are the ones written longest ago,
// and re-writing a cookie refreshes its recency.
func Test_CookieJar_PerHostEvictionDropsLeastRecentlyWritten(t *testing.T) {
	t.Parallel()

	const host = "example.com"
	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	// Fill the host to exactly its ceiling, each cookie on its own path so they
	// are distinct entries rather than replacements.
	for i := range maxCookiesPerHost {
		c := fasthttp.AcquireCookie()
		require.NoError(t, c.ParseBytes(fmt.Appendf(nil, "k%d=v; path=/p%d", i, i)))
		jar.SetByHost([]byte(host), c)
		fasthttp.ReleaseCookie(c)
	}
	require.Len(t, jar.hostCookies[host], maxCookiesPerHost)

	// Re-write the very first cookie so it becomes the most recently written.
	refreshed := fasthttp.AcquireCookie()
	require.NoError(t, refreshed.ParseBytes([]byte("k0=v2; path=/p0")))
	jar.SetByHost([]byte(host), refreshed)
	fasthttp.ReleaseCookie(refreshed)
	require.Len(t, jar.hostCookies[host], maxCookiesPerHost, "a re-write must replace, not grow")

	// Now push one more distinct cookie in, forcing a single eviction.
	extra := fasthttp.AcquireCookie()
	require.NoError(t, extra.ParseBytes([]byte("extra=v; path=/extra")))
	jar.SetByHost([]byte(host), extra)
	fasthttp.ReleaseCookie(extra)

	require.Len(t, jar.hostCookies[host], maxCookiesPerHost)

	names := make(map[string]string, maxCookiesPerHost)
	for _, sc := range jar.hostCookies[host] {
		names[string(sc.cookie.Key())] = string(sc.cookie.Value())
	}
	require.Contains(t, names, "extra", "the newest cookie must be kept")
	require.Contains(t, names, "k0", "the refreshed cookie must survive on recency")
	require.Equal(t, "v2", names["k0"], "and it must hold the re-written value")
	require.NotContains(t, names, "k1", "the least recently written cookie is the victim")
}

// Test_CookieJar_RelativePathUsesDefaultPath covers RFC 6265 Section 5.2.4:
// a Path attribute that does not begin with "/" is unusable and must fall back
// to the default-path. fasthttp's ParseBytes stores such a value verbatim, so
// without the fallback the cookie could never match any request path while
// still occupying one of the per-host slots.
func Test_CookieJar_RelativePathUsesDefaultPath(t *testing.T) {
	t.Parallel()

	for _, attr := range []string{"admin", "./admin", "../admin"} {
		t.Run(attr, func(t *testing.T) {
			t.Parallel()

			jar := AcquireCookieJar()
			defer ReleaseCookieJar(jar)

			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)
			resp.Header.Add("Set-Cookie", "a=1; Path="+attr)
			jar.parseCookiesFromResp([]byte("example.com"), []byte("/dir/page"), resp)

			require.Equal(t, "a=1", cookieHeaderFor(jar, "http://example.com/dir/x"))
			require.Empty(t, cookieHeaderFor(jar, "http://example.com/other"))
		})
	}
}

// Test_CookieJar_EvictsLeastRecentlyWritten checks that a cookie the server
// keeps refreshing survives eviction pressure from one-off cookies, which is
// exactly the pressure default-path scoping creates.
func Test_CookieJar_EvictsLeastRecentlyWritten(t *testing.T) {
	t.Parallel()

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	setCookie := func(path, header string) {
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)
		resp.Header.Add("Set-Cookie", header)
		jar.parseCookiesFromResp([]byte("example.com"), []byte(path), resp)
	}

	setCookie("/login", "session=SECRET; Path=/")
	for i := range maxCookiesPerHost * 2 {
		// Each directory mints its own entry for the same cookie name.
		setCookie(fmt.Sprintf("/page/%d/x", i), "pref=1")
		// The server re-sends the session cookie on every response.
		setCookie("/login", "session=SECRET; Path=/")
	}

	require.LessOrEqual(t, len(jar.hostCookies["example.com"]), maxCookiesPerHost)
	require.Equal(t, "session=SECRET", cookieHeaderFor(jar, "http://example.com/account"))
}

// Test_CookieJar_BoundsCookiesPerRequest checks the ceiling on what one
// request carries. The per-key cap alone does not bound it: a host-only cookie
// plus one Domain= cookie per parent label live under different keys and all
// domain-match the same request, so a host deep in a DNS tree could otherwise
// multiply its allowance by its label count.
func Test_CookieJar_BoundsCookiesPerRequest(t *testing.T) {
	t.Parallel()

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	const host = "a.b.c.d.example.com"
	for _, domain := range []string{"", "b.c.d.example.com", "c.d.example.com", "d.example.com", "example.com"} {
		for i := range maxCookiesPerHost * 2 {
			resp := fasthttp.AcquireResponse()
			attr := fmt.Sprintf("c%d_%d=v; Path=/", len(domain), i)
			if domain != "" {
				attr += "; Domain=" + domain
			}
			resp.Header.Add("Set-Cookie", attr)
			jar.parseCookiesFromResp([]byte(host), []byte("/"), resp)
			fasthttp.ReleaseResponse(resp)
		}
	}

	// The cap belongs to the wire, so it is the request that must be bounded.
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("http://" + host + "/")
	jar.dumpCookiesToReq(req)

	written := 0
	for range req.Header.Cookies() {
		written++
	}
	require.LessOrEqual(t, written, maxCookiesPerRequest)
	require.NotZero(t, written, "the request should still carry cookies")

	// Get is a storage query, not a wire operation: it must report every stored
	// cookie matching the URI, well past maxCookiesPerRequest. Capping it there
	// silently hid cookies that remain in the jar.
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	require.NoError(t, uri.Parse(nil, []byte("http://"+host+"/")))

	got := jar.Get(uri)
	require.Greater(t, len(got), maxCookiesPerRequest,
		"Get must not apply the per-request wire cap")
	for _, c := range got {
		fasthttp.ReleaseCookie(c)
	}
}

// Test_CookieJar_AttributesCookiesToRespondingHost asserts that Set-Cookie from
// a redirect target is stored against that target, not against the host the
// caller originally addressed. Crediting it to the original host would let any
// redirect target plant cookies for an origin it does not control, and those
// cookies would then ride along on every later request to that origin.
func Test_CookieJar_AttributesCookiesToRespondingHost(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/start", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusFound).To("http://attacker.example/plant")
		})
		app.Get("/plant", func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{Name: "session", Value: "planted"})
			return c.SendString("ok")
		})
		app.Get("/direct", func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{Name: "session", Value: "legitimate"})
			return c.SendString("ok")
		})
	})
	// Cleanup, not defer: the parallel subtests below resume only after this
	// function returns, so a deferred stop would race them.
	t.Cleanup(server.stop)

	jarCookies := func(t *testing.T, jar *CookieJar, host string) map[string]string {
		t.Helper()

		uri := fasthttp.AcquireURI()
		defer fasthttp.ReleaseURI(uri)
		uri.SetScheme("http")
		uri.SetHost(host)
		uri.SetPath("/")

		out := make(map[string]string)
		for _, ck := range jar.Get(uri) {
			out[string(ck.Key())] = string(ck.Value())
			fasthttp.ReleaseCookie(ck) // Get hands back copies it acquired from the pool.
		}
		return out
	}

	t.Run("redirect target cannot plant cookies for the origin", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)
		client := New().SetDial(server.dial()).SetCookieJar(jar)

		resp, err := client.Get("http://good.example/start", Config{MaxRedirects: 5})
		require.NoError(t, err)
		defer resp.Close()

		require.Empty(t, jarCookies(t, jar, "good.example"))
		require.Equal(t, map[string]string{"session": "planted"}, jarCookies(t, jar, "attacker.example"))
	})

	t.Run("a response with no redirect is still stored", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)
		client := New().SetDial(server.dial()).SetCookieJar(jar)

		resp, err := client.Get("http://good.example/direct")
		require.NoError(t, err)
		defer resp.Close()

		require.Equal(t, map[string]string{"session": "legitimate"}, jarCookies(t, jar, "good.example"))
	})
}
