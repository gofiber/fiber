package memory

import (
	"sync"
	"sync/atomic"
	"time"
)

type Storage struct {
	sync.RWMutex
	data map[string]item // data
	ts   uint32          // timestamp
}

type item struct {
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	e uint32      // exp
	v interface{} // val
}

func New() *Storage {
	store := &Storage{
		data: make(map[string]item),
		ts:   uint32(time.Now().Unix()),
	}
	go store.gc(10 * time.Millisecond)
	go store.updater(1 * time.Second)
	return store
}

// Get value by key
func (s *Storage) Get(key string) interface{} {
	s.RLock()
	v, ok := s.data[key]
	s.RUnlock()
	if !ok || v.e != 0 && v.e <= atomic.LoadUint32(&s.ts) {
		return nil
	}
	return v.v
}

// Set key with value
func (s *Storage) Set(key string, val interface{}, ttl time.Duration) {
	var exp uint32
	if ttl > 0 {
		exp = uint32(ttl.Seconds()) + atomic.LoadUint32(&s.ts)
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

func (s *Storage) updater(sleep time.Duration) {
	for {
		time.Sleep(sleep)
		atomic.StoreUint32(&s.ts, uint32(time.Now().Unix()))
	}
}
func (s *Storage) gc(sleep time.Duration) {
	expired := []string{}
	for {
		time.Sleep(sleep)
		expired = expired[:0]
		s.RLock()
		for key, v := range s.data {
			if v.e != 0 && v.e <= atomic.LoadUint32(&s.ts) {
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
