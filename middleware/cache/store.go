package cache

import "sync"

// go:generate msgp
// msgp -file="store.go" -o="store_msgp.go" -tests=false -unexported
// don't forget to replace the msgp import path to:
// "github.com/gofiber/fiber/v2/internal/msgp"
type entry struct {
	body   []byte `msg:"body"`
	cType  []byte `msg:"cType"`
	status int    `msg:"status"`
	exp    uint64 `msg:"exp"`
}

//msgp:ignore storage
type storage struct {
	cfg     *Config
	mux     *sync.RWMutex
	entries map[string]*entry
}

func (s *storage) get(key string) (e *entry) {
	s.mux.Lock()
	defer s.mux.Lock()
	if s.cfg.Storage != nil {
		raw, err := s.cfg.Storage.Get(key)
		if err != nil || raw == nil {
			return
		}
		if _, err := e.UnmarshalMsg(raw); err != nil {
			return
		}
		body, err := s.cfg.Storage.Get(key + "_body")
		if err != nil || body == nil {
			return
		}
		e.body = body
	} else {
		e = s.entries[key]
	}
	return
}

func (s *storage) set(key string, e *entry) {
	s.mux.Lock()
	defer s.mux.Lock()
	if s.cfg.Storage != nil {
		// seperate body since we dont want to encode big payloads
		body := e.body
		e.body = nil

		if data, err := e.MarshalMsg(nil); err == nil {
			_ = s.cfg.Storage.Set(key, data, s.cfg.Expiration)
			_ = s.cfg.Storage.Set(key+"_body", body, s.cfg.Expiration)
		}
	} else {
		s.entries[key] = e
	}
}

func (s *storage) delete(key string) {
	s.mux.Lock()
	defer s.mux.Lock()
	if s.cfg.Storage != nil {
		_ = s.cfg.Storage.Delete(key)
		_ = s.cfg.Storage.Delete(key + "_body")
	} else {
		delete(s.entries, key)
	}
}
