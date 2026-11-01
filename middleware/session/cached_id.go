package session

import (
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/utils/v2"
)

// sessionIDKey is the local key type used to store and retrieve the session ID in context.
type sessionIDKey int

const (
	// sessionIDContextKey is the key used to store the session ID in the context locals:
	// the ID of the store or middleware that cached one last, which the logger shows.
	// Nothing reads it to find a session.
	sessionIDContextKey sessionIDKey = iota
	// cachedIDsContextKey stores the IDs that Store.Get cached on the request, see cachedID.
	cachedIDsContextKey
)

// cachedID is a session ID that a store cached on the request, because it issued it
// for a visitor the request names no session of, with the provenance of the ID the
// request did name, if any, which says where the new one is written back to. A store
// that is asked again takes its ID, and finds the session it saved under it.
//
// The IDs of a request are a list, newest first, with one for each store: a store that
// caches another replaces its first. The request holds the head behind a pointer,
// since storing a Result by value would box it onto the heap, and fasthttp hands the
// list back through Close on request reset, as it does the extractors package's chain
// state.
type cachedID struct {
	next  *cachedID
	store *Store
	id    string
	res   extractors.Result
}

var cachedIDPool = sync.Pool{New: func() any { return new(cachedID) }}

// Close returns the list that starts at head to the pool. fasthttp calls it on every
// request-local io.Closer when it resets the request; callers should not.
func (head *cachedID) Close() error {
	for entry := head; entry != nil; {
		next := entry.next
		*entry = cachedID{}
		cachedIDPool.Put(entry)
		entry = next
	}
	return nil
}

// of returns the entry of s in the list that starts at head, nil if s has none.
func (head *cachedID) of(s *Store) *cachedID {
	for entry := head; entry != nil; entry = entry.next {
		if entry.store == s {
			return entry
		}
	}
	return nil
}

// requestIDs returns the head of the IDs that Store.Get cached on the request, newest
// first, nil if it cached none.
func requestIDs(c fiber.Ctx) *cachedID {
	head, ok := c.Locals(cachedIDsContextKey).(*cachedID)
	if !ok {
		return nil
	}
	return head
}

// cacheID records that s issued id for the request, as the newest of its IDs, and
// as the one the logger shows. head is the list of the request, res the provenance of
// the ID the request named, if any; its value points into the buffer of the request,
// which the application can rewrite, so the entry keeps a copy. The entry s had is
// reused as the new head, so that the list, which fasthttp reclaims from its head,
// stays in one piece: overwriting the local with an entry that does not lead to the old
// ones would strand them outside the pool.
func cacheID(c fiber.Ctx, head *cachedID, s *Store, id string, res extractors.Result) {
	res.Value = utils.CopyString(res.Value)
	var entry *cachedID
	for link := &head; *link != nil; link = &(*link).next {
		if (*link).store == s {
			entry, *link = *link, (*link).next
			break
		}
	}
	if entry == nil {
		var ok bool
		if entry, ok = cachedIDPool.Get().(*cachedID); !ok || entry == nil {
			entry = new(cachedID)
		}
	}
	entry.next, entry.store, entry.id, entry.res = head, s, id, res
	ctxlocal.Set(c, cachedIDsContextKey, entry)
	ctxlocal.Set(c, sessionIDContextKey, id)
}

// recacheID records that the session of s that had the ID old has the ID id now, which
// Regenerate and Reset give it, unless s has cached the ID of another session since: a
// session of the store that was never saved must not hide one that was.
func recacheID(c fiber.Ctx, s *Store, old, id string, res extractors.Result) {
	head := requestIDs(c)
	if entry := head.of(s); entry != nil && entry.id != old {
		return
	}
	cacheID(c, head, s, id, res)
}

// mayBeStored reports whether a session may be stored under id, the ID the request names:
// not when it names none, and not when s looked for a session under it earlier in the
// request and found none. The entry s cached then holds the ID the request named when s
// issued its own, or the one a Regenerate or Reset deleted, and neither names a session.
func (s *Store) mayBeStored(cached *cachedID, id string) bool {
	if id == "" {
		return false
	}
	entry := cached.of(s)
	return entry == nil || entry.res.Value != id
}

// cachedSession reads the session that was saved under the newest ID which s, or a
// store sharing a source with it, cached on the request, into found: first that of
// the nearest session middleware s runs inside that shares one, then those of
// Store.Get, cached, newest first. An ID that names no stored session, because it was
// never saved, is passed over, and the request is asked instead. Stores that share no
// source with s keep sessions of their own: their IDs are not taken, and they do not
// replace the ones of s.
func (s *Store) cachedSession(c fiber.Ctx, outer *Middleware, cached *cachedID, found *storedSession) error {
	if outer != nil {
		if id, res, ok := outer.sharedID(s); ok {
			if err := s.readSession(c, id, found); err != nil || found.raw != nil {
				found.res = s.adoptProvenance(res)
				return err
			}
		}
	}
	for ; cached != nil; cached = cached.next {
		if !s.sharesSource(cached.store) {
			continue
		}
		if err := s.readSession(c, cached.id, found); err != nil || found.raw != nil {
			found.res = s.adoptProvenance(cached.res)
			return err
		}
	}
	return nil
}
