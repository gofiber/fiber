// Package memory Is a slight copy of the memory storage, but far from the storage interface it can not only work with bytes
// but directly store any kind of data without having to encode it each time, which gives a huge speed advantage
package memory

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2/utils"
)

type Storage struct {
	sync.RWMutex
	data map[string]item // data
}

type item struct {
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	e uint32      // exp
	v interface{} // val
}

func New() *Storage {
	store := &Storage{
		data: make(map[string]item),
	}
	utils.StartTimeStampUpdater()
	go store.gc(1 * time.Second)
	return store
}

// Get value by key
func (s *Storage) Get(key string) interface{} {
	s.RLock()
	v, ok := s.data[key]
	s.RUnlock()
	if !ok || v.e != 0 && v.e <= atomic.LoadUint32(&utils.Timestamp) {
		return nil
	}
	return v.v
}

// Set key with value
func (s *Storage) Set(key string, val interface{}, ttl time.Duration) {
	var exp uint32
	if ttl > 0 {
		exp = uint32(ttl.Seconds()) + atomic.LoadUint32(&utils.Timestamp)
	}
	i := item{exp, val}
	s.Lock()
	s.data[key] = i
	s.Unlock()
}

// Delete key by key
func (s *Storage) Delete(key string) {
	s.Lock()
	delete(s.data, key)
	s.Unlock()
}

// Reset all keys
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
		ts := atomic.LoadUint32(&utils.Timestamp)
		expired = expired[:0]
		s.RLock()
		for key, v := range s.data {
			if v.e != 0 && v.e <= ts {
				expired = append(expired, key)
			}
		}
		s.RUnlock()
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
