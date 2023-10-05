---
id: compress
---

# Compress

Compression middleware for [Fiber](https://github.com/gofiber/fiber) that will compress the response using `gzip`, `deflate` and `brotli` compression depending on the [Accept-Encoding](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept-Encoding) header.

:::note
The compression middleware refrains from compressing bodies that are smaller than 200 bytes. This decision is based on the observation that, in such cases, the compressed size is likely to exceed the original size, making compression inefficient. [more](https://github.com/valyala/fasthttp/blob/497922a21ef4b314f393887e9c6147b8c3e3eda4/http.go#L1713-L1715)
:::

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

### Config

| Property | Type                    | Description                                                         | Default            |
|:---------|:------------------------|:--------------------------------------------------------------------|:-------------------|
| Next     | `func(*fiber.Ctx) bool` | Next defines a function to skip this middleware when returned true. | `nil`              |
| Level    | `Level`                 | Level determines the compression algorithm.                         | `LevelDefault (0)` |

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
