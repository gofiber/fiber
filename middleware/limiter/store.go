package limiter

import (
	"sync"
	"time"
)

// Storage interface implemented by providers
type Storage interface {
	// Get session value. If the ID is not found, this function should return
	// []byte{}, nil and not an error.
	Get(id string) ([]byte, error)
	// Set session value. `exp` will be zero for no duration.
	Set(id string, value []byte, exp time.Duration) error
	// Delete session value
	Delete(id string) error
	// Clear clears the store
	Clear() error
}

type defaultStore struct {
	stmap map[string][]byte
	mutex sync.Mutex
}

func (s *defaultStore) Get(id string) ([]byte, error) {
	s.mutex.Lock()
	val, ok := s.stmap[id]
	s.mutex.Unlock()
	if !ok {
		return []byte{}, nil
	} else {
		return val, nil
	}
}

func (s *defaultStore) Set(id string, val []byte, _ time.Duration) error {
	s.mutex.Lock()
	s.stmap[id] = val
	s.mutex.Unlock()

	return nil
}

func (s *defaultStore) Clear() error {
	s.mutex.Lock()
	s.stmap = map[string][]byte{}
	s.mutex.Unlock()

	return nil
}

func (s *defaultStore) Delete(id string) error {
	s.mutex.Lock()
	_, ok := s.stmap[id]
	if ok {
		delete(s.stmap, id)
	}
	s.mutex.Unlock()

	return nil
}
