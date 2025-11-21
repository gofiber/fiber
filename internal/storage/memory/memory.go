// Package memory Is a copy of the storage memory from the external storage packet as a purpose to test the behavior
// in the unittests when using a storages from these packets
package memory

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2/utils"
)

// Storage interface that is implemented by storage providers
type Storage struct {
	mux        sync.RWMutex
	db         map[string]entry
	gcInterval time.Duration
	done       chan struct{}
}

type entry struct {
	data []byte
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	expiry uint32
}

// New creates a new memory storage
func New(config ...Config) *Storage {
	// Set default config
	cfg := configDefault(config...)

	// Create storage
	store := &Storage{
		db:         make(map[string]entry),
		gcInterval: cfg.GCInterval,
		done:       make(chan struct{}),
	}

	// Start garbage collector
	utils.StartTimeStampUpdater()
	go store.gc()

	return store
}

// Get returns the stored value for key, ignoring missing or expired entries by
// returning nil.
func (s *Storage) Get(key string) ([]byte, error) {
	if len(key) <= 0 {
		return nil, nil
	}
	s.mux.RLock()
	v, ok := s.db[key]
	s.mux.RUnlock()
	if !ok || v.expiry != 0 && v.expiry <= atomic.LoadUint32(&utils.Timestamp) {
		return nil, nil
	}

	// Return a copy to prevent callers from mutating stored data
	return utils.CopyBytes(v.data), nil
}

// Set saves val under key and schedules it to expire after exp. A zero exp keeps
// the entry indefinitely.
func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 || len(val) <= 0 {
		return nil
	}

	var expire uint32
	if exp != 0 {
		expire = uint32(exp.Seconds()) + atomic.LoadUint32(&utils.Timestamp)
	}

	// Copy both key and value to avoid unsafe reuse from sync.Pool
	keyCopy := utils.CopyString(key)
	valCopy := utils.CopyBytes(val)

	e := entry{data: valCopy, expiry: expire}
	s.mux.Lock()
	s.db[keyCopy] = e
	s.mux.Unlock()
	return nil
}

// Delete removes the value stored for key.
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

// Reset clears all keys and values from the storage map.
func (s *Storage) Reset() error {
	ndb := make(map[string]entry)
	s.mux.Lock()
	s.db = ndb
	s.mux.Unlock()
	return nil
}

// Close stops the background garbage collector and releases resources
// associated with the storage instance.
func (s *Storage) Close() error {
	s.done <- struct{}{}
	return nil
}

func (s *Storage) gc() {
	ticker := time.NewTicker(s.gcInterval)
	defer ticker.Stop()
	var expired []string

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			ts := atomic.LoadUint32(&utils.Timestamp)
			expired = expired[:0]
			s.mux.RLock()
			for id, v := range s.db {
				if v.expiry != 0 && v.expiry <= ts {
					expired = append(expired, id)
				}
			}
			s.mux.RUnlock()

			if len(expired) == 0 {
				// avoid locking if nothing to delete
				continue
			}

			s.mux.Lock()
			// Double-checked locking.
			// We might have replaced the item in the meantime.
			for i := range expired {
				v := s.db[expired[i]]
				if v.expiry != 0 && v.expiry <= ts {
					delete(s.db, expired[i])
				}
			}
			s.mux.Unlock()
		}
	}
}

// Conn returns the underlying storage map. The map must not be modified by
// callers.
func (s *Storage) Conn() map[string]entry {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.db
}
