package retry

import (
	"crypto/rand"
	"errors"
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
		{
			name:       "With default values - successful",
			expBackoff: NewExponentialBackoff(),
			f: func() error {
				return nil
			},
		},
		{
			name: "Successful function",
			expBackoff: &ExponentialBackoff{
				InitialInterval: 1 * time.Millisecond,
				MaxBackoffTime:  100 * time.Millisecond,
				Multiplier:      2.0,
				MaxRetryCount:   5,
			},
			f: func() error {
				return nil
			},
		},
		{
			name: "Unsuccessful function",
			expBackoff: &ExponentialBackoff{
				InitialInterval: 2 * time.Millisecond,
				MaxBackoffTime:  100 * time.Millisecond,
				Multiplier:      2.0,
				MaxRetryCount:   5,
			},
			f: func() error {
				return errors.New("failed function")
			},
			expErr: errors.New("failed function"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.expBackoff.Retry(tt.f)
			require.Equal(t, tt.expErr, err)
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
		name                 string
		expBackoff           *ExponentialBackoff
		expNextTimeIntervals []time.Duration
	}{
		{
			name:       "With default values",
			expBackoff: NewExponentialBackoff(),
			expNextTimeIntervals: []time.Duration{
				1 * time.Second,
				2 * time.Second,
				4 * time.Second,
				8 * time.Second,
				16 * time.Second,
				32 * time.Second,
				32 * time.Second,
				32 * time.Second,
				32 * time.Second,
				32 * time.Second,
			},
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
			expNextTimeIntervals: []time.Duration{
				2 * time.Second,
				6 * time.Second,
				18 * time.Second,
				54 * time.Second,
				64 * time.Second,
				64 * time.Second,
				64 * time.Second,
				64 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			interval := tt.expBackoff.currentInterval
			for i := range tt.expBackoff.MaxRetryCount {
				var next time.Duration
				next, interval = tt.expBackoff.next(interval)
				if next < tt.expNextTimeIntervals[i] || next > tt.expNextTimeIntervals[i]+1*time.Second {
					t.Errorf("wrong next time:\n"+
						"actual:%v\n"+
						"expected range:%v-%v\n",
						next, tt.expNextTimeIntervals[i], tt.expNextTimeIntervals[i]+1*time.Second)
				}
			}
		})
	}
}

func Test_ExponentialBackoff_NextRandFailure(t *testing.T) {
	// Backup original reader and restore at the end
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
	attempts := 0
	require.NoError(t, backoff.Retry(func() error {
		attempts++
		if attempts < 2 {
			return errors.New("once more")
		}
		return nil
	}))
	require.Equal(t, 2, attempts)
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
			calls := 0
			require.NoError(t, backoff.Retry(func() error {
				calls++
				if calls < 2 {
					return errors.New("retry")
				}
				return nil
			}))
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

	attempts := 0
	require.NoError(t, backoff.Retry(func() error {
		attempts++
		if attempts < 2 {
			return errors.New("once more")
		}
		return nil
	}))
	require.Equal(t, 2, attempts)
	require.Equal(t, time.Microsecond, backoff.currentInterval)
}
