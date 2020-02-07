[![Fiber Logo](https://i.imgur.com/zzmW4eK.png)](https://fiber.wiki)

[Express](https://github.com/expressjs/express) inspired web framework build on [Fasthttp](https://github.com/valyala/fasthttp) for [Go](https://golang.org/doc/).  
Designed to ease things up for fast development with zero memory allocation and performance in mind.

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases)
[![](https://img.shields.io/badge/godoc-reference-blue.svg?longCache=true&style=flat)](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc)
![](https://img.shields.io/badge/coverage-84%25-brightgreen.svg?longCache=true&style=flat)
![](https://img.shields.io/badge/go-100%25-brightgreen.svg?longCache=true&style=flat)
![](https://img.shields.io/badge/goreport-A+-brightgreen.svg?longCache=true&style=flat)
[![](https://img.shields.io/badge/gitter-chat-brightgreen.svg?longCache=true&style=flat)](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc)

```golang
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

## Benchmarks

These tests are performed by [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) and [Go Web](https://github.com/smallnest/go-web-framework-benchmark). If you want to see all results, please visit our [wiki#benchmarks](https://fiber.wiki/#benchmarks).
<p float="left" align="middle">
  <img src="https://fiber.wiki/static/benchmarks/benchmark-pipeline.png" width="49%" />
  <img src="https://fiber.wiki/static/benchmarks/benchmark_alloc.png" width="49%" /> 
</p>

## Installation

Before installing, [download and install Go](https://golang.org/dl/).
Go `1.11` or higher is required.

Installation is done using the
[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) command:

```bash
go get github.com/gofiber/fiber
```

## Features

* Robust [routing](https://fiber.wiki/#/routing)
* Serve [static files](https://fiber.wiki/#/application?id=static)
* [Extreme performance](https://fiber.wiki/#/benchmarks)
* Low memory footprint
* Express [API endpoints](https://fiber.wiki/#/context)
* Middleware & [Next](https://fiber.wiki/#context?id=next) support
* Rapid server-side programming
* [And much more, click here](https://fiber.wiki/)


## Philosophy

People switching from [Node.js](https://nodejs.org/en/about/) to [Go](https://golang.org/doc/) often end up in a bad learning curve to start building their webapps or micro services. Fiber, as a web framework, was created with the idea of minimalism so new and experienced gophers can rapidly develop web application's. 

Fiber is inspired by the Express framework, the most populair web framework on web. We combined the ease of Express and raw performance of Go. If you have ever implemented a web application on Node.js using Express.js, then many methods and principles will seem very common to you.

## Examples

Listed below are some of the common examples. If you want to see more code examples, please visit our [recipes repository](https://github.com/gofiber/recipes) or [api documentation](https://fiber.wiki).

_**Static files**_
```golang
// ...
app := fiber.New()

app.Static("./public")
// http://localhost:3000/js/script.js
// http://localhost:3000/css/style.css

app.Static("/xxx", "./public")
// http://localhost:3000/xxx/js/script.js
// http://localhost:3000/xxx/css/style.css

app.Listen(3000)
```

_**Routing**_
```golang
// ...
app := fiber.New()

// param
app.Get("/:name", func(c *fiber.Ctx) {
  c.Send("Hello, " + c.Params("name"))
})

// optional param
app.Get("/:name/:lastname?", func(c *fiber.Ctx) {
  c.Send("Hello, " + c.Params("name") + " " + c.Params("lastname"))
})

// wildcard
app.Get("/api*", func(c *fiber.Ctx) {
  c.Send("/api" + c.Params("*"))
})

app.Listen(3000)
```

_**Middleware**_
```golang
// ...
app := fiber.New()

// match any post route
app.Post(func(c *fiber.Ctx) {
  user, pass, ok := c.BasicAuth()
  if !ok || user != "john" || pass != "doe" {
    c.Status(403).Send("Sorry John")
    return
  }
  c.Next()
})

// match all routes starting with /api
app.Use("/api", func(c *fiber.Ctx) {
  c.Set("Access-Control-Allow-Origin", "*")
  c.Set("Access-Control-Allow-Headers", "X-Requested-With")
  c.Next()
})

// optional param
app.Post("/api/register", func(c *fiber.Ctx) {
  username := c.Body("username")
  password := c.Body("password")
  // ..
})

app.Listen(3000)
```

_**404 Handling**_
```golang
// ...
app := fiber.New()

// ..application routes

// last route
app.Use(func (c *fiber.Ctx) {
  c.SendStatus(404)
})

app.Listen(3000)
```

_**JSON Response**_
```golang
// ...
app := fiber.New()

type Data struct {
  Name string `json:"name"`
  Age  int    `json:"age"`
}

// last route
app.Get("/json", func (c *fiber.Ctx) {
  c.JSON(&Data{
    Name: "John",
    Age:  20,
  })
})

app.Listen(3000)
```


## License

`gofiber/fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/master/LICENSE).
