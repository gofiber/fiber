# Request ID

Adds an indentifier to the response using the `X-Request-ID` header

### Example
```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/middleware"
)

func main() {
  app := fiber.New()

  // Default
  app.Use(middleware.RequestID())

  // Custom Header
  app.Use(middleware.RequestID("X-Custom-Header"))
  
  // Custom Config
  app.Use(middleware.RequestID(middleware.RequestIDConfig{
    Next: func(ctx *fiber.Ctx) bool {
      return ctx.Method() != fiber.MethodPost
    },
    Header: "X-Custom-Header",
    Generator: func() string {
      return "1234567890"
    }
  }))

  app.Listen(3000)
}
```

### Signatures
```go
func RequestID(header ...string) fiber.Handler {}
func RequestIDWithConfig(config RequestIDConfig) fiber.Handler {}
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
### Default Config
```go
var RequestIDConfigDefault = RequestIDConfig{
	Next:   nil,
	Header: fiber.HeaderXRequestID,
	Generator: func() string {
		return utils.UUID()
	},
}

```