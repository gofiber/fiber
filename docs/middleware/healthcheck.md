---
id: healthcheck
---

# Health Check

Middleware to implement liveness, readiness, and startup probes for [Fiber](https://github.com/gofiber/fiber) applications. It exposes a generic probe handler; you decide which endpoints to register. Convenience constants are available for the conventional endpoints (`/livez`, `/readyz`, and `/startupz`).

## Overview

Register the middleware on any endpoint you want to expose a probe on. The package exports constants for the conventional liveness, readiness, and startup endpoints:

```go
app.Get(healthcheck.LivenessEndpoint, healthcheck.New())
app.Get(healthcheck.ReadinessEndpoint, healthcheck.New())
app.Get(healthcheck.StartupEndpoint, healthcheck.New())
```

By default, the probe returns `true`, so each registered endpoint responds with `200 OK`. When the probe returns `false`, the middleware responds with `503 Service Unavailable`.

- **Liveness**: Checks if the server is running.
- **Readiness**: Checks if the application is ready to handle requests.
- **Startup**: Checks if the application has completed its startup sequence.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the [Fiber](https://github.com/gofiber/fiber) web framework

```go
import(
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/healthcheck"
)
```

After you initiate your [Fiber](https://github.com/gofiber/fiber) app, register the middleware on the endpoints you need:

```go
// Use the default probe on the conventional endpoints
app.Get(healthcheck.LivenessEndpoint, healthcheck.New())
app.Get(healthcheck.ReadinessEndpoint, healthcheck.New(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return serviceA.Ready() && serviceB.Ready()
    },
}))
app.Get(healthcheck.StartupEndpoint, healthcheck.New())

// Register a custom endpoint
app.Get("/healthz", healthcheck.New())
```

The middleware only responds to GET requests. To register a probe on all methods, use `app.All`. Non-GET methods fall through to the next handler:

```go
app.All("/healthz", healthcheck.New())
```

## Config

```go
type Config struct {
    // Next defines a function to skip this middleware when returned true. If this function returns true
    // and no other handlers are defined for the route, Fiber will return a status 404 Not Found, since
    // no other handlers were defined to return a different status.
    //
    // Optional. Default: nil
    Next func(fiber.Ctx) bool

    // Probe is executed to determine the current health state. It can be used for
    // liveness, readiness or startup checks. Returning true indicates the application
    // is healthy.
    //
    // Optional. Default: func(c fiber.Ctx) bool { return true }
    Probe func(fiber.Ctx) bool
}
```

## Default Config

The default configuration used by this middleware is defined as follows:

```go
func defaultProbe(_ fiber.Ctx) bool { return true }

var ConfigDefault = Config{
    Next:  nil,
    Probe: defaultProbe,
}
```
