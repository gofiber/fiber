# Key Authentication

![Release](https://img.shields.io/github/release/gofiber/keyauth.svg)
[![Discord](https://img.shields.io/badge/discord-join%20channel-7289DA)](https://gofiber.io/discord)
![Test](https://github.com/gofiber/keyauth/workflows/Test/badge.svg)
![Security](https://github.com/gofiber/keyauth/workflows/Security/badge.svg)
![Linter](https://github.com/gofiber/keyauth/workflows/Linter/badge.svg)

Special thanks to [József Sallai](https://github.com/jozsefsallai) & [Ray Mayemir](https://github.com/raymayemir)

### Install
```
go get -u github.com/gofiber/fiber/v2
go get -u github.com/gofiber/keyauth/v2
```
### Example
```go
package main

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/keyauth/v2"
)

func main() {
  app := fiber.New()
  
  app.Use(keyauth.New(keyauth.Config{
    KeyLookup: "cookie:access_token",
    ContextKey: "my_token",
  }))
  
  app.Get("/", func(c *fiber.Ctx) error {
    token, _ := c.Locals("my_token").(string)
    return c.SendString(token)
  })
  
  app.Listen(":3000")
}
```
### Test
```curl
curl -v --cookie "access_token=hello_world" http://localhost:3000
```
