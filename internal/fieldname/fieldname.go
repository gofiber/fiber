// Package fieldname reads and removes HTTP header fields by name
// case-insensitively (RFC 9110 §5.1).
//
// fasthttp's Peek, PeekAll and Del are byte-exact, which finds every line only
// while it canonicalizes the key; under DisableHeaderNormalizing the store keeps
// the spelling the peer sent, which for HTTP/2 and 3 is lower case.
//
// canonical says whether the caller knows the keys to be canonical — fasthttp
// keeps that private, so it is passed in; see headerlookup.Canonical.
package fieldname

import (
	"iter"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Peeker is the part of fasthttp's headers needed to read field lines by name.
type Peeker interface {
	Peek(key string) []byte
	PeekAll(key string) [][]byte
	All() iter.Seq2[[]byte, []byte]
	// VisitAll is deprecated in fasthttp in favor of All, but All builds an
	// iterator per call while this walks the store directly. Kept for the reads
	// that run per request and do not need to stop early.
	VisitAll(f func(key, value []byte))
}

// Deleter is the part of fasthttp's headers needed to remove a field by name.
type Deleter interface {
	Del(key string)
	PeekAll(key string) [][]byte
	All() iter.Seq2[[]byte, []byte]
}

// Lines returns every field line stored under name. The walk replaces the
// byte-exact lookup rather than backing it up: a message can carry both
// spellings, and an empty canonical line would hide the value beside it.
//
//nolint:revive // flag-parameter: canonical is a property of the header store, not a mode of operation
func Lines(h Peeker, name string, canonical bool) [][]byte {
	if canonical {
		// PeekAll hands back the header's own slice without allocating.
		return h.PeekAll(name)
	}

	// Walked rather than ranged: this runs per key header on every cacheable
	// request, and All's iterator costs more than the match itself does.
	var values [][]byte
	h.VisitAll(func(k, v []byte) {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			values = append(values, v)
		}
	})
	return values
}

// First returns the first field line stored under name that holds anything, or
// nil if there is none. The names fasthttp keeps in a slot of their own are
// unaffected either way: every spelling is routed into the slot on the way in.
//
// Empty lines are stepped over rather than answered with, because a message can
// carry the name more than once and Peek reports the first line whether or not
// it holds a value: an empty "Cache-Control:" ahead of "Cache-Control: private"
// read as a response saying nothing about caching. A field line that is present
// and empty says nothing a caller can act on, so nothing is lost by looking past
// it for one that does.
//
// Repetition itself is left to the caller to judge, because what it means
// depends on the field: a response may carry Cache-Control twice and mean the
// list, while a second Origin or Authorization line is malformed and cannot be
// resolved by picking one. headerlookup.Value refuses that case for the request
// fields where it arises.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func First(h Peeker, name string, canonical bool) []byte {
	if canonical {
		v := Peek(h, name)
		if len(v) > 0 {
			return v
		}
		if v == nil {
			// Nothing is stored under the name, so there is nothing beside it
			// to find and the walk below is skipped: a nil answer means absent,
			// a non-nil empty one means a line that is present and empty. That
			// distinction is fasthttp's to keep, so Test_First_EmptyLineIsNotNil
			// pins it — if it ever stops holding, that test says so rather than
			// this quietly reverting to reading only the first line.
			return nil
		}

		// Reached only for a field that is present and empty, which is the shape
		// being guarded against rather than one anything sends by accident.
		// PeekAll costs no allocation and resolves the slotted names.
		for _, v := range h.PeekAll(name) {
			if len(v) > 0 {
				return v
			}
		}
		return nil
	}

	// Split out rather than written below: All's iterator is heap-allocated
	// where the compiler cannot see the branch is dead, so holding it in this
	// function cost the canonical path an allocation it never used.
	return firstFold(h, name)
}

// firstFold answers First for a store whose field names are spelled however the
// peer sent them.
func firstFold(h Peeker, name string) []byte {
	// Walked rather than ranged, for the reason Lines gives: All builds a
	// heap-allocated iterator per call. Scanning past the match costs less than
	// the early exit ranging would buy.
	var found []byte
	h.VisitAll(func(k, v []byte) {
		if found == nil && len(v) > 0 && utils.EqualFold(utils.UnsafeString(k), name) {
			found = v
		}
	})
	return found
}

// Canonical reports whether every field name in h is spelled the way fasthttp
// normalizes it, so the byte-exact Peek and Del find them all.
//
// Ask this of a response rather than reading the app config: a proxied response
// is parsed by an outbound fasthttp.Client carrying its own normalizing setting,
// so a default-normalizing app can hold a response of lower-case names. It is
// one walk, against one per field for the case-insensitive reads it saves.
func Canonical(h *fasthttp.ResponseHeader) bool {
	canonical := true
	// Walked rather than ranged, for the reason Lines gives: All allocates an
	// iterator per call, and this runs per response. The deprecation is excluded
	// in .golangci.yml rather than by an inline directive, which the lint cache
	// leaves reported as unused on some runs.
	h.VisitAll(func(k, _ []byte) {
		if canonical && !isCanonicalName(k) {
			canonical = false
		}
	})
	return canonical
}

