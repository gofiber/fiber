---
id: favicon
title: Favicon
---

Favicon middleware for [Fiber](https://github.com/gofiber/fiber) that ignores favicon requests or caches a provided icon in memory to improve performance by skipping disk access. User agents request favicon.ico frequently and indiscriminately, so you may wish to exclude these requests from your logs by using this middleware before your logger middleware.

:::note
This middleware is exclusively for serving the default, implicit favicon, which is GET /favicon.ico or [custom favicon URL](#config).
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
  "github.com/gofiber/fiber/v2/middleware/favicon"
)
```

After you initiate your Fiber app, you can use the following possibilities:

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

```go
// Config defines the config for middleware.
type Config struct {
    // Next defines a function to skip this middleware when returned true.
    //
    // Optional. Default: nil
    Next func(c *fiber.Ctx) bool

    // File holds the path to an actual favicon that will be cached
    //
    // Optional. Default: ""
    File string
	
    // URL for favicon handler
    //
    // Optional. Default: "/favicon.ico"
    URL string

    // FileSystem is an optional alternate filesystem to search for the favicon in.
    // An example of this could be an embedded or network filesystem
    //
    // Optional. Default: nil
    FileSystem http.FileSystem

    // CacheControl defines how the Cache-Control header in the response should be set
    //
    // Optional. Default: "public, max-age=31536000"
    CacheControl string
}
```

## Default Config

```go
var ConfigDefault = Config{
	Next:         nil,
	File:         "",
	URL:          fPath,
	CacheControl: "public, max-age=31536000",
}
```
