package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
)

// New implementation of timeout middleware. Set custom errors(context.DeadlineExceeded vs) for get fiber.ErrRequestTimeout response.
func New(h fiber.Handler, t time.Duration, tErrs ...error) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		timeoutContext, cancel := context.WithTimeout(ctx.UserContext(), t)
		defer cancel()
		ctx.SetUserContext(timeoutContext)
		if err := h(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fiber.ErrRequestTimeout
			}
			for i := range tErrs {
				if errors.Is(err, tErrs[i]) {
					return fiber.ErrRequestTimeout
				}
			}
			return err
		}
		return nil
	}
}

// Use timeout middleware for global or group usage.
func Use(t time.Duration, tErrs ...error) fiber.Handler {
	h := func(ctx *fiber.Ctx) error {
		return ctx.Next()
	}
	return New(h, t, tErrs...)
}
