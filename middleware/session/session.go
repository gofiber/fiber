package session

import (
	"bytes"
	"context"
	"encoding/gob"
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
	ctx fiber.Ctx // fiber context of the request; read it with requestCtx, forgetContext clears it
	// rctx is the request itself, for a session a panic unwound the middleware of: the
	// fiber.Ctx goes back to the pool as soon as the handlers return, while the request
	// lives on until fasthttp resets it, and until then the session still writes its
	// cookies to it. Set by detach, cleared by forgetContext.
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

// Session pool for reusing byte buffers.
var byteBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

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
		s.data.resetAs(nil)
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
// mutate it in place without calling Set.
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

// lookup returns what is stored under key and the codec of the session, nil for
// gob. See data.lookup.
func (s *Session) lookup(key any) (any, *namedCodec) {
	if s.data == nil {
		return nil, nil
	}
	return s.data.lookup(key)
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
// value decodes into, not a check that this is what it was stored as, and src has
// to be the *Session or *Middleware itself, not a type that embeds one.
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
func GetAs[V any](src interface{ Get(key any) any }, key any) (V, bool) {
	var v any
	var codec *namedCodec
	switch typed := src.(type) {
	case nil:
		var zero V
		return zero, false
	case *Middleware:
		if typed == nil {
			var zero V
			return zero, false
		}
		v, codec = typed.lookup(key)
	case *Session:
		if typed == nil {
			var zero V
			return zero, false
		}
		v, codec = typed.lookup(key)
	default:
		// Any other reader is trusted to cope with a missing key.
		v = src.Get(key)
	}

	if codec != nil {
		return decodeAs[V](v, codec)
	}
	value, ok := v.(V)
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
	rv, err := intoTarget(target)
	if err != nil {
		return err
	}
	v, codec, err := s.fetch(key)
	if err != nil {
		return err
	}
	return valueInto(key, v, codec, rv)
}

// fetch returns what is stored under key and the codec of the session, nil for
// gob, or ErrKeyNotFound.
func (s *Session) fetch(key any) (any, *namedCodec, error) {
	if s.data == nil {
		return nil, nil, fmt.Errorf("%w: %#v", ErrKeyNotFound, key)
	}
	v, codec, ok := s.data.fetch(key)
	if !ok {
		return nil, nil, fmt.Errorf("%w: %#v", ErrKeyNotFound, key)
	}
	return v, codec, nil
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
	var codec *namedCodec
	if s.config != nil {
		codec = s.config.sessionCodec()
	}
	s.data.resetAs(codec)
	s.rawData = nil
}

// refresh generates a new session, and sets session.isFresh to be true.
func (s *Session) refresh() {
	s.id = s.config.KeyGenerator()
	s.key = ""
	s.isFresh = true
	s.loaded = false
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
		encoded, err := s.encodeSessionData()
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
		if s.extractor.Resolved && s.extractor.Key != "" {
			switch s.extractor.Source {
			case extractors.SourceCookie, extractors.SourceHeader:
				// The ID came from a writable sink; write it back to the same place.
				s.sink[0] = extractors.Extractor{Key: s.extractor.Key, Source: s.extractor.Source}
				return s.sink[:]
			case extractors.SourceAuthHeader, extractors.SourceForm,
				extractors.SourceQuery, extractors.SourceParam, extractors.SourceCustom:
				// Read-only: fall through to the refusal below.
			}
		}
		// Read-only, or an origin we cannot attribute. For an existing session
		// either would be an attacker-controlled ID, so it must not be promoted
		// into cookies/headers (session fixation). A fresh session's ID is
		// server-generated, so it may still fall through to the sinks below.
		if !s.isFresh {
			return nil
		}
	}

	return s.config.writableSinks()
}

// requestResponse returns the request and the response the session writes its ID
// to: those of its fiber.Ctx, or of the request itself once the middleware was
// unwound, or nil when the session has let go of the request. The caller holds s.mu.
func (s *Session) requestResponse() (*fasthttp.Request, *fasthttp.Response) {
	if s.ctx != nil {
		return s.ctx.Request(), s.ctx.Response()
	}
	if s.rctx != nil {
		return &s.rctx.Request, &s.rctx.Response
	}
	return nil, nil
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

// decodeSessionData decodes session data from raw bytes
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
	// Copied before decoding: the storage owns rawData and may reuse it after Get
	// returns, so the decode and the snapshot Save writes back read the same bytes.
	raw := utils.CopyBytes(rawData)
	if isRecord(raw) {
		// The caller has seen to it that the store can read the record.
		if err := s.data.loadRecord(raw); err != nil {
			return err
		}
		s.rawData = raw
		return nil
	}
	byteBuffer := byteBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
	defer byteBufferPool.Put(byteBuffer)
	defer byteBuffer.Reset()
	_, _ = byteBuffer.Write(raw)
	decCache := gob.NewDecoder(byteBuffer)
	if err := decCache.Decode(&s.data.Data); err != nil {
		return fmt.Errorf("failed to decode session data: %w", err)
	}
	// A session that a store without a codec stored stays gob, with all that goes with
	// it, until it ends or is reset: its values are not converted, which could not
	// be done without loss for every type.
	s.data.codec = nil
	s.rawData = raw
	s.data.dirty.Store(false)
	return nil
}

// encodeSessionData encodes session data to raw bytes
//
// Returns:
//   - []byte: The encoded session data.
//   - error: An error if the encoding fails.
//
// Usage:
//
//	encoded, err := s.encodeSessionData()
func (s *Session) encodeSessionData() ([]byte, error) {
	if s.data.codec != nil {
		return s.data.record()
	}
	byteBuffer := byteBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
	defer byteBufferPool.Put(byteBuffer)
	defer byteBuffer.Reset()
	encCache := gob.NewEncoder(byteBuffer)
	if err := encCache.Encode(&s.data.Data); err != nil {
		return nil, fmt.Errorf("failed to encode session data: %w", err)
	}
	// Copy the bytes
	// Copy the data in buffer
	encodedBytes := make([]byte, byteBuffer.Len())
	copy(encodedBytes, byteBuffer.Bytes())

	return encodedBytes, nil
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

// bind makes the session one of store: its configuration, and the codec its data
// holds. It is for a session that was just acquired, which nothing else refers to
// yet, so the data needs no lock.
func (s *Session) bind(store *Store) {
	s.config = store
	s.data.codec = store.sessionCodec()
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

// requestCtx returns the context of the request the session belongs to, or nil
// if it has none, or none any more.
func (s *Session) requestCtx() fiber.Ctx {
	s.mu.RLock()
	c := s.ctx
	s.mu.RUnlock()
	return c
}

// forgetContext cuts the session off from its request. The session of an
// abandoned request is not recycled, since a handler that outlives the request
// may still be using it, but the context is: it is reused for another request
// once those handlers are done, and whoever still holds the session must not
// write to that request's response, or run its storage calls under its context.
func (s *Session) forgetContext() {
	s.mu.Lock()
	s.ctx, s.rctx = nil, nil
	s.mu.Unlock()
}

// forgetContextSoon is forgetContext for a caller that must not wait: when the
// session is locked, by a save that is writing to the storage for instance, the cut
// follows from a goroutine of its own.
func (s *Session) forgetContextSoon() {
	if s.mu.TryLock() {
		s.ctx, s.rctx = nil, nil
		s.mu.Unlock()
		return
	}
	go s.forgetContext()
}

// detachSoon moves the session from its fiber.Ctx, which goes back to the pool as
// soon as the handlers return, to rctx, the request itself, which lives on until
// fasthttp resets it: a session a panic unwound the middleware of still writes its
// cookies there, for whatever recovers the panic. Like forgetContextSoon it does
// not wait for the lock.
func (s *Session) detachSoon(rctx *fasthttp.RequestCtx) {
	if s.mu.TryLock() {
		s.ctx, s.rctx = nil, rctx
		s.mu.Unlock()
		return
	}
	go func() {
		s.mu.Lock()
		s.ctx, s.rctx = nil, rctx
		s.mu.Unlock()
	}()
}
