package retry

import (
	"crypto/rand"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_ExponentialBackoff_Retry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		expErr     error
		expBackoff *ExponentialBackoff
		f          func() error
		name       string
	}{
		{name: "With default values - successful", expBackoff: NewExponentialBackoff(), f: func() error { return nil }},
		{name: "Successful function", f: func() error { return nil }, expBackoff: &ExponentialBackoff{
			InitialInterval: 1 * time.Millisecond, MaxBackoffTime: 100 * time.Millisecond,
			Multiplier: 2.0, MaxRetryCount: 5,
		}},
		{name: "Unsuccessful function", expErr: errors.New("failed function"), expBackoff: &ExponentialBackoff{
			InitialInterval: 2 * time.Millisecond, MaxBackoffTime: 100 * time.Millisecond,
			Multiplier: 2.0, MaxRetryCount: 5,
		}, f: func() error { return errors.New("failed function") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.expBackoff.Retry(tt.f)
			require.Equal(t, tt.expErr, err)
		})
	}
}

// flaky is a function to retry: call fails with err on its first fails calls,
// succeeds after them, and counts every call.
type flaky struct {
	err   error
	fails int
	calls int
}

func (f *flaky) call() error {
	f.calls++
	if f.calls <= f.fails {
		return f.err
	}
	return nil
}

// Test_ExponentialBackoff_Retry_ZeroValue covers an ExponentialBackoff that was
// not built by NewExponentialBackoff. MaxRetryCount is the number of attempts, so
// a zero one made Retry return nil without calling f: success reported for work
// that never ran. Every field left at zero takes the default instead.
func Test_ExponentialBackoff_Retry_ZeroValue(t *testing.T) {
	t.Parallel()
	t.Run("the zero value calls f", func(t *testing.T) {
		t.Parallel()
		var backoff ExponentialBackoff
		f := &flaky{}
		require.NoError(t, backoff.Retry(f.call))
		require.Equal(t, 1, f.calls)
		require.Equal(t, ExponentialBackoff{}, backoff, "Retry must leave a shared value alone")
	})

	t.Run("the zero value reports a failure instead of success", func(t *testing.T) {
		t.Parallel()
		// Fast intervals so the default ten attempts do not wait out the default
		// one to thirty-two seconds; MaxRetryCount stays unset.
		backoff := &ExponentialBackoff{InitialInterval: time.Microsecond, MaxBackoffTime: time.Millisecond}
		boom := errors.New("boom")
		f := &flaky{fails: math.MaxInt, err: boom}
		require.ErrorIs(t, backoff.Retry(f.call), boom)
		require.Equal(t, DefaultConfig.MaxRetryCount, f.calls)
	})

	t.Run("a negative attempt count is unset too", func(t *testing.T) {
		t.Parallel()
		backoff := &ExponentialBackoff{
			InitialInterval: time.Microsecond,
			MaxBackoffTime:  time.Millisecond,
			Multiplier:      2,
			MaxRetryCount:   -3,
		}
		f := &flaky{fails: math.MaxInt, err: errors.New("again")}
		require.Error(t, backoff.Retry(f.call))
		require.Equal(t, DefaultConfig.MaxRetryCount, f.calls)
	})

	t.Run("a zero max backoff does not turn the retries into a busy loop", func(t *testing.T) {
		t.Parallel()
		backoff := &ExponentialBackoff{InitialInterval: 10 * time.Millisecond, MaxRetryCount: 3}
		start := time.Now()
		f := &flaky{fails: math.MaxInt, err: errors.New("again")}
		require.Error(t, backoff.Retry(f.call))
		require.Equal(t, 3, f.calls)
		// Two waits of at least the initial interval, which a zero cap used to
		// shrink to nothing.
		require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	})
}

func Test_ExponentialBackoff_withDefaults(t *testing.T) {
	t.Parallel()
	defaults := ExponentialBackoff{
		InitialInterval: DefaultConfig.InitialInterval,
		MaxBackoffTime:  DefaultConfig.MaxBackoffTime,
		Multiplier:      DefaultConfig.Multiplier,
		MaxRetryCount:   DefaultConfig.MaxRetryCount,
	}

	tests := []struct {
		name string
		in   ExponentialBackoff
		want ExponentialBackoff
	}{
		{name: "zero value", want: defaults},
		{
			name: "negative numbers count as unset",
			in:   ExponentialBackoff{InitialInterval: -1, MaxBackoffTime: -1, Multiplier: -1, MaxRetryCount: -1},
			want: defaults,
		},
		{
			name: "values that are set are kept",
			in:   ExponentialBackoff{InitialInterval: time.Second * 3, MaxBackoffTime: time.Minute, Multiplier: 1.5, MaxRetryCount: 4},
			want: ExponentialBackoff{InitialInterval: time.Second * 3, MaxBackoffTime: time.Minute, Multiplier: 1.5, MaxRetryCount: 4},
		},
		{
			name: "an unset initial interval starts from the current one",
			in:   ExponentialBackoff{currentInterval: 7 * time.Second},
			want: ExponentialBackoff{
				InitialInterval: 7 * time.Second,
				MaxBackoffTime:  defaults.MaxBackoffTime,
				Multiplier:      defaults.Multiplier,
				MaxRetryCount:   defaults.MaxRetryCount,
				currentInterval: 7 * time.Second,
			},
		},
		{
			name: "one built by NewExponentialBackoff is unchanged",
			in:   *NewExponentialBackoff(Config{InitialInterval: 2 * time.Second, MaxRetryCount: 6}),
			want: ExponentialBackoff{
				InitialInterval: 2 * time.Second,
				MaxBackoffTime:  defaults.MaxBackoffTime,
				Multiplier:      defaults.Multiplier,
				MaxRetryCount:   6,
				currentInterval: 2 * time.Second,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := tc.in
			require.Equal(t, tc.want, in.withDefaults())
			require.Equal(t, tc.in, in, "the receiver must not change")
		})
	}
}

