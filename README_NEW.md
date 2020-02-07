[![Fiber Logo](https://i.imgur.com/zzmW4eK.png)](https://fiber.wiki)

[Express](https://github.com/expressjs/express) inspired web framework build on [Fasthttp](https://github.com/valyala/fasthttp) for [Go](https://golang.org/doc/), designed to ease things up for fast development with zero memory allocation and performance in mind.

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases)
[![](https://img.shields.io/badge/godoc-reference-blue.svg?longCache=true&style=flat)](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc)
![](https://img.shields.io/badge/coverage-84%25-brightgreen.svg?longCache=true&style=flat)
![](https://img.shields.io/badge/go-100%25-brightgreen.svg?longCache=true&style=flat)
![](https://img.shields.io/badge/goreport-A+-brightgreen.svg?longCache=true&style=flat)
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
## Benchmarks
<p float="left" align="middle">
  <img src="https://fiber.wiki/static/benchmarks/concurrency-pipeline.png" width="49%" />
  <img src="https://fiber.wiki/static/benchmarks/benchmark_alloc.png" width="49%" /> 
</p>

<p float="left" align="middle">
  <img src="https://fiber.wiki/static/benchmarks/benchmark.png" width="49%" />
  <img src="https://fiber.wiki/static/benchmarks/benchmark-pipeline.png" width="49%" /> 
</p>
