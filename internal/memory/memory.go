// Package memory Is a slight copy of the memory storage, but far from the storage interface it can not only work with bytes
// but directly store any kind of data without having to encode it each time, which gives a huge speed advantage
package memory

import (
	"sync"
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
	if !ok || v.e != 0 && v.e <= utils.Timestamp.Load() {
		return nil
	}
	return v.v
}

// Set key with value
func (s *Storage) Set(key string, val interface{}, ttl time.Duration) {
	var exp uint32
	if ttl > 0 {
		exp = uint32(ttl.Seconds()) + utils.Timestamp.Load()
	}
	s.Lock()
	s.data[key] = item{exp, val}
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
	s.Lock()
	s.data = make(map[string]item)
	s.Unlock()
}

func (s *Storage) gc(sleep time.Duration) {
	ticker := time.NewTicker(sleep)
	defer ticker.Stop()
	var expired []string

	for {
		select {
		case <-ticker.C:
			expired = expired[:0]
			s.RLock()
			for key, v := range s.data {
				if v.e != 0 && v.e <= utils.Timestamp.Load() {
					expired = append(expired, key)
				}
			}
			s.RUnlock()
			s.Lock()
			for i := range expired {
				delete(s.data, expired[i])
			}
			s.Unlock()
		}
	}
}
