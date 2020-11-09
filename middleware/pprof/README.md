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
