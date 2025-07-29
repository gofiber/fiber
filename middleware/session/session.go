package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Session represents a user session.
type Session struct {
	ctx         fiber.Ctx     // fiber context
	config      *Store        // store configuration
	data        *data         // key value data
	id          string        // session id
	idleTimeout time.Duration // idleTimeout of this session
	mu          sync.RWMutex  // Mutex to protect non-data fields
	fresh       bool          // if new session
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
	s.fresh = true
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
	s.idleTimeout = 0
	s.ctx = nil
	s.config = nil
	if s.data != nil {
		s.data.Reset()
	}
	s.mu.Unlock()
	sessionPool.Put(s)
}

// Fresh returns whether the session is new
//
// Returns:
//   - bool: True if the session is fresh, otherwise false.
//
// Usage:
//
//	isFresh := s.Fresh()
func (s *Session) Fresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fresh
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
// Usage:
//
//	value := s.Get("key")
func (s *Session) Get(key any) any {
	if s.data == nil {
		return nil
	}
	return s.data.Get(key)
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
	if s.data == nil {
		return nil
	}

	// Reset local data
	s.data.Reset()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use external Storage if exist
	var ctx context.Context = s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.config.Storage.DeleteWithContext(ctx, s.id); err != nil {
		return err
	}

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
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete old id from storage
	var ctx context.Context = s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.config.Storage.DeleteWithContext(ctx, s.id); err != nil {
		return err
	}

	// Generate a new session, and set session.fresh to true
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
	// Reset local data
	if s.data != nil {
		s.data.Reset()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reset expiration
	s.idleTimeout = 0

	// Delete old id from storage
	var ctx context.Context = s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.config.Storage.DeleteWithContext(ctx, s.id); err != nil {
		return err
	}

	// Expire session
	s.delSession()

	// Generate a new session, and set session.fresh to true
	s.refresh()

	return nil
}

// refresh generates a new session, and sets session.fresh to be true.
func (s *Session) refresh() {
	s.id = s.config.KeyGenerator()
	s.fresh = true
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
	if s.ctx == nil {
		return s.saveSession()
	}

	// If the session is being used in the handler, it should not be saved
	if m, ok := s.ctx.Locals(middlewareContextKey).(*Middleware); ok {
		if m.Session == s {
			// Session is in use, so we do nothing and return
			return nil
		}
	}

	return s.saveSession()
}

// saveSession encodes session data to saves it to storage.
func (s *Session) saveSession() error {
	if s.data == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Set idleTimeout if not already set
	if s.idleTimeout <= 0 {
		s.idleTimeout = s.config.IdleTimeout
	}

	// Update client cookie
	s.setSession()

	// Encode session data
	s.data.RLock()
	encodedBytes, err := s.encodeSessionData()
	s.data.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}

	// Pass copied bytes with session id to provider
	var ctx context.Context = s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return s.config.Storage.SetWithContext(ctx, s.id, encodedBytes, s.idleTimeout)
}

// Keys retrieves all keys in the current session.
//
// Returns:
//   - []any: A slice of all keys in the session.
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

// getExtractorInfo returns all cookie and header extractors from the chain
func (s *Session) getExtractorInfo() []Extractor {
	if s.config == nil {
		return []Extractor{{Source: SourceCookie, Key: "session_id"}} // Safe default
	}

	extractor := s.config.Extractor
	var relevantExtractors []Extractor

	// If it's a chained extractor, collect all cookie/header extractors
	if len(extractor.Chain) > 0 {
		for _, chainExtractor := range extractor.Chain {
			if chainExtractor.Source == SourceCookie || chainExtractor.Source == SourceHeader {
				relevantExtractors = append(relevantExtractors, chainExtractor)
			}
		}
	} else if extractor.Source == SourceCookie || extractor.Source == SourceHeader {
		// Single extractor - only include if it's cookie or header
		relevantExtractors = append(relevantExtractors, extractor)
	}

	// If no cookie/header extractors found and the config has a store but no explicit cookie/header extractors,
	// we should not default to cookie. This allows for SourceOther-only configurations.
	// Only add default cookie extractor if we have no extractors at all (nil config case is handled above)

	return relevantExtractors
}

