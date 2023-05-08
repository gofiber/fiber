---
id: redirect
title: Redirect
---

Redirection middleware for Fiber.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

```go
package main

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/redirect"
)

func main() {
  app := fiber.New()
  
  app.Use(redirect.New(redirect.Config{
    Rules: map[string]string{
      "/old":   "/new",
      "/old/*": "/new/$1",
    },
    StatusCode: 301,
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

## Config

```go
// Config defines the config for middleware.
type Config struct {
	// Filter defines a function to skip middleware.
	// Optional. Default: nil
	Next func(*fiber.Ctx) bool

	// Rules defines the URL path rewrite rules. The values captured in asterisk can be
	// retrieved by index e.g. $1, $2 and so on.
	// Required. Example:
	// "/old":              "/new",
	// "/api/*":            "/$1",
	// "/js/*":             "/public/javascripts/$1",
	// "/users/*/orders/*": "/user/$1/order/$2",
	Rules map[string]string

	// The status code when redirecting
	// This is ignored if Redirect is disabled
	// Optional. Default: 302 (fiber.StatusFound)
	StatusCode int

	rulesRegex map[*regexp.Regexp]string
}
```

## Default Config

```go
var ConfigDefault = Config{
	StatusCode: fiber.StatusFound,
}
```
