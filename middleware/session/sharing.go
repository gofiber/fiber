package session

import (
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/utils/v2"
)

// extractorRoot identifies an extractor well enough to notice that the Extractor of
// a store was replaced after NewStore, which Config being embedded allows: it holds
// what the cookies and headers of an extractor depend on and tells one extractor
// from another without calling it, the key and source of the root and the identity
// of its chain (a chain reports the key and source of its first child, so these two
// alone cannot tell a leaf from a chain that starts with it). Changing a nested
// extractor in place is not noticed.
type extractorRoot struct {
	key    string
	chain  []extractors.Extractor
	source extractors.Source
}

func rootOf(ext extractors.Extractor) extractorRoot {
	return extractorRoot{key: ext.Key, chain: ext.Chain, source: ext.Source}
}

// matches reports whether ext is the extractor r was taken from.
func (r extractorRoot) matches(ext extractors.Extractor) bool {
	if r.key != ext.Key || r.source != ext.Source || len(r.chain) != len(ext.Chain) {
		return false
	}
	return len(r.chain) == 0 || &r.chain[0] == &ext.Chain[0]
}

// collectSinks lists the cookies and headers an extractor reads a session ID from,
// which are the ones a session ID can be written back to: the leaves of the
// extractor that name a cookie or a header.
func collectSinks(ext extractors.Extractor) []extractors.Extractor {
	var sinks []extractors.Extractor
	// Walked rather than ranged over Chain: a nested chain reports its first
	// child's metadata, so direct children alone hide every sink inside it, and
	// a cookie one level down would never be written.
	ext.Walk(func(candidate extractors.Extractor) bool {
		// Only a leaf names a place to write, and a keyless one would name a
		// cookie or header with no name at all.
		if len(candidate.Chain) > 0 || candidate.Key == "" {
			return true
		}
		if candidate.Source == extractors.SourceCookie || candidate.Source == extractors.SourceHeader {
			sinks = append(sinks, candidate)
		}
		return true
	})
	return sinks
}

// sourceID names a source a session ID is read from: the kind of source and the
// name in it, such as the cookie "session_id", the header "X-Session" or the query
// parameter "sid". For the Authorization header the name is the scheme, since the
// header is always the same. A source without a name is named by its kind alone, so
// two custom extractors without a key count as one source.
type sourceID struct {
	name string
	kind extractors.Source
}

// sourceOf names a leaf extractor.
func sourceOf(leaf extractors.Extractor) sourceID {
	if leaf.Source == extractors.SourceAuthHeader {
		return sourceID{kind: leaf.Source, name: leaf.AuthScheme}
	}
	return sourceID{kind: leaf.Source, name: leaf.Key}
}

// same reports whether a and b are one source: the same kind and the same name,
// which for a header or an authentication scheme is compared without regard to case.
func (a sourceID) same(b sourceID) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case extractors.SourceHeader, extractors.SourceAuthHeader:
		return utils.EqualFold(a.name, b.name)
	default:
		return a.name == b.name
	}
}

// collectSources names every source an extractor reads a session ID from, as
// sharesSource compares them: the leaves of the extractor.
func collectSources(ext extractors.Extractor) []sourceID {
	var sources []sourceID
	ext.Walk(func(candidate extractors.Extractor) bool {
		if len(candidate.Chain) == 0 {
			sources = append(sources, sourceOf(candidate))
		}
		return true
	})
	return sources
}

// extractorCached reports whether sinks and sources are those of the extractor the
// store has now: NewStore worked them out, and the Extractor was not replaced since.
func (s *Store) extractorCached() bool {
	return s.sinks != nil && s.root.matches(s.Extractor)
}

// writableSinks returns the cookies and headers a session ID of s can be written
// back to. The slice is shared: do not modify it.
func (s *Store) writableSinks() []extractors.Extractor {
	if s.extractorCached() {
		return s.sinks
	}
	return collectSinks(s.Extractor)
}

// sourceList returns the sources s reads its session ID from. The slice is shared:
// do not modify it.
func (s *Store) sourceList() []sourceID {
	if s.extractorCached() {
		return s.sources
	}
	return collectSources(s.Extractor)
}

// sharesSource reports whether s and other read the session ID from a common
// source, by kind and name, or cannot be told apart because one of them names
// none.
//
// A request can hold the sessions of several stores. Stores over the same
// cookie, such as a session middleware and the store the CSRF middleware was
// given, resolve the same session and so share the ID cached on the request.
// Stores over different sources keep independent sessions: one adopting the
// other's cached ID would look up a foreign session, or write its own ID to a
// cookie it does not own. A source that cannot be written back to, such as the
// query string, counts like any other: the ID a link carries reaches the stores
// that read the query string, and no cookie store next to them.
func (s *Store) sharesSource(other *Store) bool {
	if s == other {
		return true
	}
	mine, theirs := s.sourceList(), other.sourceList()
	if len(mine) == 0 || len(theirs) == 0 {
		return true
	}
	return slices.ContainsFunc(mine, func(a sourceID) bool {
		return slices.ContainsFunc(theirs, a.same)
	})
}

// requestIDPublisher returns the store that published the session ID cached on
// the request, or nil if that is not known.
//
// The session middleware publishes the ID together with itself, so the middleware
// still current in the request names the store, and costs the request nothing
// extra. A session from Store.Get cannot be told apart that way, so it leaves its
// store behind: in the provenance box of the request, box, when there is one, and
// otherwise in sessionOwnerContextKey. The box is the one written last, since a
// publication goes to the box as soon as the request has one.
func requestIDPublisher(c fiber.Ctx, box *provenanceBox) *Store {
	if m, ok := c.Locals(middlewareContextKey).(*Middleware); ok && m != nil {
		// Under the lock: the middleware may be finishing on another goroutine,
		// as it does when a handler outlives its request.
		m.mu.RLock()
		publisher := m.store
		m.mu.RUnlock()
		return publisher
	}
	if box != nil && box.publisher != nil {
		return box.publisher
	}
	if owner, ok := c.Locals(sessionOwnerContextKey).(*Store); ok {
		return owner
	}
	return nil
}

// requestProvenance returns the provenance box cached on the request, nil if
// there is none.
func requestProvenance(c fiber.Ctx) *provenanceBox {
	box, ok := c.Locals(sessionExtractorContextKey).(*provenanceBox)
	if !ok {
		return nil
	}
	return box
}

// publisherShares reports whether the store that published the session ID cached
// on the request, if it is known, shares a cookie or header with s. box is the
// provenance box of the request, nil if it has none.
func (s *Store) publisherShares(c fiber.Ctx, box *provenanceBox) bool {
	publisher := requestIDPublisher(c, box)
	return publisher == nil || publisher == s || s.sharesSource(publisher)
}

// adoptProvenance returns res, the provenance of a session ID that another store
// cached on the request, as it holds for s. A cookie or header that is not one of
// the sinks of s is not its to write the ID back to: the ID is then treated as one
// from a source that cannot be written back to, which is what Resolved false means
// to getExtractorInfo. A read-only source, and a sink of s, stay as they are.
func (s *Store) adoptProvenance(res extractors.Result) extractors.Result {
	if res.Resolved && (res.Source == extractors.SourceCookie || res.Source == extractors.SourceHeader) && !s.ownsSink(res) {
		res.Resolved = false
	}
	return res
}

// ownsSink reports whether res names a cookie or header s reads its session ID from.
func (s *Store) ownsSink(res extractors.Result) bool {
	source := sourceID{kind: res.Source, name: res.Key}
	for _, sink := range s.writableSinks() {
		if sourceOf(sink).same(source) {
			return true
		}
	}
	return false
}
