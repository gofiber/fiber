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

// purgeSlots is a power of two, so a hash picks a slot with a mask.
const purgeSlots = 1 << 14

// purgeFoldBuf is the stack room slot folds a host or path in; a longer one is folded on the heap.
const purgeFoldBuf = 128

// purgeTable records when an unsafe request last succeeded at each path, ending entries whose request began
// earlier (RFC 9111 §4.4). A fixed table under a randomly keyed hash of host and path, so clients can neither
// grow it nor aim at another path's slot; a collision only ends an entry early. Stamps add monotonic time to
// one wall-clock reading, so clock steps cannot reorder writes and entries.
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

// stamp is the time on the table's clock, in nanoseconds since the epoch.
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

// slot is where the writes to a path are kept. Case and trailing slashes are folded, as the router does by
// default: ending too many entries costs a request, ending too few serves a stale one.
func (t *purgeTable) slot(host, path string) uint64 {
	if path = utils.TrimRight(path, '/'); path == "" {
		path = "/"
	}
	// Folded on the stack, so that hits do not allocate.
	var buf [purgeFoldBuf]byte
	h := maphash.Bytes(t.seed, utilsbytes.UnsafeToLower(append(buf[:0], host...)))
	p := maphash.Bytes(t.seed, utilsbytes.UnsafeToLower(append(buf[:0], path...)))
	return (h ^ bits.RotateLeft64(p, 32)) & (purgeSlots - 1)
}

// unixNanos is t in nanoseconds since the epoch, zero before it.
func unixNanos(t time.Time) uint64 {
	if nano := t.UnixNano(); nano > 0 {
		return uint64(nano)
	}
	return 0
}

// mark records an unsafe request's success at the path in slot, stamped when its response was ready.
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
