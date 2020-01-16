<p align="center">
  <img height="150" src="https://gofiber.github.io/fiber/static/logo.jpg">
</p>
<!--
![](https://img.shields.io/github/issues/gofiber/fiber) 
![](https://img.shields.io/github/stars/gofiber/fiber) 
-->

# Fiber ![](https://img.shields.io/github/release/gofiber/fiber) ![](https://img.shields.io/github/languages/top/gofiber/fiber) ![](https://img.shields.io/github/languages/code-size/gofiber/fiber) ![](https://godoc.org/github.com/gofiber/fiber?status.svg) ![](https://goreportcard.com/badge/github.com/gofiber/fiber)

**[Fiber](https://github.com/gofiber/fiber)** is an **[Express](https://expressjs.com/en/4x/api.html)** style HTTP framework implementation running on **[Fasthttp](https://github.com/valyala/fasthttp)**, the **fastest** HTTP engine for **[Go](https://golang.org/doc/)**. The package make use of similar framework convention as they are in express. People switching from **[Node](https://nodejs.org/en/about/)** to **[Go](https://golang.org/doc/)** often end up in a bad learning curve to start building their webapps, this project is meant to **ease** things up for **fast** development, but with **zero memory allocation** and **performance** in mind. See **[API Documentation](https://gofiber.github.io/fiber/)**

![](https://gofiber.github.io/fiber/static/benchmarks/benchmark-pipeline.png?v=12) 
## Features
* Optimized for speed and low memory usage.
* Rapid Server-Side Programming
* Easy routing with parameters
* Static files with custom prefix
* Middleware with Next support
* Express API endpoints
* **[API Documentation](https://gofiber.github.io/fiber/)**

## Installing
Assuming you’ve already installed **[Go](https://golang.org/doc/)**, install the **[Fiber](https://github.com/gofiber/fiber)** package by calling the following command:
```bash
$ go get -u github.com/gofiber/fiber
```

## Hello world
Embedded below is essentially the simplest Fiber app you can create.
```bash
$ create server.go
```
```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()
  app.Get("/:name", func(c *fiber.Ctx) {
    c.Send("Hello, " + c.Params("name") + "!")
  })
  app.Listen(8080)
}
```
```bash
$ go run server.go
```
Browse to **http://localhost:8080** and you should see `Hello, World!` on the page.

## Static files
To serve static files, use the [Static](https://gofiber.github.io/fiber/#/?id=static-files) method.
```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()
  app.Static("./public")
  app.Listen(8080)
}
```
Now, you can load the files that are in the public directory:
```shell
http://localhost:8080/images/gopher.png
http://localhost:8080/css/style.css
http://localhost:8080/js/jquery.js
http://localhost:8080/hello.html
```

## Middleware
Middleware never has been so easy!
```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()
  app.Get("/api*", func(c *fiber.Ctx) {
    c.Locals("auth", "admin")
    c.Next()
  })
  app.Get("/api/back-end/:action?", func(c *fiber.Ctx) {
    if c.Locals("auth") != "admin" {
      c.Status(403).Send("Forbidden")
      return
    }
    c.Send("Hello, Admin!")
  })
  app.Listen(8080)
}
```

## API Documentation
We created an extended API documentation including examples, **[click here](https://gofiber.github.io/fiber/)**

## License
gofiber/fiber is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/edit/master/LICENSE).



*Caught a mistake? [Edit this page on GitHub!](https://github.com/gofiber/fiber/blob/master/README.md)*
