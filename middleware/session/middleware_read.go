package session

import (
	"time"
)

// Get retrieves a value from the session by key.
//
// Parameters:
//   - key: The key to retrieve.
//
// Returns:
//   - any: The value associated with the key.
//
// Usage:
//
//	value := m.Get("key")
func (m *Middleware) Get(key any) any {
	// The lock is held for the lookup only: a codec decodes without it.
	return m.lookup(key).decode()
}

// lookup is Session.lookup under the read lock of the middleware, as Get takes it.
func (m *Middleware) lookup(key any) stored {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.lookup(key)
}

// fetch is Session.fetch under the read lock of the middleware.
func (m *Middleware) fetch(key any) (stored, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.fetch(key)
}

// GetInto stores the value under key into target, a non-nil pointer. See
// Session.GetInto.
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
//	err := m.GetInto("user", &user)
func (m *Middleware) GetInto(key, target any) error {
	rv, err := intoTarget(target)
	if err != nil {
		return err
	}
	// Under the lock, only the lookup: a codec is not called with it held.
	held, err := m.fetch(key)
	if err != nil {
		return err
	}
	return held.into(key, rv)
}

// IdleTimeout returns the idle timeout of the session. See Session.IdleTimeout.
//
// Like the other methods of the middleware it takes a lock of the session, which
// a save holds while it calls the storage: do not call it from a Storage.
//
// Returns:
//   - time.Duration: The idle timeout of the session.
//
// Usage:
//
//	timeout := m.IdleTimeout()
func (m *Middleware) IdleTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.IdleTimeout()
}

// AbsoluteExpiry returns the time at which the session ends however active it
// is, and whether it has one. See Session.AbsoluteExpiry.
//
// Returns:
//   - time.Time: The absolute expiration time, or the zero time.
//   - bool: True if the session has an absolute expiration time; otherwise, false.
//
// Usage:
//
//	expiry, ok := m.AbsoluteExpiry()
func (m *Middleware) AbsoluteExpiry() (time.Time, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.AbsoluteExpiry()
}
