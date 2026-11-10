package session

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	internalcookie "github.com/gofiber/fiber/v3/internal/cookie"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Session represents a user session.
// Session serializes access to its internal state with mutexes, but it is
// request-scoped and must not be used after the request lifecycle ends.
type Session struct {
	ctx fiber.Ctx // fiber context of the request; read it with requestCtx, which leaveCtx makes answer nil
	// rctx is the request itself, for a session a panic unwound the middleware of. It
	// still writes its cookies there, for whatever recovers the panic, while the handlers
	// of the request run, which is as long as ctx serves that request: the fiber.Ctx goes
	// back to the pool when they return, and serves another request then. Set and cleared
	// by leaveCtx, under reqMu, with rconn and rnum.
	rctx        *fasthttp.RequestCtx
	config      *Store                  // store configuration
	data        *data                   // key value data
	rawData     []byte                  // snapshot from the last decode, reused by Save while the data is unchanged
	id          string                  // session id
	key         storageKey              // storage key of id, built on first use
	unreadable  storageKey              // key of the record the request named and the store cannot read: Destroy and Reset delete it too
	extractor   extractors.Result       // provenance of the extractor that supplied the session ID
	sink        [1]extractors.Extractor // backs the one-element result of getExtractorInfo
	idleTimeout time.Duration           // idleTimeout of this session
	loadGen     uint64                  // idLock generation when the data was read from storage
	rconn       uint64                  // connection of the request rctx served when the session was told of it, and
	rnum        uint64                  // its number on that connection: rctx serves the requests that follow as well
	mu          sync.RWMutex            // Mutex to protect non-data fields
	reqMu       sync.Mutex              // held while the session touches its request, and by what ends the request for it (leaveCtx); taken after mu
	isFresh     bool                    // if new session
	loaded      bool                    // the data was read from storage under the current id
	dropped     bool                    // another request ended or rotated it: never stored or sent again
	requestOver bool                    // the request is over for the session, which touches none any more; under reqMu
}

type absExpirationKeyType int

const (
	// sessionIDContextKey is the key used to store the session ID in the context locals.
	absExpirationKey absExpirationKeyType = iota
)

var sessionPool = sync.Pool{
	New: func() any {
		return &Session{}
	},
}

// acquireSession returns a new Session from the pool.
//
// Returns:
//   - *Session: The session object.
//
// Usage:
//
//	s := acquireSession()
func acquireSession() *Session {
	s := sessionPool.Get().(*Session) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
	if s.data == nil {
		s.data = acquireData()
	}
	s.isFresh = true
	return s
}

// Release releases the session back to the pool.
//
// This function should be called after the session is no longer needed.
// This function is used to reduce the number of allocations and
// to improve the performance of the session store.
//
// The session should not be used after calling this function.
//
// Important: The Release function should only be used when accessing the session directly,
// for example, when you have called func (s *Session) Get(ctx) to get the session.
// It should not be used when using the session with a *Middleware handler in the request
// call stack, as the middleware will still need to access the session.
//
// Usage:
//
//	sess := session.Get(ctx)
//	defer sess.Release()
func (s *Session) Release() {
	if s == nil {
		return
	}
	releaseSession(s)
}

func releaseSession(s *Session) {
	s.mu.Lock()
	s.id = ""
	s.key = ""
	s.unreadable = ""
	s.idleTimeout = 0
	s.ctx = nil
	s.reqMu.Lock()
	s.rctx, s.requestOver = nil, false
	s.reqMu.Unlock()
	s.config = nil
	s.extractor = extractors.Result{}
	s.rawData = nil
	s.loaded = false
	s.dropped = false
	if s.data != nil {
		s.data.reset(gobStream)
	}
	s.mu.Unlock()
	sessionPool.Put(s)
}

// Fresh returns whether the session is new
//
// Returns:
//   - bool: True if the session is fresh; otherwise, false.
//
// Usage:
//
//	isFresh := s.Fresh()
func (s *Session) Fresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isFresh
}

// ID returns the session ID
//
// Returns:
//   - string: The session ID.
//
// Usage:
//
//	id := s.ID()
func (s *Session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// identity returns the ID of the session and the provenance of the ID the request
// named when it was loaded.
func (s *Session) identity() sessionIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identityLocked()
}

// identityLocked is identity for a caller that holds the lock of the session, or holds
// the only reference to it.
func (s *Session) identityLocked() sessionIdentity {
	return sessionIdentity{id: s.id, res: s.extractor}
}

