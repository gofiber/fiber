package timeout

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

var (
	// Custom error that we treat like a timeout when returned by the handler.
	errCustomTimeout = errors.New("custom timeout error")

	// Some unrelated error that should NOT trigger a request timeout.
	errUnrelated = errors.New("unmatched error")
)

// sleepWithContext simulates a task that takes `d` time, but returns `te` if the context is canceled.
func sleepWithContext(ctx context.Context, d time.Duration, te error) error {
	timer := time.NewTimer(d)
	defer timer.Stop() // Clean up the timer

	select {
	case <-ctx.Done():
		return te
	case <-timer.C:
		return nil
	}
}

// TestTimeout_Success tests a handler that completes within the allotted timeout.
func TestTimeout_Success(t *testing.T) {
	app := fiber.New()

	// Our middleware wraps a handler that sleeps for 10ms, well under the 50ms limit.
	app.Get("/fast", New(func(c fiber.Ctx) error {
		// Simulate some work
		if err := sleepWithContext(c.Context(), 10*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("OK")
	}, 50*time.Millisecond))

	req := httptest.NewRequest(fiber.MethodGet, "/fast", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK for fast requests")
}

// TestTimeout_Exceeded tests a handler that exceeds the provided timeout.
func TestTimeout_Exceeded(t *testing.T) {
	app := fiber.New()

	// This handler sleeps 200ms, exceeding the 100ms limit.
	app.Get("/slow", New(func(c fiber.Ctx) error {
		if err := sleepWithContext(c.Context(), 200*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("Should never get here")
	}, 100*time.Millisecond))

	req := httptest.NewRequest(fiber.MethodGet, "/slow", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode, "Expected 408 Request Timeout")
}

// TestTimeout_CustomError tests that returning a user-defined error is also treated as a timeout.
func TestTimeout_CustomError(t *testing.T) {
	app := fiber.New()

	// This handler sleeps 50ms and returns errCustomTimeout if canceled.
	app.Get("/custom", New(func(c fiber.Ctx) error {
		// Sleep might time out, or might return early. If the context is canceled,
		// we treat errCustomTimeout as a 'timeout-like' condition.
		if err := sleepWithContext(c.Context(), 200*time.Millisecond, errCustomTimeout); err != nil {
			return fmt.Errorf("wrapped: %w", err)
		}
		return c.SendString("Should never get here")
	}, 100*time.Millisecond, errCustomTimeout))

	req := httptest.NewRequest(fiber.MethodGet, "/custom", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode, "Expected 408 for custom timeout error")
}

// TestTimeout_UnmatchedError checks that if the handler returns an error
// that is neither a deadline exceeded nor a custom 'timeout' error, it is
// propagated as a regular 500 (internal server error).
func TestTimeout_UnmatchedError(t *testing.T) {
	app := fiber.New()

	app.Get("/unmatched", New(func(_ fiber.Ctx) error {
		return errUnrelated // Not in the custom error list
	}, 100*time.Millisecond, errCustomTimeout))

	req := httptest.NewRequest(fiber.MethodGet, "/unmatched", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode,
		"Expected 500 because the error is not recognized as a timeout error")
}

// TestTimeout_ZeroDuration tests the edge case where the timeout is set to zero.
// Usually this means the request can never exceed a 'deadline' – effectively no timeout.
func TestTimeout_ZeroDuration(t *testing.T) {
	app := fiber.New()

	app.Get("/zero", New(func(c fiber.Ctx) error {
		// Sleep 50ms, but there's no real 'deadline' since zero-timeout.
		time.Sleep(50 * time.Millisecond)
		return c.SendString("No timeout used")
	}, 0))

	req := httptest.NewRequest(fiber.MethodGet, "/zero", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK with zero timeout")
}
