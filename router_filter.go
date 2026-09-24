// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"math"
	"math/bits"
	"strings"

	"github.com/gofiber/utils/v2/swar"
)

// bucketFilter lets the scan rule out a bucket's routes before loading them.
// Only a bucket of filterMinBucket routes or more carries one.
type bucketFilter struct {
	// heads is parallel to the bucket: each route's scanHead
	heads []scanHead
	// prints is parallel to the bucket: each static route's fingerprint, 0 for
	// a route the fingerprint cannot speak for. nil below fingerprintMinBucket
	// static routes.
	prints []uint64
	// maxLen is the longest static path in the bucket; a longer request
	// matches none of them, so the scan need not hash it
	maxLen int
}

// newBucketFilter builds a bucket's filter, or returns nil when the bucket has
// fewer than filterMinBucket routes. Built from the finished bucket so the
// tree's pointer swap publishes routes and filter together.
func newBucketFilter(routes []*Route) *bucketFilter {
	if len(routes) < filterMinBucket {
		return nil
	}
	f := &bucketFilter{heads: make([]scanHead, len(routes))}
	static := 0
	for i, route := range routes {
		f.heads[i].init(route)
		if staticFingerprint(route) != 0 {
			static++
		}
	}
	if static < fingerprintMinBucket {
		return f
	}

	f.prints = make([]uint64, len(routes))
	for i, route := range routes {
		f.prints[i] = staticFingerprint(route)
		if f.prints[i] != 0 {
			f.maxLen = max(f.maxLen, len(route.path))
		}
	}
	return f
}

// scanHead is what a route scan needs to rule a candidate out without loading
// its Route: a leading-byte filter, a slash-count filter and the constant
// probe. A filtered bucket holds one per route at the route's index, so a
// candidate they reject costs a sequential read of its head instead of a
// pointer chase into a Route, which a large route set keeps out of L1: the
// chase was most of what a rejected candidate cost. Like every filter here it
// can only reject what Route.match would.
//
// The leading-byte filter here reads two words where Route's reads one, since
// the routes sharing a bucket tend to share their first word too: every route
// of an "/api/v1/" API does. A head costs a call to skip to get past, which a
// route that got by one word paid for in full before match turned it down.
//
// Heads are derived from their routes when the tree is built. A route's
// filters are final by then (see buildPrefixFilter), and a head is published
// together with its route by the tree's pointer swap, so the two cannot
// disagree.
type scanHead struct {
	// prefix and prefixMask filter on the first word of knownPrefix, as the
	// route's own prefix and prefixMask do; prefix2 and prefixMask2 on the
	// second, when there is one
	prefix      uint64
	prefixMask  uint64
	prefix2     uint64
	prefixMask2 uint64
	// probeWord is what the route's constant probe compares, when match
	// would consult one; see probe
	probeWord uint64
	// slashMask has bit n set when a detection path holding n '/' bytes can
	// match, bit maxSlashBit standing for that many or more; see init
	slashMask uint32
	// probeFrom and probeSkip locate the probe as constProbe's from and skip
	// do, and probeLen is how many bytes of probeWord it compares, 0 when the
	// head has no probe. They are narrow so that a head takes 48 bytes, since
	// a filtered bucket holds one per route. A probe they cannot hold, which
	// takes a leading constant over 64 KiB or more than 255 parameters before
	// the probe, is not checked before the full match: match skips its own
	// copy in a filtered bucket (see skipSlashFilters), and getMatch compares
	// the same constant, so such a route only loses the quick reject.
	probeFrom uint16
	probeSkip uint8
	probeLen  uint8
}

// maxSlashBit is the highest bit of a scanHead's slashMask, which stands for
// that many '/' bytes or more.
const maxSlashBit = 31

