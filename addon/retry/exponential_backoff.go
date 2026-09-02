package retry

import (
	"crypto/rand"
	"math/big"
	"time"
)

// ExponentialBackoff is a retry mechanism for retrying some calls.
type ExponentialBackoff struct {
	// InitialInterval is the initial time interval for backoff algorithm.
	InitialInterval time.Duration

	// MaxBackoffTime is the maximum time duration for backoff algorithm. It limits
	// the maximum sleep time.
	MaxBackoffTime time.Duration

	// Multiplier is a multiplier number of the backoff algorithm.
	Multiplier float64

	// MaxRetryCount is the maximum number of retry count.
	MaxRetryCount int

	// currentInterval is the interval a Retry call starts from when
	// InitialInterval is not set. It is never modified by Retry, so one
	// ExponentialBackoff can serve concurrent callers.
	currentInterval time.Duration
}

// NewExponentialBackoff creates a ExponentialBackoff with default values.
func NewExponentialBackoff(config ...Config) *ExponentialBackoff {
	cfg := configDefault(config...)
	return &ExponentialBackoff{
		InitialInterval: cfg.InitialInterval,
		MaxBackoffTime:  cfg.MaxBackoffTime,
		Multiplier:      cfg.Multiplier,
		MaxRetryCount:   cfg.MaxRetryCount,
		currentInterval: cfg.currentInterval,
	}
}

// Retry is the core logic of the retry mechanism. If the calling function returns
// nil as an error, then the Retry method is terminated with returning nil. Otherwise,
// if all function calls are returned error, then the method returns this error.
//
// Every call starts over at InitialInterval, and the state it advances is its
// own, so a single ExponentialBackoff can be shared by concurrent callers.
func (e *ExponentialBackoff) Retry(f func() error) error {
	interval := e.InitialInterval
	if interval <= 0 {
		interval = e.currentInterval
	}
	var err error
	for i := 0; i < e.MaxRetryCount; i++ {
		err = f()
		if err == nil {
			return nil
		}
		if i < e.MaxRetryCount-1 {
			var sleep time.Duration
			sleep, interval = e.next(interval)
			time.Sleep(sleep)
		}
	}
	return err
}

// next calculates the sleeping time for the current interval and the interval
// the following attempt starts from.
func (e *ExponentialBackoff) next(current time.Duration) (sleep, following time.Duration) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the pair for clarity
	// generate a random value between [0, 1000)
	n, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return e.MaxBackoffTime, e.MaxBackoffTime
	}
	t := current + (time.Duration(n.Int64()) * time.Millisecond)
	following = time.Duration(float64(current) * e.Multiplier)
	if t >= e.MaxBackoffTime {
		return e.MaxBackoffTime, e.MaxBackoffTime
	}
	return t, following
}
