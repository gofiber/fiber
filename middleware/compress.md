# Compress

Compression middleware for Fiber, it supports `deflate`, `gzip` and `brotli` by default.

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
  app.Use(middleware.Compress())

  // Custom compression level
  app.Use(middleware.Compress(middleware.CompressLevelBestSpeed))

  // Custom Config
  app.Use(middleware.CompressWithConfig(middleware.LoggerConfig{
    Next: func(ctx *fiber.Ctx) bool {
      return strings.HasPrefix(ctx.Path(), "/static")
    },
    Level: middleware.CompressLevelBestCompression,
  }))

  app.Listen(3000)
}
```

### Signatures
```go
func Compress(level ...int) fiber.Handler {}
func CompressWithConfig(config CompressConfig) fiber.Handler {}
```

### Config
```go
type CompressConfig struct {
  // Next defines a function to skip this middleware.
  Next func(ctx *fiber.Ctx) bool
  // Compression level for brotli, gzip and deflate
  Level int
}
```
### Compression Levels
```go
const (
	CompressLevelDisabled        = -1
	CompressLevelDefault         = 0
	CompressLevelBestSpeed       = 1
	CompressLevelBestCompression = 2
)
```

### Default Config
```go
var CompressConfigDefault = CompressConfig{
	Next:  nil,
	Level: CompressLevelDefault,
}
```