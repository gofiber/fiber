# Fiber

[Expressjs](https://github.com/expressjs/express) inspired `web framework` for [Go](https://golang.org/doc/), designed to `ease` things up for `fast development` with `zero memory allocation` and raw `performance` in mind.

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases)
![](https://img.shields.io/badge/coverage-84.6%25-brightgreen.svg?longCache=true&style=flat)
![](https://img.shields.io/badge/go-100.0%25-brightgreen.svg?longCache=true&style=flat)
![](https://img.shields.io/badge/go%20report-A+-brightgreen.svg?longCache=true&style=flat)
[![](https://img.shields.io/badge/godoc-reference-brightgreen.svg?longCache=true&style=flat)](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc)
[![](https://img.shields.io/badge/gitter-chat-brightgreen.svg?longCache=true&style=flat)](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc)

```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()

  app.Get("/", func(c *fiber.Ctx) {
    c.Write("Hello, World!")
  })

  app.Listen(3000)
}
```
