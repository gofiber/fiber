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

// GetUint retrieves a uint value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetUint(key string) (uint, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depUint, okCast := dep.(uint); okCast {
			return depUint, true
		}
	}
	return 0, false
}

// GetInt8 retrieves an int8 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetInt8(key string) (int8, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depInt8, okCast := dep.(int8); okCast {
			return depInt8, true
		}
	}
	return 0, false
}

// GetInt16 retrieves an int16 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetInt16(key string) (int16, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depInt16, okCast := dep.(int16); okCast {
			return depInt16, true
		}
	}
	return 0, false
}

// GetInt32 retrieves an int32 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetInt32(key string) (int32, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depInt32, okCast := dep.(int32); okCast {
			return depInt32, true
		}
	}
	return 0, false
}

// GetInt64 retrieves an int64 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetInt64(key string) (int64, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depInt64, okCast := dep.(int64); okCast {
			return depInt64, true
		}
	}
	return 0, false
}

// GetUint8 retrieves a uint8 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetUint8(key string) (uint8, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depUint8, okCast := dep.(uint8); okCast {
			return depUint8, true
		}
	}
	return 0, false
}

// GetUint16 retrieves a uint16 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetUint16(key string) (uint16, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depUint16, okCast := dep.(uint16); okCast {
			return depUint16, true
		}
	}
	return 0, false
}

// GetUint32 retrieves a uint32 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetUint32(key string) (uint32, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depUint32, okCast := dep.(uint32); okCast {
			return depUint32, true
		}
	}
	return 0, false
}

// GetUint64 retrieves a uint64 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetUint64(key string) (uint64, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depUint64, okCast := dep.(uint64); okCast {
			return depUint64, true
		}
	}
	return 0, false
}

// GetUintptr retrieves a uintptr value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetUintptr(key string) (uintptr, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depUintptr, okCast := dep.(uintptr); okCast {
			return depUintptr, true
		}
	}
	return 0, false
}

// GetFloat32 retrieves a float32 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetFloat32(key string) (float32, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depFloat32, okCast := dep.(float32); okCast {
			return depFloat32, true
		}
	}
	return 0, false
}

// GetComplex64 retrieves a complex64 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetComplex64(key string) (complex64, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depComplex64, okCast := dep.(complex64); okCast {
			return depComplex64, true
		}
	}
	return 0, false
}

// GetComplex128 retrieves a complex128 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetComplex128(key string) (complex128, bool) {
	dep, ok := s.Get(key)
	if ok {
		if depComplex128, okCast := dep.(complex128); okCast {
			return depComplex128, true
		}
	}
	return 0, false
}
