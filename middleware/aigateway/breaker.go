package aigateway

import (
	"sync/atomic"
	"time"
)

// defaultBreakerCooldown is how long an opened breaker skips its upstream
// when Config.BreakerCooldown is unset.
const defaultBreakerCooldown = 30 * time.Second

// upstreamBreaker is the circuit-breaker state for one upstream. All fields
// are atomics: requests from many goroutines record outcomes concurrently,
// and the check-then-open sequence tolerates races (two racing failures both
// opening the breaker is correct, just redundant).
type upstreamBreaker struct {
	// failures counts consecutive failed attempts; any received
	// non-retryable response resets it.
	failures atomic.Int32

	// openUntil is the unix-nano deadline the upstream is skipped until.
	// Zero means closed. After the deadline the upstream is probed again
	// (half-open): success closes the breaker, failure reopens it because
	// failures still exceeds the threshold.
	openUntil atomic.Int64
}

// open reports whether the breaker currently skips its upstream.
func (b *upstreamBreaker) open(now time.Time) bool {
	return now.UnixNano() < b.openUntil.Load()
}

// recordFailure counts a failed attempt (network error or retryable status)
// and opens the breaker once the consecutive-failure count reaches threshold.
func (b *upstreamBreaker) recordFailure(threshold int, cooldown time.Duration) {
	if int(b.failures.Add(1)) >= threshold {
		b.openUntil.Store(time.Now().Add(cooldown).UnixNano())
	}
}

// recordSuccess closes the breaker: any received non-retryable response
// (2xx or a client error relayed verbatim) proves the upstream healthy.
func (b *upstreamBreaker) recordSuccess() {
	b.failures.Store(0)
	b.openUntil.Store(0)
}

// candidateUpstreams returns the indexes of the upstreams to try, in chain
// order. With the breaker enabled, upstreams whose breaker is open are
// skipped — unless that would leave no candidate at all, in which case every
// upstream is tried anyway (an all-open chain must still serve the request,
// and doubles as the probe that lets breakers close again). Skipped upstream
// names are appended to ev.SkippedUpstreams.
func candidateUpstreams(cfg *Config, ev *UsageEvent) []int {
	idxs := make([]int, 0, len(cfg.Upstreams))
	if cfg.breakers == nil {
		for i := range cfg.Upstreams {
			idxs = append(idxs, i)
		}
		return idxs
	}

	now := time.Now()
	var skipped []string
	for i := range cfg.Upstreams {
		if cfg.breakers[i].open(now) {
			skipped = append(skipped, cfg.Upstreams[i].Name)
			continue
		}
		idxs = append(idxs, i)
	}
	if len(idxs) == 0 {
		for i := range cfg.Upstreams {
			idxs = append(idxs, i)
		}
		return idxs
	}
	ev.SkippedUpstreams = skipped
	return idxs
}
