package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_LookupCache pins when an answer is served again: for the same host from
// the same resolver until the TTL ends, and until another host takes its slot.
func Test_LookupCache(t *testing.T) {
	t.Parallel()

	now := time.Unix(1000, 0)
	cache := newLookupCache(1, time.Second, func() time.Time { return now }) // one slot: every host collides
	first, second := &net.Resolver{}, &net.Resolver{}
	addrs := []net.IPAddr{{IP: net.ParseIP("198.51.100.7")}}

	_, ok := cache.get("a.test", first)
	require.False(t, ok, "nothing is remembered yet")

	cache.put("a.test", first, addrs)
	got, ok := cache.get("a.test", first)
	require.True(t, ok)
	require.Equal(t, addrs, got)

	_, ok = cache.get("a.test", second)
	require.False(t, ok, "another resolver's answer is not this one's")
	_, ok = cache.get("b.test", first)
	require.False(t, ok, "another host that maps to the same slot")

	now = now.Add(time.Second - time.Nanosecond)
	_, ok = cache.get("a.test", first)
	require.True(t, ok, "just before the TTL ends")
	now = now.Add(time.Nanosecond)
	_, ok = cache.get("a.test", first)
	require.False(t, ok, "when the TTL ends")

	cache.put("a.test", first, addrs)
	cache.put("b.test", first, addrs)
	_, ok = cache.get("a.test", first)
	require.False(t, ok, "replaced by a host that shares its slot")
}

// Test_Security_ValidateHostForSSRF_RemembersTheLookup pins what the cache is for and where it stops:
// the up-front check looks a name up once, the dial guard every time, and a refused answer stays refused.
func Test_Security_ValidateHostForSSRF_RemembersTheLookup(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide DNS resolver.
	public := net.IPv4(198, 51, 100, 1)
	dnsAddr, aQueries := startRebindingDNS(t, public, public)
	withRebindingResolver(t, dnsAddr)

	for range 3 {
		require.NoError(t, validateHostForSSRF("remembered.test", upfrontLookups))
	}
	require.Equal(t, int64(1), aQueries.Load(), "the up-front check looks the name up once")

	for range 2 {
		_, err := resolveAndValidateHost("remembered.test", time.Time{})
		require.NoError(t, err)
	}
	require.Equal(t, int64(3), aQueries.Load(), "the dial guard looks it up every time")

	private := net.IPv4(10, 0, 0, 1)
	dnsAddr, aQueries = startRebindingDNS(t, private, private)
	withRebindingResolver(t, dnsAddr)

	for range 3 {
		require.ErrorIs(t, validateHostForSSRF("internal.test", upfrontLookups), ErrUpstreamHostBlocked)
	}
	require.Equal(t, int64(1), aQueries.Load(), "a refused answer is held, and refused, each time")
}

// Test_Security_LookupHost_RemembersASmallAnswerOnly pins the size of what the cache holds: an answer
// with more addresses than lookupCacheMaxAddrs is judged and forgotten, so a name whose owner picks
// the answer cannot make every slot as large as it likes.
func Test_Security_LookupHost_RemembersASmallAnswerOnly(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide DNS resolver.
	for name, tc := range map[string]struct {
		addrs      int
		remembered bool
	}{
		"at the cap":   {addrs: lookupCacheMaxAddrs, remembered: true},
		"over the cap": {addrs: lookupCacheMaxAddrs + 1},
	} {
		t.Run(name, func(t *testing.T) {
			withRebindingResolver(t, startManyAnswersDNS(t, tc.addrs))
			cache := newLookupCache(16, time.Minute, time.Now)

			addrs, err := lookupHost("big.test", cache, time.Time{})
			require.NoError(t, err)
			require.Len(t, addrs, tc.addrs, "the caller gets the whole answer either way")

			_, remembered := cache.get("big.test", dnsResolver.Load())
			require.Equal(t, tc.remembered, remembered)
		})
	}
}

