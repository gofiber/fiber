package session

import (
	"fmt"
)

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
	rv, err := intoTarget(target)
	if err != nil {
		return err
	}
	held, ok := s.fetch(key)
	if !ok {
		return keyNotFound(key)
	}
	return readInto(held, key, rv)
}

// keyNotFound is the error of GetInto for a key that the session holds no value under.
func keyNotFound(key any) error {
	return fmt.Errorf("%w: %#v", ErrKeyNotFound, key)
}
