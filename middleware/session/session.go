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
	ctx fiber.Ctx // fiber context of the request; read it with requestCtx, detach clears it
	// rctx is the request itself, for a session a panic unwound the middleware of: the
	// fiber.Ctx goes back to the pool as soon as the handlers return, while the request
	// lives on until fasthttp resets it, and until then the session still writes its
	// cookies to it. Set and cleared by detach.
	rctx        *fasthttp.RequestCtx
	config      *Store                  // store configuration
	data        *data                   // key value data
	rawData     []byte                  // snapshot from the last decode, reused by Save while the data is unchanged
	id          string                  // session id
	key         string                  // storage key of id, built on first use
	extractor   extractors.Result       // provenance of the extractor that supplied the session ID
	sink        [1]extractors.Extractor // backs the one-element result of getExtractorInfo
	idleTimeout time.Duration           // idleTimeout of this session
	loadGen     uint64                  // idLock generation when the data was read from storage
	mu          sync.RWMutex            // Mutex to protect non-data fields
	isFresh     bool                    // if new session
	loaded      bool                    // the data was read from storage under the current id
	dropped     bool                    // another request ended or rotated it: never stored or sent again
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
	s.idleTimeout = 0
	s.ctx = nil
	s.rctx = nil
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
func (s *Session) identity() (string, extractors.Result) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id, s.extractor
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
	var ended bool
	var err error
	if s.loaded {
		ended, err = s.config.deleteLoadedKey(ctx, s.storageKey(), s.loadGen)
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

	// Generate a new session, and set session.isFresh to true
	s.refresh()

	return nil
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

// resetData empties the data. A session of a store with a codec starts over as a
// session of the store, which one that was stored as gob did not belong to.
func (s *Session) resetData() {
	var f format = gobStream
	if s.config != nil {
		f = s.config.sessionFormat()
	}
	s.data.reset(f)
	s.rawData = nil
}

// refresh generates a new session, and sets session.isFresh to be true.
func (s *Session) refresh() {
	old := s.id
	s.id = s.config.KeyGenerator()
	s.key = ""
	s.isFresh = true
	s.loaded = false
	if s.ctx != nil {
		// A store that is asked again in this request finds the session under its new
		// ID, not under the one that was just deleted. Like a Save, this uses the request
		// of the session, so Regenerate and Reset belong to the goroutine that serves it.
		recacheID(s.ctx, s.config, old, s.id, s.extractor)
	}
}

// storageKey returns the key the session is stored under; the caller holds s.mu.
func (s *Session) storageKey() string {
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
	if c == nil {
		return s.saveUnlessManaged(context.Background(), nil)
	}
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
	return s.saveUnlessManaged(backgroundIfNil(ctx), s.requestCtx())
}

// saveUnlessManaged saves the session through ctx, unless a session middleware
// manages it and will save it when its handler returns. c is the request the
// session belongs to, nil if it has none, and is read once by the callers.
func (s *Session) saveUnlessManaged(ctx context.Context, c fiber.Ctx) error {
	if c != nil {
		// If the session is being used in the handler, it should not be saved. With
		// several session middleware registered, the one managing s may be an outer one.
		if current, ok := c.Locals(middlewareContextKey).(*Middleware); ok {
			managing := current.find(func(m *Middleware) bool { return m.Session == s })
			if managing != nil {
				// Session is in use, so we do nothing and return
				return nil
			}
		}
	}

	return s.saveSessionWithContext(ctx)
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
	// slides. Lock-free is enough; dirty is set before the mutation it reports.
	data := s.rawData
	changed := data == nil || s.data.dirty.Load()
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
		l := lockFor(key)
		if changed {
			// Held alone, so no write-back of an older snapshot lands after this change.
			l.mu.Lock()
			defer func() {
				l.gen.Add(1)
				l.mu.Unlock()
			}()
		} else {
			l.mu.RLock()
			defer l.mu.RUnlock()
		}
		// An ID of this shard was deleted or changed since the load; look at this one.
		if l.gen.Load() != s.loadGen {
			raw, err := s.config.Storage.GetWithContext(ctx, key)
			if err != nil {
				return err
			}
			if raw == nil {
				return nil // ended by another request: neither store it nor send its cookie again
			}
			if !changed && !bytes.Equal(raw, data) {
				s.setSession(ttl)
				return nil // changed by another request: the older snapshot must not undo that
			}
		}
	}

	// Update client cookie
	s.setSession(ttl)
	return s.config.Storage.SetWithContext(ctx, key, data, ttl)
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

// decodeSessionData loads the stored session rawData into the data of the session and
// keeps the snapshot it was decoded from, which a save writes back while the data is
// unchanged. The caller holds the lock of the data.
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
	s.data.bind(store.sessionFormat())
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
