---
id: rewrite
---

# Rewrite

Rewrite middleware rewrites the URL path based on provided rules. It can be helpful for backward compatibility or just creating cleaner and more descriptive links.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Config

| Property | Type                    | Description                                                                                          | Default    |
|:---------|:------------------------|:-----------------------------------------------------------------------------------------------------|:-----------|
| Next     | `func(*fiber.Ctx) bool` | Next defines a function to skip middleware.                                                          | `nil`      |
| Rules    | `map[string]string`     | Rules defines the URL path rewrite rules. The values captured in asterisk can be retrieved by index. | (Required) |

### Examples
```go
package main

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/rewrite"
)

func main() {
  app := fiber.New()
  
  app.Use(rewrite.New(rewrite.Config{
    Rules: map[string]string{
      "/old":   "/new",
      "/old/*": "/new/$1",
    },
  }))
  
  app.Get("/new", func(c *fiber.Ctx) error {
    return c.SendString("Hello, World!")
  })
  app.Get("/new/*", func(c *fiber.Ctx) error {
    return c.SendString("Wildcard: " + c.Params("*"))
  })
  
  app.Listen(":3000")
}

```

**Test:**

```curl
curl http://localhost:3000/old
curl http://localhost:3000/old/hello
```
