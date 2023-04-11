---
id: compress
title: Compress
---

Compression middleware for [Fiber](https://github.com/gofiber/fiber) that will compress the response using `gzip`, `deflate` and `brotli` compression depending on the [Accept-Encoding](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept-Encoding) header.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/compress"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
app.Use(compress.New())

// Or extend your config for customization
app.Use(compress.New(compress.Config{
    Level: compress.LevelBestSpeed, // 1
}))

// Skip middleware for specific routes
app.Use(compress.New(compress.Config{
  Next:  func(c *fiber.Ctx) bool {
    return c.Path() == "/dont_compress"
  },
  Level: compress.LevelBestSpeed, // 1
}))
```

## Config

```go
// Config defines the config for middleware.
type Config struct {
    // Next defines a function to skip this middleware when returned true.
    //
    // Optional. Default: nil
    Next func(c *fiber.Ctx) bool

    // Level determines the compression algoritm
    //
    // Optional. Default: LevelDefault
    // LevelDisabled:         -1
    // LevelDefault:          0
    // LevelBestSpeed:        1
    // LevelBestCompression:  2
    Level int
}
```

## Default Config

```go
var ConfigDefault = Config{
    Next:  nil,
    Level: LevelDefault,
}
```

## Constants

```go
// Compression levels
const (
    LevelDisabled        = -1
    LevelDefault         = 0
    LevelBestSpeed       = 1
    LevelBestCompression = 2
)
```
