package session

import (
	"bytes"
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
	byteBuffer  *bytes.Buffer // byte buffer for encoding/decoding
	id          string        // session id
	idleTimeout time.Duration // idleTimeout of this session
	mu          sync.RWMutex  // Mutex to protect non-data fields
	fresh       bool          // if new session
}

type expirationKeyType int

const (
	// sessionIDContextKey is the key used to store the session ID in the context locals.
	expirationKey expirationKeyType = iota
)

var sessionPool = sync.Pool{
	New: func() any {
		return &Session{
			byteBuffer: new(bytes.Buffer),
		}
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
	if s.byteBuffer != nil {
		s.byteBuffer.Reset()
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
	if err := s.config.Storage.Delete(s.id); err != nil {
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
	if err := s.config.Storage.Delete(s.id); err != nil {
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

	// Reset byte buffer
	if s.byteBuffer != nil {
		s.byteBuffer.Reset()
	}
	// Reset expiration
	s.idleTimeout = 0

	// Delete old id from storage
	if err := s.config.Storage.Delete(s.id); err != nil {
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
	encCache := gob.NewEncoder(s.byteBuffer)
	s.data.RLock()
	err := encCache.Encode(&s.data.Data)
	s.data.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}

	// Copy the data in buffer
	encodedBytes := make([]byte, s.byteBuffer.Len())
	copy(encodedBytes, s.byteBuffer.Bytes())

	// Pass copied bytes with session id to provider
	return s.config.Storage.Set(s.id, encodedBytes, s.idleTimeout)
}

// Keys retrieves all keys in the current session.
//
// Returns:
//   - []string: A slice of all keys in the session.
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

func (s *Session) setSession() {
	if s.config.source == SourceHeader {
		s.ctx.Request().Header.SetBytesV(s.config.sessionName, []byte(s.id))
		s.ctx.Response().Header.SetBytesV(s.config.sessionName, []byte(s.id))
	} else {
		fcookie := fasthttp.AcquireCookie()
		fcookie.SetKey(s.config.sessionName)
		fcookie.SetValue(s.id)
		fcookie.SetPath(s.config.CookiePath)
		fcookie.SetDomain(s.config.CookieDomain)
		// Cookies are also session cookies if they do not specify the Expires or Max-Age attribute.
		// refer: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
		if !s.config.CookieSessionOnly {
			fcookie.SetMaxAge(int(s.idleTimeout.Seconds()))
			fcookie.SetExpire(time.Now().Add(s.idleTimeout))
		}
		fcookie.SetSecure(s.config.CookieSecure)
		fcookie.SetHTTPOnly(s.config.CookieHTTPOnly)

		switch utils.ToLower(s.config.CookieSameSite) {
		case "strict":
			fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
		case "none":
			fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
		default:
			fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
		}
		s.ctx.Response().Header.SetCookie(fcookie)
		fasthttp.ReleaseCookie(fcookie)
	}
}

func (s *Session) delSession() {
	if s.config.source == SourceHeader {
		s.ctx.Request().Header.Del(s.config.sessionName)
		s.ctx.Response().Header.Del(s.config.sessionName)
	} else {
		s.ctx.Request().Header.DelCookie(s.config.sessionName)
		s.ctx.Response().Header.DelCookie(s.config.sessionName)

		fcookie := fasthttp.AcquireCookie()
		fcookie.SetKey(s.config.sessionName)
		fcookie.SetPath(s.config.CookiePath)
		fcookie.SetDomain(s.config.CookieDomain)
		fcookie.SetMaxAge(-1)
		fcookie.SetExpire(time.Now().Add(-1 * time.Minute))
		fcookie.SetSecure(s.config.CookieSecure)
		fcookie.SetHTTPOnly(s.config.CookieHTTPOnly)

		switch utils.ToLower(s.config.CookieSameSite) {
		case "strict":
			fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
		case "none":
			fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
		default:
			fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
		}

		s.ctx.Response().Header.SetCookie(fcookie)
		fasthttp.ReleaseCookie(fcookie)
	}
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
	_, _ = s.byteBuffer.Write(rawData)
	encCache := gob.NewDecoder(s.byteBuffer)
	if err := encCache.Decode(&s.data.Data); err != nil {
		return fmt.Errorf("failed to decode session data: %w", err)
	}
	return nil
}

// expiration returns the session expiration time or a zero time if not set.
//
// Returns:
//   - time.Time: The session expiration time, or a zero time if not set.
//
// Usage:
//
//	expiration := s.expiration()
func (s *Session) expiration() time.Time {
	expiration, ok := s.Get(expirationKey).(time.Time)
	if ok {
		return expiration
	}
	return time.Time{}
}

// isExpired returns true if the session is expired.
//
// If the session expiration time is zero, the session is considered to never expire.
//
// Returns:
//   - bool: True if the session is expired, otherwise false.
func (s *Session) isExpired() bool {
	expiration := s.expiration()
	return !expiration.IsZero() && time.Now().After(expiration)
}

// setExpiration sets the session expiration time.
//
// Parameters:
//   - expiration: The session expiration time.
//
// Usage:
//
//	s.setExpiration(time.Now().Add(time.Hour))
func (s *Session) setExpiration(expiration time.Time) {
	s.Set(expirationKey, expiration)
}
