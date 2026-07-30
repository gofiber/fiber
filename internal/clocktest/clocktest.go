// Package clocktest lets tests wait on the cached, second-granularity clock
// that fiber's memory storages and rate-limit windows are measured against.
package clocktest

import (
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
