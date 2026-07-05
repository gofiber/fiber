package aigateway

import (
	"sync"
	"time"
)

// defaultQuotaWindow is the quota window used when Config.QuotaWindow is
// unset.
const defaultQuotaWindow = time.Hour

// QuotaStore tracks per-identity token and cost totals over fixed windows for
// quota admission. The identity is KeyPolicy.Tenant when set, else the client
// key. Implementations must be safe for concurrent use; Add is called from
// stream writer goroutines after the handler has returned.
//
// Quotas are post-paid: the gateway admits a request when the identity's
// current-window totals are still under its limits (Peek) and commits the
// actual usage after the response (Add), so a burst of in-flight requests can
// overshoot a limit by the usage of the requests already admitted.
//
// The interface is increment-shaped so a distributed implementation (e.g.
// Redis) can be atomic: Add must apply the delta and return the new window
// totals in one step.
type QuotaStore interface {
	// Peek returns the identity's running totals for the current window.
	// An identity with no recorded usage returns zeros.
	Peek(identity string, window time.Duration) (tokens int64, cost float64, err error)

	// Add atomically adds tokens and cost to the identity's current window
	// and returns the new window totals.
	Add(identity string, window time.Duration, tokens int64, cost float64) (int64, float64, error)
}

// quotaBucket is one identity's totals for one fixed window.
type quotaBucket struct {
	start  time.Time
	tokens int64
	cost   float64
}

// memoryQuotaStore is the in-process QuotaStore used when quotas are active
// and no store is supplied. Windows are wall-aligned (all identities roll
// over together at multiples of the window since the Unix epoch), and stale
// buckets are swept opportunistically so an open-ended key space (per-key
// identities) cannot grow the map without bound.
type memoryQuotaStore struct {
	buckets   map[string]*quotaBucket
	lastSweep time.Time
	mu        sync.Mutex
}

// Compile-time check that memoryQuotaStore implements QuotaStore.
var _ QuotaStore = (*memoryQuotaStore)(nil)

func newMemoryQuotaStore() *memoryQuotaStore {
	return &memoryQuotaStore{buckets: make(map[string]*quotaBucket)}
}

// windowStart returns the wall-aligned start of the current window.
func windowStart(now time.Time, window time.Duration) time.Time {
	return now.Truncate(window)
}

//nolint:gocritic // results documented on the interface; naming them would violate nonamedreturns
func (s *memoryQuotaStore) Peek(identity string, window time.Duration) (int64, float64, error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[identity]
	if !ok || !b.start.Equal(windowStart(now, window)) {
		return 0, 0, nil
	}
	return b.tokens, b.cost, nil
}

//nolint:gocritic // results documented on the interface; naming them would violate nonamedreturns
func (s *memoryQuotaStore) Add(identity string, window time.Duration, tokens int64, cost float64) (int64, float64, error) {
	now := time.Now()
	start := windowStart(now, window)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweep(now, window)

	b, ok := s.buckets[identity]
	if !ok {
		b = &quotaBucket{start: start}
		s.buckets[identity] = b
	} else if !b.start.Equal(start) {
		// The window rolled over: reset in place.
		b.start = start
		b.tokens = 0
		b.cost = 0
	}
	b.tokens += tokens
	b.cost += cost
	return b.tokens, b.cost, nil
}

// sweep drops buckets from past windows, at most once per window, so the map
// stays bounded by the number of identities active in the current and
// previous window. Called with mu held.
func (s *memoryQuotaStore) sweep(now time.Time, window time.Duration) {
	if now.Sub(s.lastSweep) < window {
		return
	}
	s.lastSweep = now
	start := windowStart(now, window)
	for id, b := range s.buckets {
		if !b.start.Equal(start) {
			delete(s.buckets, id)
		}
	}
}

// effectiveQuota resolves the token/budget limits for one request from the
// global config and the per-key policy (>0 override, 0 inherit, <0 exempt).
//
//nolint:gocritic // the two results are the token and budget limits, in that order
func effectiveQuota(cfg *Config, policy *KeyPolicy) (int64, float64) {
	tokens := cfg.TokensPerWindow
	budget := cfg.BudgetPerWindow
	if policy != nil {
		switch {
		case policy.TokensPerWindow > 0:
			tokens = policy.TokensPerWindow
		case policy.TokensPerWindow < 0:
			tokens = 0
		}
		switch {
		case policy.BudgetPerWindow > 0:
			budget = policy.BudgetPerWindow
		case policy.BudgetPerWindow < 0:
			budget = 0
		}
	}
	return tokens, budget
}

// quotaRetryAfter returns the whole seconds until the current window rolls
// over, at least 1, for the Retry-After header of a 429.
func quotaRetryAfter(window time.Duration) int {
	now := time.Now()
	remaining := windowStart(now, window).Add(window).Sub(now)
	secs := int((remaining + time.Second - 1) / time.Second)
	return max(secs, 1)
}
