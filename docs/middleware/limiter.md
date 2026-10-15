---
id: limiter
---

# Limiter

The Limiter middleware for [Fiber](https://github.com/gofiber/fiber) throttles repeated requests to public APIs or endpoints such as password resets. It's also useful for API clients, web crawlers, or other tasks that need rate limiting.

Limiter redacts request keys in error paths by default so storage identifiers and rate-limit keys don't leak into logs. Set `DisableValueRedaction` to `true` when you need the key for troubleshooting. The key shown is the one the storage sees, so a key longer than 192 bytes appears as its SHA-256 digest.

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
    "time"

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
    ExpirationFunc: func(c fiber.Ctx) time.Duration {
      // Use longer expiration for sensitive endpoints. A window keeps the length of the
      // request that opened it, so give such a route its own key (see Dynamic expiration)
      if c.Path() == "/login" {
        return 60 * time.Second
      }
      return 30 * time.Second
    },
    KeyGenerator:          func(c fiber.Ctx) string {
        return c.Get("x-forwarded-for")
    },
    LimitReached: func(c fiber.Ctx) error {
        return c.SendFile("./toofast.html")
    },
    Storage: myCustomStorage{},
}))
```

:::caution
The default `KeyGenerator` keys requests by `c.IP()`. Behind a reverse proxy, `c.IP()` only identifies a client if you configure `TrustProxy`, `TrustProxyConfig` and `ProxyHeader`, plus `EnableIPValidation` when the header is a chain your proxy appends to, such as `X-Forwarded-For`. Without them it is either the proxy's address, so every client shares one bucket, or the raw forwarded header, which a client can vary on every request to get a fresh bucket unless your proxy overwrites it with a single address. See the [reverse proxy guide](../guide/reverse-proxy.md#getting-the-real-client-ip-address).
:::

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
weightOfPreviousWindow = previousWindowRequests * (timeLeftInCurrentWindow / Expiration)
rate = ceil(weightOfPreviousWindow) + currentWindowRequests
```

The previous window therefore counts in full at the start of a window and fades out as the window runs.

## Dynamic limit

You can also calculate the limit dynamically using the `MaxFunc` parameter. It receives the request context and allows you to compute a different limit for each request. Returning `0` skips the limiter for that request, and a negative value rejects every request. `Next` runs before `MaxFunc`, so a request that `Next` skips never reaches it.

Example:

```go
app.Use("/users/:id", limiter.New(limiter.Config{
    MaxFunc:  func(c fiber.Ctx) int {
      return getUserLimit(c.Params("id"))
    },
    Expiration:     30 * time.Second,
}))
```

## Dynamic expiration

You can also calculate the expiration dynamically using the `ExpirationFunc` parameter. It receives the request context and allows you to set a different expiration window for each request.

Window accounting is whole-second, so a positive duration below one second is treated as a one-second window. A zero or negative duration falls back to the configured `Expiration`, which is one minute unless you set another.

Requests that share a key share one window, and a window keeps the length given by the request that opened it. With the default `KeyGenerator`, which keys by client IP, a request that arrives while a window opened by another route is running joins that window: its `X-RateLimit-Reset` is what remains of it, whatever `ExpirationFunc` returns for the request. A route that needs a window of its own needs a key of its own.

The sliding window weighs the previous window's hits by the time left in the open window divided by the length the current request asks for, capped at 1. That is exact only while every request for a key asks for the same length: a longer one counts fewer of the previous hits.

Example:

```go
app.Use(limiter.New(limiter.Config{
    Max:     20,
    ExpirationFunc: func(c fiber.Ctx) time.Duration {
      return getExpirationForRoute(c.Path())
    },
    // One window per client and route
    KeyGenerator: func(c fiber.Ctx) string {
      return c.IP() + c.Path()
    },
}))
```

## Concurrency and storage access

The limiter reads an entry, updates it and writes it back while holding a lock, so concurrent requests for one key through the same limiter never read the same count. Keys are spread over 64 locks, which lets requests for different keys run in parallel. A lock is held for the storage round trips of its request: with a slow external storage, keys that share a lock wait for each other, and requests for one key are handled one at a time. That is the price of exact counts within one limiter, because `fiber.Storage` offers no atomic increment to rely on. The locks are not shared between limiters, so two limiters, in one process or several, that use the same storage and key can still admit a few extra requests when they handle that key at the same moment.

