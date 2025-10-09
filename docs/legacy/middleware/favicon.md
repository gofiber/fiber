---
id: favicon
---

# Favicon

Favicon middleware for [Fiber](https://github.com/gofiber/fiber) that drops repeated `/favicon.ico` requests or serves a cached icon from memory. Mount it before your logger to suppress noisy requests and avoid disk reads.

It handles only `GET`, `HEAD`, and `OPTIONS` to the configured URL; other methods return `405 Method Not Allowed`.

:::note
This middleware only serves the default `/favicon.ico` (or a [custom URL](#config)). For multiple icons, use the Static middleware.
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
    "github.com/gofiber/fiber/v3/middleware/favicon"
)
```

Once your Fiber app is initialized, use the middleware like this:

```go
// Initialize default config
app.Use(favicon.New())

// Or extend your config for customization
app.Use(favicon.New(favicon.Config{
    File: "./favicon.ico",
    URL: "/favicon.ico",
}))
```

## Config

| Property     | Type                    | Description                                                                      | Default                    |
|:-------------|:------------------------|:---------------------------------------------------------------------------------|:---------------------------|
| Next         | `func(fiber.Ctx) bool` | Next defines a function to skip this middleware when it returns true.              | `nil`                      |
| Data         | `[]byte`                | Raw data of the favicon file. This can be used instead of `File`.                | `nil`                      |
| File         | `string`                | File holds the path to an actual favicon that will be cached.                    | ""                         |
| URL          | `string`                | URL for favicon handler.                                                         | "/favicon.ico"             |
| FileSystem   | `fs.FS`                 | FileSystem is an optional alternate filesystem from which to load the favicon file (e.g. using `os.DirFS` or an `embed.FS`). | `nil`                      |
| CacheControl | `string`                | CacheControl defines how the Cache-Control header in the response should be set. | "public, max-age=31536000" |

## Default Config

```go
var ConfigDefault = Config{
    Next:         nil,
    File:         "",
    URL:          fPath,
    CacheControl: "public, max-age=31536000",
}
```
