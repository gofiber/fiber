package session

import (
	"time"
)

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
