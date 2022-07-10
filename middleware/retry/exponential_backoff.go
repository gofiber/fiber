package retry

import (
	"math/rand"
	"time"
)

// ExponentialBackoff is a retry mechanism for retrying some calls.
type ExponentialBackoff struct {
	InitialInterval time.Duration
	MaxBackoffTime  time.Duration
	Multiplier      float64
	MaxRetryCount   int
	currentInterval time.Duration
}

const (
	DefaultInitialInterval = 1 * time.Second
	DefaultMaxBackoffTime  = 32 * time.Second
	DefaultMultiplier      = 2.0
	DefaultMaxRetryCount   = 10
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// NewExponentialBackoff creates a ExponentialBackoff with default values.
func NewExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialInterval: DefaultInitialInterval,
		MaxBackoffTime:  DefaultMaxBackoffTime,
		Multiplier:      DefaultMultiplier,
		MaxRetryCount:   DefaultMaxRetryCount,
		currentInterval: DefaultInitialInterval,
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
	// add random value between [0, 1000)
	t := e.currentInterval + (time.Duration(rand.Int63n(1000)) * time.Millisecond)
	e.currentInterval = time.Duration(float64(e.currentInterval) * e.Multiplier)
	if t >= e.MaxBackoffTime {
		e.currentInterval = e.MaxBackoffTime
		return e.MaxBackoffTime
	}
	return t
}
