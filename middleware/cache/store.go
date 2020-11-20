package cache

import (
	"github.com/gofiber/fiber/v2/internal/mapstore"
)

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
	cfg   *Config
	store *mapstore.MapStore
}

func newStorage(cfg *Config) *storage {
	store := &storage{
		cfg: cfg,
	}
	if cfg.Storage == nil {
		store.store = mapstore.New()
	}
	return store
}
func (s *storage) get(key string) *entry {
	if s.cfg.Storage != nil {
		raw, err := s.cfg.Storage.Get(key)
		if err != nil || raw == nil {
			return nil
		}
		e := &entry{}
		if _, err := e.UnmarshalMsg(raw); err != nil {
			return nil
		}
		body, err := s.cfg.Storage.Get(key + "_body")
		if err != nil || body == nil {
			return nil
		}
		e.body = body
		return e
	} else {
		val := s.store.Get(key)
		if val != nil {
			return val.(*entry)
		}
	}
	return nil
}

func (s *storage) set(key string, e *entry) {
	if s.cfg.Storage != nil {
		// seperate body since we dont want to encode big payloads
		body := e.body
		e.body = nil

		if data, err := e.MarshalMsg(nil); err == nil {
			_ = s.cfg.Storage.Set(key, data, s.cfg.Expiration)
			_ = s.cfg.Storage.Set(key+"_body", body, s.cfg.Expiration)
		}
	} else {
		s.store.Set(key, e, s.cfg.Expiration)
	}
}

func (s *storage) delete(key string) {
	if s.cfg.Storage != nil {
		_ = s.cfg.Storage.Delete(key)
		_ = s.cfg.Storage.Delete(key + "_body")
	} else {
		s.store.Delete(key)
	}
}
