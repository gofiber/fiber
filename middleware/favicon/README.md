# Favicon Middleware

Favicon middleware for [Fiber](https://github.com/gofiber/fiber) that ignores favicon requests or caches a provided icon in memory to improve performance by skipping disk access. User agents request favicon.ico frequently and indiscriminately, so you may wish to exclude these requests from your logs by using this middleware before your logger middleware.

**Note** This middleware is exclusively for serving the default, implicit favicon, which is GET /favicon.ico or [custom favicon URL](#config).

## Table of Contents
- [Favicon Middleware](#favicon-middleware)
	- [Table of Contents](#table-of-contents)
	- [Signatures](#signatures)
	- [Examples](#examples)
		- [Default Config](#default-config)
		- [Custom Config](#custom-config)
		- [Config](#config)
		- [Default Config](#default-config-1)
		
## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

First import the middleware from Fiber,

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/favicon"
)
```

Then create a Fiber app with `app := fiber.New()`.

### Default Config

```go
app.Use(favicon.New())
```

### Custom Config
```go
app.Use(favicon.New(favicon.Config{
	File: "./favicon.ico",
	URL: "/favicon.ico",
}))
```

### Config

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

### Default Config

```go
var ConfigDefault = Config{
	Next: nil,
	File:	"",
	URL: "/favicon.ico",
}
```
