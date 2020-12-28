# Favicon Middleware

Favicon middleware for [Fiber](https://github.com/gofiber/fiber) that ignores favicon requests or caches a provided icon in memory to improve performance by skipping disk access. User agents request favicon.ico frequently and indiscriminately, so you may wish to exclude these requests from your logs by using this middleware before your logger middleware.

**Note** This middleware is exclusively for serving the default, implicit favicon, which is GET /favicon.ico.

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
  "github.com/gofiber/fiber/v2/middleware/cors"
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
}
```

### Default Config

```go
var ConfigDefault = Config{
	Next: nil,
	File:	""
}
```
