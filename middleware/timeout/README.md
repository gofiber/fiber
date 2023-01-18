# Timeout
Timeout middleware for Fiber. As a `fiber.Handler` wrapper, it creates a context with `context.WithTimeout` and pass it in `UserContext`.

If the context passed executions (eg. DB ops, Http calls) takes longer than the given duration to return, the timeout error is set and forwarded to the centralized `ErrorHandler`.

It has no race conditions, ready to use on production.

### Table of Contents
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

Sample timeout middleware usage
```go
func main() {
	app := fiber.New()
	h := func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContext(c.UserContext(), sleepTime); err != nil {
			return fmt.Errorf("%w: execution error", err)
		}
		return nil
	}

	app.Get("/foo/:sleepTime", timeout.New(h, 2*time.Second))
	_ = app.Listen(":3000")
}

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

Test http 200 with curl:
```bash
curl --location -I --request GET 'http://localhost:3000/foo/1000' 
```

Test http 408 with curl:
```bash
curl --location -I --request GET 'http://localhost:3000/foo/3000' 
```


When using with custom error:
```go
var ErrFooTimeOut = errors.New("foo context canceled")

func main() {
	app := fiber.New()
	h := func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContextWithCustomError(c.UserContext(), sleepTime); err != nil {
			return fmt.Errorf("%w: execution error", err)
		}
		return nil
	}

	app.Get("/foo/:sleepTime", timeout.New(h, 2*time.Second, ErrFooTimeOut))
	_ = app.Listen(":3000")
}

func sleepWithContextWithCustomError(ctx context.Context, d time.Duration) error {
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
