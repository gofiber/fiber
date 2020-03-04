<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="125" src="https://github.com/gofiber/docs/blob/master/static/fiber_v2_logo.svg">
  </a>
  <br>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/gb.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ru.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/ru.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_es.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/es.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ja.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/jp.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_pt.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/pt.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_zh-CN.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/cn.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_de.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/de.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/kr.svg">
  </a>
  <!-- <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/fr.svg">
  </a> -->
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_tr.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/tr.svg">
  </a>
  <br><br>
  <a href="https://github.com/gofiber/fiber/releases">
    <img src="https://img.shields.io/github/release/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://fiber.wiki">
    <img src="https://img.shields.io/badge/api-documentation-blue?style=flat-square">
  </a>
  <a href="https://pkg.go.dev/github.com/gofiber/fiber?tab=doc">
    <img src="https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square">
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
</p>
<p align="center">
  <b>Fiber</b> est un framework web  inspiré d' <a href="https://github.com/expressjs/express">Express</a>. Il se base sur <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, l'implémentation HTTP de <a href="https://golang.org/doc/">Go</a> <b>la plus rapide</b>. Conçu pour <b>faciliter</b> les choses pour des développements <b>rapides</b>, Fiber garde à l'esprit <b>l'absence d'allocations mémoires</b>, ainsi que les <b>performances</b>.
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