func (s *Session) setSession() {
	if s.ctx == nil {
		return
	}

	// Get all relevant extractors
	extractors := s.getExtractorInfo()

	// Set session ID for each extractor type
	for _, ext := range extractors {
		switch ext.Source {
		case SourceHeader:
			s.ctx.Response().Header.SetBytesV(ext.Key, []byte(s.id))
		case SourceCookie:
			fcookie := fasthttp.AcquireCookie()

			fcookie.SetKey(ext.Key)
			fcookie.SetValue(s.id)
			fcookie.SetPath(s.config.CookiePath)
			fcookie.SetDomain(s.config.CookieDomain)
			// Cookies are also session cookies if they do not specify the Expires or Max-Age attribute.
			// refer: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
			if !s.config.CookieSessionOnly {
				fcookie.SetMaxAge(int(s.idleTimeout.Seconds()))
				fcookie.SetExpire(time.Now().Add(s.idleTimeout))
			}

			s.setCookieAttributes(fcookie)
			s.ctx.Response().Header.SetCookie(fcookie)
			fasthttp.ReleaseCookie(fcookie)
		case SourceOther:
			// No action required for SourceOther
		}
	}
}

func (s *Session) delSession() {
	if s.ctx == nil {
		return
	}

	// Get all relevant extractors
	extractors := s.getExtractorInfo()

	// Delete session ID for each extractor type
	for _, ext := range extractors {
		switch ext.Source {
		case SourceHeader:
			s.ctx.Request().Header.Del(ext.Key)
			s.ctx.Response().Header.Del(ext.Key)
		case SourceCookie:
			s.ctx.Request().Header.DelCookie(ext.Key)
			s.ctx.Response().Header.DelCookie(ext.Key)

			fcookie := fasthttp.AcquireCookie()

			fcookie.SetKey(ext.Key)
			fcookie.SetPath(s.config.CookiePath)
			fcookie.SetDomain(s.config.CookieDomain)
			fcookie.SetMaxAge(-1)
			fcookie.SetExpire(time.Now().Add(-1 * time.Minute))

			s.setCookieAttributes(fcookie)
			s.ctx.Response().Header.SetCookie(fcookie)
			fasthttp.ReleaseCookie(fcookie)
		case SourceOther:
			// No action required for SourceOther
		}
	}
}

// setCookieAttributes sets the cookie attributes based on the session config.
func (s *Session) setCookieAttributes(fcookie *fasthttp.Cookie) {
	// Set SameSite attribute
	switch {
	case utils.EqualFold(s.config.CookieSameSite, fiber.CookieSameSiteStrictMode):
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case utils.EqualFold(s.config.CookieSameSite, fiber.CookieSameSiteNoneMode):
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	}

	// The Secure attribute is required for SameSite=None
	if fcookie.SameSite() == fasthttp.CookieSameSiteNoneMode {
		fcookie.SetSecure(true)
	} else {
		fcookie.SetSecure(s.config.CookieSecure)
	}

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
	byteBuffer := byteBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
	defer byteBufferPool.Put(byteBuffer)
	defer byteBuffer.Reset()
	_, _ = byteBuffer.Write(rawData)
	decCache := gob.NewDecoder(byteBuffer)
	if err := decCache.Decode(&s.data.Data); err != nil {
		return fmt.Errorf("failed to decode session data: %w", err)
	}
	return nil
}

// encodeSessionData encodes session data to raw bytes
//
// Parameters:
//   - rawData: The raw byte data to encode.
//
// Returns:
//   - error: An error if the encoding fails.
//
// Usage:
//
//	err := s.encodeSessionData(rawData)
func (s *Session) encodeSessionData() ([]byte, error) {
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

// absExpiration returns the session absolute expiration time or a zero time if not set.
//
// Returns:
//   - time.Time: The session absolute expiration time. Zero time if not set.
//
// Usage:
//
//	expiration := s.absExpiration()
func (s *Session) absExpiration() time.Time {
	absExpiration, ok := s.Get(absExpirationKey).(time.Time)
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
//   - bool: True if the session is expired, otherwise false.
func (s *Session) isAbsExpired() bool {
	absExpiration := s.absExpiration()
	return !absExpiration.IsZero() && time.Now().After(absExpiration)
}

// setAbsoluteExpiration sets the absolute session expiration time.
//
// Parameters:
//   - expiration: The session expiration time.
//
// Usage:
//
//	s.setExpiration(time.Now().Add(time.Hour))
func (s *Session) setAbsExpiration(absExpiration time.Time) {
	s.Set(absExpirationKey, absExpiration)
}
