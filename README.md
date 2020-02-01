# Fiber Web Framework <img src="images/flags/en.svg" alt="en"/> <a href="README_RU.md"><img src="images/flags/ru.svg" alt="ru"/></a>

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases) ![](https://img.shields.io/github/languages/top/gofiber/fiber) [![](https://godoc.org/github.com/gofiber/fiber?status.svg)](https://godoc.org/github.com/gofiber/fiber) ![](https://goreportcard.com/badge/github.com/gofiber/fiber)

<img align="right" height="180px" src="https://gofiber.github.io/fiber/static/logo.jpg" />

**[Fiber](https://github.com/gofiber/fiber)** is an [Express](https://expressjs.com/en/4x/api.html)-styled HTTP web framework implementation running on [Fasthttp](https://github.com/valyala/fasthttp), the **fastest** HTTP engine for Go (Golang). The package make use of similar framework convention as they are in Express. 

People switching from [Node.js](https://nodejs.org/en/about/) to [Go](https://golang.org/doc/) often end up in a bad learning curve to start building their webapps, this project is meant to **ease** things up for **fast** development, but with **zero memory allocation** and **performance** in mind. 

📚 See **[API Documentation](https://gofiber.github.io/fiber/)**.

[![](https://gofiber.github.io/fiber/static/benchmarks/benchmark.png)](https://gofiber.github.io/fiber/#/benchmarks)

👉 **[Click here](https://gofiber.github.io/fiber/#/benchmarks)** to see all benchmark results.

## Features

* Optimized for speed and low memory usage
* Rapid Server-Side Programming
* Easy routing with parameters
* Static files with custom prefix
* Middleware with Next support
* Express API endpoints
* [Comprehensible documentation](https://gofiber.github.io/fiber/)

## Installing

Assuming you’ve already installed Go `1.11+` 😉

Install the [Fiber](https://github.com/gofiber/fiber) package by calling the following command:

```console
$ go get -u github.com/gofiber/fiber
```

## Hello, world!

Embedded below is essentially the simplest Fiber app you can create.

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

```console
$ go run server.go
```

And now, browse to **http://localhost:8080** and you should see `Hello, World!` on the page! 🎉

## Static files

To serve static files, use the [Static](https://gofiber.github.io/fiber/#/?id=static-files) method.

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

```console
http://localhost:8080/hello.html
http://localhost:8080/js/script.js
http://localhost:8080/css/style.css
```

## Middleware

Middleware has never been so easy, just like Express you call the `Next()` matching route function!

```go
package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Define all used middlewares in Use()
  
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

## API Documentation

We created an extended API documentation including examples, **[click here](https://gofiber.github.io/fiber/)**.


## Stargazers over time

[![Stargazers over time](https://starchart.cc/gofiber/fiber.svg)](https://starchart.cc/gofiber/fiber)

## License

☝️ _Please note:_ `gofiber/fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/edit/master/LICENSE).

*Caught a mistake? [Edit this page on GitHub!](https://github.com/gofiber/fiber/blob/master/README.md)*
