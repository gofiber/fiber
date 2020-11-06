package sessions

import (
	"errors"
	"sync"
	"time"
)

// copy of https://github.com/gofiber/storage/tree/main/memory
type memory struct {
	mux        sync.RWMutex
	db         map[string]memoryEntry
	gcInterval time.Duration
	done       chan struct{}
}

var errNotExist = errors.New("key does not exist")

type memoryEntry struct {
	data   []byte
	expiry int64
}

func memoryStorage() *memory {
	// Create storage
	store := &memory{
		db:         make(map[string]memoryEntry),
		gcInterval: 10 * time.Second,
		done:       make(chan struct{}),
	}

	// Start garbage collector
	go store.gc()

	return store
}

// Get value by key
func (s *memory) Get(key string) ([]byte, error) {
	s.mux.RLock()
	v, ok := s.db[key]
	s.mux.RUnlock()
	if !ok || v.expiry != 0 && v.expiry <= time.Now().Unix() {
		return nil, errNotExist
	}

	return v.data, nil
}

// Set key with value
func (s *memory) Set(key string, val []byte, exp time.Duration) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 || len(val) <= 0 {
		return nil
	}

	var expire int64
	if exp != 0 {
		expire = time.Now().Add(exp).Unix()
	}

	s.mux.Lock()
	s.db[key] = memoryEntry{val, expire}
	s.mux.Unlock()
	return nil
}

// Delete key by key
func (s *memory) Delete(key string) error {
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
func (s *memory) Reset() error {
	s.mux.Lock()
	s.db = make(map[string]memoryEntry)
	s.mux.Unlock()
	return nil
}

// Close the memory storage
func (s *memory) Close() error {
	s.done <- struct{}{}
	return nil
}

func (s *memory) gc() {
	ticker := time.NewTicker(s.gcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case t := <-ticker.C:
			now := t.Unix()
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
