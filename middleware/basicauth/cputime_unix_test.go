//go:build linux || darwin

package basicauth

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cpuTime returns the CPU time this process has used so far. Unlike the wall
// clock it does not advance while other processes hold the CPUs.
func cpuTime(tb testing.TB) time.Duration {
	tb.Helper()
	var usage syscall.Rusage
	require.NoError(tb, syscall.Getrusage(syscall.RUSAGE_SELF, &usage))
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}
