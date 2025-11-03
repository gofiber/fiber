// Package memory Is a slight copy of the memory storage, but far from the storage interface it can not only work with bytes
// but directly store any kind of data without having to encode it each time, which gives a huge speed advantage
package memory

import (
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
)

// Storage stores arbitrary values in memory for use in tests and benchmarks.
type Storage struct {
	data map[string]item // data
	sync.RWMutex
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
	go store.gc(1 * time.Second)
	return store
}

// Get retrieves the value stored under key, returning nil when the entry does
// not exist or has expired.
func (s *Storage) Get(key string) any {
	s.RLock()
	v, ok := s.data[key]
	s.RUnlock()
	if !ok || v.e != 0 && v.e <= utils.Timestamp() {
		return nil
	}
	return v.v
}

// Set stores val under key and applies the optional ttl before expiring the
// entry. A non-positive ttl keeps the item forever.
func (s *Storage) Set(key string, val any, ttl time.Duration) {
	var exp uint32
	if ttl > 0 {
		exp = uint32(ttl.Seconds()) + utils.Timestamp()
	}
	i := item{e: exp, v: val}
	s.Lock()
	s.data[key] = i
	s.Unlock()
}

// Delete removes key and its associated value from the storage.
func (s *Storage) Delete(key string) {
	s.Lock()
	delete(s.data, key)
	s.Unlock()
}

// Reset clears the storage by dropping every stored key.
func (s *Storage) Reset() {
	nd := make(map[string]item)
	s.Lock()
	s.data = nd
	s.Unlock()
}

func (s *Storage) gc(sleep time.Duration) {
	ticker := time.NewTicker(sleep)
	defer ticker.Stop()
	var expired []string

	for range ticker.C {
		ts := utils.Timestamp()
		expired = expired[:0]
		s.RLock()
		for key, v := range s.data {
			if v.e != 0 && v.e <= ts {
				expired = append(expired, key)
			}
		}
		s.RUnlock()

		if len(expired) == 0 {
			// avoid locking if nothing to delete
			continue
		}

		s.Lock()
		// Double-checked locking.
		// We might have replaced the item in the meantime.
		for i := range expired {
			v := s.data[expired[i]]
			if v.e != 0 && v.e <= ts {
				delete(s.data, expired[i])
			}
		}
		s.Unlock()
	}
}
