package middleware

import (
	"time"

	fiber "github.com/gofiber/fiber"
)

// Timeout wraps a handler and aborts the process of the handler if the timeout is reached
func Timeout(handler fiber.Handler, timeout time.Duration) fiber.Handler {
	// fHandler := func(fctx *fasthttp.RequestCtx) {
	// 	// Emulate long-running task, which touches ctx.
	// 	doneCh := make(chan struct{})
	// 	go func() {
	// 		workDuration := time.Millisecond * time.Duration(rand.Intn(2000))
	// 		time.Sleep(workDuration)

	// 		fmt.Fprintf(fctx, "ctx has been accessed by long-running task\n")
	// 		fmt.Fprintf(fctx, "The reuqestHandler may be finished by this time.\n")

	// 		close(doneCh)
	// 	}()

	// 	select {
	// 	case <-doneCh:
	// 	case <-time.After(timeout):
	// 		fctx.TimeoutError("Timeout!")
	// 	}
	// }
	// return func(ctx *fiber.Ctx) {
	// 	fHandler(ctx.Fasthttp)
	// }

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
