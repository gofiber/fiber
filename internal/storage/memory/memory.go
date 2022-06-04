package memory

import (
	"sync"
	"time"
)

// Storage interface that is implemented by storage providers
type Storage struct {
	mux        sync.RWMutex
	db         map[string]entry
	gcInterval time.Duration
	done       chan struct{}
}

type entry struct {
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	expiry uint32
	data   []byte
}

// New creates a new memory storage
func New(interval ...time.Duration) *Storage {
	// Custom gc interval
	var gcInterval time.Duration = 10
	if len(interval) > 1 {
		gcInterval = interval[0]
	}

	// Create storage
	store := &Storage{
		db:         make(map[string]entry),
		gcInterval: gcInterval * time.Second,
		done:       make(chan struct{}),
	}

	// Start garbage collector
	go store.gc()

	return store
}

// Get value by key
func (s *Storage) Get(key string) ([]byte, error) {
	if len(key) <= 0 {
		return nil, nil
	}
	s.mux.RLock()
	v, ok := s.db[key]
	s.mux.RUnlock()
	if !ok || v.expiry != 0 && v.expiry <= uint32(time.Now().Unix()) {
		return nil, nil
	}

	return v.data, nil
}

// Set key with value
func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 || len(val) <= 0 {
		return nil
	}

	var expire uint32
	if exp != 0 {
		expire = uint32(time.Now().Add(exp).Unix())
	}

	s.mux.Lock()
	s.db[key] = entry{expire, val}
	s.mux.Unlock()
	return nil
}

// Delete key by key
func (s *Storage) Delete(key string) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 {
		return nil
	}
	s.mux.Lock()
	delete(s.db, key)
	s.mux.Unlock()
	return nil
}

// Reset all keys
func (s *Storage) Reset() error {
	s.mux.Lock()
	s.db = make(map[string]entry)
	s.mux.Unlock()
	return nil
}

// Close the memory storage
func (s *Storage) Close() error {
	s.done <- struct{}{}
	return nil
}

func (s *Storage) gc() {
	ticker := time.NewTicker(s.gcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case t := <-ticker.C:
			now := uint32(t.Unix())
			s.mux.Lock()
			for id, v := range s.db {
				if v.expiry != 0 && v.expiry < now {
					delete(s.db, id)
				}
			}
			s.mux.Unlock()
		}
	}
}
