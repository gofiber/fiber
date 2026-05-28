// Package memory provides a high-performance in-memory storage that can store
// any type without encoding overhead. Unlike the standard storage interface,
// this storage works directly with Go types for maximum speed.
//
// # Safety Considerations
//
// This storage automatically performs defensive copying for:
//   - String keys: Copied to prevent corruption from pooled buffers
//   - []byte values: Copied on both Set and Get to prevent external mutation
//
// For other types (structs, ints, etc.), Go's value semantics provide natural
// protection. However, if storing pointers or slices of non-byte types,
// callers are responsible for not mutating the underlying data.
//
// This storage is primarily used internally by middleware for performance-
// critical operations where the stored data types are known and controlled.
package memory

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/utils/v2"
)

// Storage stores arbitrary values in memory for use in tests and benchmarks.
// Storage is safe for concurrent use.
type Storage struct {
	data   map[string]item // data
	lastGC atomic.Uint32
	mu     sync.RWMutex
}

type item struct {
	v any // val
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	e uint32 // exp
}

// New constructs an in-memory Storage initialized with a background GC loop.
func New() *Storage {
	store := &Storage{
		data: make(map[string]item),
	}
	utils.StartTimeStampUpdater()
	store.lastGC.Store(utils.Timestamp())
	return store
}

// Get retrieves the value stored under key, returning nil when the entry does
// not exist or has expired.
//
// For []byte values, this returns a defensive copy to prevent callers from
// mutating the stored data. Other types are returned as-is.
func (s *Storage) Get(key string) any {
	ts := utils.Timestamp()
	s.maybeGC(ts)

	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	if !ok || v.e != 0 && v.e <= ts {
		return nil
	}

	// Defensive copy for byte slices to prevent external mutation
	if b, ok := v.v.([]byte); ok {
		return utils.CopyBytes(b)
	}

	return v.v
}

// Set stores val under key and applies the optional ttl before expiring the
// entry. A non-positive ttl keeps the item forever.
//
// String keys are defensively copied to prevent corruption from pooled buffers.
// []byte values are also copied to prevent external mutation of stored data.
// Other types are stored as-is (structs are copied by value automatically).
func (s *Storage) Set(key string, val any, ttl time.Duration) {
	ts := utils.Timestamp()
	s.maybeGC(ts)

	var exp uint32
	if ttl > 0 {
		exp = uint32(ttl.Seconds()) + ts
	}

	// Defensive copies to prevent unsafe reuse from sync.Pool
	keyCopy := utils.CopyString(key)

	// Copy byte slices to prevent external mutation
	if b, ok := val.([]byte); ok {
		val = utils.CopyBytes(b)
	}

	i := item{e: exp, v: val}
	s.mu.Lock()
	s.data[keyCopy] = i
	s.mu.Unlock()
}

// Delete removes key and its associated value from the storage.
func (s *Storage) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}

// Reset clears the storage by dropping every stored key.
func (s *Storage) Reset() {
	nd := make(map[string]item)
	s.mu.Lock()
	s.data = nd
	s.mu.Unlock()
}

func (s *Storage) maybeGC(ts uint32) {
	for {
		lastGC := s.lastGC.Load()
		if ts <= lastGC {
			return
		}
		if s.lastGC.CompareAndSwap(lastGC, ts) {
			break
		}
	}

	s.mu.Lock()
	for key, v := range s.data {
		if v.e != 0 && v.e <= ts {
			delete(s.data, key)
		}
	}
	s.mu.Unlock()
}
