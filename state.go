package fiber

import (
	"encoding/hex"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const servicesStatePrefix = "gofiber-services-"

var servicesStatePrefixHash string

func init() {
	servicesStatePrefixHash = hex.EncodeToString([]byte(servicesStatePrefix + uuid.New().String()))
}

// State is a key-value store for Fiber's app in order to be used as a global storage for the app's dependencies.
// It's a thread-safe implementation of a map[string]any, using sync.Map.
type State struct {
	dependencies  sync.Map
	servicePrefix string
}

// NewState creates a new instance of State.
func newState() *State {
	// Initialize the services state prefix using a hashed random string
	return &State{
		dependencies:  sync.Map{},
		servicePrefix: servicesStatePrefixHash,
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
	return GetState[string](s, key)
}

// GetInt retrieves an integer value from the State.
// It returns the int and a boolean indicating successful type assertion.
func (s *State) GetInt(key string) (int, bool) {
	return GetState[int](s, key)
}

// GetBool retrieves a boolean value from the State.
// It returns the bool and a boolean indicating successful type assertion.
func (s *State) GetBool(key string) (value, ok bool) { //nolint:nonamedreturns // Better idea to use named returns here
	return GetState[bool](s, key)
}

// GetFloat64 retrieves a float64 value from the State.
// It returns the float64 and a boolean indicating successful type assertion.
func (s *State) GetFloat64(key string) (float64, bool) {
	return GetState[float64](s, key)
}

// GetUint retrieves a uint value from the State.
// It returns the uint and a boolean indicating successful type assertion.
func (s *State) GetUint(key string) (uint, bool) {
	return GetState[uint](s, key)
}

// GetInt8 retrieves an int8 value from the State.
// It returns the int8 and a boolean indicating successful type assertion.
func (s *State) GetInt8(key string) (int8, bool) {
	return GetState[int8](s, key)
}

// GetInt16 retrieves an int16 value from the State.
// It returns the int16 and a boolean indicating successful type assertion.
func (s *State) GetInt16(key string) (int16, bool) {
	return GetState[int16](s, key)
}

// GetInt32 retrieves an int32 value from the State.
// It returns the int32 and a boolean indicating successful type assertion.
func (s *State) GetInt32(key string) (int32, bool) {
	return GetState[int32](s, key)
}

// GetInt64 retrieves an int64 value from the State.
// It returns the int64 and a boolean indicating successful type assertion.
func (s *State) GetInt64(key string) (int64, bool) {
	return GetState[int64](s, key)
}

// GetUint8 retrieves a uint8 value from the State.
// It returns the uint8 and a boolean indicating successful type assertion.
func (s *State) GetUint8(key string) (uint8, bool) {
	return GetState[uint8](s, key)
}

// GetUint16 retrieves a uint16 value from the State.
// It returns the uint16 and a boolean indicating successful type assertion.
func (s *State) GetUint16(key string) (uint16, bool) {
	return GetState[uint16](s, key)
}

// GetUint32 retrieves a uint32 value from the State.
// It returns the uint32 and a boolean indicating successful type assertion.
func (s *State) GetUint32(key string) (uint32, bool) {
	return GetState[uint32](s, key)
}

// GetUint64 retrieves a uint64 value from the State.
// It returns the uint64 and a boolean indicating successful type assertion.
func (s *State) GetUint64(key string) (uint64, bool) {
	return GetState[uint64](s, key)
}

// GetUintptr retrieves a uintptr value from the State.
// It returns the uintptr and a boolean indicating successful type assertion.
func (s *State) GetUintptr(key string) (uintptr, bool) {
	return GetState[uintptr](s, key)
}

// GetFloat32 retrieves a float32 value from the State.
// It returns the float32 and a boolean indicating successful type assertion.
func (s *State) GetFloat32(key string) (float32, bool) {
	return GetState[float32](s, key)
}

// GetComplex64 retrieves a complex64 value from the State.
// It returns the complex64 and a boolean indicating successful type assertion.
func (s *State) GetComplex64(key string) (complex64, bool) {
	return GetState[complex64](s, key)
}

// GetComplex128 retrieves a complex128 value from the State.
// It returns the complex128 and a boolean indicating successful type assertion.
func (s *State) GetComplex128(key string) (complex128, bool) {
	return GetState[complex128](s, key)
}

// serviceKey returns a key for a service in the State.
// A key is composed of the State's servicePrefix (hashed) and the hash of the service string.
// This way we can avoid collisions and have a unique key for each service.
func (s *State) serviceKey(key string) string {
	// hash the service string to avoid collisions
	return s.servicePrefix + hex.EncodeToString([]byte(key))
}

// setService sets a service in the State.
func (s *State) setService(srv Service) {
	// Always prepend the service key with the servicesStateKey to avoid collisions
	s.Set(s.serviceKey(srv.String()), srv)
}

// Delete removes a key-value pair from the State.
func (s *State) deleteService(srv Service) {
	s.Delete(s.serviceKey(srv.String()))
}

// serviceKeys returns a slice containing all keys present for services in the application's State.
func (s *State) serviceKeys() []string {
	keys := make([]string, 0)
	s.dependencies.Range(func(key, _ any) bool {
		keyStr, ok := key.(string)
		if !ok {
			return false
		}

		if !strings.HasPrefix(keyStr, s.servicePrefix) {
			return true // Continue iterating if key doesn't have service prefix
		}

		keys = append(keys, keyStr)
		return true
	})

	return keys
}

// Services returns a map containing all services present in the State.
// The key is the hash of the service String() value and the value is the service itself.
func (s *State) Services() map[string]Service {
	services := make(map[string]Service)

	for _, key := range s.serviceKeys() {
		services[key] = MustGetState[Service](s, key)
	}

	return services
}

// ServicesLen returns the number of keys for services in the State.
func (s *State) ServicesLen() int {
	length := 0
	s.dependencies.Range(func(key, _ any) bool {
		if str, ok := key.(string); ok && strings.HasPrefix(str, s.servicePrefix) {
			length++
		}
		return true
	})

	return length
}

// GetService returns a service present in the application's State.
func GetService[T Service](s *State, key string) (T, bool) {
	srv, ok := GetState[T](s, s.serviceKey(key))
	return srv, ok
}

// MustGetService returns a service present in the application's State.
// It panics if the service is not found.
func MustGetService[T Service](s *State, key string) T {
	srv, ok := GetService[T](s, key)
	if !ok {
		panic("state: service not found!")
	}

	return srv
}
