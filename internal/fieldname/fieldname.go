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
		v := h.Peek(name)
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
	for k, v := range h.All() {
		if len(v) > 0 && utils.EqualFold(utils.UnsafeString(k), name) {
			return v
		}
	}
	return nil
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
	//nolint:staticcheck // All allocates an iterator per call; this runs per response
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

	// Restart after each removal rather than deleting while ranging over the
	// store being modified. A field arrives under one spelling essentially
	// always, so this repeats only for a peer that sent several.
	for {
		other := ""
		for k := range h.All() {
			if utils.EqualFold(utils.UnsafeString(k), name) {
				other = string(k)
				break
			}
		}
		if other == "" {
			return
		}
		h.Del(other)
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