// init fills h, a zero scanHead, with the head of r. newBucketFilter fills
// its heads in place with it, since copying each one out of a constructor was
// a third of what building a filter cost. The first prefix word is the route's
// own leading-byte filter, which buildPrefixFilter computed when the route was
// registered.
//
// The slash counts a route can match become a bitmask, so the scan tests them
// with one AND. A route without parameters is compared against r.path, in full
// or as a prefix (use), so it takes that path's count, or any count from it
// up. A route with parameters gets the bounds routeParser.computeSlashBounds
// found and matchParams checks: a prefix (use) route only the lower one. Star
// and root routes are answered before either comparison, so they keep every
// bit, as does every route for bit 0, which a scan uses for a count it did not
// compute. The probe is taken where matchParams would consult it.
func (h *scanHead) init(r *Route) {
	h.slashMask = ^uint32(0)
	h.prefix, h.prefixMask = r.prefix, r.prefixMask
	if prefix := knownPrefix(r); len(prefix) > swar.WordLen {
		h.prefix2, h.prefixMask2 = packConst(prefix[swar.WordLen:])
	}
	switch {
	case r.star || r.root:
		return
	case len(r.Params) == 0:
		n := min(strings.Count(r.path, string(slashDelimiter)), maxSlashBit)
		if r.use {
			h.slashMask <<= n
		} else {
			h.slashMask = uint32(1) << n
		}
	default:
		p := &r.routeParser
		// Every count from the minimum up, capped at maxSlashBit, which a
		// count at or past the minimum always sets.
		h.slashMask <<= min(max(p.minSlashes, 0), maxSlashBit)
		if !r.use && p.maxBounded && p.maxSlashes < maxSlashBit {
			// ...and none past the maximum. A maximum below the minimum
			// leaves no count at all, apart from bit 0 below.
			h.slashMask &= uint32(1)<<(max(p.maxSlashes, -1)+1) - 1
		}
		h.setProbe(p.probe)
	}
	h.slashMask |= 1
}

// setProbe stores p, a route's constant probe, in h, unless it has none or h's
// narrow fields cannot hold it. A probe's mask covers the leading lanes of its
// word (see packConst), so its length stands in for it.
func (h *scanHead) setProbe(p constProbe) {
	n := bits.Len64(p.mask) / 8
	// A probe lies past the route's leading constant, so from is positive,
	// which also keeps its memo key off the 0 that stands for none.
	if n == 0 || p.mask != lanesMask(n) || p.from <= 0 || p.from > math.MaxUint16 || p.skip < 0 || p.skip > math.MaxUint8 {
		return
	}
	h.probeWord = p.word
	h.probeFrom = uint16(p.from)
	h.probeSkip = uint8(p.skip)
	h.probeLen = uint8(n) //nolint:gosec // G115 - at most a word's lanes
}

// probe returns the constant probe h holds, which h.probeLen says it does.
func (h *scanHead) probe() constProbe {
	return constProbe{word: h.probeWord, mask: lanesMask(int(h.probeLen)), from: int32(h.probeFrom), skip: int32(h.probeSkip)}
}

// headRejects is constProbe.rejects for the constant probe h holds, which
// h.probeLen says it does, searching for the slash only when the probe the
// memo last served read another one: the key is the probe's from and its
// skip, since probes that share a from can still differ in the slash they
// read. The probe is only assembled when the memo has to search.
// detectionPath must be the same on every call.
func (m *probeMemo) headRejects(h *scanHead, detectionPath string) bool {
	if k := uint64(h.probeFrom)<<32 | uint64(h.probeSkip); k != m.key {
		p := h.probe()
		m.word, m.ok = p.locate(detectionPath)
		m.key = k
	}
	return !m.ok || m.word&lanesMask(int(h.probeLen)) != h.probeWord
}

// lanesMask covers the first n lanes of a word, 0 <= n <= 8.
func lanesMask(n int) uint64 {
	return ^uint64(0) >> (64 - 8*uint(n))
}

// slashBit is the bit a detection path's slash count selects in a scanHead's
// slashMask: bit n for n '/' bytes, bit maxSlashBit for that many or more, and
// bit 0 for a count of 0, which is how pathSlashCount reports one it did not
// compute and which every head accepts.
func slashBit(pathSlashes int) uint32 {
	return uint32(1) << min(pathSlashes, maxSlashBit)
}

