package timeoutcontext

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
)

// New implementation of timeout middleware. Set custom errors(context.DeadlineExceeded vs) for get fiber.ErrRequestTimeout response.
func New(handler fiber.Handler, timeout time.Duration, timeoutErrors ...error) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		timeoutContext, cancel := context.WithTimeout(ctx.UserContext(), timeout)
		defer cancel()
		ctx.SetUserContext(timeoutContext)
		if err := handler(ctx); err != nil {
			unwrappedErr := errors.Unwrap(err)
			if unwrappedErr == context.DeadlineExceeded {
				return fiber.ErrRequestTimeout
			}
			for i := range timeoutErrors {
				if unwrappedErr == timeoutErrors[i] {
					return fiber.ErrRequestTimeout
				}
			}
			return err
		}
		return nil
	}
}
