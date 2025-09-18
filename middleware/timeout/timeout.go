package timeout

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
)

// New enforces a timeout for each incoming request. It replaces the request's
// context with one that has the configured deadline, which is exposed through
// c.Context(). If the timeout expires or any of the specified errors occur,
// fiber.ErrRequestTimeout is returned.
func New(h fiber.Handler, config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(ctx fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(ctx) {
			return h(ctx)
		}

		timeout := cfg.Timeout
		if timeout <= 0 {
			return runHandler(ctx, h, cfg)
		}

		parent := ctx.Context()
		tCtx, cancel := context.WithTimeout(parent, timeout)
		ctx.SetContext(tCtx)
		defer func() {
			cancel()
			ctx.SetContext(parent)
		}()
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

		err := safeCall(func() error {
			select {
			case err := <-done:
				return err
			case <-panicChan:
				return fiber.ErrRequestTimeout
			case <-tCtx.Done():
				if cfg.OnTimeout != nil {
					return cfg.OnTimeout(ctx)
				}
				return fiber.ErrRequestTimeout
			}
		})
		return err
	}
}

func safeCall(fn func() error) error {
	err := error(nil)
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fiber.ErrRequestTimeout
			}
		}()
		err = fn()
	}()
	return err
}

// runHandler executes the handler and returns fiber.ErrRequestTimeout if it
// sees a deadline exceeded error or one of the custom "timeout-like" errors.
func runHandler(c fiber.Ctx, h fiber.Handler, cfg Config) error {
	err := h(c)
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || (len(cfg.Errors) > 0 && isCustomError(err, cfg.Errors))) {
		if cfg.OnTimeout != nil {
			if toErr := cfg.OnTimeout(c); toErr != nil {
				return toErr
			}
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
