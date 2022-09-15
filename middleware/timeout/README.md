# Timeout
Timeout middleware for Fiber. As a `fiber.Handler` wrapper, it creates a context with `context.WithTimeout` and pass it in `UserContext`.

If the context passed executions (eg. DB ops, Http calls) takes longer than the given duration to return, the timeout error is set and forwarded to the centralized `ErrorHandler`.

It has no race conditions, ready to use on production.

### Table of Contents
- [Timeout](#timeout)
		- [Table of Contents](#table-of-contents)
		- [Signatures](#signatures)
		- [Examples](#examples)


### Signatures
```go
func New(handler fiber.Handler, timeout time.Duration, timeoutErrors ...error) fiber.Handler
```

### Examples
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

app.Get("/foo", timeoutcontext.New(h, 5 * time.Second))

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

app.Get("/foo", timeoutcontext.New(h, 5 * time.Second), ErrFooTimeOut)

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