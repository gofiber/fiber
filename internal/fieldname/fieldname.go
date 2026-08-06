// Package fieldname reads and removes HTTP header fields by name the way a
// recipient must: case-insensitively (RFC 9110 Section 5.1).
//
// fasthttp's Peek, PeekAll and Del compare the stored key byte for byte. That
// finds every line while fasthttp canonicalizes the key, which is the default,
// but under DisableHeaderNormalizing the store keeps the spelling the peer
// sent — and lower case is not exotic there: HTTP/2 and HTTP/3 require it on
// the wire, so it is exactly what a front end translating down to HTTP/1.1
// preserves. A lookup written byte-for-byte then finds nothing, and a header
// nobody can find is a header that is not there.
//
// Every function here takes a canonical flag saying whether the caller knows
// the stored keys to be canonical. When it is true they do exactly what the
// fasthttp call does and cost exactly the same; the walk exists only for the
// case where it does not hold. The flag is passed rather than asked because
// fasthttp keeps the answer private — see headerlookup.Canonical for where a
// caller holding a fiber.Ctx gets it.
//
// This package takes fasthttp header types rather than a fiber.Ctx so that
// package fiber can use it too. The copy that was not written is the one that
// stayed broken.
package fieldname

import (
	"iter"

	"github.com/gofiber/utils/v2"
)

// Peeker is the part of fasthttp's request and response headers needed to read
// the field lines stored under a name.
type Peeker interface {
	Peek(key string) []byte
	PeekAll(key string) [][]byte
	All() iter.Seq2[[]byte, []byte]
}

// Deleter is the part of fasthttp's request and response headers needed to find
// a field by name and take it away.
type Deleter interface {
	Del(key string)
	PeekAll(key string) [][]byte
	All() iter.Seq2[[]byte, []byte]
}

// Lines returns every field line stored under name.
//
// The walk replaces the byte-for-byte lookup rather than backing it up: a
// message can carry both spellings, and an empty canonical line would satisfy a
// fallback-only fast path while hiding the value on the line beside it. That is
// not hypothetical — it is how an empty "Authorization:" hid the credential on
// the "authorization:" line next to it.
//
//nolint:revive // flag-parameter: canonical is a property of the header store that only the caller can know, not a mode of operation
func Lines(h Peeker, name string, canonical bool) [][]byte {
	if canonical {
		// Every stored key is canonical, so a byte-for-byte lookup finds all of
		// them and PeekAll hands back the header's own slice without
		// allocating.
		return h.PeekAll(name)
	}

	var values [][]byte
	for k, v := range h.All() {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			values = append(values, v)
		}
	}
	return values
}

// First returns the first field line stored under name, or nil if there is
// none.
//
// The names fasthttp keeps in a slot of their own — Content-Type,
// Content-Encoding, Server, Set-Cookie, Trailer, Content-Length — are not
// affected either way. Peek answers those from the slot, and every spelling is
// routed into it on the way in, so they need no help here.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func First(h Peeker, name string, canonical bool) []byte {
	if canonical {
		return h.Peek(name)
	}

	for k, v := range h.All() {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			return v
		}
	}
	return nil
}

// Del removes every field line named name, whatever case it is stored under.
//
// Where the caller is a sanitizer, this is the only thing standing between a
// peer and a header it must not have, so a miss here is not a lost value but a
// leaked one.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func Del(h Deleter, name string, canonical bool) {
	h.Del(name)
	if canonical {
		// The scan below would only confirm that Del had already found
		// everything, at the cost of an iterator allocation per name on a path
		// that otherwise allocates nothing.
		return
	}

	// Restart the scan after each removal rather than deleting while ranging
	// over the store being modified. A field arrives under one spelling
	// essentially always, so this repeats only for a peer that went out of its
	// way to send several.
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
// caller is about to write or delete under that exact key.
//
// Every other one, not just the first found. A message can carry more than two
// — one from an outer middleware, one from the handler — and stopping at the
// first leaves the rest beside the value written there, which is the pair the
// caller is trying to prevent.
//
// Collected before anything is deleted: Del is byte-exact too, and removing an
// entry while ranging over the store it belongs to is not safe.
func DelOthers(h Deleter, name string) {
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

// DelSet removes every field line whose name matches any of names, whatever
// case each is stored under, in a single pass.
//
// One pass costs the same whether or not any spelling turned out to be unusual,
// and is cheaper than the per-name scan Del avoids, since that one repeats the
// walk for each name. Use this where the whole set is known up front.
//
// Keys are collected before anything is deleted, since fasthttp's iterator
// walks the storage the deletes rewrite. A repeated name is collected and
// deleted repeatedly; Del removes every line at once so the second call finds
// nothing, and scanning to avoid it costs more than making it.
func DelSet(h Deleter, names []string) {
	var removed []string
	for k := range h.All() {
		if ContainsFold(names, utils.UnsafeString(k)) {
			removed = append(removed, string(k))
		}
	}

	for _, name := range removed {
		h.Del(name)
	}
}

// ContainsFold reports whether needle equals any of haystack under
// case-insensitive comparison.
func ContainsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if utils.EqualFold(h, needle) {
			return true
		}
	}
	return false
}
