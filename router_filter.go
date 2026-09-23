// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
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
		f.heads[i] = newScanHead(route)
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
	// slashMask has bit n set when a detection path holding n '/' bytes can
	// match, bit 63 standing for 63 or more; see newScanHead
	slashMask uint64
	// probe is the route's constant probe when match would consult it, zero
	// otherwise
	probe constProbe
}

// newScanHead derives a route's scanHead.
//
// The slash counts a route can match become a bitmask, so the scan tests them
// with one AND. A route without parameters is compared against r.path, in full
// or as a prefix (use), so it takes that path's count, or any count from it
// up. A route with parameters gets the bounds routeParser.computeSlashBounds
// found and matchParams checks: a prefix (use) route only the lower one. Star
// and root routes are answered before either comparison, so they keep every
// bit, as does every route for bit 0, which a scan uses for a count it did not
// compute. The probe is taken where matchParams would consult it.
func newScanHead(r *Route) scanHead {
	h := scanHead{slashMask: ^uint64(0)}
	prefix := knownPrefix(r)
	h.prefix, h.prefixMask = packConst(prefix)
	if len(prefix) > swar.WordLen {
		h.prefix2, h.prefixMask2 = packConst(prefix[swar.WordLen:])
	}
	switch {
	case r.star || r.root:
		return h
	case len(r.Params) == 0:
		n := min(strings.Count(r.path, string(slashDelimiter)), 63)
		if r.use {
			h.slashMask <<= n
		} else {
			h.slashMask = uint64(1) << n
		}
	default:
		p := &r.routeParser
		// Every count from the minimum up, capped at bit 63, which a count at
		// or past the minimum always sets.
		h.slashMask <<= min(max(p.minSlashes, 0), 63)
		if !r.use && p.maxBounded && p.maxSlashes < 63 {
			// ...and none past the maximum. A maximum below the minimum
			// leaves no count at all, apart from bit 0 below.
			h.slashMask &= uint64(1)<<(max(p.maxSlashes, -1)+1) - 1
		}
		h.probe = p.probe
	}
	h.slashMask |= 1
	return h
}

// slashBit is the bit a detection path's slash count selects in a scanHead's
// slashMask: bit n for n '/' bytes, bit 63 for 63 or more, and bit 0 for a
// count of 0, which is how pathSlashCount reports one it did not compute and
// which every head accepts.
func slashBit(pathSlashes int) uint64 {
	return uint64(1) << min(pathSlashes, 63)
}

// rejects reports whether the leading-byte filter or the slash-count filter
// rules the head's route out for a detection path whose first two words are
// head and head2, packed by pathHeadWord, and whose slash count selects slash
// (see slashBit). The masked compares are combined without a branch.
func (h *scanHead) rejects(head, head2, slash uint64) bool {
	return (head^h.prefix)&h.prefixMask|(head2^h.prefix2)&h.prefixMask2|slash&^h.slashMask != 0
}

// routeScan is the request's side of a scan through filtered buckets: what
// their filters compare against. A scan sets it up when it first meets a
// filtered bucket, and skip then walks such a bucket to the next route the
// filter lets through.
type routeScan struct {
	detectionPath string
	// head and head2 are the path's first two words, packed by pathHeadWord
	head  uint64
	head2 uint64
	// slash is slashBit of the path's slash count
	slash uint64
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
	if len(detectionPath) > swar.WordLen {
		s.head2 = pathHeadWord(detectionPath[swar.WordLen:])
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
			h.probe.mask != 0 && s.probes.rejects(&h.probe, s.detectionPath) {
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
		// skip applied the slash count and the probe: see next for the 0
		if route.match(s.detectionPath, path, values, 0) {
			return i, true
		}
	}
}
