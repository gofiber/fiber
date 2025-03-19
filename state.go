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

// GetString retrieves a string value from the State.
func (s *State) GetString(key string) (string, bool) {
	dep, ok := s.Get(key)
	if ok {
		depString, okCast := dep.(string)
		return depString, okCast
	}

	return "", false
}

// GetInt retrieves an int value from the State.
func (s *State) GetInt(key string) (int, bool) {
	dep, ok := s.Get(key)
	if ok {
		depInt, okCast := dep.(int)
		return depInt, okCast
	}

	return 0, false
}

// GetBool retrieves a bool value from the State.
func (s *State) GetBool(key string) (value, ok bool) { //nolint:nonamedreturns // Better idea to use named returns here
	dep, ok := s.Get(key)
	if ok {
		depBool, okCast := dep.(bool)
		return depBool, okCast
	}

	return false, false
}

// GetFloat64 retrieves a float64 value from the State.
func (s *State) GetFloat64(key string) (float64, bool) {
	dep, ok := s.Get(key)
	if ok {
		depFloat64, okCast := dep.(float64)
		return depFloat64, okCast
	}

	return 0, false
}

// MustGet retrieves a value from the State and panics if the key is not found.
func (s *State) MustGet(key string) any {
	if dep, ok := s.Get(key); ok {
		return dep
	}

	panic("state: dependency not found!")
}

// MustGetString retrieves a string value from the State and panics if the key is not found.
func (s *State) Delete(key string) {
	s.dependencies.Delete(key)
}

// Reset resets the State.
func (s *State) Clear() {
	s.dependencies.Clear()
}

// Keys retrieves all the keys from the State.
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

// Len retrieves the number of dependencies in the State.
func (s *State) Len() int {
	length := 0
	s.dependencies.Range(func(_, _ any) bool {
		length++
		return true
	})

	return length
}

// GetState retrieves a value from the State and casts it to the desired type.
func GetState[T any](s *State, key string) (T, bool) {
	dep, ok := s.Get(key)

	if ok {
		depT, okCast := dep.(T)
		return depT, okCast
	}

	var zeroVal T
	return zeroVal, false
}

// MustGetState retrieves a value from the State and casts it to the desired type, panicking if the key is not found.
func MustGetState[T any](s *State, key string) T {
	dep, ok := GetState[T](s, key)
	if !ok {
		panic("state: dependency not found!")
	}

	return dep
}

// GetStateWithDefault retrieves a value from the State and casts it to the desired type, returning a default value in case the key is not found.
func GetStateWithDefault[T any](s *State, key string, defaultVal T) T {
	dep, ok := GetState[T](s, key)
	if !ok {
		return defaultVal
	}

	return dep
}
