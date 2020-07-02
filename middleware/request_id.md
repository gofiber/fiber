# RequestID
Adds an indentifier to the response using the `X-Request-ID` header

### Example
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/middleware"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
func main() {
  app := fiber.New()
      
  // Default RequestID
  app.Use(middleware.RequestID())

  // Custom Header
  app.Use(middleware.RequestID("X-Custom-Header"))

  // Custom ID generator
  app.Use(middleware.RequestID(func() string {
    return "1234567890"
  }))

  // Custom Config
  app.Use(middleware.RequestID(middleware.RequestIDConfig{
    Next: func(ctx *fiber.Ctx) bool {
      return ctx.Method() != fiber.MethodPost
    },
    Header: "X-Custom-Header",
    Generator: func() string {
      return "1234567890"
    },
  }))

  // ...
}
```

### Signatures
```go
func RequestID(options ...interface{}) fiber.Handler {}
```

### Config
```go
type RequestIDConfig struct {		
  // Next defines a function to skip this middleware.
  Next func(ctx *fiber.Ctx) bool

  // Header is the header key where to get/set the unique ID
  // Optiona. Defaults: X-Request-ID
  Header string

  // Generator defines a function to generate the unique identifier.
  // Optional. Default: func() string {
  //   return utils.UUID()
  // }
  Generator func() string
}
```