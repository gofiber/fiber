---
id: limiter
title: Limiter
---

Limiter middleware for [Fiber](https://github.com/gofiber/fiber) used to limit repeated requests to public APIs and/or endpoints such as password reset etc. Also useful for API clients, web crawling, or other tasks that need to be throttled.

:::note
This module does not share state with other processes/servers by default.
:::

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

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
    Expiration:     30 * time.Second,
    KeyGenerator:          func(c *fiber.Ctx) string {
        return c.Get("x-forwarded-for")
    },
    LimitReached: func(c *fiber.Ctx) error {
        return c.SendFile("./toofast.html")
    },
    Storage: myCustomStorage{},
}))
```

## Sliding window

Instead of using the standard fixed window algorithm, you can enable the [sliding window](https://en.wikipedia.org/wiki/Sliding_window_protocol) algorithm.

A example of such configuration is:

```go
app.Use(limiter.New(limiter.Config{
    Max:            20,
    Expiration:     30 * time.Second,
    LimiterMiddleware: limiter.SlidingWindow{},
}))
```

This means that every window will take into account the previous window(if there was any). The given formula for the rate is:
```
weightOfPreviousWindpw = previous window's amount request * (whenNewWindow / Expiration)
rate = weightOfPreviousWindpw + current window's amount request.
```

## Config

```go
// Config defines the config for middleware.
type Config struct {
    // Next defines a function to skip this middleware when returned true.
    //
    // Optional. Default: nil
    Next func(c *fiber.Ctx) bool

    // Max number of recent connections during `Expiration` seconds before sending a 429 response
    //
    // Default: 5
    Max int

    // KeyGenerator allows you to generate custom keys, by default c.IP() is used
    //
    // Default: func(c *fiber.Ctx) string {
    //   return c.IP()
    // }
    KeyGenerator func(*fiber.Ctx) string

    // Expiration is the time on how long to keep records of requests in memory
    //
    // Default: 1 * time.Minute
    Expiration time.Duration

    // LimitReached is called when a request hits the limit
    //
    // Default: func(c *fiber.Ctx) error {
    //   return c.SendStatus(fiber.StatusTooManyRequests)
    // }
    LimitReached fiber.Handler

    // When set to true, requests with StatusCode >= 400 won't be counted.
    //
    // Default: false
    SkipFailedRequests bool

    // When set to true, requests with StatusCode < 400 won't be counted.
    //
    // Default: false
    SkipSuccessfulRequests bool

    // Store is used to store the state of the middleware
    //
    // Default: an in memory store for this process only
    Storage fiber.Storage

    // LimiterMiddleware is the struct that implements limiter middleware.
    //
    // Default: a new Fixed Window Rate Limiter
    LimiterMiddleware LimiterHandler
}
```

A custom store can be used if it implements the `Storage` interface - more details and an example can be found in `store.go`.

## Default Config

```go
var ConfigDefault = Config{
    Max:        5,
    Expiration: 1 * time.Minute,
    KeyGenerator: func(c *fiber.Ctx) string {
        return c.IP()
    },
    LimitReached: func(c *fiber.Ctx) error {
        return c.SendStatus(fiber.StatusTooManyRequests)
    },
    SkipFailedRequests: false,
    SkipSuccessfulRequests: false,
    LimiterMiddleware: FixedWindow{},
}
```
