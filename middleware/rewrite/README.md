# Rewrite

![Release](https://img.shields.io/github/release/gofiber/rewrite.svg)
[![Discord](https://img.shields.io/badge/discord-join%20channel-7289DA)](https://gofiber.io/discord)
![Test](https://github.com/gofiber/rewrite/workflows/Test/badge.svg)
![Security](https://github.com/gofiber/rewrite/workflows/Security/badge.svg)
![Linter](https://github.com/gofiber/rewrite/workflows/Linter/badge.svg)

### Install
```
go get -u github.com/gofiber/fiber/v2
go get -u github.com/gofiber/rewrite/v2
```
### Example
```go
package main

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/rewrite/v2"
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
### Test
```curl
curl http://localhost:3000/old
curl http://localhost:3000/old/hello
```
