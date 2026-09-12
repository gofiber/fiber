---
id: compress
---

# Compress

Compression middleware for [Fiber](https://github.com/gofiber/fiber) that automatically compresses responses with `gzip`, `deflate`, `brotli`, or `zstd` based on the client's [Accept-Encoding](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept-Encoding) header.

:::note
Bodies smaller than 200 bytes remain uncompressed because compression would likely increase their size and waste CPU cycles. [See the fasthttp source](https://github.com/valyala/fasthttp/blob/497922a21ef4b314f393887e9c6147b8c3e3eda4/http.go#L1713-L1715).
:::

## Behavior

- Skips compression for responses that already define `Content-Encoding`, for range requests, `206` responses, status codes without bodies, or when either side sends `Cache-Control: no-transform`.
- The encoding is negotiated from the `Accept-Encoding` header with its weights, wildcard and list syntax honored (RFC 9110 §12.5.3): the coding the client weighs highest wins, and among equal weights the middleware prefers `br`, then `zstd`, `gzip` and `deflate`. An element with no `q` parameter, or an unparsable one, carries the default weight of `1`. A request without the header, or one that gives every supported coding a weight of `0`, is not compressed. The client's header is left untouched for later handlers.
- `HEAD` requests are not compressed; only `Accept-Encoding` is merged into `Vary`.
- When compression runs, strong `ETag` values are recomputed from the compressed bytes; a streamed body cannot be hashed without buffering it, so its strong `ETag` becomes a weak one (`W/`). When compression is skipped, `Accept-Encoding` is still merged into `Vary` unless the header is `*` or already present.
- Request-body decompression is still handled by Fiber's request APIs (for example `c.Body()`), and those decoders enforce the app `BodyLimit` for compressed payloads.

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
