package cache

import (
	"hash/maphash"
	"math/bits"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// purgeSlots is how many paths a purgeTable tells apart: a power of two, so a hash picks a slot with a mask.
const purgeSlots = 1 << 14

// purgeTable remembers when a request that is not safe last succeeded at a path. A cache must invalidate
// the target URI when it gets a non-error response to one (RFC 9111 §4.4), and an entry stored by a
// request that began before the change may hold what it was before.
//
// A fixed table indexed by a keyed hash of the host and the path rather than a record for each path: the
// client chooses the paths of its requests, and a record apiece would grow with them, while a record
// only for the paths that hold an entry would have to know them. Two paths that share a slot share what a
// write to either does, which ends an entry early and never keeps one. The seed is random for every
// table, so a client cannot pick a path that lands in the slot of another. The slots are allocated at the
// first write, so an app that never sees one pays a pointer load per hit.
type purgeTable struct {
	slots atomic.Pointer[[purgeSlots]atomic.Uint64]
	seed  maphash.Seed
}

func newPurgeTable() *purgeTable {
	return &purgeTable{seed: maphash.MakeSeed()}
}

// unsafeMethod reports whether a request of the method may change what the server holds: any method that
// is not one of the safe ones (RFC 9110 §9.2.1, and QUERY, RFC 10008), unknown ones included (RFC 9111 §4.4).
func unsafeMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery:
		return false
	default:
		return true
	}
}

// slot is where the writes to a path are kept. The host and path are folded to lower case: a write to
// "/Item" may well have changed what "/item" holds, and ending too many entries costs a request, where
// ending too few serves one that is out of date.
func (t *purgeTable) slot(host, path string) uint64 {
	h := maphash.String(t.seed, utilsstrings.ToLower(host))
	p := maphash.String(t.seed, utilsstrings.ToLower(path))
	return (h ^ bits.RotateLeft64(p, 32)) & (purgeSlots - 1)
}

// unixNanos is t in nanoseconds since the epoch, zero before it.
func unixNanos(t time.Time) uint64 {
	if nano := t.UnixNano(); nano > 0 {
		return uint64(nano)
	}
	return 0
}

// mark notes that a request that is not safe succeeded at the path in slot at the time, which is when
// its response was ready: every request that began before it may have read what it changed.
func (t *purgeTable) mark(slot, at uint64) {
	slots := t.slots.Load()
	if slots == nil {
		t.slots.CompareAndSwap(nil, new([purgeSlots]atomic.Uint64))
		slots = t.slots.Load()
	}
	raise(&slots[slot], at)
}

// supersedes reports whether a request that is not safe succeeded at the path of the request after the
// request that stored e began, so that e is not the response to what the path holds now. An entry that
// carries no such time, one an earlier version stored, is superseded by any write there, as it cannot say.
func (t *purgeTable) supersedes(host, path string, e *item) bool {
	slots := t.slots.Load()
	if slots == nil {
		return false
	}
	last := slots[t.slot(host, path)].Load()
	return last != 0 && e.stored <= last
}
