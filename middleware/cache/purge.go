package cache

import (
	"hash/maphash"
	"math/bits"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsbytes "github.com/gofiber/utils/v2/bytes"
)

const purgeSlots = 1 << 14

const purgeFoldBuf = 128

// purgeTable records when an unsafe request last succeeded at each path, to end older entries (RFC 9111 §4.4).
// It is fixed in size and randomly keyed, so clients can neither grow it nor target a path; a collision only
// ends an entry early. Stamps count monotonic time from one wall-clock reading, so clock steps reorder nothing.
type purgeTable struct {
	slots atomic.Pointer[[purgeSlots]atomic.Uint64]
	now   func() time.Time
	start time.Time
	seed  maphash.Seed
	base  uint64 // start in nanoseconds since the epoch
}

func newPurgeTable(now func() time.Time) *purgeTable {
	start := now()
	return &purgeTable{seed: maphash.MakeSeed(), now: now, start: start, base: unixNanos(start)}
}

func (t *purgeTable) stamp() uint64 {
	elapsed := t.now().Sub(t.start)
	if elapsed < 0 {
		// A clock that keeps no monotonic reading, as a test's, set back past start.
		return t.base
	}
	return t.base + uint64(elapsed)
}

// unsafeMethod reports whether method is unsafe, unknown ones included (RFC 9110 §9.2.1, RFC 10008).
func unsafeMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery:
		return false
	default:
		return true
	}
}

// slot folds case and trailing slashes, as the router does by default: better to end too many entries.
func (t *purgeTable) slot(host, path string) uint64 {
	if path = utils.TrimRight(path, '/'); path == "" {
		path = "/"
	}
	var buf [purgeFoldBuf]byte
	h := maphash.Bytes(t.seed, utilsbytes.UnsafeToLower(append(buf[:0], host...)))
	p := maphash.Bytes(t.seed, utilsbytes.UnsafeToLower(append(buf[:0], path...)))
	return (h ^ bits.RotateLeft64(p, 32)) & (purgeSlots - 1)
}

func unixNanos(t time.Time) uint64 {
	if nano := t.UnixNano(); nano > 0 {
		return uint64(nano)
	}
	return 0
}

func (t *purgeTable) mark(slot, at uint64) {
	slots := t.slots.Load()
	if slots == nil {
		t.slots.CompareAndSwap(nil, new([purgeSlots]atomic.Uint64))
		slots = t.slots.Load()
	}
	raise(&slots[slot], at)
}

// supersedes reports whether a write succeeded at the path after e's request began; any write ends an unstamped e.
func (t *purgeTable) supersedes(host, path string, e *item) bool {
	slots := t.slots.Load()
	if slots == nil {
		return false
	}
	last := slots[t.slot(host, path)].Load()
	return last != 0 && e.stored <= last
}
