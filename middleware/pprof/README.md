# Pprof
Pprof middleware for [Fiber](https://github.com/gofiber/fiber) that serves via its HTTP server runtime profiling data in the format expected by the pprof visualization tool. The package is typically only imported for the side effect of registering its HTTP handlers. The handled paths all begin with /debug/pprof/.

- [Signatures](#signatures)
- [Examples](#examples)

### Signatures
```go
func New() fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/pprof"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Default middleware
app.Use(pprof.New())
```

In systems where you have multiple ingress endpoints, it is common to add a URL prefix, like so:

```go
// Default middleware
app.Use(pprof.New(pprof.Config{Prefix: "/endpoint-prefix"}))
```

This prefix will be added to the default path of "/debug/pprof/", for a resulting URL of:
"/endpoint-prefix/debug/pprof/".

## Config

```go
// Config defines the config for middleware.
type Config struct {	
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Prefix defines a URL prefix added before "/debug/pprof".
	// Note that it should start with (but not end with) a slash.
	// Example: "/federated-fiber"
	//
	// Optional. Default: ""
	Prefix string
}
```

## Default Config

```go
var ConfigDefault = Config{
	Next: nil,
}
```
