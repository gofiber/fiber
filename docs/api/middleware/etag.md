---
id: etag
---

# ETag

ETag middleware for [Fiber](https://github.com/gofiber/fiber) that lets caches be more efficient and save bandwidth, as a web server does not need to resend a full response if the content has not changed.

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

```go
// Initialize default config
app.Use(etag.New())

// Get / receives Etag: "13-1831710635" in response header
app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("Hello, World!")
})

// Or extend your config for customization
app.Use(etag.New(etag.Config{
    Weak: true,
}))

// Get / receives Etag: "W/"13-1831710635" in response header
app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("Hello, World!")
})
```

## Config

| Property | Type                    | Description                                                                                                        | Default |
|:---------|:------------------------|:-------------------------------------------------------------------------------------------------------------------|:--------|
| Weak     | `bool`                  | Weak indicates that a weak validator is used. Weak etags are easy to generate but are less useful for comparisons. | `false` |
| Next     | `func(*fiber.Ctx) bool` | Next defines a function to skip this middleware when returned true.                                                | `nil`   |

## Default Config

```go
var ConfigDefault = Config{
    Next: nil,
    Weak: false,
}
```
