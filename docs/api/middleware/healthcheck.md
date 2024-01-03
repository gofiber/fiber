---
id: healthcheck
title: healthcheck
---

Liveness and readiness probes middleware for [Fiber](https://github.com/gofiber/fiber) that provides two endpoints for checking the health and ready state of HTTP applications.

## Overview

- **Liveness Probe**: Checks if the server is up and running.
  - **Default Endpoint**: `/livez`
  - **Behavior**: Returns `true` immediately when the server is operational.

- **Readiness Probe**: Assesses if the application is ready to handle requests.
  - **Default Endpoint**: `/readyz`
  - **Behavior**: Requires an `IsReady` function implementation. Without this function, the endpoint does not respond.

- **HTTP Status Codes**:
  - `200 OK`: Returned when the checker function evaluates to `true`.
  - `503 Service Unavailable`: Returned when the checker function evaluates to `false`.

## Usage

### Installation

First, import the healthcheck middleware package from the Fiber web framework:
```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/healthcheck"
)
```

### Implementation

After initializing your Fiber app, configure the middleware as follows:

**Default Configuration**:
```go
app.Use(healthcheck.New())
```

**Custom Configuration**:
```go
app.Use(healthcheck.New(healthcheck.Config{
    IsLive: func(c *fiber.Ctx) bool {
        return true
    },
    LivenessEndpoint: "/live",
    IsReady: func(c *fiber.Ctx) bool {
        return serviceA.Ready() && serviceB.Ready() && ...
    },
    ReadinessEndpoint: "/ready",
}))
```

## Configuration Options

The `Config` struct offers the following customization options:
```go
type Config struct {
    // Function to skip middleware. Optional. Default: nil
    Next func(c *fiber.Ctx) bool

    // Liveness probe configuration. Optional. Default: Always true
    IsLive HealthChecker

    // Liveness probe HTTP endpoint. Optional. Default: "/livez"
    LivenessEndpoint string

    // Readiness probe configuration. Optional. Default: nil
    IsReady HealthChecker

    // Readiness probe HTTP endpoint. Optional. Default: "/readyz"
    ReadinessEndpoint string
}
```

## Default Configuration

The default configuration is defined as follows:
```go
func defaultLivenessFunc(*fiber.Ctx) bool { return true }

var ConfigDefault = Config{
    IsLive:            defaultLivenessFunc,
    LivenessEndpoint:  "/livez",
    ReadinessEndpoint: "/readyz",
}
```