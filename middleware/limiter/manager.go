package limiter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/memory"
	"github.com/gofiber/utils/v2"
)

// msgp -file="manager.go" -o="manager_msgp.go" -tests=false -unexported
//
//go:generate msgp -o=manager_msgp.go -tests=false -unexported
type item struct {
	currHits int
	prevHits int
	exp      uint64
}

//msgp:ignore manager
type manager struct {
	pool             sync.Pool
	memory           *memory.Storage
	storage          fiber.Storage
	shouldRedactKeys bool
}

const redactedKey = "[redacted]"

// maxKeyLength bounds a rate-limit key before it ever reaches storage or the
// in-memory map.
//
// KeyGenerator's default returns c.IP(), which — with app.Config.TrustProxy
// and ProxyHeader set and the (default) EnableIPValidation left off — is the
// raw ProxyHeader value verbatim: an attacker-controlled string with no
// length limit (see the ProxyHeader "Behavior note" in docs/api/fiber.md). A
// custom KeyGenerator can return any other request-derived value just as
// easily. Left unbounded, a client can grow the backing store, or the
// unbounded in-memory map on the default path, by an amount of its own
// choosing on every request with a fresh value.
//
// Bounded here, at the one seam both window strategies call through, so
// neither strategy nor the in-memory fallback nor a future one needs to
// remember to do it. 192 matches middleware/cache's own per-segment bound
// (maxKeyDimensionSegmentLength) for the same reason cache picked it: real
// keys are far shorter, so this only ever engages against abuse.
const maxKeyLength = 192

// hashPrefix namespaces a bounded key that was too long to keep verbatim.
// Same literal cache/keygen.go uses, so an operator inspecting either
// middleware's storage sees the same shape; the two are not unified into a
// shared helper because cache's bound also escapes delimiters between
// multiple concatenated segments, a concern this single-segment key has none
// of.
const hashPrefix = "sha256:"

// boundKey caps key at maxKeyLength, replacing it with a deterministic digest
// when it is too long or already claims the reserved hashPrefix namespace —
// otherwise a short adversarial key could collide with a genuinely-hashed
// long one. Hashing rather than truncating or rejecting keeps distinct
// oversized inputs mapped to distinct buckets and keeps the request rate
// limited rather than either exempted or itself denied.
func boundKey(key string) string {
	if len(key) <= maxKeyLength && !strings.HasPrefix(key, hashPrefix) {
		return key
	}
	hash := sha256.Sum256(utils.UnsafeBytes(key))
	return hashPrefix + hex.EncodeToString(hash[:])
}

func newManager(storage fiber.Storage, shouldRedactKeys bool) *manager {
	// Create new storage handler
	manager := &manager{
		pool: sync.Pool{
			New: func() any {
				return new(item)
			},
		},
		shouldRedactKeys: shouldRedactKeys,
	}
	if storage != nil {
		// Use provided storage if provided
		manager.storage = storage
	} else {
		// Fallback too memory storage
		manager.memory = memory.New()
	}
	return manager
}

// acquire returns an *entry from the sync.Pool
func (m *manager) acquire() *item {
	return m.pool.Get().(*item) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
}

// release and reset *entry to sync.Pool
func (m *manager) release(e *item) {
	e.prevHits = 0
	e.currHits = 0
	e.exp = 0
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(ctx context.Context, key string) (*item, error) {
	key = boundKey(key)
	if m.storage != nil {
		raw, err := m.storage.GetWithContext(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("limiter: failed to get key %q from storage: %w", m.logKey(key), err)
		}
		if raw != nil {
			it := m.acquire()
			if _, err := it.UnmarshalMsg(raw); err != nil {
				m.release(it)
				return nil, fmt.Errorf("limiter: failed to unmarshal key %q: %w", m.logKey(key), err)
			}
			return it, nil
		}
		return m.acquire(), nil
	}

	value := m.memory.Get(key)
	if value == nil {
		return m.acquire(), nil
	}

	it, ok := value.(*item)
	if !ok {
		return nil, fmt.Errorf("limiter: unexpected entry type %T for key %q", value, m.logKey(key))
	}

	return it, nil
}

// set data to storage or memory
func (m *manager) set(ctx context.Context, key string, it *item, exp time.Duration) error {
	key = boundKey(key)
	if m.storage != nil {
		raw, err := it.MarshalMsg(nil)
		if err != nil {
			m.release(it)
			return fmt.Errorf("limiter: failed to marshal key %q: %w", m.logKey(key), err)
		}
		if err := m.storage.SetWithContext(ctx, key, raw, exp); err != nil {
			m.release(it)
			return fmt.Errorf("limiter: failed to store key %q: %w", m.logKey(key), err)
		}
		m.release(it)
		return nil
	}

	m.memory.Set(key, it, exp)
	return nil
}

func (m *manager) logKey(key string) string {
	if m.shouldRedactKeys {
		return redactedKey
	}
	return key
}
