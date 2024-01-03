---
id: healthcheck
title: healthcheck
---

Liveness and readiness probes middleware for [Fiber](https://github.com/gofiber/fiber) that provides two endpoints for checking the health and ready state of HTTP applications.

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
    LivenessProbe: func(c *fiber.Ctx) bool {
        return true
    },
    LivenessEndpoint: "/live",
    ReadinessProbe: func(c *fiber.Ctx) bool {
        return serviceA.Ready() && serviceB.Ready() && ...
    },
    ReadinessEndpoint: "/ready",
}))
```

## Configuration Options

The `Config` struct offers the following customization options:
```go
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Function to be used for checking the liveness of the application.
	// Returns true if the application is running and false if it is not.
	// The liveness probe is typically used to indicate if the application is in a state where
	// it can handle requests (e.g., the server is up and running).
	//
	// Optional. Default: func(c *fiber.Ctx) bool { return true }
	LivenessProbe HealthChecker

	// HTTP endpoint at which the liveness probe will be available.
	//
	// Optional. Default: "/livez"
	LivenessEndpoint string

	// Function to be used for checking the readiness of the application.
	// Returns true if the application is ready to process requests and false otherwise.
	// The readiness probe typically checks if all necessary services, databases, and other dependencies
	// are available for the application to function correctly.
	//
	// Optional. Default: func(c *fiber.Ctx) bool { return true }
	ReadinessProbe HealthChecker

	// HTTP endpoint at which the readiness probe will be available.
	// Optional. Default: "/readyz"
	ReadinessEndpoint string
}
```

## Default Configuration

The default configuration is defined as follows:
```go
const (
	DefaultLivenessEndpoint  = "/livez"
	DefaultReadinessEndpoint = "/readyz"
)

func defaultLivenessProbe(*fiber.Ctx) bool { return true }

func defaultReadinessProbe(*fiber.Ctx) bool { return true }

// ConfigDefault is the default config
var ConfigDefault = Config{
	LivenessProbe:     defaultLivenessProbe,
	ReadinessProbe:    defaultReadinessProbe,
	LivenessEndpoint:  DefaultLivenessEndpoint,
	ReadinessEndpoint: DefaultReadinessEndpoint,
}
```