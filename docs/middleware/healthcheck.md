---
id: healthcheck
---

# Health Check

Liveness, readiness and startup probes middleware for [Fiber](https://github.com/gofiber/fiber) that provides three endpoints for checking the liveness, readiness, and startup state of HTTP applications.

## Overview

- **Liveness Probe**: Checks if the server is up and running.
  - **Default Endpoint**: `/livez`
  - **Behavior**: By default returns `true` immediately when the server is operational.

- **Readiness Probe**: Assesses if the application is ready to handle requests.
  - **Default Endpoint**: `/readyz`
  - **Behavior**: By default returns `true` immediately when the server is operational.

- **Startup Probe**: Checks if the application has completed its startup sequence and is ready to proceed with initialization and readiness checks.
  - **Default Endpoint**: `/startupz`
  - **Behavior**: By default returns `true` immediately when the server is operational.

- **HTTP Status Codes**:
  - `200 OK`: Returned when the checker function evaluates to `true`.
  - `503 Service Unavailable`: Returned when the checker function evaluates to `false`.

## Signatures

```go
func NewHealthChecker(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the [Fiber](https://github.com/gofiber/fiber) web framework

```go
import(
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/healthcheck"
)
```

After you initiate your [Fiber](https://github.com/gofiber/fiber) app, you can use the following options:

```go
// Provide a minimal config for liveness check
app.Get(healthcheck.LivenessEndpoint, healthcheck.NewHealthChecker())

// Provide a minimal config for readiness check
app.Get(healthcheck.ReadinessEndpoint, healthcheck.NewHealthChecker())

// Provide a minimal config for startup check
app.Get(healthcheck.StartupEndpoint, healthcheck.NewHealthChecker())

// Provide a minimal config for check with custom endpoint
app.Get("/live", healthcheck.NewHealthChecker())

// Or extend your config for customization
app.Get(healthcheck.LivenessEndpoint, healthcheck.NewHealthChecker(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))

// And it works the same for readiness, just change the route
app.Get(healthcheck.ReadinessEndpoint, healthcheck.NewHealthChecker(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))

// And it works the same for startup, just change the route
app.Get(healthcheck.StartupEndpoint, healthcheck.NewHealthChecker(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))

// With a custom route and custom probe
app.Get("/live", healthcheck.NewHealthChecker(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))

// It can also be used with app.All, although it will only respond to requests with the GET method
// in case of calling the route with any method which isn't GET, the return will be 404 Not Found when app.All is used
// and 405 Method Not Allowed when app.Get is used
app.All(healthcheck.ReadinessEndpoint, healthcheck.NewHealthChecker(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))
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

    // Function used for checking the liveness of the application. Returns true if the application
    // is running and false if it is not. The liveness probe is typically used to indicate if 
    // the application is in a state where it can handle requests (e.g., the server is up and running).
    // The readiness probe is typically used to indicate if the application is ready to start accepting traffic (e.g., all necessary components 
    // are initialized and dependent services are available) and the startup probe typically used to 
    // indicate if the application has completed its startup sequence and is ready to proceed with
    // initialization and readiness checks
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
    Probe:     defaultProbe,
}
```
