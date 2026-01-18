package timeout

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
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
		if err := sleepWithContext(c.Context(), 10*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("OK")
	}, Config{Timeout: 50 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/fast", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK for fast requests")
}

// TestTimeout_Exceeded tests a handler that exceeds the provided timeout.
func TestTimeout_Exceeded(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// This handler listens for context cancelation and returns early when timeout occurs.
	app.Get("/slow", New(func(c fiber.Ctx) error {
		if err := sleepWithContext(c.Context(), 200*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("Should never get here")
	}, Config{Timeout: 50 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/slow", http.NoBody)
	start := time.Now()
	resp, err := app.Test(req)
	elapsed := time.Since(start)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode, "Expected 408 Request Timeout")
	// Handler should return shortly after timeout (not wait full 200ms)
	require.Less(t, elapsed, 150*time.Millisecond, "handler should return early on context cancelation")
}

// TestTimeout_ContextPropagation verifies that the timeout context is properly
// passed to the handler so it can detect cancelation (Issue #3671).
func TestTimeout_ContextPropagation(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	var contextCanceled atomic.Bool

	app.Get("/context-aware", New(func(c fiber.Ctx) error {
		// Handler that properly listens for context cancelation
		select {
		case <-c.Context().Done():
			contextCanceled.Store(true)
			return c.Context().Err()
		case <-time.After(500 * time.Millisecond):
			return c.SendString("completed")
		}
	}, Config{Timeout: 50 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/context-aware", http.NoBody)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
	require.True(t, contextCanceled.Load(), "Handler should have detected context cancelation")
}

// TestTimeout_HandlerReturnsEarlyOnCancel verifies that handlers checking context
// can return early, making the overall request faster than the handler's work time.
func TestTimeout_HandlerReturnsEarlyOnCancel(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/early-return", New(func(c fiber.Ctx) error {
		// Handler that would take 500ms but checks context
		for i := 0; i < 50; i++ {
			select {
			case <-c.Context().Done():
				return c.Context().Err()
			case <-time.After(10 * time.Millisecond):
				// Continue work
			}
		}
		return c.SendString("completed")
	}, Config{Timeout: 30 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/early-return", http.NoBody)
	start := time.Now()
	resp, err := app.Test(req)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
	// Should complete much faster than 500ms because handler checks context
	require.Less(t, elapsed, 100*time.Millisecond)
}

// TestTimeout_CustomError tests that returning a user-defined error is also treated as a timeout.
func TestTimeout_CustomError(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// This handler sleeps 50ms and returns errCustomTimeout if canceled.
	app.Get("/custom", New(func(c fiber.Ctx) error {
		// Sleep might time out, or might return early. If the context is canceled,
		// we treat errCustomTimeout as a 'timeout-like' condition.
		if err := sleepWithContext(c.Context(), 200*time.Millisecond, errCustomTimeout); err != nil {
			return fmt.Errorf("wrapped: %w", err)
		}
		return c.SendString("Should never get here")
	}, Config{Timeout: 50 * time.Millisecond, Errors: []error{errCustomTimeout}}))

	req := httptest.NewRequest(fiber.MethodGet, "/custom", http.NoBody)
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

	req := httptest.NewRequest(fiber.MethodGet, "/unmatched", http.NoBody)
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

	req := httptest.NewRequest(fiber.MethodGet, "/zero", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK with zero timeout")
}

// TestTimeout_NegativeDuration ensures negative timeout values fall back to zero.
func TestTimeout_NegativeDuration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/negative", New(func(c fiber.Ctx) error {
		time.Sleep(50 * time.Millisecond)
		return c.SendString("No timeout used")
	}, Config{Timeout: -100 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/negative", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req) should not fail")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK with zero timeout")
}

// TestTimeout_CustomHandler ensures that a custom handler runs on timeout.
func TestTimeout_CustomHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	var called atomic.Int32

	app.Get("/custom-handler", New(func(c fiber.Ctx) error {
		if err := sleepWithContext(c.Context(), 100*time.Millisecond, context.DeadlineExceeded); err != nil {
			return err
		}
		return c.SendString("should not reach")
	}, Config{
		Timeout: 20 * time.Millisecond,
		OnTimeout: func(c fiber.Ctx) error {
			called.Add(1)
			return c.Status(408).JSON(fiber.Map{"error": "timeout"})
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/custom-handler", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
	require.Equal(t, int32(1), called.Load())
}

// TestTimeout_PanicInHandler verifies that panics in the handler return 500.
func TestTimeout_PanicInHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/panic", New(func(_ fiber.Ctx) error {
		panic("test panic")
	}, Config{Timeout: 100 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/panic", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	// Panic in handler results in 500 Internal Server Error
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// TestIsTimeoutError_DeadlineExceeded ensures context.DeadlineExceeded triggers timeout.
func TestIsTimeoutError_DeadlineExceeded(t *testing.T) {
	t.Parallel()

	require.True(t, isTimeoutError(context.DeadlineExceeded, nil))
	require.True(t, isTimeoutError(fmt.Errorf("wrap: %w", context.DeadlineExceeded), nil))
}

// TestIsTimeoutError_CustomErrors verifies custom errors are detected.
func TestIsTimeoutError_CustomErrors(t *testing.T) {
	t.Parallel()

	customErr := errors.New("custom timeout")
	require.True(t, isTimeoutError(customErr, []error{customErr}))
	require.True(t, isTimeoutError(fmt.Errorf("wrap: %w", customErr), []error{customErr}))
	require.False(t, isTimeoutError(errUnrelated, []error{customErr}))
}

// TestIsTimeoutError_WithOnTimeout verifies that custom OnTimeout is called for custom errors.
func TestIsTimeoutError_WithOnTimeout(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	called := false
	cfg := Config{
		Timeout: 100 * time.Millisecond,
		Errors:  []error{errCustomTimeout},
		OnTimeout: func(_ fiber.Ctx) error {
			called = true
			return errors.New("handled")
		},
	}

	// Test via full middleware to ensure OnTimeout is called
	handler := New(func(_ fiber.Ctx) error {
		return fmt.Errorf("wrap: %w", errCustomTimeout)
	}, cfg)

	err := handler(ctx)
	require.True(t, called)
	require.EqualError(t, err, "handled")
}

// TestTimeout_HandlerHung_ReturnsWithinTimeout ensures we still respond when the handler never exits
// (when GracePeriod is configured).
func TestTimeout_HandlerHung_ReturnsWithinTimeout(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	block := make(chan struct{})
	handlerDone := make(chan struct{})
	app.Get("/hung", New(func(_ fiber.Ctx) error {
		// Intentionally ignore context cancelation to simulate a stuck handler.
		defer close(handlerDone)
		<-block // Ignore context cancelation to simulate a hung handler
		return nil
	}, Config{Timeout: 20 * time.Millisecond, GracePeriod: 50 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/hung", http.NoBody)
	start := time.Now()
	resp, err := app.Test(req)
	elapsed := time.Since(start)

	close(block) // Unblock goroutine to avoid leaks in the test process
	select {
	case <-handlerDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("handler did not exit after timeout")
	}

	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
	require.Less(t, elapsed, 150*time.Millisecond, "timeout middleware should respond even if handler is stuck")
}

// TestTimeout_PanicAfterTimeout ensures panics after a timeout are handled.
func TestTimeout_PanicAfterTimeout(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/panic-after-timeout", New(func(c fiber.Ctx) error {
		<-c.Context().Done()
		panic("panic after timeout")
	}, Config{Timeout: 20 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/panic-after-timeout", http.NoBody)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// TestTimeout_GracePeriodConfigured tests that a configured GracePeriod is respected.
func TestTimeout_GracePeriodConfigured(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	block := make(chan struct{})
	app.Get("/grace-configured", New(func(_ fiber.Ctx) error {
		<-block // ignore cancelation to force timeout path
		return nil
	}, Config{Timeout: 10 * time.Millisecond, GracePeriod: 30 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/grace-configured", http.NoBody)
	start := time.Now()
	resp, err := app.Test(req)
	elapsed := time.Since(start)
	close(block)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
	// Should take roughly Timeout + GracePeriod (10ms + 30ms = ~40ms)
	require.GreaterOrEqual(t, elapsed, 30*time.Millisecond, "should wait at least GracePeriod")
	require.Less(t, elapsed, 150*time.Millisecond, "should not wait too long")
}

// TestTimeout_DefaultWaitsForHandler ensures that by default (GracePeriod == 0)
// the middleware waits indefinitely for the handler to finish.
func TestTimeout_DefaultWaitsForHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	handlerDelay := 100 * time.Millisecond
	app.Get("/wait-default", New(func(c fiber.Ctx) error {
		// Handler that takes longer than timeout but respects context cancelation
		select {
		case <-c.Context().Done():
			// Simulate some cleanup time after cancelation
			time.Sleep(handlerDelay)
			return c.Context().Err()
		case <-time.After(500 * time.Millisecond):
			return c.SendString("completed")
		}
	}, Config{Timeout: 20 * time.Millisecond})) // No GracePeriod = wait indefinitely

	req := httptest.NewRequest(fiber.MethodGet, "/wait-default", http.NoBody)
	start := time.Now()
	resp, err := app.Test(req)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
	// Should wait for handler to finish (timeout + handlerDelay)
	require.GreaterOrEqual(t, elapsed, handlerDelay, "should wait for handler to finish")
	require.Less(t, elapsed, 300*time.Millisecond, "should not wait too long")
}

// TestTimeout_Next verifies the Next function skips the middleware.
func TestTimeout_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/skip", New(func(c fiber.Ctx) error {
		time.Sleep(100 * time.Millisecond)
		return c.SendString("OK")
	}, Config{
		Timeout: 10 * time.Millisecond,
		Next: func(_ fiber.Ctx) bool {
			return true // Always skip
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/skip", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Middleware should be skipped")
}

// TestTimeout_NegativeGracePeriod verifies that negative GracePeriod is treated as 0 (default).
func TestTimeout_NegativeGracePeriod(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	handlerDelay := 50 * time.Millisecond
	app.Get("/negative-grace", New(func(c fiber.Ctx) error {
		<-c.Context().Done()
		time.Sleep(handlerDelay) // Simulate cleanup after cancelation
		return c.Context().Err()
	}, Config{Timeout: 20 * time.Millisecond, GracePeriod: -100 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/negative-grace", http.NoBody)
	start := time.Now()
	resp, err := app.Test(req)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
	// Negative GracePeriod should be treated as 0, meaning wait indefinitely for handler
	require.GreaterOrEqual(t, elapsed, handlerDelay, "should wait for handler (GracePeriod normalized to 0)")
}

// TestTimeout_ContextDeadlineDetection verifies that context deadline is detected
// even if handler doesn't return an error.
func TestTimeout_ContextDeadlineDetection(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/deadline", New(func(c fiber.Ctx) error {
		// Wait for context to be done, then return nil (not an error)
		<-c.Context().Done()
		return nil
	}, Config{Timeout: 20 * time.Millisecond}))

	req := httptest.NewRequest(fiber.MethodGet, "/deadline", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	// Should still be 408 because context deadline was exceeded
	require.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
}