// rejects reports whether the leading-byte filter or the slash-count filter
// rules the head's route out for a detection path whose first two words are
// head and head2, packed by pathHeadWord, and whose slash count selects slash
// (see slashBit). The masked compares are combined without a branch.
func (h *scanHead) rejects(head, head2 uint64, slash uint32) bool {
	return (head^h.prefix)&h.prefixMask|(head2^h.prefix2)&h.prefixMask2|uint64(slash&^h.slashMask) != 0
}

// routeScan is the request's side of a scan through filtered buckets: what
// their filters compare against. A scan sets it up when it first has to walk
// a filtered bucket past a candidate, and skip then walks such a bucket to
// the next route the filter lets through.
type routeScan struct {
	detectionPath string
	// head and head2 are the path's first two words, packed by pathHeadWord
	head  uint64
	head2 uint64
	// slash is slashBit of the path's slash count
	slash uint32
	// probes shares the constant probes' slash search among candidates
	probes probeMemo
}

// init sets up s, a zero routeScan, for detectionPath, whose leading bytes
// pathHeadWord packed into head and which holds pathSlashes '/' bytes (0 when
// the count was not computed). It fills s in place, since copying the struct
// out of a constructor stalled.
func (s *routeScan) init(detectionPath string, head uint64, pathSlashes int) {
	s.detectionPath = detectionPath
	s.head = head
	if n := len(detectionPath); n > swar.WordLen {
		// The second word as pathHeadWord packs it: a path under two words
		// is loaded from its end, overlapping the first word, and shifted
		// down so that its byte 8 lands in lane 0 and the lanes past its end
		// are zero: one load in place of a byte loop or a call.
		off := min(n, 2*swar.WordLen) - swar.WordLen
		s.head2 = swar.Load8(detectionPath, off) >> (8 * (swar.WordLen - off))
	}
	s.slash = slashBit(pathSlashes)
}

// skip returns the index of the first route at or after i that f, the filter
// of the bucket being scanned, does not rule out, or the bucket's length when
// none is left. first is where the scan started: the fingerprints are not
// consulted there, so a hit on the first candidate never hashes the path.
// pathPrint holds the path's fingerprint, 0 until a scan first needs it, when
// skip stores it there; DefaultCtx keeps it across the scans of a request. It
// is a parameter rather than a field so that a caller's local does not escape.
func (s *routeScan) skip(f *bucketFilter, i, first int, pathPrint *uint64) int {
	heads := f.heads
	// The request fingerprint: fingerprintNone when no static route in the
	// bucket is as long as the path, else *pathPrint, hashed the first time a
	// scan needs it.
	prints := f.prints
	want := *pathPrint
	if len(s.detectionPath) > f.maxLen {
		want = fingerprintNone
	}
	for ; i < len(heads); i++ {
		// Reject static routes by fingerprint first. prints is nil below
		// fingerprintMinBucket static routes, and 0 marks a route it cannot
		// judge.
		if i > first && i < len(prints) {
			if want == 0 {
				want = pathFingerprint(s.detectionPath)
				*pathPrint = want
			}
			for i < len(prints) && prints[i] != 0 && prints[i] != want {
				i++
			}
			if i >= len(heads) {
				break
			}
		}
		if h := &heads[i]; h.rejects(s.head, s.head2, s.slash) ||
			h.probeLen != 0 && s.probes.headRejects(h, s.detectionPath) {
			continue
		}
		return i
	}
	return len(heads)
}

// endpoint walks tree, a bucket that f filters, for the first endpoint (a
// non-use route) matching the request, as the scans deciding between 404 and
// 405 walk the other methods' buckets. It returns the endpoint's index and
// true, with its parameters written to values, or the bucket's last index and
// false when there is none, which is where those scans leave the context.
// pathPrint is as for skip.
func (s *routeScan) endpoint(tree []*Route, f *bucketFilter, path string, values *[maxParams]string, pathPrint *uint64) (int, bool) {
	for i := 0; ; i++ {
		if i = s.skip(f, i, 0, pathPrint); i >= len(tree) {
			return len(tree) - 1, false
		}
		route := tree[i]
		if route.use {
			continue
		}
		// skip applied the slash count and the probe
		if route.match(s.detectionPath, path, values, skipSlashFilters) {
			return i, true
		}
	}
}
