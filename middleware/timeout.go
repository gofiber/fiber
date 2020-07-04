package middleware

import (
	"time"

	fiber "github.com/gofiber/fiber"
)

// Timeout wraps a handler and aborts the process of the handler if the timeout is reached
func Timeout(handler fiber.Handler, timeout time.Duration) fiber.Handler {
	if timeout <= 0 {
		return handler
	}

	// logic is from fasthttp.TimeoutWithCodeHandler https://github.com/valyala/fasthttp/blob/master/server.go#L418
	return func(ctx *fiber.Ctx) {
		ch := make(chan struct{}, 1)

		go func() {
			defer func() {
				_ = recover()
			}()
			handler(ctx)
			ch <- struct{}{}
		}()

		select {
		case <-ch:
		case <-time.After(timeout):
			ctx.Next(fiber.ErrRequestTimeout)
		}
	}
}
