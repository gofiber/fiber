package idempotency

// stubLock implements Locker for testing purposes.
type stubLock struct {
	lockErr   error
	unlockErr error
	afterLock func()
	locked    []string // the keys Lock was called with
}

func (s *stubLock) Lock(key string) error {
	s.locked = append(s.locked, key)
	if s.afterLock != nil {
		s.afterLock()
	}
	return s.lockErr
}
func (s *stubLock) Unlock(string) error { return s.unlockErr }
