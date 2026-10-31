package limiter

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_secondsToDuration(t *testing.T) {
	t.Parallel()
	d, ok := secondsToDuration(5)
	require.True(t, ok)
	require.Equal(t, 5*time.Second, d)

	const maxSeconds = uint64(math.MaxInt64 / int64(time.Second))
	d, ok = secondsToDuration(maxSeconds + 1)
	require.False(t, ok)
	require.Equal(t, time.Duration(math.MaxInt64), d)
}

func Test_ttlDuration(t *testing.T) {
	t.Parallel()
	require.Equal(t, 3*time.Second, ttlDuration(1, 2))

	const maxSeconds = uint64(math.MaxInt64 / int64(time.Second))

	require.Equal(t, time.Duration(math.MaxInt64), ttlDuration(maxSeconds+1, 1))

	require.Equal(t, time.Duration(math.MaxInt64), ttlDuration(1, maxSeconds+1))

	// The sum overflows although each fits.
	require.Equal(t, time.Duration(math.MaxInt64), ttlDuration(maxSeconds, maxSeconds))
}

// A pre-epoch clock is clamped to 0 instead of wrapping to a huge uint64.
func Test_Config_currentSecond_NegativeClock(t *testing.T) {
	t.Parallel()
	cfg := &Config{clock: func() time.Time { return time.Unix(-100, 0) }}
	require.Equal(t, uint64(0), cfg.currentSecond())

	cfg.clock = func() time.Time { return time.Unix(42, 0) }
	require.Equal(t, uint64(42), cfg.currentSecond())
}

func Test_windowSeconds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		d          time.Duration
		configured time.Duration
		want       uint64
	}{
		{name: "whole seconds", d: 30 * time.Second, configured: time.Minute, want: 30},
		{name: "fractions of a second are dropped", d: 2500 * time.Millisecond, configured: time.Minute, want: 2},
		{name: "sub-second floors to one", d: 500 * time.Millisecond, configured: time.Minute, want: 1},
		{name: "zero falls back to the configured window", d: 0, configured: 10 * time.Second, want: 10},
		{name: "negative falls back to the configured window", d: -time.Second, configured: 10 * time.Second, want: 10},
		{name: "a sub-second configured window floors to one", d: 0, configured: 500 * time.Millisecond, want: 1},
		{name: "nothing configured uses the default minute", d: 0, configured: 0, want: 60},
		{name: "negative everywhere uses the default minute", d: -1, configured: -1, want: 60},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, windowSeconds(tc.d, tc.configured))
		})
	}
}
