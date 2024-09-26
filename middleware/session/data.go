package session

import (
	"sync"
)

// msgp -file="data.go" -o="data_msgp.go" -tests=true -unexported
//
//go:generate msgp -o=data_msgp.go -tests=true -unexported
type data struct {
	Data         map[any]any
	sync.RWMutex `msg:"-"`
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
		return d
	}
	// Handle unexpected type in the pool
	panic("unexpected type in data pool")
}

// Reset clears the data map and resets the data object.
//
// Usage:
//
//	d.Reset()
func (d *data) Reset() {
	d.Lock()
	defer d.Unlock()
	d.Data = make(map[any]any)
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
	return d.Data[key]
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
