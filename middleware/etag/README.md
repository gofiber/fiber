# ETag Middleware

ETag middleware for [Fiber](https://github.com/gofiber/fiber) that lets caches be more efficient and save bandwidth, as a web server does not need to resend a full response if the content has not changed.

## Table of Contents

- [ETag Middleware](#etag-middleware)
	- [Table of Contents](#table-of-contents)
	- [Signatures](#signatures)
	- [Examples](#examples)
		- [Default Config](#default-config)
		- [Custom Config](#custom-config)
	- [Config](#config)
	- [Default Config](#default-config-2)

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/etag"
)
```

After you initiate your Fiber app, you can use the following possibilities:

### Default Config

```go
app.Use(etag.New())

// Get / receives Etag: "13-1831710635" in response header
app.Get("/", func(c *fiber.Ctx) error {
	return c.SendString("Hello, World!")
})
```

### Custom Config

```go
app.Use(etag.New(etag.Config{
    Weak: true,
}))

// Get / receives Etag: "W/"13-1831710635" in response header
app.Get("/", func(c *fiber.Ctx) error {
	return c.SendString("Hello, World!")
})
```

## Config

```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Weak indicates that a weak validator is used. Weak etags are easy
	// to generate, but are far less useful for comparisons. Strong
	// validators are ideal for comparisons but can be very difficult
	// to generate efficiently. Weak ETag values of two representations
	// of the same resources might be semantically equivalent, but not
	// byte-for-byte identical. This means weak etags prevent caching
	// when byte range requests are used, but strong etags mean range
	// requests can still be cached.
	Weak bool
}
```

## Default Config

```go
var ConfigDefault = Config{
	Next: nil,
	Weak: false,
}
```
