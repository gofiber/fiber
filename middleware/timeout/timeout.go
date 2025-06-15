package timeout

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
)

// New returns middleware that enforces a timeout for each request. Configuration
// is provided via an optional Config parameter. If no Config is supplied,
// ConfigDefault is used.
func New(h fiber.Handler, config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Pre-build skip path map for faster lookups.
	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}

	return func(ctx fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(ctx) {
			return h(ctx)
		}
		if _, ok := skip[ctx.Path()]; ok {
			return h(ctx)
		}

		timeout := cfg.Timeout
		if cfg.Routes != nil {
			if t, ok := cfg.Routes[ctx.Path()]; ok {
				timeout = t
			}
		}

		if timeout <= 0 {
			return runHandler(ctx, h, cfg)
		}

		tCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		err := runHandler(ctx, h, cfg)
		if errors.Is(tCtx.Err(), context.DeadlineExceeded) {
			if cfg.OnTimeout != nil {
				return cfg.OnTimeout(ctx)
			}
			return fiber.ErrRequestTimeout
		}
		return err
	}
}

// runHandler executes the handler and handles timeout-like errors.
func runHandler(c fiber.Ctx, h fiber.Handler, cfg Config) error {
	err := h(c)
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || (len(cfg.Errors) > 0 && isCustomError(err, cfg.Errors))) {
		if cfg.OnTimeout != nil {
			return cfg.OnTimeout(c)
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