func Test_ExponentialBackoff_Retry_NoSleepAfterLastAttempt(t *testing.T) {
	t.Parallel()
	const (
		largeInterval = 5 * time.Second // would be used for sleep if bug existed
		maxAcceptable = 2 * time.Second // Retry must return well before largeInterval
	)

	eb := &ExponentialBackoff{
		InitialInterval: largeInterval,
		MaxBackoffTime:  largeInterval * 2,
		Multiplier:      2.0,
		MaxRetryCount:   1,
	}

	start := time.Now()
	err := eb.Retry(func() error { return errors.New("only attempt") })
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Equal(t, "only attempt", err.Error())
	require.Less(t, elapsed, maxAcceptable,
		"Retry must not sleep after the last failed attempt; took %v (expected < %v)", elapsed, maxAcceptable)
}

func Test_ExponentialBackoff_Next(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		expBackoff  *ExponentialBackoff
		nextSeconds []int // the least each sleep may be, in seconds
	}{
		{
			name:        "With default values",
			expBackoff:  NewExponentialBackoff(),
			nextSeconds: []int{1, 2, 4, 8, 16, 32, 32, 32, 32, 32},
		},
		{
			name: "Custom values",
			expBackoff: &ExponentialBackoff{
				InitialInterval: 2.0 * time.Second,
				MaxBackoffTime:  64 * time.Second,
				Multiplier:      3.0,
				MaxRetryCount:   8,
				currentInterval: 2.0 * time.Second,
			},
			nextSeconds: []int{2, 6, 18, 54, 64, 64, 64, 64},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			interval := tt.expBackoff.currentInterval
			for i := range tt.expBackoff.MaxRetryCount {
				var next time.Duration
				next, interval = tt.expBackoff.next(interval)
				want := time.Duration(tt.nextSeconds[i]) * time.Second
				if next < want || next > want+1*time.Second {
					t.Errorf("wrong next time:\nactual:%v\nexpected range:%v-%v\n", next, want, want+1*time.Second)
				}
			}
		})
	}
}

func Test_ExponentialBackoff_NextRandFailure(t *testing.T) {
	// Not parallel: it swaps crypto/rand.Reader, which the parallel tests read.
	original := rand.Reader
	defer func() { rand.Reader = original }()
	rand.Reader = failingReader{}

	expBackoff := &ExponentialBackoff{
		InitialInterval: 1 * time.Second,
		MaxBackoffTime:  10 * time.Second,
		Multiplier:      2,
		MaxRetryCount:   3,
		currentInterval: 1 * time.Second,
	}
	next, following := expBackoff.next(expBackoff.currentInterval)
	require.Equal(t, expBackoff.MaxBackoffTime, next)
	require.Equal(t, expBackoff.MaxBackoffTime, following)
	// the shared state never changes, random failure or not
	require.Equal(t, 1*time.Second, expBackoff.currentInterval)
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) { return 0, errors.New("fail") }

func Test_ExponentialBackoff_Retry_StartsFromInitialInterval(t *testing.T) {
	t.Parallel()
	backoff := NewExponentialBackoff(Config{
		InitialInterval: time.Millisecond,
		MaxBackoffTime:  50 * time.Millisecond,
		Multiplier:      100,
		MaxRetryCount:   3,
	})
	failing := func() error { return errors.New("nope") }

	require.Error(t, backoff.Retry(failing))

	// One sleep, starting over at the initial interval plus up to a second of jitter.
	start := time.Now()
	f := &flaky{fails: 1, err: errors.New("once more")}
	require.NoError(t, backoff.Retry(f.call))
	require.Equal(t, 2, f.calls)
	require.Equal(t, time.Millisecond, backoff.currentInterval, "the shared state must not drift between calls")
	require.Less(t, time.Since(start), 5*time.Second)
}

func Test_ExponentialBackoff_Retry_Concurrent(t *testing.T) {
	t.Parallel()
	backoff := NewExponentialBackoff(Config{
		InitialInterval: time.Millisecond,
		MaxBackoffTime:  2 * time.Millisecond,
		Multiplier:      2,
		MaxRetryCount:   3,
	})

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			f := &flaky{fails: 1, err: errors.New("retry")}
			require.NoError(t, backoff.Retry(f.call))
		})
	}
	wg.Wait()
}

func Test_ExponentialBackoff_Retry_UnsetInitialInterval(t *testing.T) {
	t.Parallel()
	backoff := &ExponentialBackoff{
		MaxBackoffTime:  time.Millisecond,
		Multiplier:      2,
		MaxRetryCount:   2,
		currentInterval: time.Microsecond,
	}

	f := &flaky{fails: 1, err: errors.New("once more")}
	require.NoError(t, backoff.Retry(f.call))
	require.Equal(t, 2, f.calls)
	require.Equal(t, time.Microsecond, backoff.currentInterval)
}
