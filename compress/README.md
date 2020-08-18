# Compress
Compression middleware for Fiber that supports `gzip`, `deflate` and `brotlit` compression depending on the `Accept-Encoding` header.

### Example
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/compress"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Default compression config
app.Use(compress.New())

// Provide a custom compression level
app.Use(compress.New(compress.Config{
    Level: compress.LevelBestSpeed, // 1
}))

// Skip compression for specific routes
app.Use(compress.New(compress.Config{
  Next:  func(c *fiber.Ctx) bool {
    return c.Path() == "/dont_compress"
  },
  Level: compress.LevelBestSpeed, // 1
})
```

### Signatures
```go
func New(config ...Config) fiber.Handler {}
```

### Config
```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// CompressLevel determins the compression algoritm
	//
	// Optional. Default: LevelDefault
	// LevelDisabled:         -1
	// LevelDefault:          0
	// LevelBestSpeed:        1
	// LevelBestCompression:  2
	Level int
}

```
### Compression Levels
```go
// Compression levels
const (
	LevelDisabled        = -1
	LevelDefault         = 0
	LevelBestSpeed       = 1
	LevelBestCompression = 2
)
```