// Test_Security_LookupHost_EndsAtTheDeadline pins that the lookup of a dial gives up with the time the
// request left it, not at the lookup limit.
func Test_Security_LookupHost_EndsAtTheDeadline(t *testing.T) {
	t.Parallel()

	_, err := lookupHost("deadline.test", nil, time.Now().Add(-time.Second))
	require.ErrorIs(t, err, ErrUpstreamHostUnresolvable)
	var dnsErr *net.DNSError
	require.ErrorAs(t, err, &dnsErr)
	require.True(t, dnsErr.IsTimeout, "%v", err)
}

// Test_Security_LookupsFor pins which clients the up-front check may serve from the cache: only the
// shared client, whose dials are checked again where they are made.
func Test_Security_LookupsFor(t *testing.T) {
	// Intentionally not parallel: it reads and replaces the process-wide client.
	restoreGlobalProxyClient(t)

	shared := &fasthttp.Client{}
	WithClient(shared)
	require.Same(t, upfrontLookups, lookupsFor(shared))
	require.Nil(t, lookupsFor(&fasthttp.Client{}), "a client passed per call may have dialed before the guard reached it")

	dialsByItself := &fasthttp.Client{Transport: noopRoundTripper{}}
	WithClient(dialsByItself)
	require.Nil(t, lookupsFor(dialsByItself), "a client with a Transport is not covered by the dial guard")
}

// Test_Security_Do_UnguardedClientIsJudgedByAFreshLookup pins what the cache costs a client the dial
// guard cannot cover: nothing, because it never sees the cache. A name rebound within the cache's
// lifetime is judged by its new answer.
func Test_Security_Do_UnguardedClientIsJudgedByAFreshLookup(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide DNS resolver and security policy.
	policy := DefaultSecurityPolicy() // AllowPrivateIPs == false
	withSecurityPolicyForTest(t, policy)

	// The first A answer for a name is public; every one after is internal.
	dnsAddr, _ := startRebindingDNS(t, net.IPv4(198, 51, 100, 1), net.IPv4(10, 0, 0, 1))
	withRebindingResolver(t, dnsAddr)

	// What the cache does: the second judgment is the first answer, remembered.
	_, err := validateUpstream("http://cached.test/", policy, upfrontLookups)
	require.NoError(t, err)
	_, err = validateUpstream("http://cached.test/", policy, upfrontLookups)
	require.NoError(t, err)
	_, err = validateUpstream("http://cached.test/", policy, nil)
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)

	// The client below dials by itself: only the up-front check stands between it and the internal answer.
	var served atomic.Int32
	client := &fasthttp.Client{Transport: roundTripperFunc(func(_ *fasthttp.Request, resp *fasthttp.Response) error {
		served.Add(1)
		resp.SetStatusCode(fiber.StatusNoContent)
		return nil
	})}
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return Do(c, "http://fresh.test/", client) })

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode, "the rebound answer is judged")
	require.NoError(t, resp.Body.Close())
	require.Equal(t, int32(1), served.Load(), "and the second request never reaches the transport")
}

// Test_Security_FollowRedirects_UnguardedClientHopsAreJudgedByFreshLookups pins that the hops of a
// redirect a client the dial guard cannot cover follows never see the cache either: a name rebound
// between two hops is judged by its new answer.
func Test_Security_FollowRedirects_UnguardedClientHopsAreJudgedByFreshLookups(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide DNS resolver.
	// The first A answer for a name is public; every one after is internal.
	dnsAddr, _ := startRebindingDNS(t, net.IPv4(198, 51, 100, 1), net.IPv4(10, 0, 0, 1))
	withRebindingResolver(t, dnsAddr)

	var hops []string
	client, _ := newCountingRedirectClient(map[string]redirectStep{
		"http://start.example/": {status: fasthttp.StatusFound, location: "http://rebind.test/a"},
		"http://rebind.test/a":  {status: fasthttp.StatusFound, location: "http://rebind.test/b"},
	})
	client.onRequest = func(req *fasthttp.Request) { hops = append(hops, req.URI().String()) }

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)
	})
	req.SetRequestURI("http://start.example/")

	err := followRedirects(client.client, req, resp, 5, mustParseTestURL(t, "http://start.example/"), DefaultSecurityPolicy())
	require.ErrorIs(t, err, ErrUpstreamHostBlocked, "the second hop to the rebound name is judged by its new answer")
	require.Equal(t, []string{"http://start.example/", "http://rebind.test/a"}, hops, "and never sent")
}
