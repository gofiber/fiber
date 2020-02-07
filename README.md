# 🚀 Fiber <a href="README_RU.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ru.svg" alt="ru"/></a> <a href="README_CH.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ch.svg" alt="ch"/></a>

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases) ![](https://img.shields.io/github/languages/top/gofiber/fiber) [![](https://godoc.org/github.com/gofiber/fiber?status.svg)](https://godoc.org/github.com/gofiber/fiber) ![](https://goreportcard.com/badge/github.com/gofiber/fiber) [![GitHub license](https://img.shields.io/github/license/gofiber/fiber.svg)](https://github.com/gofiber/fiber/blob/master/LICENSE) [![Join the chat at https://gitter.im/gofiber/community](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/gofiber/community)

<img align="right" height="180px" src="https://github.com/gofiber/docs/blob/master/static/logo_320px_trans.png" alt="Fiber logo" />

**[Fiber](https://github.com/gofiber/fiber)** is an [Express.js](https://expressjs.com/en/4x/api.html) styled HTTP web framework implementation running on [Fasthttp](https://github.com/valyala/fasthttp), the **fastest** HTTP engine for Go (Golang). The package make use of **similar framework convention** as they are in Express.

People switching from [Node.js](https://nodejs.org/en/about/) to [Go](https://golang.org/doc/) often end up in a bad learning curve to start building their webapps, this project is meant to **ease** things up for **fast** development, but with **zero memory allocation** and **performance** in mind.

## API Documentation

📚 We created an extended **API documentation** (_including examples_), **[Visit fiber.wiki](https://fiber.wiki/)**.

## Benchmark

[![](https://fiber.wiki/static/benchmarks/benchmark.png)](https://fiber.wiki/#/benchmarks)

👉 **[Click here](https://fiber.wiki/#/benchmarks)** to see all benchmark results.

## Features

- Optimized for speed and low memory usage
- Rapid Server-Side Programming
- Easy routing with parameters
- Static files with custom prefix
- Middleware with Next support
- Express API endpoints
- [Extended documentation](https://fiber.wiki/)

## Installing

Assuming you’ve already installed Go `1.11+` 😉

Install the [Fiber](https://github.com/gofiber/fiber) package by calling the following command:

```bash
go get -u github.com/gofiber/fiber
```

## Hello, world!

Embedded below is essentially the simplest Fiber app you can create:

```go
// server.go

package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Create new route with GET method
  app.Get("/", func(c *fiber.Ctx) {
    c.Send("Hello, World!")
  })

  // Start server on http://localhost:8080
  app.Listen(8080)
}
```

Go to console and run:

```bash
go run server.go
```

And now, browse to `http://localhost:8080` and you should see `Hello, World!` on the page! 🎉

## Static files

To serve static files, use the [Static](https://fiber.wiki/#/?id=static-files) method:

```go
package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Serve all static files on ./public folder
  app.Static("./public")

  // Start server on http://localhost:8080
  app.Listen(8080)
}
```

Now, you can load the files that are in the public directory:

```bash
http://localhost:8080/hello.html
http://localhost:8080/js/script.js
http://localhost:8080/css/style.css
```

## Middleware

Middleware has never been so easy! Just like Express you call the `Next()` matching route function:

```go
package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Define all used middleware in Use()

  app.Use(func(c *fiber.Ctx) {
    c.Write("Match anything!\n")
    c.Next()
  })

  app.Use("/api", func(c *fiber.Ctx) {
    c.Write("Match starting with /api\n")
    c.Next()
  })

  app.Get("/api/user", func(c *fiber.Ctx) {
    c.Write("Match exact path /api/user\n")
  })

  // Start server on http://localhost:8080
  app.Listen(8080)
}
```

## 👍 Project assistance

If you want to say **thank you** or/and support active development `gofiber/fiber`:

1. Add a GitHub Star to project.
2. Tweet about project [on your Twitter](https://twitter.com/intent/tweet?text=%F0%9F%94%8C%20Fiber%20is%20an%20Express.js%20inspired%20Go%20web%20framework%20build%20on%20%F0%9F%9A%80%20Fasthttp%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Help us to translate this `README` and [API Docs](https://fiber.wiki/) to another language.

Thanks for your support! 😘 Together, we make `Fiber`.

## ⭐️ Stars over time

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## License

⚠️ _Please note:_ `gofiber/fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/master/LICENSE).
