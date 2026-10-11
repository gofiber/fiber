//go:build !linux && !darwin

package basicauth

import (
	"testing"
	"time"
)

var cpuTimeStart = time.Now()

// cpuTime falls back to the wall clock where process CPU time is not at hand or too coarse.
func cpuTime(testing.TB) time.Duration {
	return time.Since(cpuTimeStart)
}
