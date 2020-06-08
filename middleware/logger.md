# Logger

HTTP request/response logger for Fiber

### Example
```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/middleware"
)

func main() {
  app := fiber.New()

  // Default
  app.Use(middleware.Logger())

  // Custom logging format
  app.Use(middleware.Logger("${method} - ${path}"))

  // Custom Config
  app.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
    Next: func(ctx *fiber.Ctx) bool {
      return ctx.Path() != "/private"
    },
    Format: "${method} - ${path}",
    Output: io.Writer,
  }))

  app.Listen(3000)
}
```

### Signatures
```go
func Logger(format ...string) fiber.Handler {}
func LoggerWithConfig(config LoggerConfig) fiber.Handler {}
```

### Config
```go
type LoggerConfig struct {
  // Next defines a function to skip this middleware.
  Next func(ctx *fiber.Ctx) bool

  // Format defines the logging tags
  //
  // - time
  // - ip
  // - ips
  // - url
  // - host
  // - method
  // - path
  // - protocol
  // - route
  // - referer
  // - ua
  // - latency
  // - status
  // - body
  // - error
  // - bytesSent
  // - bytesReceived
  // - header:<key>
  // - query:<key>
  // - form:<key>
  // - cookie:<key>
  //
  // Optional. Default: ${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n
  Format string

  // TimeFormat https://programming.guide/go/format-parse-string-time-date-example.html
  //
  // Optional. Default: 15:04:05
  TimeFormat string

  // Output is a writter where logs are written
  //
  // Default: os.Stderr
  Output io.Writer
}
```
### Default Config
```go
var LoggerConfigDefault = LoggerConfig{
	Next:       nil,
	Format:     "${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n",
	TimeFormat: "15:04:05",
	Output:     os.Stderr,
}
```