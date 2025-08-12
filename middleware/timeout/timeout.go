package timeout

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
)

// New enforces a timeout for each incoming request. If the timeout expires or
// any of the specified errors occur, fiber.ErrRequestTimeout is returned.
func New(h fiber.Handler, config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(ctx fiber.Ctx) (err error) {
		if cfg.Next != nil && cfg.Next(ctx) {
			return h(ctx)
		}

		timeout := cfg.Timeout
		if timeout <= 0 {
			return runHandler(ctx, h, cfg)
		}
		tCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		done := make(chan error, 1)
		panicChan := make(chan any, 1)

		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			done <- runHandler(ctx, h, cfg)
		}()

		select {
		case err := <-done:
			return err
		case _ = <-panicChan:
			return fiber.ErrRequestTimeout
		case <-tCtx.Done():
			if cfg.OnTimeout != nil {
				return callOnTimeoutSafe(ctx, cfg)
			}
			return fiber.ErrRequestTimeout
		}
	}
}

func callOnTimeoutSafe(ctx fiber.Ctx, cfg Config) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fiber.ErrRequestTimeout
		}
	}()
	return cfg.OnTimeout(ctx)
}

// runHandler executes the handler and returns fiber.ErrRequestTimeout if it
// sees a deadline exceeded error or one of the custom "timeout-like" errors.
func runHandler(c fiber.Ctx, h fiber.Handler, cfg Config) error {
	err := h(c)
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || (len(cfg.Errors) > 0 && isCustomError(err, cfg.Errors))) {
		if cfg.OnTimeout != nil {
			return callOnTimeoutSafe(c, cfg)
		}
		return fiber.ErrRequestTimeout
	}
	return err
}

// isCustomError checks whether err matches any error in errList using errors.Is.
func isCustomError(err error, errList []error) bool {
	for _, e := range errList {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
