package session

import (
	"sync"
)

// msgp -file="data.go" -o="data_msgp.go" -tests=true -unexported
//
//go:generate msgp -o=data_msgp.go -tests=true -unexported
type data struct {
	Data         map[string]any
	sync.RWMutex `msg:"-"`
}

var dataPool = sync.Pool{
	New: func() any {
		d := new(data)
		d.Data = make(map[string]any)
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
	return dataPool.Get().(*data) //nolint:forcetypeassert // We store nothing else in the pool
}

// Reset clears the data map and resets the data object.
//
// Usage:
//
//	d.Reset()
func (d *data) Reset() {
	d.Lock()
	d.Data = make(map[string]any)
	d.Unlock()
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
func (d *data) Get(key string) any {
	d.RLock()
	v := d.Data[key]
	d.RUnlock()
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
func (d *data) Set(key string, value any) {
	d.Lock()
	d.Data[key] = value
	d.Unlock()
}

// Delete removes a key-value pair from the data map.
//
// Parameters:
//   - key: The key to delete.
//
// Usage:
//
//	d.Delete("key")
func (d *data) Delete(key string) {
	d.Lock()
	delete(d.Data, key)
	d.Unlock()
}

// Keys retrieves all keys in the data map.
//
// Returns:
//   - []string: A slice of all keys in the data map.
//
// Usage:
//
//	keys := d.Keys()
func (d *data) Keys() []string {
	d.Lock()
	keys := make([]string, 0, len(d.Data))
	for k := range d.Data {
		keys = append(keys, k)
	}
	d.Unlock()
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
	return len(d.Data)
}
