---
id: compress
---

# Compress

Compression middleware for [Fiber](https://github.com/gofiber/fiber) that automatically compresses responses with `gzip`, `deflate`, `brotli`, or `zstd` based on the client's [Accept-Encoding](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept-Encoding) header.

:::note
Bodies smaller than 200 bytes remain uncompressed because compression would likely increase their size and waste CPU cycles. [See the fasthttp source](https://github.com/valyala/fasthttp/blob/497922a21ef4b314f393887e9c6147b8c3e3eda4/http.go#L1713-L1715).
:::

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/compress"
)
```

Once your Fiber app is initialized, use the middleware like this:

```go
// Initialize default config
app.Use(compress.New())

// Or extend your config for customization
app.Use(compress.New(compress.Config{
    Level: compress.LevelBestSpeed, // 1
}))

// Skip middleware for specific routes
app.Use(compress.New(compress.Config{
    Next:  func(c fiber.Ctx) bool {
      return c.Path() == "/dont_compress"
    },
    Level: compress.LevelBestSpeed, // 1
}))
```

## Config

| Property | Type                   | Description                                                 | Default            |
|:-------- |:-----------------------|:------------------------------------------------------------|:-------------------|
| Next     | `func(fiber.Ctx) bool` | Skips this middleware when the function returns `true`.     | `nil`              |
| Level    | `Level`                | Compression level to use.                                   | `LevelDefault (0)` |

Possible values for the "Level" field are:

- `LevelDisabled (-1)`: Compression is disabled.
- `LevelDefault (0)`: Default compression level.
- `LevelBestSpeed (1)`: Best compression speed.
- `LevelBestCompression (2)`: Best compression.

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
