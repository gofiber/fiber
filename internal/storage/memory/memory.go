// Package memory Is a copy of the storage memory from the external storage packet as a purpose to test the behavior
// in the unittests when using a storages from these packets
package memory

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
)

const numShards = 32

// Storage provides an in-memory implementation of the storage interface for
// testing purposes.
// Storage is safe for concurrent use, except when callers keep using the live
// map returned by Conn. Access to that map requires external synchronization.

type Storage struct {
	shards    []*Shard
	done      chan struct{}
	closeOnce sync.Once
}

// Shard represents a shard of the storage.
type Shard struct {
	db         map[string]Entry
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

	//Implementation of shards
	shards := make([]*Shard, numShards)
	for i := range numShards {
		shards[i] = &Shard{
			db:         make(map[string]Entry),
			gcInterval: cfg.GCInterval,
			mux:        sync.RWMutex{},
		}
	}

	// Create storage
	store := &Storage{
		shards: shards,
		done:   make(chan struct{}),
	}

	// Start garbage collector
	utils.StartTimeStampUpdater()
	go store.gc()

	return store
}

// Get returns the stored value for key, ignoring missing or expired entries by
// returning nil.
func (s *Storage) Get(key string) ([]byte, error) {
	if key == "" {
		return nil, nil
	}

	shardID := getHash(key) % numShards
	s.shards[shardID].mux.RLock()

	v, ok := s.shards[shardID].db[key]

	s.shards[shardID].mux.RUnlock()
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
	if key == "" || len(val) == 0 {
		return nil
	}

	var expire uint32
	if exp != 0 {
		expire = uint32(exp.Seconds()) + utils.Timestamp()
	}

	shardID := getHash(key) % numShards

	// Copy both key and value to avoid unsafe reuse from sync.Pool
	keyCopy := utils.CopyString(key)
	valCopy := utils.CopyBytes(val)

	e := Entry{data: valCopy, expiry: expire}
	s.shards[shardID].mux.Lock()
	s.shards[shardID].db[keyCopy] = e
	s.shards[shardID].mux.Unlock()
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
	if key == "" {
		return nil
	}

	getShardID := getHash(key) % numShards
	s.shards[getShardID].mux.Lock()
	delete(s.shards[getShardID].db, key)
	s.shards[getShardID].mux.Unlock()
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
	wg := &sync.WaitGroup{}

	for _, shard := range s.shards {
		wg.Add(1)
		go func(shrd *Shard) {
			defer wg.Done()

			shrd.mux.Lock()
			shrd.db = make(map[string]Entry)
			shrd.mux.Unlock()
		}(shard)
	}

	wg.Wait()
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
	s.closeOnce.Do(func() {
		close(s.done)
	})
	return nil
}

func (s *Storage) gc() {
	ticker := time.NewTicker(s.shards[0].gcInterval)
	defer ticker.Stop()
	var expired []string
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			ts := utils.Timestamp()

			for _, shard := range s.shards {
				expired = expired[:0]
				shard.mux.RLock()
				for id, v := range shard.db {
					if v.expiry != 0 && v.expiry < ts {
						expired = append(expired, id)
					}
				}
				shard.mux.RUnlock()

				if len(expired) == 0 {
					// avoid locking if nothing to delete
					continue
				}
				shard.mux.Lock()
				for _, key := range expired {
					v, ok := shard.db[key]
					if ok && v.expiry != 0 && v.expiry <= ts {
						delete(shard.db, key)
					}
				}
				shard.mux.Unlock()
			}

		}
	}

}

// Conn returns the underlying storage map. The returned map remains shared with
// the storage, so callers must not modify it and must synchronize any access
// that overlaps with other storage operations.
func (s *Storage) Conn() map[string]Entry {
	var allocatedMapLen = 0
	for _, shard := range s.shards {
		shard.mux.RLock()
		allocatedMapLen += len(shard.db)
		shard.mux.RUnlock()
	}
	mergedMaps := make(map[string]Entry, allocatedMapLen)
	for _, shard := range s.shards {
		shard.mux.RLock()
		for k, v := range shard.db {
			mergedMaps[k] = v
		}
		shard.mux.RUnlock()
	}
	return mergedMaps
}

// Keys returns all keys stored in the memory storage.
func (s *Storage) Keys() ([][]byte, error) {
	wg := &sync.WaitGroup{}
	var keysLen = 0
	for _, shard := range s.shards {
		shard.mux.RLock()
		keysLen += len(shard.db)
		shard.mux.RUnlock()
	}

	//  check if no valid keys were found
	if keysLen == 0 {
		return nil, nil
	}
	localKeys := make([][][]byte, numShards)
	ts := utils.Timestamp()
	for i, shard := range s.shards {
		wg.Add(1)
		go func(idx int, shrd *Shard) {
			defer wg.Done()
			shrd.mux.RLock()
			defer shrd.mux.RUnlock()
			for key, v := range shrd.db {
				// Filter out the expired keys
				if v.expiry == 0 || v.expiry > ts {
					localKeys[idx] = append(localKeys[idx], []byte(key))
				}
			}
		}(i, shard)

	}

	wg.Wait()

	keys := make([][]byte, 0, keysLen)
	for _, shardKeys := range localKeys {
		keys = append(keys, shardKeys...)
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

func getHash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}