A request that passes costs one storage read and one write. A second read and write follows the handler when:

- `SkipSuccessfulRequests` applies to the response (status below 400) or `SkipFailedRequests` does (status 400 or above), to give the hit back.
- The algorithm is the sliding window and headers are enabled, so that `X-RateLimit-Remaining` and `X-RateLimit-Reset` describe the state at the end of the request. Set `DisableHeaders` to avoid it.

## Config

| Property               | Type                      | Description                                                                                 | Default                                  |
|:-----------------------|:--------------------------|:--------------------------------------------------------------------------------------------|:-----------------------------------------|
| Next                   | `func(fiber.Ctx) bool`   | Next defines a function to skip this middleware when it returns true. It runs before `MaxFunc`. | `nil`                                    |
| Max                    | `int`                     | Maximum number of recent connections within `Expiration` seconds before sending a 429 response. A value of `0` or less selects the default. | 5                                        |
| MaxFunc                | `func(fiber.Ctx) int`     | Function that calculates the maximum number of recent connections within `Expiration` seconds before sending a 429 response. Returning `0` skips the limiter for that request; a negative value rejects every request. | A function that returns `cfg.Max`    |
| KeyGenerator           | `func(fiber.Ctx) string` | Function to generate custom keys; uses `c.IP()` by default. A key longer than 192 bytes, or starting with `sha256:`, is replaced by its SHA-256 digest (`sha256:<hex>`) before it reaches the storage, so oversized keys are still limited per client. | A function using `c.IP()` as the default   |
| Expiration             | `time.Duration`           | Duration to keep request records in memory, which is also the length of the window. It is counted in whole seconds: a positive value below one second is floored to one second, and zero or a negative value selects the default. | 1 * time.Minute                          |
| ExpirationFunc         | `func(fiber.Ctx) time.Duration` | Function that calculates the expiration duration dynamically. Positive values below one second are floored to one second; non-positive values fall back to `Expiration`. Requests that share a key share a window, which keeps the length given by the request that opened it. | A function that returns `cfg.Expiration` |
| LimitReached           | `fiber.Handler`           | Called when a request exceeds the limit.                                       | A function sending a 429 response          |
| SkipFailedRequests     | `bool`                    | When set to `true`, requests with status code ≥ 400 aren't counted. A handler that returns an error other than a `*fiber.Error` counts as a 500 unless it already set an error status. | false                                    |
| SkipSuccessfulRequests | `bool`                    | When set to `true`, requests with status code < 400 aren't counted. A handler that returns an error other than a `*fiber.Error` counts as a 500 unless it already set an error status. | false                                    |
| DisableHeaders         | `bool`                    | When set to `true`, the middleware omits rate limit headers (`X-RateLimit-*` and `Retry-After`). Otherwise a request that passes carries `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `X-RateLimit-Reset` (seconds until the window ends), and a rejected one carries `Retry-After` (seconds). | false                                    |
| DisableValueRedaction  | `bool`                    | Disables redaction of limiter keys in error messages and logs. A key longer than 192 bytes is shown as its SHA-256 digest. | false                                    |
| Storage                | `fiber.Storage`           | Persists middleware state.                                         | An in-memory store for this process only |
| LimiterMiddleware      | `limiter.Handler`         | Selects the algorithm implementation. Implementations now receive a pointer to the active config when their `New` method is invoked. | A new Fixed Window Rate Limiter          |

:::note
A custom store can be used if it implements the `fiber.Storage` interface, which is declared in [`storage_interface.go`](https://github.com/gofiber/fiber/blob/main/storage_interface.go).
:::

## Default Config

```go
var ConfigDefault = Config{
    Max:        5,
    // MaxFunc defaults to nil and is set dynamically to return cfg.Max
    Expiration: 1 * time.Minute,
    // ExpirationFunc defaults to nil and is set dynamically to return cfg.Expiration
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
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3/v2

app.Use(limiter.New(limiter.Config{
    Storage: storage,
}))
```
