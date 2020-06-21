package middleware

import (
	"time"

	fiber "github.com/gofiber/fiber"
	fasthttp "github.com/valyala/fasthttp"
)

var concurrencyCh = make(chan struct{}, fasthttp.DefaultConcurrency)

// Timeout wraps a handler and aborts the process of the handler if the timeout is reached
func Timeout(handler fiber.Handler, timeout time.Duration) fiber.Handler {
	if timeout <= 0 {
		return handler
	}

	return func(ctx *fiber.Ctx) {
		select {
		case concurrencyCh <- struct{}{}:
		default:
			ctx.Next(fiber.ErrTooManyRequests)
			return
		}
		ch := make(chan struct{}, 1)

		go func() {
			handler(ctx)
			ch <- struct{}{}
			<-concurrencyCh
		}()
		timeoutTimer := time.NewTimer(timeout)
		select {
		case <-ch:
		case <-timeoutTimer.C:
			ctx.Next(fiber.ErrRequestTimeout)
		}
		if !timeoutTimer.Stop() {
			// Collect possibly added time from the channel
			// if timer has been stopped and nobody collected its' value.
			select {
			case <-timeoutTimer.C:
			default:
			}
		}
	}
}
