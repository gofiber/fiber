---
id: etag
---

# ETag

ETag middleware for [Fiber](https://github.com/gofiber/fiber) that helps caches validate responses and saves bandwidth by avoiding full retransmits when content is unchanged.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/etag"
)
```

Once your Fiber app is initialized, use the middleware like this:

```go
// Initialize default config
app.Use(etag.New())

// GET / -> ETag: "13-1831710635"
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Hello, World!")
})

// Or extend your config for customization
app.Use(etag.New(etag.Config{
    Weak: true,
}))

// GET / -> ETag: W/"13-1831710635"
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Hello, World!")
})
```

Entity tags in requests must be quoted per RFC 9110. For example:

```text
If-None-Match: "example-etag"
```

## Config

| Property | Type                    | Description                                                                                                        | Default |
|:---------|:------------------------|:-------------------------------------------------------------------------------------------------------------------|:--------|
| Weak     | `bool`                  | Enables weak validators. Weak ETags are easier to generate but less reliable for comparisons. | `false` |
| Next     | `func(fiber.Ctx) bool` | Next defines a function to skip this middleware when it returns true.                                                | `nil`   |

## Default Config

```go
var ConfigDefault = Config{
    Next: nil,
    Weak: false,
}
```
