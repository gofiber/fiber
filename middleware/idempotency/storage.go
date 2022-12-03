package idempotency

import (
	"sync"
	"time"
)

type Storage interface {
	Get(key string) ([]byte, error)
	Set(key string, val []byte, lifetime time.Duration) error
}

type MemoryStorage struct {
	mu sync.RWMutex

	items map[string]*item
}

type item struct {
	mu sync.RWMutex

	val []byte
	eol time.Time
}

func (s *MemoryStorage) Get(key string) ([]byte, error) {
	s.mu.RLock()
	itm, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}

	itm.mu.RLock()
	eol := itm.eol
	val := itm.val
	itm.mu.RUnlock()

	// TODO: Make this faster by not calling time.Now()
	if time.Now().After(eol) {
		// Expired item
		return nil, nil
	}

	return val, nil
}

func (s *MemoryStorage) Set(key string, val []byte, lifetime time.Duration) error {
	newitm := &item{
		val: val,
		// TODO: Make this faster by not calling time.Now()
		eol: time.Now().Add(lifetime),
	}

	s.mu.RLock()
	itm, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		// Double-checked locking.
		s.mu.Lock()
		itm, ok = s.items[key]
		if !ok {
			s.items[key] = newitm
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
	}

	itm.mu.Lock()
	itm.val = newitm.val
	itm.eol = newitm.eol
	itm.mu.Unlock()

	return nil
}

func NewMemoryStorage(cleanupInterval time.Duration) *MemoryStorage {
	s := &MemoryStorage{
		items: make(map[string]*item),
	}

	// Periodically clean up expired keys
	go func() {
		cleanup := func(now time.Time) {
			var expired []string
			s.mu.RLock()
			for key, itm := range s.items {
				itm.mu.RLock()
				eol := itm.eol
				itm.mu.RUnlock()
				if now.After(eol) {
					expired = append(expired, key)
				}
			}
			s.mu.RUnlock()

			if len(expired) == 0 {
				return
			}

			// Double-checked locking.
			for _, key := range expired {
				s.mu.Lock()
				itm, ok := s.items[key]
				if ok {
					itm.mu.RLock()
					eol := itm.eol
					itm.mu.RUnlock()
					if now.After(eol) {
						// Still expired
						delete(s.items, key)
					}
				}
				s.mu.Unlock()
			}
		}

		t := time.NewTicker(cleanupInterval)
		defer t.Stop()

		for {
			now := <-t.C
			cleanup(now)
		}
	}()

	return s
}

var _ Storage = (*MemoryStorage)(nil)

// TODO(leon): Ensure that all types satisfying the "idempotency.Storage" interface also implement the "fiber.Storage" interface.
