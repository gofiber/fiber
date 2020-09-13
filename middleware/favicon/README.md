# Favicon Authentication
Favicon middleware for [Fiber](https://github.com/gofiber/fiber) that ignores favicon requests or caches a provided icon in memory to improve performance by skipping disk access. User agents request favicon.ico frequently and indiscriminately, so you may wish to exclude these requests from your logs by using this middleware before your logger middleware.

**Note** This middleware is exclusively for serving the default, implicit favicon, which is GET /favicon.ico.

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)


### Signatures
```go
func New(config ...Config) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/favicon"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Provide a minimal config
app.Use(favicon.New())

// Or extend your config for customization
app.Use(favicon.New(favicon.Config{
	File: "./favicon.ico"
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
