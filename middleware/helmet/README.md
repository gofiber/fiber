# Helmet

![Release](https://img.shields.io/github/release/gofiber/helmet.svg)
[![Discord](https://img.shields.io/badge/discord-join%20channel-7289DA)](https://gofiber.io/discord)
![Test](https://github.com/gofiber/helmet/workflows/Test/badge.svg)
![Security](https://github.com/gofiber/helmet/workflows/Security/badge.svg)
![Linter](https://github.com/gofiber/helmet/workflows/Linter/badge.svg)

### Install
```
go get -u github.com/gofiber/fiber/v2
go get -u github.com/gofiber/helmet/v2
```
### Example
```go
package main

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/helmet/v2"
)

func main() {
  app := fiber.New()

  app.Use(helmet.New())

  app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("Welcome!")
  })

  app.Listen(":3000")
}
```
### Test
```curl
curl -I http://localhost:3000
```
