package session

import (
	"slices"

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

// writesBack reports whether a session ID can be written back to a source of this
// kind: only a cookie or a header names a place the response can carry it to.
func writesBack(kind extractors.Source) bool {
	return kind == extractors.SourceCookie || kind == extractors.SourceHeader
}

// leaves calls visit for every extractor of ext that reads a source itself, in order.
// The tree is walked rather than ranged over Chain: a nested chain reports the metadata
// of its first child, so the direct children alone hide every leaf inside it, and a
// cookie one level down would never be found.
func leaves(ext extractors.Extractor, visit func(leaf extractors.Extractor)) {
	ext.Walk(func(candidate extractors.Extractor) bool {
		if len(candidate.Chain) == 0 {
			visit(candidate)
		}
		return true
	})
}

// collectSinks lists the cookies and headers an extractor reads a session ID from,
// which are the ones a session ID can be written back to: the leaves of the
// extractor that name a cookie or a header.
func collectSinks(ext extractors.Extractor) []extractors.Extractor {
	var sinks []extractors.Extractor
	leaves(ext, func(leaf extractors.Extractor) {
		// A keyless leaf would name a cookie or header with no name at all.
		if leaf.Key != "" && writesBack(leaf.Source) {
			sinks = append(sinks, leaf)
		}
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
	leaves(ext, func(leaf extractors.Extractor) {
		sources = append(sources, sourceOf(leaf))
	})
	return sources
}

// extractorProfile is what a store works out from its Extractor once, so that no request
// has to walk it: where a session ID can be written back to, and every source it is
// read from. Config is embedded in the Store, so the Extractor can be replaced after
// NewStore; root tells that, and the store then walks the extractor it has now.
type extractorProfile struct {
	// sinks are the cookies and headers a session ID can be written back to, in the
	// order of the extractor. Read-only: every session of the store shares the slice.
	sinks []extractors.Extractor
	// sources names every source the store reads the session ID from, as
	// sharesSource compares them.
	sources []sourceID
	// root identifies the extractor that sinks and sources were worked out from.
	root extractorRoot
	// ready is set by profileOf: the zero value of a Store built without NewStore is
	// not a profile of an extractor that has no sinks.
	ready bool
}

// profileOf works out the profile of ext.
func profileOf(ext extractors.Extractor) extractorProfile {
	return extractorProfile{
		root:    rootOf(ext),
		sinks:   collectSinks(ext),
		sources: collectSources(ext),
		ready:   true,
	}
}

// current reports whether the profile is that of ext: it was worked out, and ext is
// the extractor it was worked out from.
func (p *extractorProfile) current(ext extractors.Extractor) bool {
	return p.ready && p.root.matches(ext)
}

// writableSinks returns the cookies and headers a session ID of s can be written
// back to. The slice is shared: do not modify it.
func (s *Store) writableSinks() []extractors.Extractor {
	if s.profile.current(s.Extractor) {
		return s.profile.sinks
	}
	return collectSinks(s.Extractor)
}

// sourceList returns the sources s reads its session ID from. The slice is shared:
// do not modify it.
func (s *Store) sourceList() []sourceID {
	if s.profile.current(s.Extractor) {
		return s.profile.sources
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

// sharedID returns the ID and the provenance of the session of the nearest session
// middleware, m or one it runs inside, whose store shares a source with s. A
// middleware does not cache its ID on the request: it holds the session, and what it
// holds is gone with it, which is why a Store.Get after it reads the request again.
func (m *Middleware) sharedID(s *Store) (string, extractors.Result, bool) {
	var (
		id  string
		res extractors.Result
	)
	found := m.find(func(cur *Middleware) bool {
		if cur.Session == nil || !s.sharesSource(cur.store) {
			return false
		}
		id, res = cur.Session.identity()
		return true
	})
	return id, res, found != nil
}

// adoptProvenance returns res, the provenance of a session ID that another store
// cached on the request, as it holds for s. A cookie or header that is not one of
// the sinks of s is not its to write the ID back to: the ID is then treated as one
// from a source that cannot be written back to, which is what Resolved false means
// to getExtractorInfo. A read-only source, and a sink of s, stay as they are.
func (s *Store) adoptProvenance(res extractors.Result) extractors.Result {
	if res.Resolved && writesBack(res.Source) && !s.ownsSink(res) {
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
