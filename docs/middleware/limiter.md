---
id: limiter
---

# Limiter

The Limiter middleware for [Fiber](https://github.com/gofiber/fiber) throttles repeated requests to public APIs or endpoints such as password resets. It's also useful for API clients, web crawlers, or other tasks that need rate limiting.

Limiter redacts request keys in error paths by default so storage identifiers and rate-limit keys don't leak into logs. Set `DisableValueRedaction` to `true` when you explicitly need the raw key for troubleshooting.

:::note
This middleware uses our [Storage](https://github.com/gofiber/storage) package to support various databases through a single interface. The default configuration for this middleware saves data to memory, see the examples below for other databases.
:::

:::note
This module does not share state with other processes/servers by default.
:::

## Signatures

```go
func New(config ...Config) fiber.Handler

type Handler interface {
    New(config *Config) fiber.Handler
}
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/limiter"
)
```

Once your Fiber app is initialized, use the middleware like this:

```go
// Initialize default config
app.Use(limiter.New())

// Or extend your config for customization
app.Use(limiter.New(limiter.Config{
    Next: func(c fiber.Ctx) bool {
        return c.IP() == "127.0.0.1"
    },
    Max:          20,
    MaxFunc: func(c fiber.Ctx) int {
      return 20
    },
    Expiration:     30 * time.Second,
    KeyGenerator:          func(c fiber.Ctx) string {
        return c.Get("x-forwarded-for")
    },
    LimitReached: func(c fiber.Ctx) error {
        return c.SendFile("./toofast.html")
    },
    Storage: myCustomStorage{},
}))
```

## Sliding window

Instead of using the standard fixed window algorithm, you can enable the [sliding window](https://en.wikipedia.org/wiki/Sliding_window_protocol) algorithm.

An example configuration is:

```go
app.Use(limiter.New(limiter.Config{
    Max:            20,
    Expiration:     30 * time.Second,
    LimiterMiddleware: limiter.SlidingWindow{},
}))
```

Each new window also considers the previous one (if any). The rate is calculated as:

```text
weightOfPreviousWindow = previousWindowRequests * (elapsedInCurrentWindow / Expiration)
rate = weightOfPreviousWindow + currentWindowRequests
```

## Dynamic limit

You can also calculate the limit dynamically using the `MaxFunc` parameter. It receives the request context and allows you to compute a different limit for each request.

Example:

```go
app.Use(limiter.New(limiter.Config{
    MaxFunc:  func(c fiber.Ctx) int {
      return getUserLimit(ctx.Param("id"))
    },
    Expiration:     30 * time.Second,
}))
```

## Config

| Property               | Type                      | Description                                                                                 | Default                                  |
|:-----------------------|:--------------------------|:--------------------------------------------------------------------------------------------|:-----------------------------------------|
| Next                   | `func(fiber.Ctx) bool`   | Next defines a function to skip this middleware when it returns true.                         | `nil`                                    |
| Max                    | `int`                     | Maximum number of recent connections within `Expiration` seconds before sending a 429 response. | 5                                        |
| MaxFunc                | `func(fiber.Ctx) int`     | Function that calculates the maximum number of recent connections within `Expiration` seconds before sending a 429 response. | A function that returns `cfg.Max`    |
| KeyGenerator           | `func(fiber.Ctx) string` | Function to generate custom keys; uses `c.IP()` by default.                 | A function using `c.IP()` as the default   |
| Expiration             | `time.Duration`           | Duration to keep request records in memory.                   | 1 * time.Minute                          |
| LimitReached           | `fiber.Handler`           | Called when a request exceeds the limit.                                       | A function sending a 429 response          |
| SkipFailedRequests     | `bool`                    | When set to `true`, requests with status code ≥ 400 aren't counted.                         | false                                    |
| SkipSuccessfulRequests | `bool`                    | When set to `true`, requests with status code < 400 aren't counted.                          | false                                    |
| DisableHeaders         | `bool`                    | When set to `true`, the middleware omits rate limit headers (`X-RateLimit-*` and `Retry-After`). | false                                    |
| DisableValueRedaction  | `bool`                    | Disables redaction of limiter keys in error messages and logs.                                 | false                                    |
| Storage                | `fiber.Storage`           | Persists middleware state.                                         | An in-memory store for this process only |
| LimiterMiddleware      | `limiter.Handler`         | Selects the algorithm implementation. Implementations now receive a pointer to the active config when their `New` method is invoked. | A new Fixed Window Rate Limiter          |

:::note
A custom store can be used if it implements the `Storage` interface - more details and an example can be found in `store.go`.
:::

## Default Config

```go
var ConfigDefault = Config{
    Max:        5,
    MaxFunc: func(c fiber.Ctx) int {
      return 5
    },
    Expiration: 1 * time.Minute,
    KeyGenerator: func(c fiber.Ctx) string {
        return c.IP()
    },
    LimitReached: func(c fiber.Ctx) error {
        return c.SendStatus(fiber.StatusTooManyRequests)
    },
    SkipFailedRequests: false,
    SkipSuccessfulRequests: false,
    DisableHeaders:        false,
    DisableValueRedaction: false,
    LimiterMiddleware: FixedWindow{},
}
```

### Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3

app.Use(limiter.New(limiter.Config{
    Storage: storage,
}))
```
