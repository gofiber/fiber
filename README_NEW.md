<img height="160px" src="https://github.com/gofiber/docs/blob/master/static/logo_320px_trans.png" alt="Fiber logo" />

# 🚀 Fiber <a href="README_RU.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ru.svg" alt="ru"/></a> <a href="README_CH.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ch.svg" alt="ch"/></a>

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases) ![](https://img.shields.io/github/languages/top/gofiber/fiber) [![](https://godoc.org/github.com/gofiber/fiber?status.svg)](https://godoc.org/github.com/gofiber/fiber) ![](https://goreportcard.com/badge/github.com/gofiber/fiber) [![GitHub license](https://img.shields.io/github/license/gofiber/fiber.svg)](https://github.com/gofiber/fiber/blob/master/LICENSE) [![Join the chat at https://gitter.im/gofiber/community](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/gofiber/community)

**Fiber** — is an [Express.js](https://github.com/expressjs/express) **inspired** web framework build on [Fasthttp](https://github.com/valyala/fasthttp) for [Go](https://golang.org/doc/). Designed to **ease** things up for **fast** development with **zero memory allocation** and **performance** in mind.

## ⚡️ Quick start

```go
package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Create new route with GET method
  app.Get("/", func(c *fiber.Ctx) {
    c.Write("Hello, World!")
  })

  // Start server on http://localhost:3000
  app.Listen(3000)
}
```

## ⚙️ Installation

Before installing, [download and install](https://golang.org/dl/) Go.

> Go `1.11` (with enabled Go Modules) or higher is required.

Installation is done using the
[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) command:

```bash
go get github.com/gofiber/fiber
```

## 🤖 Benchmarks

These tests are performed by [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) and [Go Web](https://github.com/smallnest/go-web-framework-benchmark). If you want to see all results, please visit our [Wiki](https://fiber.wiki/#benchmarks).

<p float="left" align="middle">
  <img src="https://fiber.wiki/static/benchmarks/benchmark-pipeline.png" width="49%" />
  <img src="https://fiber.wiki/static/benchmarks/benchmark_alloc.png" width="49%" />
</p>

## 🎯 Main features

- Robust [routing](https://fiber.wiki/#/routing)
- Serve [static files](https://fiber.wiki/#/application?id=static)
- [Extreme performance](https://fiber.wiki/#/benchmarks)
- Low memory footprint
- Express [API endpoints](https://fiber.wiki/#/context)
- Middleware & [Next](https://fiber.wiki/#context?id=next) support
- Rapid server-side programming
- And much more, [visit our Wiki](https://fiber.wiki/)

## 💡 Philosophy

People switching from [Node.js](https://nodejs.org/en/about/) to [Go](https://golang.org/doc/) often don't understand how to start building their web applications or microservices. Fiber, as a **web framework**, was created with the idea of **minimalism** and follow **UNIX way**, so that newbie gophers can quickly enter the new world of Go, but be in it like home.

Fiber is **inspired** by the Express framework, the most popular web framework on Internet. We combined the **ease** of Express and **raw performance** of Go. If you have ever implemented a web application on Node.js (_using Express.js or similar_), then many methods and principles will seem **very common** to you.

## 👀 Examples

Listed below are some of the common examples. If you want to see more code examples, please visit Fiber community [Cookbook repository](https://github.com/gofiber/recipes) or [API documentation](https://fiber.wiki).

### Static files

```go
// ...
app := fiber.New()

// Without prefix
app.Static("./public")

// => http://localhost:3000/js/script.js
// => http://localhost:3000/css/style.css

// With prefix
app.Static("/your-prefix", "./public")

// => http://localhost:3000/your-prefix/js/script.js
// => http://localhost:3000/your-prefix/css/style.css

app.Listen(3000)
```

### Routing

```go
// ...
app := fiber.New()

// URL with param
app.Get("/:name", func(c *fiber.Ctx) {
  c.Send("Hello, " + c.Params("name"))
})

// URL optional param
app.Get("/:name/:lastname?", func(c *fiber.Ctx) {
  c.Send("Hello, " + c.Params("name") + " " + c.Params("lastname"))
})

// URL with wildcard
app.Get("/api*", func(c *fiber.Ctx) {
  c.Send("/api" + c.Params("*"))
})

app.Listen(3000)
```

### Middleware

```go
// ...
app := fiber.New()

// Match any post route
app.Post(func(c *fiber.Ctx) {
  user, pass, ok := c.BasicAuth()
  if !ok || user != "john" || pass != "doe" {
    c.Status(403).Send("Sorry John")
    return
  }
  c.Next()
})

// Match all routes starting with /api
app.Use("/api", func(c *fiber.Ctx) {
  c.Set("Access-Control-Allow-Origin", "*")
  c.Set("Access-Control-Allow-Headers", "X-Requested-With")
  c.Next()
})

// Optional param
app.Post("/api/register", func(c *fiber.Ctx) {
  username := c.Body("username")
  password := c.Body("password")
  // ..
})

app.Listen(3000)
```

### 404 Handling

```go
// ...
app := fiber.New()

// ... app routes here

// The last route
app.Use(func (c *fiber.Ctx) {
  c.SendStatus(404)
})

app.Listen(3000)
```

### JSON Response

```go
// ...
app := fiber.New()

// Data structure
type Data struct {
  Name string `json:"name"`
  Age  int    `json:"age"`
}

// Return JSON data
app.Get("/json", func (c *fiber.Ctx) {
  c.JSON(&Data{
    Name: "John",
    Age:  20,
  })
})

app.Listen(3000)
```

## 💬 What people say about Fiber?

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) _by [Vic Shóstak](https://github.com/koddr), 03 Feb 2020_

## 👍 Project assistance

If you want to say **thank you** or/and support active development `gofiber/fiber`:

1. Add a GitHub Star to project.
2. Tweet about project [on your Twitter](https://twitter.com/intent/tweet?text=%F0%9F%94%8C%20Fiber%20is%20an%20Express.js%20inspired%20Go%20web%20framework%20build%20on%20%F0%9F%9A%80%20Fasthttp%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Help us to translate this `README` and [API Docs](https://fiber.wiki/) to another language.

Thanks for your support! 😘 Together, we make `Fiber`.

### ⭐️ Stars over time

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ License

`Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/master/LICENSE).
