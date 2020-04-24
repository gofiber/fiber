<p align="center">
  <a href="https://gofiber.io">
    <img alt="Fiber" height="125" src="https://github.com/gofiber/docs/blob/master/static/fiber_v2_logo.svg">
  </a>
  <br>
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/en.svg">
  </a>-->
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ru.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ru.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_es.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/es.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ja.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/jp.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_pt.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/pt.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_zh-CN.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ch.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_de.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/de.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_nl.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/nl.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ko.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/fr.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_tr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/tr.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_id.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/id.svg">
  </a>
  <br><br>
  <a href="https://pkg.go.dev/github.com/gofiber/fiber?tab=doc">
    <img src="https://img.shields.io/badge/go.dev-007d9c?logo=go&logoColor=white&style=flat-square">
  </a>
  <a href="https://github.com/gofiber/fiber/releases">
    <img src="https://img.shields.io/github/release/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://docs.gofiber.io">
    <img src="https://img.shields.io/badge/api-docs-blue?style=flat-square">
  </a>
  <a href="#">
    <img src="https://img.shields.io/badge/goreport-A%2B-brightgreen?style=flat-square">
  </a>
  <a href="https://gocover.io/github.com/gofiber/fiber">
    <img src="https://img.shields.io/badge/coverage-91%25-brightgreen?style=flat-square">
  </a>
  <a href="https://travis-ci.org/gofiber/fiber">
    <img src="https://img.shields.io/travis/gofiber/fiber/master.svg?label=linux&style=flat-square">
  </a>
  <a href="https://travis-ci.org/gofiber/fiber">
    <img src="https://img.shields.io/travis/gofiber/fiber/master.svg?label=windows&style=flat-square">
  </a>
  <a href="https://travis-ci.org/gofiber/fiber">
    <img src="https://img.shields.io/travis/gofiber/fiber/master.svg?label=osx&style=flat-square">
  </a>
  <a href="https://t.me/gofiber">
    <img src="https://img.shields.io/badge/telegram-join%20chat-0088cc?style=flat-square">
  </a>
</p>
<p align="center">
  <b>Fiber</b> is an <a href="https://github.com/expressjs/express">Express</a> inspired <b>web framework</b> built on top of <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, the <b>fastest</b> HTTP engine for <a href="https://golang.org/doc/">Go</a>. Designed to <b>ease</b> things up for <b>fast</b> development with <b>zero memory allocation</b> and <b>performance</b> in mind.
</p>

## ⚡️ Quickstart

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
go get -u github.com/gofiber/fiber
```

## 🤖 Benchmarks

These tests are performed by [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) and [Go Web](https://github.com/smallnest/go-web-framework-benchmark). If you want to see all results, please visit our [Wiki](https://docs.gofiber.io/benchmarks).

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Features

- Robust [routing](https://docs.gofiber.io/routing)
- Serve [static files](https://docs.gofiber.io/application#static)
- Extreme [performance](https://docs.gofiber.io/benchmarks)
- [Low memory](https://docs.gofiber.io/benchmarks) footprint
- [API endpoints](https://docs.gofiber.io/context)
- [Middleware](https://docs.gofiber.io/middleware) & [Next](https://docs.gofiber.io/context#next) support
- [Rapid](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) server-side programming
- [Template engines](https://docs.gofiber.io/middleware#template)
- [WebSocket support](https://docs.gofiber.io/middleware#websocket)
- [Rate Limiter](https://docs.gofiber.io/middleware#limiter)
- Translated in 10 other languages
- And much more, [explore Fiber](https://docs.gofiber.io/)

## 💡 Philosophy

New gophers that make the switch from [Node.js](https://nodejs.org/en/about/) to [Go](https://golang.org/doc/) are dealing with a learning curve before they can start building their web applications or microservices. Fiber, as a **web framework**, was created with the idea of **minimalism** and follows the **UNIX way**, so that new gophers can quickly enter the world of Go with a warm and trusted welcome.

Fiber is **inspired** by Express, the most popular web framework on the Internet. We combined the **ease** of Express and **raw performance** of Go. If you have ever implemented a web application in Node.js (_using Express or similar_), then many methods and principles will seem **very common** to you.

We **listen** to our users in [issues](https://github.com/gofiber/fiber/issues) (_and all over the Internet_) to create a **fast**, **flexible** and **friendly** Go web framework for **any** task, **deadline** and developer **skill**! Just like Express does in the JavaScript world.

## 👀 Examples

Listed below are some of the common examples. 

> If you want to see more code examples, please visit our [Recipes repository](https://github.com/gofiber/recipes) or visit our [API documentation](https://docs.gofiber.io).

### Routing

📖 https://docs.gofiber.io/#basic-routing  


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
  app.Get("/api/*", func(c *fiber.Ctx) {
    fmt.Printf("/api/%s", c.Params("*"))
    // => /api/register
  })

  app.Listen(3000)
}
```

### Serve static files

📖 https://docs.gofiber.io/application#static  

