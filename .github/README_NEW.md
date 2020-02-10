![](https://i.imgur.com/Nwvx4cu.png)<a href="https://github.com/gofiber/fiber/blob/master/.github/README_RU.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ru.svg" alt="ru"/></a> <a href="https://github.com/gofiber/fiber/blob/master/.github/README_CH.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ch.svg" alt="ch"/></a>

[![](https://img.shields.io/github/release/gofiber/fiber?style=flat-square)](https://github.com/gofiber/fiber/releases) [![](https://img.shields.io/badge/api-documentation-blue?style=flat-square)](https://fiber.wiki) ![](https://img.shields.io/badge/goreport-A%2B-brightgreen?style=flat-square) [![](https://img.shields.io/badge/coverage-91%25-brightgreen?style=flat-square)](https://gocover.io/github.com/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=linux&style=flat-square)](https://travis-ci.org/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=windows&style=flat-square)](https://travis-ci.org/gofiber/fiber)

**Fiber** is an [Expressjs](https://github.com/expressjs/express) inspired **web framework** build on top of [Fasthttp](https://github.com/valyala/fasthttp), the **fastest** HTTP engine for [Go](https://golang.org/doc/).  
Designed to **ease** things up for **fast** development with **zero memory allocation** and **performance** in mind.

## ⚡️ Quick start

```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()

  app.Get("/", func(c *fiber.Ctx) {
    c.Send("Hello, World!")
  })

  app.Listen(3000)
}
```

## ⚙️ Installation

First of all, [download](https://golang.org/dl/) and install Go. `1.11` or higher is required.

Installation is done using the [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) command:

```bash
go get github.com/gofiber/fiber
```

## 🤖 Benchmarks

These tests are performed by [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) and [Go Web](https://github.com/smallnest/go-web-framework-benchmark). If you want to see all results, please visit our [Wiki](https://fiber.wiki/benchmarks).

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/static/benchmarks/benchmark-pipeline.png" width="49%" />
  <img src="https://github.com/gofiber/docs/blob/master/static/benchmarks/benchmark_alloc.png" width="49%" />
</p>

## 🎯 Features

- Robust [routing](https://fiber.wiki/routing)
- Serve [static files](https://fiber.wiki/application#static)
- [Extreme performance](https://fiber.wiki/benchmarks)
- Low memory footprint
- Express [API endpoints](https://fiber.wiki/context)
- Middleware & [Next](https://fiber.wiki/context#next) support
- Rapid server-side programming
- And much more, [explore Fiber](https://fiber.wiki/)

## 💡 Philosophy

People switching from [Node.js](https://nodejs.org/en/about/) to [Go](https://golang.org/doc/) having a heard time on how to start building their web applications or microservices. Fiber, as a **web framework**, was created with the idea of **minimalism** and follow **UNIX way**, so that new gophers can quickly enter the world of Go, but with a warm welcome.

Fiber is **inspired** by the Express framework, the most popular web framework on Internet. We combined the **ease** of Express and **raw performance** of Go. If you have ever implemented a web application on Node.js (_using Express.js or similar_), then many methods and principles will seem **very common** to you.

## 👀 Examples

Listed below are some of the common examples. If you want to see more code examples, please visit our [Recipes repository](https://github.com/gofiber/recipes) or visit our [API documentation](https://fiber.wiki).

### Static files

```go
func main() {
  app := fiber.New()

  app.Static("./public")
  // => http://localhost:3000/js/script.js
  // => http://localhost:3000/css/style.css

  app.Static("/prefix", "./public")
  // => http://localhost:3000/prefix/js/script.js
  // => http://localhost:3000/prefix/css/style.css

  app.Listen(3000)
}
```

### Routing

```go
func main() {
  app := fiber.New()

  // GET /john
  app.Get("/:name", func(c *fiber.Ctx) {
    fmt.Printf("Hello %s!", c.Params("name"))
    // => Hello john!
  })

  // GET /john
  app.Get("/:name/:age?", func(c *fiber.Ctx) {
    fmt.Printf("Name: %s, Age: %s", c.Params("name"), c.Params("age"))
    // => Name: john, Age:
  })

  // GET /api/register
  app.Get("/api*", func(c *fiber.Ctx) {
    fmt.Printf("/api%s", c.Params("*"))
    // => /api/register
  })

  app.Listen(3000)
}
```

### Middleware

```go
func main() {
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
}
```

### 404 Handling

```go
func main() {
  app := fiber.New()

  // Serve static files from "public" directory
  app.Static("./public")

  // Last middleware
  app.Use(func (c *fiber.Ctx) {
    c.SendStatus(404) // => 404 "Not Found"
  })

  app.Listen(3000)
}
```

### JSON Response

```go
func main() {
  app := fiber.New()

  type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
  }

  // Serialize JSON
  app.Get("/json", func (c *fiber.Ctx) {
    c.JSON(&User{"John", 20})
  })

  app.Listen(3000)
}
```

## 💬 Media

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) _by [Vic Shóstak](https://github.com/koddr), 03 Feb 2020_

## 👍 Contribute

If you want to say **thank you** and/or support the active development of `fiber`:

1. Add a GitHub Star to project.
2. Tweet about project [on your Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Write a review or tutorial on [Medium](https://medium.com/), [Dev.to](https://dev.to/) or personal blog.
4. Help us to translate this `README` and [API Docs](https://fiber.wiki/) to another language.

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/default-orange.png" alt="Buy Me A Coffee" style="height: 51px !important;width: 217px !important;" ></a>

Thanks for your support! Together, we make `Fiber`.

### ⭐️ Stars

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ License

`Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/master/LICENSE).
