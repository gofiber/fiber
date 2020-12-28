package memory

import (
	"sync"
	"sync/atomic"
	"time"
)

type Storage struct {
	sync.RWMutex
	data map[string]item // data
	ts   uint64          // timestamp
}

type item struct {
	v interface{} // val
	e uint64      // exp
}

func New() *Storage {
	store := &Storage{
		data: make(map[string]item),
		ts:   uint64(time.Now().Unix()),
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
	if !ok || v.e != 0 && v.e <= atomic.LoadUint64(&s.ts) {
		return nil
	}
	return v.v
}

// Set key with value
func (s *Storage) Set(key string, val interface{}, ttl time.Duration) {
	var exp uint64
	if ttl > 0 {
		exp = uint64(ttl.Seconds()) + atomic.LoadUint64(&s.ts)
	}
	s.Lock()
	s.data[key] = item{val, exp}
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
		atomic.StoreUint64(&s.ts, uint64(time.Now().Unix()))
	}
}
func (s *Storage) gc(sleep time.Duration) {
	expired := []string{}
	for {
		time.Sleep(sleep)
		expired = expired[:0]
		s.RLock()
		for key, v := range s.data {
			if v.e != 0 && v.e <= atomic.LoadUint64(&s.ts) {
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
