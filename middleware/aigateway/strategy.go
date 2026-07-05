package aigateway

import (
	"math/rand/v2"
	"sort"
)

// Strategy selects which upstream a request tries first. Whatever the
// strategy, failover then proceeds through the remaining candidates, so every
// strategy degrades to the same exhaustive chain walk under failures.
type Strategy int

const (
	// StrategyOrdered always tries upstreams in chain order: the first is
	// the primary, the rest are fallbacks.
	StrategyOrdered Strategy = iota

	// StrategyRoundRobin rotates the starting upstream across requests,
	// preserving relative chain order for failover.
	StrategyRoundRobin

	// StrategyWeighted picks the first upstream at random in proportion to
	// Upstream.Weight; the remaining candidates follow by descending weight
	// (chain order breaking ties) so failover is deterministic.
	StrategyWeighted
)

// randIntN is swapped out by tests for deterministic weighted picks.
var randIntN = rand.IntN

// orderCandidates reorders the breaker-filtered upstream indexes in place
// according to cfg.Strategy. idxs is freshly allocated per request by
// candidateUpstreams, so in-place mutation is safe.
func orderCandidates(cfg *Config, idxs []int) {
	if len(idxs) < 2 {
		return
	}
	switch cfg.Strategy {
	case StrategyOrdered:
	case StrategyRoundRobin:
		rotate(idxs, int(cfg.rr.Add(1)%uint64(len(idxs)))) //nolint:gosec // len is tiny, no overflow
	case StrategyWeighted:
		orderWeighted(cfg, idxs)
	}
}

// rotate left-rotates idxs by n (0 <= n < len).
func rotate(idxs []int, n int) {
	if n == 0 {
		return
	}
	rotated := make([]int, 0, len(idxs))
	rotated = append(rotated, idxs[n:]...)
	rotated = append(rotated, idxs[:n]...)
	copy(idxs, rotated)
}

// orderWeighted puts a weighted-random pick first and sorts the rest by
// descending weight (index order breaking ties, so failover is deterministic).
func orderWeighted(cfg *Config, idxs []int) {
	total := 0
	for _, i := range idxs {
		total += cfg.Upstreams[i].Weight
	}
	r := randIntN(total)
	pick := len(idxs) - 1
	for n, i := range idxs {
		w := cfg.Upstreams[i].Weight
		if r < w {
			pick = n
			break
		}
		r -= w
	}
	idxs[0], idxs[pick] = idxs[pick], idxs[0]

	// rest starts in ascending index order, so a stable sort by descending
	// weight leaves equal weights in index order.
	rest := idxs[1:]
	sort.SliceStable(rest, func(a, b int) bool {
		return cfg.Upstreams[rest[a]].Weight > cfg.Upstreams[rest[b]].Weight
	})
}