// Set updates or creates a new key-value pair in the session.
//
// Parameters:
//   - key: The key to set.
//   - val: The value to set.
//
// Usage:
//
//	s.Set("key", "value")
func (s *Session) Set(key, val any) {
	if s.data == nil {
		return
	}
	s.data.Set(key, val)
}

// Delete removes the key-value pair from the session.
//
// Parameters:
//   - key: The key to delete.
//
// Usage:
//
//	s.Delete("key")
func (s *Session) Delete(key any) {
	if s.data == nil {
		return
	}
	s.data.Delete(key)
}

// Destroy deletes the session from storage and expires the session cookie.
//
// Returns:
//   - error: An error if the destruction fails.
//
// Usage:
//
//	err := s.Destroy()
func (s *Session) Destroy() error {
	return s.DestroyWithContext(s.resolveContext())
}

// DestroyWithContext deletes the session from storage and expires the session cookie,
// using the provided context for cancellation and timeout control.
//
// Parameters:
//   - ctx: The context to use for the storage operation.
//
// Returns:
//   - error: An error if the destruction fails.
//
// Usage:
//
//	err := s.DestroyWithContext(ctx)
func (s *Session) DestroyWithContext(ctx context.Context) error {
	ctx = backgroundIfNil(ctx)

	if s.data == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Use external Storage if exist
	if err := s.config.deleteKey(ctx, s.storageKey()); err != nil {
		return err
	}
	if err := s.deleteUnreadable(ctx); err != nil {
		return err
	}

	// Reset local data only after the storage delete succeeded, so a
	// canceled/failed delete leaves the session data intact.
	s.resetData()
	s.loaded = false

	// Expire session
	s.delSession()
	return nil
}

// Regenerate generates a new session id and deletes the old one from storage.
//
// Returns:
//   - error: An error if the regeneration fails.
//
// Usage:
//
//	err := s.Regenerate()
func (s *Session) Regenerate() error {
	return s.RegenerateWithContext(s.resolveContext())
}

