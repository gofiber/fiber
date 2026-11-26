// Package personalheader names the response headers that belong to the one client a
// response is for. A middleware that writes such a header, as the session middleware
// writes the session ID into one, marks it on the request; a middleware that keeps
// responses to give them to other clients, as the cache does, leaves it out. A stored
// copy that kept it would hand the session of the client that caused the miss to every
// client that hits the entry, which is why Set-Cookie is never stored either.
package personalheader

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Names is a set of header names. It is built once, shared and never changed, so a
// request marks it without allocating. The nil set is empty.
type Names struct {
	names []string
}

// New returns the set of names, or nil when none is left once duplicates are dropped
// (header names are equal regardless of case) and the empty name is left out.
func New(names ...string) *Names {
	var kept []string
	for _, name := range names {
		if name != "" && !contains(kept, name) {
			kept = append(kept, name)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return &Names{names: kept}
}

// Has reports whether name, in any case, is in the set.
func (n *Names) Has(name []byte) bool {
	if n == nil {
		return false
	}
	return contains(n.names, utils.UnsafeString(name))
}

// covers reports whether every name of other is in n. Neither is nil.
func (n *Names) covers(other *Names) bool {
	for _, name := range other.names {
		if !contains(n.names, name) {
			return false
		}
	}
	return true
}

// contains reports whether names holds name, in any case.
func contains(names []string, name string) bool {
	for _, have := range names {
		if utils.EqualFold(have, name) {
			return true
		}
	}
	return false
}

// key is the request value the marked names are stored under, which is where Fiber
// keeps its locals: they are gone when the request is.
type key struct{}

// Mark records that the response to the request of rctx may carry the headers of
// names for the client that sent it. Marking twice, or several sets, adds up. The
// first set is stored as it is, so the common case takes no allocation.
func Mark(rctx *fasthttp.RequestCtx, names *Names) {
	if rctx == nil || names == nil {
		return
	}
	marked := Marked(rctx)
	switch {
	case marked == nil:
		rctx.SetUserValue(key{}, names)
	case !marked.covers(names):
		rctx.SetUserValue(key{}, New(append(append([]string(nil), marked.names...), names.names...)...))
	default:
	}
}

// responseKey is the request value MarkResponse sets.
type responseKey struct{}

// MarkResponse records that the response to the request of rctx is for the client that sent
// it as a whole, as one that sets a cookie is, although it may set none.
func MarkResponse(rctx *fasthttp.RequestCtx) {
	if rctx != nil {
		rctx.SetUserValue(responseKey{}, true)
	}
}

// ResponseMarked reports whether MarkResponse was called for the request of rctx.
func ResponseMarked(rctx *fasthttp.RequestCtx) bool {
	if rctx == nil {
		return false
	}
	marked, ok := rctx.UserValue(responseKey{}).(bool)
	return ok && marked
}

// Marked returns the names that were marked for the request of rctx, or nil.
func Marked(rctx *fasthttp.RequestCtx) *Names {
	if rctx == nil {
		return nil
	}
	names, ok := rctx.UserValue(key{}).(*Names)
	if !ok {
		return nil
	}
	return names
}
