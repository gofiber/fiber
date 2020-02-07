# Fiber

[Expressjs](https://github.com/expressjs/express) inspired _web framework_ for [Go](https://golang.org/doc/), designed to _ease_ things up for _fast development_, but with _zero memory allocation_ and _performance_ in mind.

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases) ![](https://img.shields.io/badge/coverage-84.6%25-brightgreen.svg?longCache=true&style=flat) ![](https://img.shields.io/github/languages/top/gofiber/fiber) ![](https://goreportcard.com/badge/github.com/gofiber/fiber) [![](https://godoc.org/github.com/gofiber/fiber?status.svg)](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc) [![Join the chat at https://gitter.im/gofiber/community](https://img.shields.io/badge/gitter-chat-blue.svg?longCache=true&style=flat)](https://gitter.im/gofiber/community)

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