// RegenerateWithContext generates a new session id and deletes the old one from storage,
// using the provided context for cancellation and timeout control.
//
// Like Save, it tells the request of the session about the new ID, so a store that is
// asked again finds the session under it: call it from the goroutine that serves the request.
//
// Parameters:
//   - ctx: The context to use for the storage operation.
//
// Returns:
//   - error: An error if the regeneration fails.
//
// Usage:
//
//	err := s.RegenerateWithContext(ctx)
func (s *Session) RegenerateWithContext(ctx context.Context) error {
	ctx = backgroundIfNil(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete old id from storage
	var (
		stored []byte
		ended  bool
		err    error
	)
	if s.loaded {
		stored, ended, err = s.config.deleteLoadedKey(ctx, s.storageKey(), s.loadGen)
	} else {
		err = s.config.deleteKey(ctx, s.storageKey())
	}
	if err != nil {
		return err
	}
	// Another request ended or rotated it: still rotate, as another instance may write
	// the old ID back, but store and send nothing.
	if ended {
		s.dropped = true
	}
	// Another request changed it since the load and this one changed nothing, in place
	// included: the rotated session carries that change, not the snapshot this request loaded.
	if stored != nil && s.overtakenBy(stored) {
		// Rotated even when the decode fails: the old key is already gone, so the save
		// stores this request's snapshot, which a failed decode leaves as it was, under
		// the new ID rather than nothing.
		err = s.decodeSessionData(stored)
	}

	// Generate a new session, and set session.isFresh to true
	s.refresh()

	return err
}

// Reset generates a new session id, deletes the old one from storage, and resets the associated data.
//
// Returns:
//   - error: An error if the reset fails.
//
// Usage:
//
//	err := s.Reset()
func (s *Session) Reset() error {
	return s.ResetWithContext(s.resolveContext())
}

// ResetWithContext generates a new session id, deletes the old one from storage, and resets the associated data,
// using the provided context for cancellation and timeout control.
//
// Like Save, it tells the request of the session about the new ID, so a store that is
// asked again finds the session under it: call it from the goroutine that serves the request.
//
// Parameters:
//   - ctx: The context to use for the storage operation.
//
// Returns:
//   - error: An error if the reset fails.
//
// Usage:
//
//	err := s.ResetWithContext(ctx)
func (s *Session) ResetWithContext(ctx context.Context) error {
	ctx = backgroundIfNil(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete old id from storage
	if err := s.config.deleteKey(ctx, s.storageKey()); err != nil {
		return err
	}
	if err := s.deleteUnreadable(ctx); err != nil {
		return err
	}

	// Reset local state only after the storage delete succeeded, so a
	// canceled/failed delete leaves the session data intact.
	if s.data != nil {
		s.resetData()
		// Reset wiped the absolute expiration, so the rotated session must be armed again.
		if s.config != nil && s.config.AbsoluteTimeout > 0 {
			s.setAbsExpiration(time.Now().Add(s.config.AbsoluteTimeout))
		}
	}
	s.idleTimeout = 0

	// Expire session
	s.delSession()

	// Generate a new session, and set session.isFresh to true
	s.refresh()

	return nil
}

// deleteUnreadable deletes the record the request named and the store cannot read, if
// there is one. The session took its place, so ending the session ends that one too: a
// logout holds on whichever instance it lands, and not only on one that reads the record.
// A request that does not end its session leaves the record alone, for the instance that
// reads it. The caller holds s.mu.
func (s *Session) deleteUnreadable(ctx context.Context) error {
	if s.unreadable == "" {
		return nil
	}
	if err := s.config.deleteKey(ctx, s.unreadable); err != nil {
		return err
	}
	s.unreadable = ""
	return nil
}

// resetData empties the data. A session of a store with a codec starts over as a
// session of the store, which one that was stored as gob did not belong to.
func (s *Session) resetData() {
	s.data.reset(s.config.sessionFormat())
	s.rawData = nil
}

// refresh generates a new session, and sets session.isFresh to be true.
func (s *Session) refresh() {
	old := s.id
	s.id = s.config.KeyGenerator()
	s.key = ""
	s.isFresh = true
	s.loaded = false
	s.recache(old)
}

// recache tells the request of the session that its ID is no longer old: a store that is
// asked again in this request finds the session under its new ID, not under the one that
// was just deleted. Like a Save, this uses the request of the session, so Regenerate and
// Reset belong to the goroutine that serves it. It goes through the fiber.Ctx only: once
// that is no longer the session's to use, no store asks the request for the session
// again. The caller holds s.mu.
func (s *Session) recache(old string) {
	s.reqMu.Lock()
	defer s.reqMu.Unlock()
	if s.ctx == nil || s.rctx != nil || s.requestOver {
		return
	}
	recacheID(s.ctx, s.config, old, s.identityLocked())
}

// storageKey returns the key the session is stored under; the caller holds s.mu.
func (s *Session) storageKey() storageKey {
	if s.key == "" {
		s.key = s.config.storageKey(s.id)
	}
	return s.key
}

// Save saves the session data and updates the cookie
//
// Note: If the session is being used in the handler, calling Save will have
// no effect and the session will automatically be saved when the handler returns.
//
// Returns:
//   - error: An error if the save operation fails.
//
// Usage:
//
//	err := s.Save()
func (s *Session) Save() error {
	c := s.requestCtx()
	return s.saveUnlessManaged(c, c)
}

// SaveWithContext saves the session data and updates the cookie,
// using the provided context for cancellation and timeout control.
//
// Note: If the session is being used in the handler, calling SaveWithContext will have
// no effect and the session will automatically be saved when the handler returns.
// To tell, it takes the read lock of the session middleware, which is held while
// that middleware destroys, resets or regenerates its session, and the lock of the
// session, to read its context, which is held while it saves: do not call it from
// a Storage then.
//
// Parameters:
//   - ctx: The context to use for the storage operation.
//
// Returns:
//   - error: An error if the save operation fails.
//
// Usage:
//
//	err := s.SaveWithContext(ctx)
func (s *Session) SaveWithContext(ctx context.Context) error {
	return s.saveUnlessManaged(ctx, s.requestCtx())
}

// saveUnlessManaged saves the session through ctx, a background context when it is nil,
// unless a session middleware manages it and will save it when its handler returns. c is
// the request the session belongs to, nil if it has none, and is read once by the callers.
func (s *Session) saveUnlessManaged(ctx context.Context, c fiber.Ctx) error {
	if c != nil {
		// If the session is being used in the handler, it should not be saved. With
		// several session middleware registered, the one managing s may be an outer one.
		if current := currentMiddleware(c); current != nil &&
			current.find(func(m *Middleware) bool { return m.Session == s }) != nil {
			// Session is in use, so we do nothing and return
			return nil
		}
	}

	return s.saveSessionWithContext(backgroundIfNil(ctx))
}

// saveSessionWithContext encodes session data and saves it to storage using the provided context.
func (s *Session) saveSessionWithContext(ctx context.Context) error {
	if s.data == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dropped {
		return nil
	}

	// Set idleTimeout if not already set
	if s.idleTimeout <= 0 {
		s.idleTimeout = s.config.IdleTimeout
	}

	// Neither the record nor the cookie may outlive the absolute deadline.
	ttl := s.lifetime()

	// Unchanged since the decode: write the snapshot back so the idle timeout still
	// slides. Lock-free is enough; the flags are set before what they report.
	data := s.rawData
	// Set, Delete or Reset changed the session. A read of a value the caller may change
	// in place only makes the snapshot stale: the data is encoded again, and whether it
	// changed is asked only once another request's change is there to yield to.
	mutated := s.data.mutated.Load()
	changed := data == nil || mutated || s.data.dirty.Load()
	if changed {
		s.data.RLock()
		encoded, err := s.data.encode()
		s.data.RUnlock()
		if err != nil {
			return fmt.Errorf("failed to encode data: %w", err)
		}
		data = encoded
	}

	key := s.storageKey()
	if s.loaded {
		l := key.lock()
		if changed {
			// Held alone, so no write-back of an older snapshot lands after this change.
			l.mu.Lock()
			defer l.mu.Unlock()
		} else {
			l.mu.RLock()
			defer l.mu.RUnlock()
		}
		raw, moved, err := s.config.storedSince(ctx, key, s.loadGen)
		if err != nil {
			return err
		}
		if moved {
			if raw == nil {
				return nil // ended by another request: neither store it nor send its cookie again
			}
			if s.overtakenBy(raw) {
				s.setSession(ttl)
				return nil // changed by another request: a snapshot this one did not change must not undo that
			}
		}
		if changed {
			defer l.gen.Add(1) // before the unlock: the shard changed, which a save that stored nothing leaves as it was
		}
	}

	// Update client cookie
	s.setSession(ttl)
	return s.config.Storage.SetWithContext(ctx, string(key), data, ttl)
}

// overtakenBy reports whether raw, what the storage holds now, is a change another request
// stored since the load that this one must not undo: one that neither called Set, Delete or
// Reset nor changed a value Get returned in place. Such an edit counts as the request's own
// change, as a Set does; it is told from a read by decoding the snapshot again, which only the
// rare save that a concurrent change reaches pays for.
func (s *Session) overtakenBy(raw []byte) bool {
	if s.data.mutated.Load() || bytes.Equal(raw, s.rawData) {
		return false
	}
	return !s.data.dirty.Load() || !s.data.editedSince(s.rawData)
}

// Keys retrieves all keys in the current session.
//
// Returns:
//   - []any: A slice of all keys in the session.
//
// Note: a key that can alias the session has the same effect as in Get.
//
// Usage:
//
//	keys := s.Keys()
func (s *Session) Keys() []any {
	if s.data == nil {
		return []any{}
	}
	return s.data.Keys()
}

// SetIdleTimeout used when saving the session on the next call to `Save()`.
//
// Parameters:
//   - idleTimeout: The duration for the idle timeout.
//
// Usage:
//
//	s.SetIdleTimeout(time.Hour)
func (s *Session) SetIdleTimeout(idleTimeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idleTimeout = idleTimeout
}

// getExtractorInfo returns the cookie and header extractors the session ID is
// written to. The slice is only good until the next call, and must not be
// modified: it is the store's own, or backed by the session.
func (s *Session) getExtractorInfo() []extractors.Extractor {
	if s.config == nil {
		return []extractors.Extractor{{Source: extractors.SourceCookie, Key: "session_id"}} // Safe default
	}

	// Prefer the extractor that actually supplied the incoming session ID.
	//
	// Gated on Value, not Key: Key is the sink's name, and a keyless extractor
	// — a hand-rolled child, or FromCustom("") — would otherwise skip this
	// guard entirely and let a read-only ID reach the sinks below.
	if s.extractor.Value != "" {
		// Only observed provenance names a sink. A chain whose own Extract
		// answered without a child winning reports its first child's declared
		// metadata, which says nothing about where the value came from.
		// Key must name a sink for it to be writable at all.
		if s.extractor.Resolved && s.extractor.Key != "" && writesBack(s.extractor.Source) {
			// The ID came from a writable sink; write it back to the same place. Any
			// other source is read-only: fall through to the refusal below.
			s.sink[0] = extractors.Extractor{Key: s.extractor.Key, Source: s.extractor.Source}
			return s.sink[:]
		}
		// Read-only, or an origin we cannot attribute. For an existing session
		// either would be an attacker-controlled ID, so it must not be promoted
		// into cookies/headers (session fixation). A fresh session's ID is
		// server-generated, so it may still fall through to the sinks below.
		if !s.isFresh {
			return nil
		}
	}

	return s.config.places().sinks
}

// setSession writes the session ID to the response, with a lifetime of ttl.
func (s *Session) setSession(ttl time.Duration) {
	s.reqMu.Lock()
	defer s.reqMu.Unlock()
	_, resp := s.requestResponse()
	if resp == nil {
		return
	}

	// Get all relevant extractors
	relevantExtractors := s.getExtractorInfo()

	// Set session ID for each extractor type
	for _, ext := range relevantExtractors {
		switch ext.Source {
		case extractors.SourceHeader:
			resp.Header.SetBytesV(ext.Key, utils.UnsafeBytes(s.id))
		case extractors.SourceCookie:
			fcookie := fasthttp.AcquireCookie()

			fcookie.SetKey(ext.Key)
			fcookie.SetValue(s.id)
			fcookie.SetPath(s.config.CookiePath)
			fcookie.SetDomain(s.config.CookieDomain)
			// Cookies are also session cookies if they do not specify the Expires or Max-Age attribute.
			// refer: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
			if !s.config.CookieSessionOnly {
				fcookie.SetMaxAge(int(ttl.Seconds()))
				fcookie.SetExpire(time.Now().Add(ttl))
			}

			s.setCookieAttributes(fcookie)
			resp.Header.SetCookie(fcookie)
			fasthttp.ReleaseCookie(fcookie)
		default:
			// For non-cookie/header sources, do nothing (read-only)
		}
	}
}

func (s *Session) delSession() {
	s.reqMu.Lock()
	defer s.reqMu.Unlock()
	req, resp := s.requestResponse()
	if resp == nil {
		return
	}

	// Get all relevant extractors
	relevantExtractors := s.getExtractorInfo()

	// Delete session ID for each extractor type
	for _, ext := range relevantExtractors {
		switch ext.Source {
		case extractors.SourceHeader:
			req.Header.Del(ext.Key)
			resp.Header.Del(ext.Key)
		case extractors.SourceCookie:
			req.Header.DelCookie(ext.Key)
			resp.Header.DelCookie(ext.Key)

			fcookie := fasthttp.AcquireCookie()

			fcookie.SetKey(ext.Key)
			fcookie.SetPath(s.config.CookiePath)
			fcookie.SetDomain(s.config.CookieDomain)
			fcookie.SetMaxAge(-1)
			fcookie.SetExpire(time.Now().Add(-1 * time.Minute))

			s.setCookieAttributes(fcookie)
			resp.Header.SetCookie(fcookie)
			fasthttp.ReleaseCookie(fcookie)
		default:
			// For non-cookie/header sources, do nothing (read-only)
		}
	}
}

// setCookieAttributes sets the cookie attributes based on the session config.
func (s *Session) setCookieAttributes(fcookie *fasthttp.Cookie) {
	sameSite, _ := internalcookie.ParseSameSite(s.config.CookieSameSite)
	fcookie.SetSameSite(sameSite.FastHTTPMode)
	// The Secure attribute is required for SameSite=None; every other mode
	// preserves the configured value.
	fcookie.SetSecure(s.config.CookieSecure || sameSite.RequiresSecure)

	fcookie.SetHTTPOnly(s.config.CookieHTTPOnly)
}

// decodeSessionData replaces the data of the session with the stored session rawData and
// keeps the snapshot it was decoded from, which a save writes back while the data is
// unchanged. It takes the lock of the data. Data that holds keys is left as it was when the
// decode fails.
//
// Parameters:
//   - rawData: The raw byte data to decode.
//
// Returns:
//   - error: An error if the decoding fails.
//
// Usage:
//
//	err := s.decodeSessionData(rawData)
func (s *Session) decodeSessionData(rawData []byte) error {
	s.data.Lock()
	defer s.data.Unlock()
	snapshot, err := s.data.decode(rawData)
	if err != nil {
		return err
	}
	s.rawData = snapshot
	return nil
}

// isEmptyFresh reports whether the session is new and holds nothing a handler
// stored in it. The absolute-timeout bookkeeping, which every new session of a
// store with an AbsoluteTimeout carries, does not count.
//
// Returns:
//   - bool: True if the session is fresh and has no data of its own; otherwise, false.
func (s *Session) isEmptyFresh() bool {
	s.mu.RLock()
	fresh := s.isFresh
	s.mu.RUnlock()
	return fresh && s.data != nil && s.data.isEmpty()
}

// bind makes the session one of store: its configuration, and the format its data
// holds. It is for a session that was just acquired, which nothing else refers to
// yet, so the data needs no lock.
func (s *Session) bind(store *Store) {
	s.config = store
	s.data.format = store.sessionFormat()
}

// absExpiration returns the session absolute expiration time or a zero time if not set.
//
// Returns:
//   - time.Time: The session absolute expiration time. Zero time if not set.
//
// Usage:
//
//	expiration := s.absExpiration()
func (s *Session) absExpiration() time.Time {
	if s.data == nil {
		return time.Time{}
	}
	absExpiration, ok := s.data.getRaw(absExpirationKey).(time.Time)
	if ok {
		return absExpiration
	}
	return time.Time{}
}

// isAbsExpired returns true if the session is expired.
//
// If the session has an absolute expiration time set, this function will return true if the
// current time is after the absolute expiration time.
//
// Returns:
//   - bool: True if the session is expired; otherwise, false.
func (s *Session) isAbsExpired() bool {
	absExpiration := s.absExpiration()
	return !absExpiration.IsZero() && time.Now().After(absExpiration)
}

// setAbsExpiration sets the absolute session expiration time.
//
// Parameters:
//   - expiration: The session expiration time.
//
// Usage:
//
//	s.setAbsExpiration(time.Now().Add(time.Hour))
func (s *Session) setAbsExpiration(absExpiration time.Time) {
	if s.data == nil {
		return
	}
	s.data.setRaw(absExpirationKey, absExpiration)
}

// backgroundIfNil returns ctx unchanged when it is non-nil, and
// context.Background() otherwise. It normalizes a caller-supplied context for
// the *WithContext methods so a nil context never reaches storage.
func backgroundIfNil(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// resolveContext returns the session's stored fiber context if available,
// otherwise returns context.Background().
// fiber.Ctx implements context.Context directly, so no allocation is needed.
// This is used as the default context for the non-WithContext variants of
// Destroy, Regenerate and Reset. Save reads the context itself, since it needs
// the request as well to tell whether a middleware manages the session.
func (s *Session) resolveContext() context.Context {
	if c := s.requestCtx(); c != nil {
		return c
	}
	return context.Background()
}

// Get returns the value associated with the given key.
//
// Parameters:
//   - key: The key to retrieve.
//
// Returns:
//   - any: The value associated with the key.
//
// Note: returning a value that can alias the session (map, slice, array,
// pointer, struct) makes every later Save re-encode, since the caller may
// mutate it in place without calling Set. Such an edit counts as the request's
// own change, as a Set does, so its save stands over a change another request
// stored meanwhile; a read alone yields to it.
//
// Usage:
//
//	value := s.Get("key")
func (s *Session) Get(key any) any {
	if s.data == nil {
		return nil
	}
	return s.data.Get(key)
}

// fetch returns what is stored under key, as it is held, and whether there is a value.
// See data.fetch.
func (s *Session) fetch(key any) (any, bool) {
	if s.data == nil {
		return nil, false
	}
	return s.data.fetch(key)
}

// GetAs returns the value stored under key as a V, and whether it is one: ok is
// false when the key is absent or holds a value of another type, and the zero
// value is returned with it. It spares the type assertion, and with it the
// panic of an unchecked one.
//
// src is a *Session or a *Middleware. A nil one finds nothing, so the result of
// FromContext can be passed as it is.
//
// Without a Codec the value is the one that was stored, and like Get, GetAs gives
// up the saved snapshot when it could share memory with the session (a map,
// slice, array, pointer or struct). With a Codec the value is decoded into a new
// V, ok is false when that fails, and nothing is shared: V is what the stored
// value decodes into, not a check that this is what it was stored as.
//
// Parameters:
//   - src: The session or middleware to read from.
//   - key: The key to retrieve.
//
// Returns:
//   - V: The value, or the zero value of V.
//   - bool: True if the key holds a V; otherwise, false.
//
// Usage:
//
//	name, ok := session.GetAs[string](session.FromContext(c), "name")
func GetAs[V any, S *Session | *Middleware](src S, key any) (V, bool) {
	var held any
	switch typed := any(src).(type) {
	case *Middleware:
		if typed != nil {
			held, _ = typed.fetch(key)
		}
	case *Session:
		if typed != nil {
			held, _ = typed.fetch(key)
		}
	}

	if e, ok := held.(*encodedValue); ok {
		return decodeAs[V](e)
	}
	value, ok := held.(V)
	return value, ok
}

// GetInto stores the value under key into target, a non-nil pointer, and reports
// why it could not: ErrKeyNotFound when there is no such key, in which case target
// is left as it is.
//
// With a Codec, only that value is decoded, and nothing is shared with the
// session: the other values are not touched. A target that holds its zero value,
// as a fresh variable does, is decoded into directly, so nothing is copied. One
// that holds something is replaced by a new value, so that nothing of the old value
// is merged into the result. Either way a failure leaves target as it was.
// Without a Codec, the stored value is assigned to target, which has to be able to
// hold it, and like Get, GetInto gives up the saved snapshot when the value could
// share memory with the session.
//
// Parameters:
//   - key: The key to retrieve.
//   - target: A pointer to the variable to fill.
//
// Returns:
//   - error: ErrKeyNotFound, or why the value could not be decoded or assigned.
//
// Usage:
//
//	var user User
//	err := s.GetInto("user", &user)
func (s *Session) GetInto(key, target any) error {
	return getInto(s.fetch, key, target)
}

// keyNotFound is the error of GetInto for a key that the session holds no value under.
func keyNotFound(key any) error {
	return keyNotFoundError{key: key}
}

// keyNotFoundError formats its message when it is asked for it: a missing key is an ordinary
// answer, which callers test for with errors.Is and seldom print. The key is quoted, so that
// one that came from a client cannot forge a line of a log.
type keyNotFoundError struct{ key any }

func (e keyNotFoundError) Error() string { return fmt.Sprintf("%v: %#v", ErrKeyNotFound, e.key) }
func (keyNotFoundError) Unwrap() error   { return ErrKeyNotFound }

// minLifetime is the shortest lifetime the absolute deadline cuts a save down
// to. Storages count whole seconds, and a shorter one could round down to zero,
// which many of them read as "never expires".
const minLifetime = time.Second

// lifetime returns how long a save keeps the record and the cookie: the idle
// timeout, cut down to the time left until the absolute deadline, but never
// below minLifetime. A save in the last second before the deadline, or after it,
// therefore keeps the record for minLifetime, not for less, and never leaves one
// that does not expire.
//
// Returns:
//   - time.Duration: The time to live for the storage and the cookie.
func (s *Session) lifetime() time.Duration {
	if s.config.AbsoluteTimeout <= 0 {
		return s.idleTimeout
	}
	return s.cappedLifetime()
}

// cappedLifetime is lifetime for a store with an absolute timeout.
func (s *Session) cappedLifetime() time.Duration {
	ttl := s.idleTimeout
	deadline := s.absExpiration()
	if deadline.IsZero() {
		return ttl
	}
	return min(ttl, max(time.Until(deadline), minLifetime))
}

// IdleTimeout returns the idle timeout of the session: the one set with
// SetIdleTimeout, or else the store's. A save applies it, cut down to the time
// left until the absolute deadline. A timeout set with SetIdleTimeout holds for
// every later save of this session in the request, unless the session is reset,
// but is not kept between requests, so the next request has to set it again.
//
// Returns:
//   - time.Duration: The idle timeout of this session.
//
// Usage:
//
//	timeout := s.IdleTimeout()
func (s *Session) IdleTimeout() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.idleTimeout > 0 || s.config == nil {
		return s.idleTimeout
	}
	return s.config.IdleTimeout
}

// AbsoluteExpiry returns the time at which the session ends however active it
// is, and whether it has such a time at all: a store without an AbsoluteTimeout
// hands out sessions that only the idle timeout ends.
//
// Returns:
//   - time.Time: The absolute expiration time, or the zero time.
//   - bool: True if the session has an absolute expiration time; otherwise, false.
//
// Usage:
//
//	if expiry, ok := s.AbsoluteExpiry(); ok {
//	    // warn the user when time.Until(expiry) is short
//	}
func (s *Session) AbsoluteExpiry() (time.Time, bool) {
	expiry := s.absExpiration()
	return expiry, !expiry.IsZero()
}

// requestCtx returns the context of the request the session belongs to, or nil if it
// has none, or was told to leave it (leaveCtx): the fiber.Ctx may serve another request
// by then, and is no longer asked which middleware manages the session, or given to the
// storage as the context of a call.
func (s *Session) requestCtx() fiber.Ctx {
	s.mu.RLock()
	c := s.ctx
	s.mu.RUnlock()
	if c == nil {
		return nil
	}
	s.reqMu.Lock()
	left := s.rctx != nil || s.requestOver
	s.reqMu.Unlock()
	if left {
		return nil
	}
	return c
}

// requestResponse returns the request and the response the session writes its ID to:
// those of its fiber.Ctx, or nil when it has none or the request is over for it. Once a
// panic unwound the middleware they are those of the request itself, and only while the
// handlers of that request can still be running (handlersRun): after that the response
// is being sent, and the request is reused. The caller holds s.mu and s.reqMu.
func (s *Session) requestResponse() (*fasthttp.Request, *fasthttp.Response) {
	switch {
	case s.requestOver || s.ctx == nil:
		return nil, nil
	case s.rctx == nil:
		return s.ctx.Request(), s.ctx.Response()
	case s.handlersRun():
		return &s.rctx.Request, &s.rctx.Response
	default:
		return nil, nil
	}
}

// handlersRun reports whether the handlers of the request a panic unwound the middleware
// in can still be running, which a session can only tell by its fiber.Ctx: that goes back
// to the pool when they return, and serves another request then. It still serves this one
// when it holds the same fasthttp context under the same connection and request number;
// the context alone does not say, since it serves the next request of the connection as
// well, and another connection after that. A fiber.Ctx that was abandoned is not given
// back when they return, which leaves the session no way to tell, so it writes to nothing
// then, as after a timeout. The caller holds s.reqMu.
//
// Nothing orders the question after the return of the handlers, which gives the fiber.Ctx
// back without a lock: a goroutine that outlives them can be told that they still run
// just as they return, and then writes to the response of its own request while it is
// sent. That write does not reach another request: the end of the request waits for it
// (leaveCtx) and comes before the response is reset. A session that never learns of that
// end, because the value that tells it was lost to another write to the values of the
// request, is not waited for, and still writes to no request that follows: it is not the
// one it was unwound in.
func (s *Session) handlersRun() bool {
	rctx := s.ctx.RequestCtx()
	return rctx == s.rctx && !s.ctx.IsAbandoned() && rctx.ConnID() == s.rconn && rctx.ConnRequestNum() == s.rnum
}

// leaveCtx tells the session that the fiber.Ctx of its request is no longer its to use
// as it was: from now on it writes to rctx, the request itself as it is served now, and
// only while the handlers of that request still run (see handlersRun), or to nothing
// when rctx is nil. The session of an abandoned or unwound request is not recycled,
// since a handler that outlives the request may still be using it, but the fiber.Ctx is:
// it is reused for another request as soon as the handlers have returned, and whoever
// still holds the session must not write to that request's response. A session a panic
// unwound the middleware of still writes its cookies to its own request, for whatever
// recovers the panic.
//
// It takes reqMu and no other lock. reqMu is held while the session touches its request,
// to write the cookies and headers that carry its ID or to cache its ID there, and for
// nothing else, so this does not wait for a call that holds the session across the
// storage, as mu and the lock of the middleware are held: it waits for a write to the
// request that is under way, and when it returns none is, and none follows that goes to
// anything but rctx. Letting go of the request altogether is final: a rctx that comes
// after it is not taken up, so the order in which the two reach a session that is busy
// does not matter.
func (s *Session) leaveCtx(rctx *fasthttp.RequestCtx) {
	s.reqMu.Lock()
	defer s.reqMu.Unlock()
	if rctx == nil {
		s.requestOver = true
	}
	if s.requestOver {
		rctx = nil
	}
	s.rctx = rctx
	if rctx != nil {
		s.rconn, s.rnum = rctx.ConnID(), rctx.ConnRequestNum()
	}
}
