package csrf

import (
	"sync"
)

// We only use Keys in Storage, so we need a dummy value
var emptyByte = []byte{'+'}

type storage struct {
	cfg     *Config
	mux     *sync.RWMutex
	entries map[string][]byte
}

func (s *storage) get(key string) bool {
	if s.cfg.Storage != nil {
		val, err := s.cfg.Storage.Get(key)
		if err == nil && val != nil {
			return true
		}
	} else {
		s.mux.Lock()
		_, ok := s.entries[key]
		s.mux.Unlock()
		if ok {
			return true
		}
	}
	return false
}

func (s *storage) set(key string) {
	if s.cfg.Storage != nil {
		_ = s.cfg.Storage.Set(key, emptyByte, s.cfg.Expiration)
	} else {
		s.mux.Lock()
		s.entries[key] = emptyByte
		s.mux.Unlock()
	}
}

func (s *storage) delete(key string) {
	if s.cfg.Storage != nil {
		_ = s.cfg.Storage.Delete(key)
	} else {
		s.mux.Lock()
		delete(s.entries, key)
		s.mux.Unlock()
	}
}
