package idempotency

import (
	"context"
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

func (s *stubStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	// Call Get method to avoid code duplication
	return s.Get(key)
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

func (s *stubStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	// Call Set method to avoid code duplication
	return s.Set(key, val, 0)
}

func (s *stubStorage) Delete(key string) error {
	if s.data != nil {
		delete(s.data, key)
	}
	return nil
}

func (s *stubStorage) DeleteWithContext(_ context.Context, key string) error {
	// Call Delete method to avoid code duplication
	return s.Delete(key)
}

func (s *stubStorage) Reset() error {
	s.data = make(map[string][]byte)
	return nil
}

func (s *stubStorage) ResetWithContext(_ context.Context) error {
	// Call Reset method to avoid code duplication
	return s.Reset()
}

func (*stubStorage) Close() error { return nil }
