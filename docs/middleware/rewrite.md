---
id: rewrite
---

# Rewrite

The Rewrite middleware remaps the request path using custom rules, helping with backward compatibility and cleaner URLs.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Config

| Property | Type                  | Description                                           | Default    |
|:---------|:----------------------|:------------------------------------------------------|:-----------|
| Next     | `func(fiber.Ctx) bool` | Skip when function returns `true`.                    | `nil`      |
| Rules    | `map[string]string`   | Map paths to new values; use `$1`, `$2` for wildcard captures.| (Required) |

:::note
Rules are stored in a map, so iteration order is undefined. Avoid overlapping patterns if precedence matters.
:::

### Examples

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/rewrite"
)

func main() {
    app := fiber.New()

    app.Use(rewrite.New(rewrite.Config{
      Rules: map[string]string{
        "/old":   "/new",
        "/old/*": "/new/$1",
      },
    }))

    app.Get("/new", func(c fiber.Ctx) error {
      return c.SendString("Hello, World!")
    })
    app.Get("/new/*", func(c fiber.Ctx) error {
      return c.SendString("Wildcard: " + c.Params("*"))
    })

    app.Listen(":3000")
}
```

## Test

```bash
curl http://localhost:3000/old
curl http://localhost:3000/old/hello
```
