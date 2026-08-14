package session

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// msgp -file="data.go" -o="data_msgp.go" -tests=true -unexported
// Session state should remain small to fit common storage payload limits.
//
//go:generate msgp -o=data_msgp.go -tests=true -unexported
//msgp:ignore data
type data struct {
	Data         map[any]any // Session key counts are expected to be bounded.
	sync.RWMutex `msg:"-"`

	// dirty is set when Data may no longer match the snapshot it was decoded from
	dirty atomic.Bool
}

// valueMayAlias reports whether v could share memory with the stored session
// data, meaning the caller may mutate the session through it without calling
// Set. Scalars are safe; everything else is treated as aliasing.
func valueMayAlias(v any) bool {
	switch v.(type) {
	case nil,
		string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64, complex64, complex128,
		time.Time, time.Duration:
		return false
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		// named scalar types are plain copies as well
		return false
	default:
		return true
	}
}

var dataPool = sync.Pool{
	New: func() any {
		d := new(data)
		d.Data = make(map[any]any)
		return d
	},
}

// acquireData returns a new data object from the pool.
//
// Returns:
//   - *data: The data object.
//
// Usage:
//
//	d := acquireData()
func acquireData() *data {
	obj := dataPool.Get()
	if d, ok := obj.(*data); ok {
		d.Reset()
		return d
	}
	// Handle unexpected type in the pool
	panic("unexpected type in data pool")
}

// releaseData resets the data object and returns it to the pool.
// d must not be used after calling this function.
// If d is nil, releaseData is a no-op.
func releaseData(d *data) {
	if d == nil {
		return
	}
	d.Reset()
	dataPool.Put(d)
}

// Reset clears the data map and resets the data object.
//
// Usage:
//
//	d.Reset()
func (d *data) Reset() {
	d.Lock()
	defer d.Unlock()
	clear(d.Data)
	d.dirty.Store(true)
}

// Get retrieves a value from the data map by key.
//
// Parameters:
//   - key: The key to retrieve.
//
// Returns:
//   - any: The value associated with the key.
//
// Usage:
//
//	value := d.Get("key")
func (d *data) Get(key any) any {
	d.RLock()
	defer d.RUnlock()
	v := d.Data[key]
	if valueMayAlias(v) {
		// the caller may mutate the session through this value
		d.dirty.Store(true)
	}
	return v
}

// Set updates or creates a new key-value pair in the data map.
//
// Parameters:
//   - key: The key to set.
//   - value: The value to set.
//
// Usage:
//
//	d.Set("key", "value")
func (d *data) Set(key, value any) {
	d.Lock()
	defer d.Unlock()
	d.Data[key] = value
	d.dirty.Store(true)
}

// Delete removes a key-value pair from the data map.
//
// Parameters:
//   - key: The key to delete.
//
// Usage:
//
//	d.Delete("key")
func (d *data) Delete(key any) {
	d.Lock()
	defer d.Unlock()
	delete(d.Data, key)
	d.dirty.Store(true)
}

// Keys retrieves all keys in the data map.
//
// Returns:
//   - []any: A slice of all keys in the data map.
//
// Usage:
//
//	keys := d.Keys()
func (d *data) Keys() []any {
	d.RLock()
	defer d.RUnlock()
	keys := make([]any, 0, len(d.Data))
	for k := range d.Data {
		// pointer keys can be mutated by the caller just like values
		if valueMayAlias(k) {
			d.dirty.Store(true)
		}
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of key-value pairs in the data map.
//
// Returns:
//   - int: The number of key-value pairs.
//
// Usage:
//
//	length := d.Len()
func (d *data) Len() int {
	d.RLock()
	defer d.RUnlock()
	return len(d.Data)
}
