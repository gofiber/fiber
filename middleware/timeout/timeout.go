package timeout

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	utils "github.com/gofiber/utils/v2"
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
			return h(ctx)
		}

		err := make(chan error, 1)
		go func() {
			err <- h(ctx)
		}()
		select {
		case err := <-err:
			if err != nil && (len(cfg.Errors) > 0 && isCustomError(err, cfg.Errors)) {
				if cfg.OnTimeout != nil {
					if toErr := cfg.OnTimeout(ctx); toErr != nil {
						return toErr
					}
				}
				return fiber.ErrRequestTimeout
			}
			return err
		case <-time.After(timeout):
			if cfg.OnTimeout != nil {
				err := cfg.OnTimeout(ctx)
				ctx.RequestCtx().TimeoutErrorWithResponse(&ctx.RequestCtx().Response)
				return err
			}
			ctx.RequestCtx().TimeoutErrorWithCode(utils.StatusMessage(fiber.StatusRequestTimeout), fiber.StatusRequestTimeout)
			return fiber.ErrRequestTimeout
		}
	}
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
