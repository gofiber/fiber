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
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_de.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/de.svg">
  </a>-->
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/kr.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/fr.svg">
  </a>
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
<strong>Fiber</strong> ist ein von <a href="https://github.com/expressjs/express">Expressjs</a> inspiriertes <strong>Web-Framework</strong>, aufgebaut auf <a href="https://github.com/valyala/fasthttp">Fasthttp</a> - die <strong>schnellste</strong> HTTP engine für <a href="https://golang.org/doc/">Go</a>. Kreiert um Dinge zu <strong>vereinfachen</strong>, für <strong>schnelle</strong> Entwicklung mit <strong>keinen Speicherzuweisungen</strong> und <strong>Performance</strong> im Hinterkopf.
</p>

## ⚡️ Schnellstart

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

Als erstes, [downloade](https://golang.org/dl/) und installiere Go. `1.11` oder höher.

Die Installation wird durch das  [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) Kommando gestartet:

```bash
go get -u github.com/gofiber/fiber/...
```

## 🤖 Benchmarks

Diese Tests wurden von [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) und [Go Web](https://github.com/smallnest/go-web-framework-benchmark) ausgeführt. Falls du alle Resultate sehen möchtest, besuche bitte unser [Wiki](https://fiber.wiki/benchmarks).

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Eigenschaften

- Robustes [Routing](https://fiber.wiki/routing)
- Bereitstellen von [statischen Dateien](https://fiber.wiki/application#static)
- Extreme [Performance](https://fiber.wiki/benchmarks)
- [Geringe Arbeitsspeicher](https://fiber.wiki/benchmarks) verwendung
- Express [API Endpunkte](https://fiber.wiki/context)
- Middleware & [Next](https://fiber.wiki/context#next) Support
- [Schnelle](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) serverseitige Programmierung
- Übersetzt in [5 Sprachen](https://fiber.wiki/)
- Und vieles mehr - [erkunde Fiber](https://fiber.wiki/)

## 💡 Philosophie

Neue gopher welche von [Node.js](https://nodejs.org/en/about/) zu [Go](https://golang.org/doc/) umsteigen, müssen eine Lernkurve durchlaufen, bevor sie ihre Webanwendungen oder Microservices erstellen können. Fiber, als ein **Web-Framework**, wurde erschaffen mit der Idee von **Minimalismus** und folgt dem **UNIX Weg** damit neue Gophers mit einem herzlichen und vertrauenswürdigen Willkommen schnell in die Welt von Go eintreten können.

Fiber ist **inspiriert** von Expressjs, dem beliebtesten Web-Framework im Internet. Wir haben die **Leichtigkeit** von Express und die **Rohleistung** von Go kombiniert. Wenn du jemals eine Webanwendung mit Node.js implementiert hast (_mit Express.js oder ähnlichem_), werden dir viele Methoden und Prinzipien **sehr vertraut** vorkommen.

## 👀 Beispiele

Nachfolgend sind einige der gängigen Beispiele aufgeführt. Wenn du weitere Codebeispiele sehen möchten, besuche bitte unser ["Recipes Repository"](https://github.com/gofiber/recipes) oder besuche unsere [API Dokumentation](https://fiber.wiki).

### Serve static files

```go
func main() {
  app := fiber.New()

  app.Static("/public")
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

### Middleware & Next

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
  <summary>📜 Show more code examples</summary>

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

```go
func main() {
  app := fiber.New()

  type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
  }

  // Serialize JSON
  app.Get("/json", func(c *fiber.Ctx) {
    c.JSON(&User{"John", 20})
    // => {"name":"John", "age":20}
  })

  app.Listen(3000)
}
```


### Recover from panic

```go
func main() {
  app := fiber.New()

  app.Get("/", func(c *fiber.Ctx) {
    panic("Something went wrong!")
  })

  app.Recover(func(c *fiber.Ctx) {
    c.Status(500).Send(c.Error())
    // => 500 "Something went wrong!"
  })

  app.Listen(3000)
}
```
</details>

## 💬 Medien

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) _von [Vic Shóstak](https://github.com/koddr), 03 Feb 2020_

## 👍 Mitwirken

Falls du **danke** sagen möchtest und/oder aktiv die Entwicklung von `fiber` fördern möchtest:

1. Füge dem Projekt einen [GitHub Stern](https://github.com/gofiber/fiber/stargazers) hinzu.
2. Twittere über das Projekt [auf deinem Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Schreibe eine Rezension auf [Medium](https://medium.com/), [Dev.to](https://dev.to/) oder einem persönlichem Blog.
4. Hilf uns diese `README` und die [API Docs](https://fiber.wiki/) in eine andere Sprache zu übersetzen.

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

## ⭐️ Sterne

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ Lizenz

`Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE).
