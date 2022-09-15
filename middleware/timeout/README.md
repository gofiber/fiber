# Timeout
Timeout middleware for [Fiber](https://github.com/gofiber/fiber) wraps a `fiber.Handler` with a timeout. If the handler takes longer than the given duration to return, the timeout error is set and forwarded to the centralized [ErrorHandler](https://docs.gofiber.io/error-handling).

Also has timeout with context middleware by `NewWithContext`, it creates a context with `context.WithTimeout` and pass it in `UserContext`.

If the context passed executions (eg. DB ops, Http calls) takes longer than the given duration to return, it cancels current operations using context and  the timeout error is set and forwarded to the centralized `ErrorHandler`. `UserContext` needs to be passed all long running operations.

`NewWithContext` has no race conditions, ready to use on production.


### Table of Contents
- [Timeout](#timeout)
		- [Table of Contents](#table-of-contents)
		- [Signatures](#signatures)
		- [Examples](#examples)
		- [NewWithContext Examples](#newwithcontext-examples)


### Signatures
```go
func New(h fiber.Handler, t time.Duration) fiber.Handler
func NewWithContext(h fiber.Handler, t time.Duration, tErrs ...error) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/timeout"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
handler := func(ctx *fiber.Ctx) error {
	err := ctx.SendString("Hello, World 👋!")
	if err != nil {
		return err
	}
	return nil
}

app.Get("/foo", timeout.New(handler, 5 * time.Second))
```

### NewWithContext Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/timeout"
)
```

After you initiate your Fiber app, you can use:
```go
h := func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContextWithCustomError(c.UserContext(), sleepTime); err != nil {
			return fmt.Errorf("%w: execution error", err)
		}
		return nil
	}

app.Get("/foo", timeoutcontext.NewWithContext(h, 5 * time.Second))

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return context.DeadlineExceeded
	case <-timer.C:
	}
	return nil
}

```

Use with custom error:
```go
var ErrFooTimeOut = errors.New("foo context canceled")

h := func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContextWithCustomError(c.UserContext(), sleepTime); err != nil {
			return fmt.Errorf("%w: execution error", err)
		}
		return nil
	}

app.Get("/foo", timeoutcontext.NewWithContext(h, 5 * time.Second), ErrFooTimeOut)

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ErrFooTimeOut
	case <-timer.C:
	}
	return nil
}

```
