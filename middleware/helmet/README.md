# Helmet

## Install

```bash
go get -u github.com/gofiber/fiber/v3
go get -u github.com/gofiber/middleware/helmet
```

## Example

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/helmet"
)

func main() {
    app := fiber.New()

    app.Use(helmet.New())

    app.Get("/", func(c fiber.Ctx) error {
      return c.SendString("Welcome!")
    })

    app.Listen(":3000")
}
```

## Test

```bash
curl -I http://localhost:3000
```
