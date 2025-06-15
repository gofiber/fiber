package idempotency

import (
	"time"
)

// stubLock implements Locker for testing purposes.
type stubLock struct {
	lockErr   error
	unlockErr error
	afterLock func()
}

func (s *stubLock) Lock(string) error {
	if s.afterLock != nil {
		s.afterLock()
	}
	return s.lockErr
}
func (s *stubLock) Unlock(string) error { return s.unlockErr }

// stubStorage implements fiber.Storage for testing.
type stubStorage struct {
	data     map[string][]byte
	getErr   error
	setErr   error
	setCount int
}

func (s *stubStorage) Get(key string) ([]byte, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.data == nil {
		return nil, nil
	}
	return s.data[key], nil
}

func (s *stubStorage) Set(key string, val []byte, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.data == nil {
		s.data = make(map[string][]byte)
	}
	s.data[key] = val
	s.setCount++
	return nil
}

func (s *stubStorage) Delete(key string) error {
	if s.data != nil {
		delete(s.data, key)
	}
	return nil
}

func (s *stubStorage) Reset() error {
	s.data = make(map[string][]byte)
	return nil
}

func (_ *stubStorage) Close() error { return nil }
