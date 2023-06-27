package timeout

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/gofiber/fiber/v2"
)

var once sync.Once

// New wraps a handler and aborts the process of the handler if the timeout is reached.
//
// Deprecated: This implementation contains data race issues. Use NewWithContext instead.
// Find documentation and sample usage on https://docs.gofiber.io/api/middleware/timeout
func New(handler fiber.Handler, timeout time.Duration) fiber.Handler {
	once.Do(func() {
		log.Warn("[TIMEOUT] timeout contains data race issues, not ready for production!")
	})

	if timeout <= 0 {
		return handler
	}

	// logic is from fasthttp.TimeoutWithCodeHandler https://github.com/valyala/fasthttp/blob/master/server.go#L418
	return func(ctx *fiber.Ctx) error {
		ch := make(chan struct{}, 1)

		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("[TIMEOUT] recover error %v", err)
				}
			}()
			if err := handler(ctx); err != nil {
				log.Errorf("[TIMEOUT] handler error %v", err)
			}
			ch <- struct{}{}
		}()

		select {
		case <-ch:
		case <-time.After(timeout):
			return fiber.ErrRequestTimeout
		}

		return nil
	}
}

// NewWithContext implementation of timeout middleware. Set custom errors(context.DeadlineExceeded vs) for get fiber.ErrRequestTimeout response.
func NewWithContext(h fiber.Handler, t time.Duration, tErrs ...error) fiber.Handler {
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
