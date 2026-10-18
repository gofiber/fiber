package proxy

import (
	"hash/maphash"
	"net"
	"sync/atomic"
	"time"

	"github.com/gofiber/utils/v2"
)

const (
	// lookupCacheTTL is how long the up-front check keeps a hostname's answer. The dial
	// guard never reads the cache, so a rebound answer is still caught at dial time.
	lookupCacheTTL = time.Second

	// lookupCacheSlots fixes the memory the cache can take. Two hostnames sharing a slot evict each
	// other and pay a real lookup per request, so it is sized for the hostnames a proxy serves at once.
	lookupCacheSlots = 1024
)

// upfrontLookups serves validateHostForSSRF, which runs on every request.
var upfrontLookups = newLookupCache(lookupCacheSlots, lookupCacheTTL, time.Now)

// lookupCache remembers successful lookups briefly, so the up-front check does not repeat
// one. It is direct-mapped under a seeded hash: its size is fixed, a flood of hostnames can
// only evict entries, and an entry belongs to the resolver that produced it.
type lookupCache struct {
	now   func() time.Time
	slots []atomic.Pointer[lookupEntry]
	seed  maphash.Seed
	ttl   time.Duration
}

type lookupEntry struct {
	expires  time.Time
	resolver *net.Resolver
	host     string
	addrs    []net.IPAddr
}

func newLookupCache(slots int, ttl time.Duration, now func() time.Time) *lookupCache {
	return &lookupCache{
		now:   now,
		slots: make([]atomic.Pointer[lookupEntry], slots),
		seed:  maphash.MakeSeed(),
		ttl:   ttl,
	}
}

func (c *lookupCache) slot(host string) *atomic.Pointer[lookupEntry] {
	return &c.slots[maphash.String(c.seed, host)%uint64(len(c.slots))]
}

// get returns what resolver answered for host, if the answer is still fresh.
func (c *lookupCache) get(host string, resolver *net.Resolver) ([]net.IPAddr, bool) {
	e := c.slot(host).Load()
	if e == nil || e.host != host || e.resolver != resolver || !c.now().Before(e.expires) {
		return nil, false
	}
	return e.addrs, true
}

// put remembers what resolver answered for host, replacing whatever shares its slot.
func (c *lookupCache) put(host string, resolver *net.Resolver, addrs []net.IPAddr) {
	c.slot(host).Store(&lookupEntry{
		// host may alias a request buffer.
		host:     utils.CopyString(host),
		resolver: resolver,
		addrs:    addrs,
		expires:  c.now().Add(c.ttl),
	})
}
