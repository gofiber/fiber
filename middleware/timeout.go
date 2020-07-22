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
		ch := make(chan interface{}, 1)
		c := ctx.Clone()
		// Get cloned ctx's response reference
		resp := &c.Fasthttp.Response

		go func() {
			defer func() {
				c.App().ReleaseCtx(c)
				ch <- recover()
			}()
			handler(c)
		}()

		select {
		case r := <-ch:
			if r != nil {
				// Pass internal panic
				panic(r)
			}
			resp.CopyTo(&ctx.Fasthttp.Response)
		case <-time.After(timeout):
			ctx.Next(fiber.ErrRequestTimeout)
		}
	}
}
