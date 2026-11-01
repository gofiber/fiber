// Package clocktest lets tests wait on the cached, second-granularity clock
// that fiber's memory storages and rate-limit windows are measured against, and
// provides a manually advanced clock for code that takes its time source.
package clocktest

import (
	"sync"
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
)

// SleepPast waits out ttl in wall time and then blocks until utils.Timestamp()
// has advanced by ttl truncated to whole seconds, the granularity the storages
// compare expiry against.
//
// Expiry is compared against that cached timestamp, and the goroutine
// refreshing it once per second can be scheduled well over a second late on a
// loaded machine, so a wall-clock sleep alone leaves entries live past their
// TTL often enough to turn CI red.
func SleepPast(tb testing.TB, ttl time.Duration) {
	tb.Helper()
	utils.StartTimeStampUpdater() // no-op when already running
	from := utils.Timestamp()

	time.Sleep(ttl + 100*time.Millisecond)

	want := from + uint32(ttl.Seconds())
	deadline := time.Now().Add(10 * time.Second)
	for utils.Timestamp() < want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	// A silent return here would surface as a confusing "not expired" assertion
	if now := utils.Timestamp(); now < want {
		tb.Fatalf("cached clock stuck at %d, want %d after waiting out %s", now, want, ttl)
	}
}

// Clock is a manually advanced clock, safe for the concurrent reads a handler
// performs. Tests that depend on a TTL or window boundary advance it with Add
// instead of sleeping past the boundary, which is racy under -race and -shuffle.
// Hand its Now method to the code under test as the time source.
type Clock struct {
	now time.Time
	// Step, when set before the clock is shared, is added to the clock on every
	// Now, so a read of the clock the code under test did not need shows up as
	// elapsed time.
	Step time.Duration
	mu   sync.Mutex
}

// NewClock returns a Clock that reads start until it is advanced.
func NewClock(start time.Time) *Clock { return &Clock{now: start} }

// Now advances the clock by Step and returns the time it then reads.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(c.Step)
	return c.now
}

// Add moves the clock forward by d.
func (c *Clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
