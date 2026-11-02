package apptest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Recv returns the next value from ch and fails the test when none arrives
// within timeout. msgAndArgs say what the test was waiting for, as the optional
// message of a testify assertion does. Like the other helpers it stops the test
// with FailNow, so it must be called from the goroutine running the test.
func Recv[T any](tb testing.TB, ch <-chan T, timeout time.Duration, msgAndArgs ...any) T {
	tb.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(timeout):
		require.FailNow(tb, "no value arrived within "+timeout.String(), msgAndArgs...)
		panic("unreachable") // FailNow stops the test
	}
}
