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

	// currentInterval tracks the current sleep time.
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
func (e *ExponentialBackoff) Retry(f func() error) error {
	if e.currentInterval <= 0 {
		e.currentInterval = e.InitialInterval
	}
	var err error
	for i := 0; i < e.MaxRetryCount; i++ {
		err = f()
		if err == nil {
			return nil
		}
		next := e.next()
		time.Sleep(next)
	}
	return err
}

// next calculates the next sleeping time interval.
func (e *ExponentialBackoff) next() time.Duration {
	// generate a random value between [0, 1000)
	n, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return e.MaxBackoffTime
	}
	t := e.currentInterval + (time.Duration(n.Int64()) * time.Millisecond)
	e.currentInterval = time.Duration(float64(e.currentInterval) * e.Multiplier)
	if t >= e.MaxBackoffTime {
		e.currentInterval = e.MaxBackoffTime
		return e.MaxBackoffTime
	}
	return t
}
