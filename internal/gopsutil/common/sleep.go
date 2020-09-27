package common

import (
	"context"
	"time"
)

// Sleep awaits for provided interval.
// Can be interrupted by context cancelation.
func Sleep(ctx context.Context, interval time.Duration) error {
	var timer = time.NewTimer(interval)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
