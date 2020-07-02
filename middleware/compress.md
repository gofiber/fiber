# Compress
Compression middleware for Fiber with support for `deflate`, `gzip` and `brotli`.

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
// Default compression config
app.Use(middleware.Compress())

// Provide a custom compression level
app.Use(middleware.Compress(middleware.CompressLevelBestSpeed))

// Provide a full CompressConfig
app.Use(middleware.Compress(middleware.CompressConfig{
  Next:  func(c *fiber.Ctx) bool {
    return c.Path() == "/ignore"
  },
  Level: CompressLevelDefault,
})
```

### Signatures
```go
func Compress(options ...interface{}) fiber.Handler {}
```

### Config
```go
type CompressConfig struct {
  // Next defines a function to skip this middleware.
  // Default: nil
  Next func(*fiber.Ctx) bool

  // Compression level for brotli, gzip and deflate
  // Default: CompressLevelDefault
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