package limiter

import (
	"github.com/gofiber/fiber/v2/internal/mapstore"
)

// go:generate msgp
// msgp -file="store.go" -o="store_msgp.go" -tests=false -unexported
// don't forget to replace the msgp import path to:
// "github.com/gofiber/fiber/v2/internal/msgp"
type entry struct {
	hits int    `msg:"hits"`
	exp  uint64 `msg:"exp"`
}

//msgp:ignore storage
type storage struct {
	cfg   *Config
	store *mapstore.Storage
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
func (s *storage) get(key string) (e entry) {
	if s.cfg.Storage != nil {
		raw, err := s.cfg.Storage.Get(key)
		if err != nil || raw == nil {
			return
		}
		if _, err := e.UnmarshalMsg(raw); err != nil {
			return
		}
		return
	} else {
		// val := s.mem.Get(key).(*entry)
		var ok bool
		e, ok = s.store.Get(key).(entry)
		if !ok {
			return
		}

	}
	return
}

func (s *storage) set(key string, e entry) {
	if s.cfg.Storage != nil {
		if data, err := e.MarshalMsg(nil); err == nil {
			_ = s.cfg.Storage.Set(key, data, s.cfg.Expiration)
		}
	} else {
		s.store.Set(key, e, s.cfg.Expiration)
	}
}

func (s *storage) delete(key string) {
	if s.cfg.Storage != nil {
		_ = s.cfg.Storage.Delete(key)
	} else {
		s.store.Delete(key)
	}
}
