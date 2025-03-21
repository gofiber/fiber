package fiber

import (
	"sync"
)

// State is a key-value store for Fiber's app in order to be used as a global storage for the app's dependencies.
// It's a thread-safe implementation of a map[string]any, using sync.Map.
type State struct {
	dependencies sync.Map
}

// NewState creates a new instance of State.
func newState() *State {
	return &State{
		dependencies: sync.Map{},
	}
}

// Set sets a key-value pair in the State.
func (s *State) Set(key string, value any) {
	s.dependencies.Store(key, value)
}

// Get retrieves a value from the State.
func (s *State) Get(key string) (any, bool) {
	return s.dependencies.Load(key)
}

// MustGet retrieves a value from the State and panics if the key is not found.
func (s *State) MustGet(key string) any {
	if dep, ok := s.Get(key); ok {
		return dep
	}

	panic("state: dependency not found!")
}

// GetString retrieves a string value from the State.
// It returns the string and a boolean indicating successful type assertion.
func (s *State) GetString(key string) (string, bool) {
	dep, ok := s.Get(key)
	if ok {
		depString, okCast := dep.(string)
		return depString, okCast
	}

	return "", false
}

// GetInt retrieves an integer value from the State.
// It returns the int and a boolean indicating successful type assertion.
func (s *State) GetInt(key string) (int, bool) {
	dep, ok := s.Get(key)
	if ok {
		depInt, okCast := dep.(int)
		return depInt, okCast
	}

	return 0, false
}

// GetBool retrieves a boolean value from the State.
// It returns the bool and a boolean indicating successful type assertion.
func (s *State) GetBool(key string) (value, ok bool) { //nolint:nonamedreturns // Better idea to use named returns here
	dep, ok := s.Get(key)
	if ok {
		depBool, okCast := dep.(bool)
		return depBool, okCast
	}

	return false, false
}

// GetFloat64 retrieves a float64 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetFloat64(key string) (float64, bool) {
	dep, ok := s.Get(key)
	if ok {
		depFloat64, okCast := dep.(float64)
		return depFloat64, okCast
	}

	return 0, false
}

// Has checks if a key is present in the State.
// It returns a boolean indicating if the key is present.
func (s *State) Has(key string) bool {
	_, ok := s.Get(key)
	return ok
}

// Delete removes a key-value pair from the State.
func (s *State) Delete(key string) {
	s.dependencies.Delete(key)
}

// Reset resets the State by removing all keys.
func (s *State) Reset() {
	s.dependencies.Clear()
}

// Keys returns a slice containing all keys present in the State.
func (s *State) Keys() []string {
	keys := make([]string, 0)
	s.dependencies.Range(func(key, _ any) bool {
		keyStr, ok := key.(string)
		if !ok {
			return false
		}

		keys = append(keys, keyStr)
		return true
	})

	return keys
}

// Len returns the number of keys in the State.
func (s *State) Len() int {
	length := 0
	s.dependencies.Range(func(_, _ any) bool {
		length++
		return true
	})

	return length
}

// GetState retrieves a value from the State and casts it to the desired type.
// It returns the casted value and a boolean indicating if the cast was successful.
func GetState[T any](s *State, key string) (T, bool) {
	dep, ok := s.Get(key)

	if ok {
		depT, okCast := dep.(T)
		return depT, okCast
	}

	var zeroVal T
	return zeroVal, false
}

// MustGetState retrieves a value from the State and casts it to the desired type.
// It panics if the key is not found or if the type assertion fails.
func MustGetState[T any](s *State, key string) T {
	dep, ok := GetState[T](s, key)
	if !ok {
		panic("state: dependency not found!")
	}

	return dep
}

// GetStateWithDefault retrieves a value from the State,
// casting it to the desired type. If the key is not present,
// it returns the provided default value.
func GetStateWithDefault[T any](s *State, key string, defaultVal T) T {
	dep, ok := GetState[T](s, key)
	if !ok {
		return defaultVal
	}

	return dep
}
