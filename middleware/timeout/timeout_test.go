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
	"github.com/valyala/fasthttp"
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
	t.Parallel()
	app := fiber.New()

	// Our middleware wraps a handler that sleeps for 10ms, well under the 50ms limit.
	app.Get("/fast", New(func(c fiber.Ctx) error {
		// Simulate some work
		if err := sleepWithContext(c, 10*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("OK")
	}, Config{Timeout: 50 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/fast", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK for fast requests")
}

// TestTimeout_Exceeded tests a handler that exceeds the provided timeout.
func TestTimeout_Exceeded(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// This handler sleeps 200ms, exceeding the 100ms limit.
	app.Get("/slow", New(func(c fiber.Ctx) error {
		if err := sleepWithContext(c, 200*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("Should never get here")
	}, Config{Timeout: 100 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/slow", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode, "Expected 408 Request Timeout")
}

// TestTimeout_CustomError tests that returning a user-defined error is also treated as a timeout.
func TestTimeout_CustomError(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// This handler sleeps 50ms and returns errCustomTimeout if canceled.
	app.Get("/custom", New(func(c fiber.Ctx) error {
		// Sleep might time out, or might return early. If the context is canceled,
		// we treat errCustomTimeout as a 'timeout-like' condition.
		if err := sleepWithContext(c, 200*time.Millisecond, errCustomTimeout); err != nil {
			return fmt.Errorf("wrapped: %w", err)
		}
		return c.SendString("Should never get here")
	}, Config{Timeout: 100 * time.Millisecond, Errors: []error{errCustomTimeout}}))

	req := httptest.NewRequest(fiber.MethodGet, "/custom", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode, "Expected 408 for custom timeout error")
}

// TestTimeout_UnmatchedError checks that if the handler returns an error
// that is neither a deadline exceeded nor a custom 'timeout' error, it is
// propagated as a regular 500 (internal server error).
func TestTimeout_UnmatchedError(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/unmatched", New(func(_ fiber.Ctx) error {
		return errUnrelated // Not in the custom error list
	}, Config{Timeout: 100 * time.Millisecond, Errors: []error{errCustomTimeout}}))

	req := httptest.NewRequest(fiber.MethodGet, "/unmatched", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode,
		"Expected 500 because the error is not recognized as a timeout error")
}

// TestTimeout_ZeroDuration tests the edge case where the timeout is set to zero.
// Usually this means the request can never exceed a 'deadline' – effectively no timeout.
func TestTimeout_ZeroDuration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/zero", New(func(c fiber.Ctx) error {
		// Sleep 50ms, but there's no real 'deadline' since zero-timeout.
		time.Sleep(50 * time.Millisecond)
		return c.SendString("No timeout used")
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/zero", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK with zero timeout")
}

// TestTimeout_NegativeDuration tests the edge case where the timeout is set to zero.
// Usually this means the request can never exceed a 'deadline' – effectively no timeout.
func TestTimeout_NegativeDuration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/negative", New(func(c fiber.Ctx) error {
		// Sleep 50ms, but there's no real 'deadline' since zero-timeout.
		time.Sleep(50 * time.Millisecond)
		return c.SendString("No timeout used")
	}, Config{Timeout: -100 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/negative", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK with zero timeout")
}

// TestTimeout_SkipPath verifies that requests to specified paths bypass the timeout logic.
func TestTimeout_SkipPath(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/health", New(func(c fiber.Ctx) error {
		time.Sleep(50 * time.Millisecond)
		return c.SendString("ok")
	}, Config{Timeout: 10 * time.Millisecond, SkipPaths: []string{"/health"}}))

	req := httptest.NewRequest(fiber.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK for skipped path")
}

// TestTimeout_PerRoute ensures that per-route timeouts override the default.
func TestTimeout_PerRoute(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/report", New(func(c fiber.Ctx) error {
		if err := sleepWithContext(c, 40*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("done")
	}, Config{
		Timeout: 10 * time.Millisecond,
		Routes:  map[string]time.Duration{"/report": 100 * time.Millisecond},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/report", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK with route-specific timeout")
}

// TestTimeout_CustomHandler checks that a custom timeout handler is executed.
func TestTimeout_CustomHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/custom-handler", New(func(c fiber.Ctx) error {
		if err := sleepWithContext(c, 100*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("should not reach")
	}, Config{
		Timeout: 20 * time.Millisecond,
		OnTimeout: func(c fiber.Ctx) error {
			return c.Status(408).JSON(fiber.Map{"error": "timeout"})
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/custom-handler", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
}

// TestTimeout_SkipPathAndRoute ensures that a path listed in SkipPaths ignores
// any route-specific timeout value.
func TestTimeout_SkipPathAndRoute(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/mixed", New(func(c fiber.Ctx) error {
		// Sleep longer than the default timeout. Because the path is in
		// SkipPaths, the request should still succeed.
		if err := sleepWithContext(c, 150*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("done")
	}, Config{
		Timeout:   20 * time.Millisecond,
		SkipPaths: []string{"/mixed"},
		Routes:    map[string]time.Duration{"/mixed": 200 * time.Millisecond},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/mixed", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK when path is skipped even with route-specific timeout")
}

// TestRunHandler_DefaultOnTimeout verifies that when the wrapped handler returns
// context.DeadlineExceeded and no custom OnTimeout is provided, runHandler
// returns fiber.ErrRequestTimeout.
func TestRunHandler_DefaultOnTimeout(t *testing.T) {
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	err := runHandler(ctx, func(c fiber.Ctx) error {
		return context.DeadlineExceeded
	}, Config{})
	require.Equal(t, fiber.ErrRequestTimeout, err)
}

// TestRunHandler_CustomOnTimeout verifies that if a custom error from cfg.Errors
// is returned and a custom OnTimeout handler is defined, it is executed.
func TestRunHandler_CustomOnTimeout(t *testing.T) {
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	called := false
	cfg := Config{
		Errors: []error{errCustomTimeout},
		OnTimeout: func(c fiber.Ctx) error {
			called = true
			return errors.New("handled")
		},
	}

	err := runHandler(ctx, func(c fiber.Ctx) error {
		return fmt.Errorf("wrap: %w", errCustomTimeout)
	}, cfg)

	require.True(t, called, "expected OnTimeout to be called")
	require.EqualError(t, err, "handled")
}