```go
func main() {
  app := fiber.New()

  app.Static("/", "/public")
  // => http://localhost:3000/js/script.js
  // => http://localhost:3000/css/style.css

  app.Static("/prefix", "/public")
  // => http://localhost:3000/prefix/js/script.js
  // => http://localhost:3000/prefix/css/style.css

  app.Static("*", "/public/index.html")
  // => http://localhost:3000/any/path/shows/index/html

  app.Listen(3000)
}
```

### Middleware & Next

📖 https://docs.gofiber.io/routing#middleware  
📖 https://docs.gofiber.io/context#next  

```go
func main() {
  app := fiber.New()

  // Match any route
  app.Use(func(c *fiber.Ctx) {
    fmt.Println("First middleware")
    c.Next()
  })

  // Match all routes starting with /api
  app.Use("/api", func(c *fiber.Ctx) {
    fmt.Println("Second middleware")
    c.Next()
  })

  // GET /api/register
  app.Get("/api/list", func(c *fiber.Ctx) {
    fmt.Println("Last middleware")
    c.Send("Hello, World!")
  })

  app.Listen(3000)
}
```

<details>
  <summary>📚 Show more code examples</summary>

### Template engines

📖 https://docs.gofiber.io/application#settings  
📖 https://docs.gofiber.io/context#render  
📖 https://docs.gofiber.io/middleware#template  