// isCanonicalName reports whether name is upper case at the start of each
// "-"-separated token and lower case everywhere else, which is what fasthttp's
// normalization produces.
func isCanonicalName(name []byte) bool {
	upper := true
	for _, c := range name {
		if upper {
			if c >= 'a' && c <= 'z' {
				return false
			}
		} else if c >= 'A' && c <= 'Z' {
			return false
		}
		upper = c == '-'
	}
	return true
}

// Del removes every field line named name, whatever case it is stored under.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func Del(h Deleter, name string, canonical bool) {
	h.Del(name)
	if canonical {
		// The scan below would only confirm Del found everything, at the cost
		// of an iterator allocation per name on an otherwise alloc-free path.
		return
	}

	// Collected before deleting, like DelOthers, rather than rescanning after
	// each removal: ResponseHeader.All reports a default Content-Type whenever
	// the slot is empty, so a rescan would keep finding the name it just
	// deleted. A name stored non-canonically in a normalizing store is the same
	// shape, since Del normalizes the key before matching and so cannot remove
	// it at all.
	var others []string
	for k := range h.All() {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			others = append(others, string(k))
		}
	}

	for _, k := range others {
		h.Del(k)
	}
}

// DelOthers removes every stored spelling of name except name itself, which the
// caller is about to write or delete under that key. Every other one, not just
// the first: stopping early leaves the rest beside the value the caller writes.
func DelOthers(h Deleter, name string) {
	// Collected before deleting: removing while ranging over the store is not safe.
	var others []string
	for k := range h.All() {
		if ks := utils.UnsafeString(k); ks != name && utils.EqualFold(ks, name) {
			others = append(others, string(k))
		}
	}

	for _, k := range others {
		h.Del(k)
	}
}

// DelSet removes every field line matching any of names in a single pass, which
// beats Del's per-name scan when the whole set is known up front.
func DelSet(h Deleter, names []string) {
	var removed []string
	for k := range h.All() {
		if ContainsFold(names, utils.UnsafeString(k)) {
			removed = append(removed, string(k))
		}
	}

	// A repeated name is deleted twice; Del removes every line at once, so
	// scanning to avoid that costs more than making the call.
	for _, name := range removed {
		h.Del(name)
	}
}

// ContainsFold reports whether needle equals any of haystack, case-insensitively.
func ContainsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if utils.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// Byte classes of a field name, for IsCanonical.
const (
	keyOther   byte = iota // a token byte that is not a letter
	keyLower               // a-z
	keyUpper               // A-Z
	keyInvalid             // not a token byte (RFC 9110 Section 5.6.2)
)

// keyClass classifies every byte a field name can hold. The token set is the
// one fasthttp normalizes; any other byte makes it store the name as sent.
var keyClass = func() [256]byte {
	var t [256]byte
	for i := range t {
		t[i] = keyInvalid
	}
	for _, c := range []byte("!#$%&'*+-.^_`|~0123456789") {
		t[c] = keyOther
	}
	for c := byte('a'); c <= 'z'; c++ {
		t[c] = keyLower
		t[c-'a'+'A'] = keyUpper
	}
	return t
}()

// IsCanonical reports whether name is already in fasthttp's canonical form: a
// token with an upper-case letter first and after each '-' and lower case
// elsewhere. fasthttp derives that form again on every keyed call, three
// passes and a copy, only to arrive at the same bytes; a name that already has
// it can take the *Canonical methods instead, which store and find exactly what
// the normalizing ones would whether or not the store normalizes.
// Test_IsCanonical_MatchesFasthttp keeps the form in step.
func IsCanonical(name string) bool {
	n := len(name)
	if n == 0 {
		return false
	}
	// Names such as X-Request-ID fail only at their last byte, so the tail is
	// checked first: an upper-case letter not following a '-' cannot end a
	// canonical name.
	if n > 1 && name[n-1] >= 'A' && name[n-1] <= 'Z' && name[n-2] != '-' {
		return false
	}
	upper := true
	for i := range n {
		c := name[i]
		switch keyClass[c] {
		case keyInvalid:
			return false
		case keyLower:
			if upper {
				return false
			}
		case keyUpper:
			if !upper {
				return false
			}
		}
		upper = c == '-'
	}
	return true
}

// Peek is fasthttp's byte-exact Peek, minus the key normalization when name is
// already canonical; Test_Peek_MatchesFasthttp keeps the two answering alike in
// both store modes.
func Peek(h Peeker, name string) []byte {
	if IsCanonical(name) {
		// Concrete calls only: through the interface the key would escape.
		switch h := h.(type) {
		case *fasthttp.RequestHeader:
			return h.PeekCanonical(utils.UnsafeBytes(name))
		case *fasthttp.ResponseHeader:
			return h.PeekCanonical(utils.UnsafeBytes(name))
		}
	}
	return h.Peek(name)
}
