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

	// currentInterval is the starting interval when InitialInterval is unset; Retry never modifies it.
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

// withDefaults returns a copy of e in which every field left unset takes the value
// NewExponentialBackoff gives it, so that a zero ExponentialBackoff, or a struct
// literal that names only some of the fields, retries like the default instead
// of never calling f. A zero or negative number counts as unset. e itself is not
// changed, since one ExponentialBackoff can be shared.
func (e *ExponentialBackoff) withDefaults() ExponentialBackoff {
	eff := *e
	if eff.InitialInterval <= 0 {
		eff.InitialInterval = eff.currentInterval
	}
	if eff.InitialInterval <= 0 {
		eff.InitialInterval = DefaultConfig.InitialInterval
	}
	if eff.MaxBackoffTime <= 0 {
		eff.MaxBackoffTime = DefaultConfig.MaxBackoffTime
	}
	if eff.Multiplier <= 0 {
		eff.Multiplier = DefaultConfig.Multiplier
	}
	if eff.MaxRetryCount <= 0 {
		eff.MaxRetryCount = DefaultConfig.MaxRetryCount
	}
	return eff
}

// Retry is the core logic of the retry mechanism. If the calling function returns
// nil as an error, then the Retry method is terminated with returning nil. Otherwise,
// if all function calls are returned error, then the method returns this error.
//
// f is called at least once: a field left at its zero value takes its default, so
// even a zero ExponentialBackoff makes up to DefaultConfig.MaxRetryCount attempts.
//
// Each call starts from InitialInterval with its own state, so one ExponentialBackoff can be shared.
func (e *ExponentialBackoff) Retry(f func() error) error {
	eff := e.withDefaults()
	interval := eff.InitialInterval
	var err error
	for i := 0; i < eff.MaxRetryCount; i++ {
		err = f()
		if err == nil {
			return nil
		}
		if i < eff.MaxRetryCount-1 {
			var sleep time.Duration
			sleep, interval = eff.next(interval)
			time.Sleep(sleep)
		}
	}
	return err
}

// next returns the sleep for the current interval and the interval of the following attempt.
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
