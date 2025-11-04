// Package memory Is a copy of the storage memory from the external storage packet as a purpose to test the behavior
// in the unittests when using a storages from these packets
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
)

// Storage provides an in-memory implementation of the storage interface for
// testing purposes.
type Storage struct {
	db         map[string]Entry
	done       chan struct{}
	gcInterval time.Duration
	mux        sync.RWMutex
}

// Entry represents a value stored in memory along with its expiration.
type Entry struct {
	data []byte
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	expiry uint32
}

// New creates a new memory storage.
func New(config ...Config) *Storage {
	// Set default config
	cfg := configDefault(config...)

	// Create storage
	store := &Storage{
		db:         make(map[string]Entry),
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
	if len(key) == 0 {
		return nil, nil
	}
	s.mux.RLock()
	v, ok := s.db[key]
	s.mux.RUnlock()

	if !ok || v.expiry != 0 && v.expiry <= utils.Timestamp() {
		return nil, nil
	}

	// Return a copy to prevent callers from mutating stored data
	return utils.CopyBytes(v.data), nil
}

// GetWithContext retrieves the value for the given key while honoring context
// cancellation.
func (s *Storage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if err := wrapContextError(ctx, "get"); err != nil {
		return nil, err
	}
	return s.Get(key)
}

// Set saves val under key and schedules it to expire after exp. A zero exp keeps
// the entry indefinitely.
func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	// Ain't Nobody Got Time For That
	if len(key) == 0 || len(val) == 0 {
		return nil
	}

	var expire uint32
	if exp != 0 {
		expire = uint32(exp.Seconds()) + utils.Timestamp()
	}

	// Copy both key and value to avoid unsafe reuse from sync.Pool
	keyCopy := utils.CopyString(key)
	valCopy := utils.CopyBytes(val)

	e := Entry{data: valCopy, expiry: expire}
	s.mux.Lock()
	s.db[keyCopy] = e
	s.mux.Unlock()
	return nil
}

// SetWithContext sets the value for the given key while honoring context
// cancellation.
func (s *Storage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if err := wrapContextError(ctx, "set"); err != nil {
		return err
	}
	return s.Set(key, val, exp)
}

// Delete removes the value stored for key.
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

// DeleteWithContext removes the value for the given key while honoring
// context cancellation.
func (s *Storage) DeleteWithContext(ctx context.Context, key string) error {
	if err := wrapContextError(ctx, "delete"); err != nil {
		return err
	}
	return s.Delete(key)
}

// Reset clears all keys and values from the storage map.
func (s *Storage) Reset() error {
	ndb := make(map[string]Entry)
	s.mux.Lock()
	s.db = ndb
	s.mux.Unlock()
	return nil
}

// ResetWithContext clears all stored keys while honoring context
// cancellation.
func (s *Storage) ResetWithContext(ctx context.Context) error {
	if err := wrapContextError(ctx, "reset"); err != nil {
		return err
	}
	return s.Reset()
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
			ts := utils.Timestamp()
			expired = expired[:0]
			s.mux.RLock()
			for id, v := range s.db {
				if v.expiry != 0 && v.expiry < ts {
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
func (s *Storage) Conn() map[string]Entry {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.db
}

// Keys returns all keys stored in the memory storage.
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

func wrapContextError(ctx context.Context, op string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memory storage %s: %w", op, err)
	}
	return nil
}
