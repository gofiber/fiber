# Limiter
Limiter middleware for [Fiber](https://github.com/gofiber/fiber) used to limit repeated requests to public APIs and/or endpoints such as password reset etc. Also useful for API clients, web crawling, or other tasks that need to be throttled.

**Note: this module does not share state with other processes/servers by default.**

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)


### Signatures
```go
func New(config ...Config) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/limiter"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Default middleware config
app.Use(limiter.New())

// Or extend your config for customization
app.Use(limiter.New(limiter.Config{
	Next: func(c *fiber.Ctx) bool {
		return c.IP() == "127.0.0.1"
	},
	Max:          20,
	Duration:     30 * time.Second,
	Key:          func(c *fiber.Ctx) string {
		return c.Get("x-forwarded-for")
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.SendFile("./toofast.html")
	},
}))
```

### Config
```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Max number of recent connections during `Duration` seconds before sending a 429 response
	//
	// Default: 5
	Max int

	// Duration is the time on how long to keep records of requests in memory
	//
	// Default: time.Minute
	Duration time.Duration

	// Key allows you to generate custom keys, by default c.IP() is used
	//
	// Default: func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	Key func(*fiber.Ctx) string

	// LimitReached is called when a request hits the limit
	//
	// Default: func(c *fiber.Ctx) error {
	//   return c.SendStatus(fiber.StatusTooManyRequests)
	// }
	LimitReached fiber.Handler
}
```

### Default Config
```go
var ConfigDefault = Config{
	Next:     nil,
	Max:      5,
	Duration: time.Minute,
	Key: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
}
```