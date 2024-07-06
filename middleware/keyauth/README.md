# Key Authentication

## Install

```bash
go get -u github.com/gofiber/fiber/v3
go get -u github.com/gofiber/keyauth/v2
```

## Example

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/middleware/keyauth"
)

func main() {
    app := fiber.New()
    
    app.Use(keyauth.New(keyauth.Config{
      KeyLookup: "cookie:access_token",
      ContextKey: "my_token",
    }))
    
    app.Get("/", func(c fiber.Ctx) error {
      token := c.TokenFromContext(c) // "" is returned if not found
      return c.SendString(token)
    })
    
    app.Listen(":3000")
}
```

## Test

```bash
curl -v --cookie "access_token=hello_world" http://localhost:3000
```