Premièrement, [téléchargez](https://golang.org/dl/) et installez Go. Version `1.11` ou supérieur requise.

L'installation est ensuite lancée via la commande  [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them):

```bash
go get -u github.com/gofiber/fiber/...
```

## 🤖 Benchmarks

Ces tests sont effectués par [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) et [Go Web](https://github.com/smallnest/go-web-framework-benchmark). Si vous voulez voir tous les résultats, n'hésitez pas à consulter notre [Wiki](https://fiber.wiki/benchmarks).

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Features

-  [Routing](https://fiber.wiki/routing) robuste
- Serve [static files](https://fiber.wiki/application#static)
- [Performances](https://fiber.wiki/benchmarks) extrêmes
- [Faible empreinte mémoire](https://fiber.wiki/benchmarks)
- [API endpoints](https://fiber.wiki/context)
- Middleware & [Next](https://fiber.wiki/context#next) support
- Programmation côté serveur [rapide](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497)
- Disponible en [5 langues](https://fiber.wiki/)
- Et plus encore, [explorez Fiber](https://fiber.wiki/)

## 💡 Philosophie

Les nouveaux gophers qui passent de [Node.js](https://nodejs.org/en/about/) à [Go](https://golang.org/doc/) sont confrontés à une courbe d'apprentissage, avant de pouvoir construire leurs applications web et microservices. Fiber, en tant que **framework web**, a été mis au point avec en tête l'idée de **minimalisme**, tout en suivant l'**UNIX way**, afin que les nouveaux gophers puissent rapidement entrer dans le monde de Go, avec un accueil chaleureux, de confiance.

Fiber est **inspiré** par Express, le framework web le plus populaire d'Internet. Nous avons combiné la **facilité** d'Express, et la **performance brute** de Go. Si vous avez déja développé une application web en Node.js (_en utilisant Express ou équivalent_), alors de nombreuses méthodes et principes vous sembleront **familiers**.

## 👀 Exemples

Ci-dessous quelques exemples courants. Si vous voulez voir plus d'exemples, rendez-vous sur notre ["Recipes repository"](https://github.com/gofiber/recipes) ou visitez notre [documentation API](https://fiber.wiki).

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

### Serve static files
https://fiber.wiki/application#static
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
https://fiber.wiki/routing#middleware
https://fiber.wiki/context#next
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

  // POST /api/register
  app.Post("/api/register", func(c *fiber.Ctx) {
    fmt.Println("Last middleware")
    c.Send("Hello, World!")
  })

  app.Listen(3000)
}
```

<details>
  <summary>📚 Show more code examples</summary>

### Template engines
https://fiber.wiki/application#settings
https://fiber.wiki/context#render

Supported engines:
- [html](https://golang.org/pkg/html/template/)
- [amber](https://github.com/eknkc/amber)
- [handlebars](https://github.com/aymerick/raymond)
- [mustache](https://github.com/cbroglie/mustache)
- [pug](https://github.com/Joker/jade)

```go
func main() {
  // You can setup template engine before initiation app:
  app := fiber.New(&fiber.Settings{
    TemplateEngine:    "mustache",
    TemplateFolder:    "./views",
    TemplateExtension: ".tmpl",
  })

  // OR after initiation app at any convenient location:
  app.Settings.TemplateEngine = "mustache"
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
https://fiber.wiki/application#group
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
https://fiber.wiki/middleware#logger
```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/fiber/middleware"
)

func main() {
    app := fiber.New()
    
    // If you want to change default Logger config
    loggerConfig := middleware.LoggerConfig{
      Format:     "${time} - ${method} ${path}\n",
      TimeFormat: "Mon, 2 Jan 2006 15:04:05 MST",
    }

    // Middleware for Logger with config
    app.Use(middleware.Logger(loggerConfig))

    // ...
}
```

### Cross-Origin Resource Sharing (CORS)
https://fiber.wiki/middleware#cors

[CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) is a mechanism that uses additional HTTP headers to tell browsers to give a web application running at one origin, access to selected resources from a different origin. A web application executes a cross-origin HTTP request when it requests a resource that has a different origin (domain, protocol, or port) from its own.

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/fiber/middleware"
)

func main() {
    app := fiber.New()

    // Connect CORS for each route as middleware
    app.Use(middleware.CORS())

    app.Get("/", func(c *fiber.Ctx) {
        c.Send("CORS is enabled!")
    })

    app.Listen(3000)
}
```

Check CORS by passing any domain in `Origin` header: 

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```

### Custom 404 response

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
    c.SendStatus(404) // => 404 "Not Found"
  })

  app.Listen(3000)
}
```

### JSON Response
https://fiber.wiki/context#json
```go
type User struct {
  Name string `json:"name"`
  Age  int    `json:"age"`
}

func main() {
  app := fiber.New()

  app.Get("/user", func(c *fiber.Ctx) {
    c.JSON(&User{"John", 20})
    // {"name":"John", "age":20}
  })

  app.Get("/json", func(c *fiber.Ctx) {
    c.JSON(&fiber.Map{
			"success": true,
			"message": "Hi John!",
    })
    // {"success":true, "message":"Hi John!"}
  })

  app.Listen(3000)
}
```

### WebSocket support
https://fiber.wiki/application#websocket
```go
func main() {
  app := fiber.New()

  app.WebSocket("/ws", func(c *fiber.Conn) {
    for {
      mt, msg, err := c.ReadMessage()
      if err != nil {
        log.Println("read:", err)
        break
      }

      log.Printf("recovery: %s", msg)

      err = c.WriteMessage(mt, msg)
      if err != nil {
        log.Println("write:", err)
        break
      }
    }
  })

  // Listen on ws://localhost:3000/ws
  app.Listen(3000)
}
```

### Recover middleware
https://fiber.wiki/middleware#recover
```go
package main

import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/fiber/middleware"
)

func main() {
  app := fiber.New()

  app.Use(middleware.Recover(func(c *fiber.Ctx, err error) {
    log.Println(err)  // "Something went wrong!"
    c.SendStatus(500) // Internal Server Error
  })))
  
  app.Get("/", func(c *fiber.Ctx) {
    panic("Something went wrong!")
  })

  app.Listen(3000)
}
```
</details>

## 💬 Media

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) _par [Vic Shóstak](https://github.com/koddr), 03 Février 2020_

## 👍 Contribuer

Si vous voulez nous remercier et/ou soutenir le développement actif de `Fiber`:

1. Ajoutez une [GitHub Star](https://github.com/gofiber/fiber/stargazers) à ce projet.
2. Twittez à propos de ce projet [sur votre Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Ecrivez un article (review, tutorial) sur [Medium](https://medium.com/), [Dev.to](https://dev.to/), ou encore un blog personnel.
4. Aidez nous à traduire ce `README` dans d'autres langages.

## ☕ Supporters

<a href="https://www.buymeacoffee.com/fenny" target="_blank">
  <img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" height="100" >
</a>
<table>
  <tr>
    <td align="center">
        <a href="https://github.com/bihe">
          <img src="https://avatars1.githubusercontent.com/u/635852?s=460&v=4" width="100px"></br>
          <sub><b>HenrikBinggl</b></sub>
        </a>
    </td>
    <td align="center">
      <a href="https://github.com/koddr">
        <img src="https://avatars0.githubusercontent.com/u/11155743?s=460&v=4" width="100px"></br>
        <sub><b>koddr</b></sub>
      </a>
    </td>
    <td align="center">
      <a href="https://github.com/MarvinJWendt">
        <img src="https://avatars1.githubusercontent.com/u/31022056?s=460&v=4" width="100px"></br>
        <sub><b>MarvinJWendt</b></sub>
      </a>
    </td>
    <td align="center">
      <a href="https://github.com/toishy">
        <img src="https://avatars1.githubusercontent.com/u/31921460?s=460&v=4" width="100px"></br>
        <sub><b>ToishY</b></sub>
      </a>
    </td>
  </tr>
</table>

## ⭐️ Stars

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ Licence

`Fiber` est un logiciel gratuit et open-source, sous [Licence MIT](https://github.com/gofiber/fiber/blob/master/LICENSE).
