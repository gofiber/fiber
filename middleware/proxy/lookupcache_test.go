package proxy

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

// Test_Security_ValidateHostForSSRF_RemembersTheLookup pins what the cache is
// for and where it stops: the up-front check of a request looks a name up once,
// the dial guard looks it up every time, and an answer the blocklist refuses is
// refused from the cache too.
func Test_Security_ValidateHostForSSRF_RemembersTheLookup(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide DNS resolver.
	public := net.IPv4(198, 51, 100, 1)
	dnsAddr, aQueries := startRebindingDNS(t, public, public)
	withRebindingResolver(t, dnsAddr)

	for range 3 {
		require.NoError(t, validateHostForSSRF("remembered.test"))
	}
	require.Equal(t, int64(1), aQueries.Load(), "the up-front check looks the name up once")

	for range 2 {
		_, err := resolveAndValidateHost("remembered.test")
		require.NoError(t, err)
	}
	require.Equal(t, int64(3), aQueries.Load(), "the dial guard looks it up every time")

	private := net.IPv4(10, 0, 0, 1)
	dnsAddr, aQueries = startRebindingDNS(t, private, private)
	withRebindingResolver(t, dnsAddr)

	for range 3 {
		require.ErrorIs(t, validateHostForSSRF("internal.test"), ErrUpstreamHostBlocked)
	}
	require.Equal(t, int64(1), aQueries.Load(), "a refused answer is held, and refused, each time")
}
