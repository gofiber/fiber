# Favicon
Favicon middleware ignores favicon requests or caches a provided icon in memory to improve performance by skipping disk access. User agents request `/favicon.ico` frequently and indiscriminately, so you may wish to exclude these requests from your logs by using this middleware before your logger middleware.

**Note**: This middleware is exclusively for serving the default, implicit favicon, which is `GET /favicon.ico`.

- [Signatures](#signatures)
- [Config](#config)
- [Examples](#examples)

<!-- 
### Config

| Signature | Description | Required | Default |
| :--- | :--- | ---: | ---: |
| `Next func(c *fiber.Ctx) bool` | Defines a function to skip this middleware when returned true. | `✘` | `nil` |
| `Level int` | Determines the compression algoritm: `-1`, `0`, `1` or `2` | `✔` | `0` | -->

### Signatures
```go
func New(config ...Config) fiber.Handler
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

### Example
Import the compress package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/favicon"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Default favicon config
app.Use(favicon.New())

// Provide a custom favicon to load into memory
app.Use(favicon.New(favicon.Config{
    File: "./favicon.ico",
}))

// Skip middleware for specific routes
app.Use(compress.New(compress.Config{
  Next:  func(c *fiber.Ctx) bool {
    return c.Path() == "/admin"
  },
  File: "./favicon.ico",
}))
```