Fiber supports the default [Go template engine](https://golang.org/pkg/html/template/)

But if you want to use another template engine like [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) or [pug](https://github.com/Joker/jade).

You can use our [Template Middleware](https://docs.gofiber.io/middleware#template).

```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/template"
)

func main() {
  // You can setup template engine before initiation app:
  app := fiber.New(&fiber.Settings{
    TemplateEngine:    template.Mustache(),
    TemplateFolder:    "./views",
    TemplateExtension: ".tmpl",
  })

  // OR after initiation app at any convenient location:
  app.Settings.TemplateEngine = template.Mustache()
  app.Settings.TemplateFolder = "./views"
  app.Settings.TemplateExtension = ".tmpl"

  // And now, you can call template `./views/home.tmpl` like this:
  app.Get("/", func(c *fiber.Ctx) {
    c.Render("home", fiber.Map{
      "title": "Homepage",
      "year":  1999,
    })
  })

  // ...
}
```

### Grouping routes into chains

📖 https://docs.gofiber.io/application#group  

```go
func main() {
  app := fiber.New()

  // Root API route
  api := app.Group("/api", cors())  // /api

  // API v1 routes
  v1 := api.Group("/v1", mysql())   // /api/v1
  v1.Get("/list", handler)          // /api/v1/list
  v1.Get("/user", handler)          // /api/v1/user

  // API v2 routes
  v2 := api.Group("/v2", mongodb()) // /api/v2
  v2.Get("/list", handler)          // /api/v2/list
  v2.Get("/user", handler)          // /api/v2/user

  // ...
}
```

### Middleware logger

📖 https://docs.gofiber.io/middleware#logger  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/logger"
)

func main() {
    app := fiber.New()

    // Optional logger config
    config := logger.Config{
      Format:     "${time} - ${method} ${path}\n",
      TimeFormat: "Mon, 2 Jan 2006 15:04:05 MST",
    }

    // Logger with config
    app.Use(logger.New(config))

    app.Listen(3000)
}
```

### Cross-Origin Resource Sharing (CORS)

📖 https://docs.gofiber.io/middleware#cors  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/cors"
)

func main() {
    app := fiber.New()

    // CORS with default config
    app.Use(cors.New())

    app.Listen(3000)
}
```

Check CORS by passing any domain in `Origin` header:

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```

### Custom 404 response

📖 https://docs.gofiber.io/application#http-methods  

```go
func main() {
  app := fiber.New()

  app.Static("/public")

  app.Get("/demo", func(c *fiber.Ctx) {
    c.Send("This is a demo!")
  })

  app.Post("/register", func(c *fiber.Ctx) {
    c.Send("Welcome!")
  })

  // Last middleware to match anything
  app.Use(func(c *fiber.Ctx) {
    c.SendStatus(404) 
    // => 404 "Not Found"
  })

  app.Listen(3000)
}
```

### JSON Response

📖 https://docs.gofiber.io/context#json  

```go
type User struct {
  Name string `json:"name"`
  Age  int    `json:"age"`
}

func main() {
  app := fiber.New()

  app.Get("/user", func(c *fiber.Ctx) {
    c.JSON(&User{"John", 20})
    // => {"name":"John", "age":20}
  })

  app.Get("/json", func(c *fiber.Ctx) {
    c.JSON(fiber.Map{
      "success": true,
      "message": "Hi John!",
    })
    // => {"success":true, "message":"Hi John!"}
  })

  app.Listen(3000)
}
```

### WebSocket Upgrade

📖 https://docs.gofiber.io/middleware#websocket  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/websocket"
)

func main() {
  app := fiber.New()

  app.Get("/ws", websocket.New(func(c *websocket.Conn) {
    for {
      mt, msg, err := c.ReadMessage()
      if err != nil {
        log.Println("read:", err)
        break
      }
      log.Printf("recv: %s", msg)
      err = c.WriteMessage(mt, msg)
      if err != nil {
        log.Println("write:", err)
        break
      }
    }
  }))

  app.Listen(3000)
  // ws://localhost:3000/ws
}
```

### Recover middleware

📖 https://docs.gofiber.io/middleware#recover  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/recover"
)

func main() {
  app := fiber.New()

  // Optional recover config
  config := recover.Config{
    Handler: func(c *fiber.Ctx, err error) {
			c.SendString(err.Error())
			c.SendStatus(500)
		},
  }

  // Logger with custom config
  app.Use(recover.New(config))

  app.Listen(3000)
}
```
</details>

## 🧬 Available Middlewares

For _easier_ and _more clear_ work, we've put [middleware](https://docs.gofiber.io/middleware) into separate repositories:

- [Basic Authentication](https://github.com/gofiber/basicauth)
- [Key Authentication](https://github.com/gofiber/keyauth)
- [Compression](https://github.com/gofiber/compression)
- [Request ID](https://github.com/gofiber/requestid)
- [WebSocket](https://github.com/gofiber/websocket)
- [Rewrite](https://github.com/gofiber/rewrite)
- [Recover](https://github.com/gofiber/recover)
- [Limiter](https://github.com/gofiber/limiter)
- [Session](https://github.com/gofiber/session)
- [Logger](https://github.com/gofiber/logger)
- [Helmet](https://github.com/gofiber/helmet)
- [CORS](https://github.com/gofiber/cors)
- [CSRF](https://github.com/gofiber/csrf)
- [JWT](https://github.com/gofiber/jwt)

## 💬 Media

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) — _03 Feb 2020_
- [Fiber released v1.7! 🎉 What's new and is it still fast, flexible and friendly?](https://dev.to/koddr/fiber-v2-is-out-now-what-s-new-and-is-he-still-fast-flexible-and-friendly-3ipf) — _21 Feb 2020_
- [🚀 Fiber v1.8. What's new, updated and re-thinked?](https://dev.to/koddr/fiber-v1-8-what-s-new-updated-and-re-thinked-339h) — _03 Mar 2020_
- [Is switching from Express to Fiber worth it? 🤔](https://dev.to/koddr/are-sure-what-your-lovely-web-framework-running-so-fast-2jl1) — _01 Apr 2020_
- [Creating Fast APIs In Go Using Fiber](https://dev.to/jozsefsallai/creating-fast-apis-in-go-using-fiber-59m9) — _07 Apr 2020_

## 👍 Contribute

If you want to say **thank you** and/or support the active development of `Fiber`:

1. Add a [GitHub Star](https://github.com/gofiber/fiber/stargazers) to the project.
2. Tweet about the project [on your Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Write a review or tutorial on [Medium](https://medium.com/), [Dev.to](https://dev.to/) or personal blog.
4. Help us to translate this `README` to another language.


## ☕ Supporters

Fiber is an open source project that runs on donations to pay the bills e.g. our domain name, gitbook, netlify and serverless hosting.

<a href="https://www.buymeacoffee.com/fenny" target="_blank">
  <img src="https://raw.githubusercontent.com/gofiber/docs/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" height="100" >
</a>


| | User | Donation
| --- | --- | --- |
![](https://avatars.githubusercontent.com/u/59947262?s=25 ) | [@thomasvvugt](https://github.com/thomasvvugt) | ☕ x 5
![](https://avatars.githubusercontent.com/u/1094221?s=25 ) | [@ekaputra07](https://github.com/ekaputra07) | ☕ x 5
![](https://avatars.githubusercontent.com/u/635852?s=25 ) | [@bihe](https://github.com/bihe) | ☕ x 3
![](https://avatars.githubusercontent.com/u/59947262?s=25 ) | @justdave | ☕ x 3
![](https://avatars.githubusercontent.com/u/11155743?s=25 ) | [@koddr](https://github.com/koddr) | ☕ x 1
![](https://avatars.githubusercontent.com/u/5638101?s=25 ) | [@raymayemir](https://github.com/raymayemir) | ☕ x 1
![](https://avatars.githubusercontent.com/u/619996?s=25 ) | [@melkorm](https://github.com/melkorm) | ☕ x 1
![](https://avatars.githubusercontent.com/u/31022056?s=25 ) | [@marvinjwendt](https://github.com/thomasvvugt) | ☕ x 1
![](https://avatars.githubusercontent.com/u/31921460?s=25 ) | [@toishy](https://github.com/toishy) | ☕ x 1

## ‎‍💻 Code Contributors

<img src="https://opencollective.com/fiber/contributors.svg?width=890&button=false" alt="Code Contributors" style="max-width:100%;">

## ⚠️ License

Copyright (c) 2019-present [Fenny](https://github.com/fenny) and [Fiber Contributors](https://github.com/gofiber/fiber/graphs/contributors). `Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE). Official logo was created by [Vic Shóstak](https://github.com/koddr) and distributed under [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) license (CC BY-SA 4.0 International).
