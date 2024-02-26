---
id: healthcheck
---

# Health Check

Liveness and readiness probes middleware for [Fiber](https://github.com/gofiber/fiber) that provides two endpoints for checking the liveness and readiness state of HTTP applications.

## Overview

- **Liveness Probe**: Checks if the server is up and running.
  - **Default Endpoint**: `/livez`
  - **Behavior**: By default returns `true` immediately when the server is operational.

- **Readiness Probe**: Assesses if the application is ready to handle requests.
  - **Default Endpoint**: `/readyz`
  - **Behavior**: By default returns `true` immediately when the server is operational.

- **HTTP Status Codes**:
  - `200 OK`: Returned when the checker function evaluates to `true`.
  - `503 Service Unavailable`: Returned when the checker function evaluates to `false`.

## Signatures

```go
func New(config Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the [Fiber](https://github.com/gofiber/fiber) web framework
```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/healthcheck"
)
```

After you initiate your [Fiber](https://github.com/gofiber/fiber) app, you can use the following possibilities:

```go
// Provide a minimal config for liveness check
app.Get(healthcheck.DefaultLivenessEndpoint, healthcheck.New())
// Provide a minimal config for readiness check
app.Get(healthcheck.DefaultReadinessEndpoint, healthcheck.New())
// Provide a minimal config for check with custom endpoint
app.Get("/live", healthcheck.New())

// Or extend your config for customization
app.Get(healthcheck.DefaultLivenessEndpoint, healthcheck.New(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))
// And it works the same for readiness, just change the route
app.Get(healthcheck.DefaultReadinessEndpoint, healthcheck.New(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))
// With a custom route and custom probe
app.Get("/live", healthcheck.New(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))

// It can also be used with app.All, although it will only respond to requests with the GET method
// in case of calling the route with any method which isn't GET, the return will be 404 Not Found when app.All is used
// and 405 Method Not Allowed when app.Get is used
app.All(healthcheck.DefaultReadinessEndpoint, healthcheck.New(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))
```

## Config

```go
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(fiber.Ctx) bool

	// Function used for checking the liveness of the application. Returns true if the application
	// is running and false if it is not. The liveness probe is typically used to indicate if 
	// the application is in a state where it can handle requests (e.g., the server is up and running).
	//
	// Optional. Default: func(c fiber.Ctx) bool { return true }
	Probe HealthChecker
}
```

## Default Config

The default configuration used by this middleware is defined as follows:
```go
func defaultProbe(fiber.Ctx) bool { return true }

var ConfigDefault = Config{
	Probe:     defaultProbe,
}
```
