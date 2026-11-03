package retry_test

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v3/addon/retry"
)

// ExampleExponentialBackoff_Retry demonstrates retrying a transient failure.
func ExampleExponentialBackoff_Retry() {
	backoff := retry.NewExponentialBackoff(retry.Config{MaxBackoffTime: time.Nanosecond})
	attempts := 0
	err := backoff.Retry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporarily unavailable")
		}
		return nil
	})
	fmt.Fprintf(os.Stdout, "attempts: %d\nerror: %v\n", attempts, err)
	// Output:
	// attempts: 3
	// error: <nil>
}

// ExampleNewExponentialBackoff shows a custom backoff configuration.
func ExampleNewExponentialBackoff() {
	backoff := retry.NewExponentialBackoff(retry.Config{
		InitialInterval: 2 * time.Second,
		MaxBackoffTime:  64 * time.Second,
		Multiplier:      2,
		MaxRetryCount:   15,
	})
	fmt.Fprintln(os.Stdout, backoff.InitialInterval, backoff.MaxBackoffTime, backoff.Multiplier, backoff.MaxRetryCount)
	// Output: 2s 1m4s 2 15
}

// ExampleNewExponentialBackoff_defaults shows the default retry settings.
func ExampleNewExponentialBackoff_defaults() {
	backoff := retry.NewExponentialBackoff()
	fmt.Fprintln(os.Stdout, backoff.InitialInterval, backoff.MaxBackoffTime, backoff.Multiplier, backoff.MaxRetryCount)
	// Output: 1s 32s 2 10
}
