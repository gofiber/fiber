// Package memory Is a copy of the storage memory from the external storage packet as a purpose to test the behavior
// in the unittests when using a storages from these packets
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
)

// Storage interface that is implemented by storage providers
type Storage struct {
	db         map[string]entry
	done       chan struct{}
	gcInterval time.Duration
	mux        sync.RWMutex
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

// Get value by key
func (s *Storage) Get(key string) ([]byte, error) {
	if len(key) == 0 {
		return nil, nil
	}
	s.mux.RLock()
	v, ok := s.db[key]
	s.mux.RUnlock()
	if !ok || v.expiry != 0 && v.expiry <= utils.Timestamp() {
		return nil, nil
	}

	return v.data, nil
}

func (s *Storage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Continue execution if context is not done
	}

	return s.Get(key)
}

// Set key with value
func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	// Ain't Nobody Got Time For That
	if len(key) == 0 || len(val) == 0 {
		return nil
	}

	var expire uint32
	if exp != 0 {
		expire = uint32(exp.Seconds()) + utils.Timestamp()
	}

	e := entry{data: val, expiry: expire}
	s.mux.Lock()
	s.db[key] = e
	s.mux.Unlock()
	return nil
}

// SetWithContext sets key with value
func (s *Storage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue execution if context is not done
	}

	return s.Set(key, val, exp)
}

// Delete key by key
func (s *Storage) Delete(key string) error {
	// Ain't Nobody Got Time For That
	if len(key) == 0 {
		return nil
	}
	s.mux.Lock()
	delete(s.db, key)
	s.mux.Unlock()
	return nil
}

func (s *Storage) DeleteWithContext(ctx context.Context, key string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue execution if context is not done
	}

	return s.Delete(key)
}

// Reset all keys
func (s *Storage) Reset() error {
	ndb := make(map[string]entry)
	s.mux.Lock()
	s.db = ndb
	s.mux.Unlock()
	return nil
}

// ResetWithContext resets all keys with a context
func (s *Storage) ResetWithContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue execution if context is not done
	}

	return s.Reset()
}

// Close the memory storage
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
			ts := utils.Timestamp()
			expired = expired[:0]
			s.mux.RLock()
			for id, v := range s.db {
				if v.expiry != 0 && v.expiry < ts {
					expired = append(expired, id)
				}
			}
			s.mux.RUnlock()
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

// Return database client
func (s *Storage) Conn() map[string]entry {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.db
}

// Return all the keys
func (s *Storage) Keys() ([][]byte, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	if len(s.db) == 0 {
		return nil, nil
	}

	ts := utils.Timestamp()
	keys := make([][]byte, 0, len(s.db))
	for key, v := range s.db {
		// Filter out the expired keys
		if v.expiry == 0 || v.expiry > ts {
			keys = append(keys, []byte(key))
		}
	}

	// Double check if no valid keys were found
	if len(keys) == 0 {
		return nil, nil
	}

	return keys, nil
}
