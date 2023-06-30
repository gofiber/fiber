---
id: probecheker
title: probecheker
---

Liveness and readiness probes middleware for [Fiber](https://github.com/gofiber/fiber) that provides two endpoints for checking the health and ready state of any HTTP application.

The endpoint values default to `/livez` for liveness and `/readyz` for readiness. Both functions are optional, the liveness endpoint will return `true` right when the server is up and running but the readiness endpoint will not answer any requests if an `IsReady` function isn't provided. 

The HTTP status returned to the containerized environment are: 200 OK if the checker function returns true and 503 Service Unavailable if the checker function returns false.

## Signatures

```go
func New() fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/probechecker"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initializing with default config
app.Use(probechecker.New())

// Initialize with custom config
app.Use(
	probechecker.New(
		IsLive: func (c *fiber.Ctx) bool {
    	return true
  	},
  	IsLiveEndpoint: "/livez",
  	IsReady: func (c *fiber.Ctx) bool {
	    return serviceA.Ready() && serviceB.Ready() && ...
	  }
	  IsReadyEndpoint: "/readyz",
	)
)

```

## Config

```go
type Config struct {
	// Config for liveness probe of the container engine being used
	//
	// Optional. Default: func(c *Ctx) bool { return true }
	IsLive ProbeChecker

	// HTTP endpoint of the liveness probe
	//
	// Optional. Default: /livez
	IsLiveEndpoint string

	// Config for readiness probe of the container engine being used
	//
	// Optional. Default: nil
	IsReady ProbeChecker

	// HTTP endpoint of the readiness probe
	//
	// Optional. Default: /readyz
	IsReadyEndpoint string
}
```