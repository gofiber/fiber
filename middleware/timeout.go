package middleware

import (
	"time"

	fiber "github.com/gofiber/fiber"
)

// timeoutWrapper contains all timeout relevant properties
type timeoutWrapper struct {
	concurrencyCh chan struct{}
}

// Timeout create a new instance of an timeout wrapper
func Timeout(app *fiber.App) *timeoutWrapper {
	return &timeoutWrapper{
		concurrencyCh: make(chan struct{}, app.Settings.Concurrency),
	}
}

// WrapHandler wraps a handler and aborts the process of the handler if the timeout is reached
func (wrapper *timeoutWrapper) WrapHandler(handler fiber.Handler, timeout time.Duration) fiber.Handler {
	if timeout <= 0 {
		return handler
	}

	return func(ctx *fiber.Ctx) {
		select {
		case wrapper.concurrencyCh <- struct{}{}:
		default:
			ctx.Next(fiber.ErrTooManyRequests)
			return
		}
		ch := make(chan struct{}, 1)

		go func() {
			handler(ctx)
			ch <- struct{}{}
			<-wrapper.concurrencyCh
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
