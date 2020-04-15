<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="125" src="https://github.com/gofiber/docs/blob/master/static/fiber_v2_logo.svg">
  </a>
  <br>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/en.svg">
  </a>
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
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_nl.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/nl.svg">
  </a>-->
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
  <a href="https://fiber.wiki">
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
  <b>Fiber</b> is een <b>web framework</b> geïnspireerd door <a href="https://github.com/expressjs/express">Express</a> gebouwd bovenop <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, de <b>snelste</b> HTTP-engine voor <a href="https://golang.org/doc/">Go</a>. Ontworpen om <b>snelle</b> ontwikkeling <b>gemakkelijker</b> te maken <b>zonder geheugenallocatie</b> tezamen met <b>hoge prestaties</b>.
</p>

## ⚡️ Bliksemsnelle start

```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()

  app.Get("/", func(c *fiber.Ctx) {
    c.Send("Hallo, Wereld!")
  })

  app.Listen(3000)
}
```

## ⚙️ Installatie

Allereerst, [download](https://golang.org/dl/) en installeer Go. `1.11` of hoger is vereist.

Installatie wordt gedaan met behulp van het [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) commando:

```bash
go get -u github.com/gofiber/fiber
```

## 🤖 Benchmarks

Deze tests zijn uitgevoerd door [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) en [Go Web](https://github.com/smallnest/go-web-framework-benchmark). Bezoek onze [Wiki](https://fiber.wiki/benchmarks) voor alle benchmark resultaten.

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Features

- Robuuste [routing](https://fiber.wiki/routing)
- Serveer [statische bestanden](https://fiber.wiki/application#static)
- Extreme [prestaties](https://fiber.wiki/benchmarks)
- [Weinig geheugenruimte](https://fiber.wiki/benchmarks)
- [API endpoints](https://fiber.wiki/context)
- [Middleware](https://fiber.wiki/middleware) & [Next](https://fiber.wiki/context#next) ondersteuning
- [Snelle](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) server-side programmering
- [Template engines](https://fiber.wiki/middleware#template)
- [WebSocket ondersteuning](https://fiber.wiki/middleware#websocket)
- [Rate Limiter](https://fiber.wiki/middleware#limiter)
- Vertaald in 11 andere talen
- En nog veel meer, [ontdek Fiber](https://fiber.wiki/)

## 💡 Filosofie

Nieuwe gophers die de overstap maken van [Node.js](https://nodejs.org/en/about/) naar [Go](https://golang.org/doc/), hebben te maken met een leercurve voordat ze kunnen beginnen met het bouwen van hun webapplicaties of microservices. Fiber, als een **web framework**, is gebouwd met het idee van **minimalisme** en volgt de **UNIX-manier**, zodat nieuwe gophers snel de wereld van Go kunnen betreden met een warm en vertrouwd welkom.\

Fiber is **geïnspireerd** door Express, het populairste webframework op internet. We hebben het **gemak** van Express gecombineerd met de **onbewerkte prestaties** van Go. Als je ooit een webapplicatie in Node.js hebt geïmplementeerd (_zoals Express of vergelijkbaar_), dan zullen veel methoden en principes **heel gewoon** voor je lijken.

We **luisteren** naar onze gebruikers in [issues](https://github.com/gofiber/fiber/issues) (_en overal op het internet_) om een **snelle**, **flexibele** en **vriendelijk** Go web framework te maken voor **elke** taak, **deadline** en ontwikkelaar **vaardigheid**! Net zoals Express dat doet in de JavaScript-wereld.

## 👀 Voorbeelden

Hieronder staan enkele van de meest voorkomende voorbeelden.

> Bekijk ons [Recepten repository](https://github.com/gofiber/recipes) voor meer voorbeelden met code of bezoek onze [API documentatie](https://fiber.wiki).

### Routing

📖 https://fiber.wiki/#basic-routing  


```go
func main() {
  app := fiber.New()

  // GET /john
  app.Get("/:naam", func(c *fiber.Ctx) {
    fmt.Printf("Hallo %s!", c.Params("naam"))
    // => Hallo john!
  })

  // GET /john
  app.Get("/:naam/:leeftijd?", func(c *fiber.Ctx) {
    fmt.Printf("Naam: %s, Leeftijd: %s", c.Params("naam"), c.Params("leeftijd"))
    // => Naam: john, Leeftijd:
  })

  // GET /api/registreer
  app.Get("/api/*", func(c *fiber.Ctx) {
    fmt.Printf("/api/%s", c.Params("*"))
    // => /api/registreer
  })

  app.Listen(3000)
}
```

### Serveer statische bestanden

📖 https://fiber.wiki/application#static  

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

📖 https://fiber.wiki/routing#middleware
📖 https://fiber.wiki/context#next

```go
func main() {
  app := fiber.New()

  // Komt overeen met elke route
  app.Use(func(c *fiber.Ctx) {
    fmt.Println("Eerste middleware")
    c.Next()
  })

  // Komt overeen met alle routes welke beginnen met /api
  app.Use("/api", func(c *fiber.Ctx) {
    fmt.Println("Tweede middleware")
    c.Next()
  })

  // GET /api/registreer
  app.Get("/api/registreer", func(c *fiber.Ctx) {
    fmt.Println("Laatste middleware")
    c.Send("Hallo, Wereld!")
  })

  app.Listen(3000)
}
```

<details>
  <summary>📚 Bekijk meer code voorbeelden</summary>

### Template engines

📖 https://fiber.wiki/application#settings  
📖 https://fiber.wiki/context#render  
📖 https://fiber.wiki/middleware#template  

Fiber ondersteunt de standaard [Go template engine](https://golang.org/pkg/html/template/)

Maar het is ook mogelijk om andere template engines te gebruiken zoals [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) of [pug](https://github.com/Joker/jade).

Gebruik hiervoor onze [Template Middleware](https://fiber.wiki/middleware#template).

```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/template"
)

func main() {
  // Stel een template engine in tijdens de aanvang van de app:
  app := fiber.New(&fiber.Settings{
    TemplateEngine:    template.Mustache(),
    TemplateFolder:    "./views",
    TemplateExtension: ".tmpl",
  })

  // OF na de aanvang van de app op elke geschikte locatie:
  app.Settings.TemplateEngine = template.Mustache()
  app.Settings.TemplateFolder = "./views"
  app.Settings.TemplateExtension = ".tmpl"

  // Het aanroepen van de template `./views/home.tmpl` kan als volgt:
  app.Get("/", func(c *fiber.Ctx) {
    c.Render("home", fiber.Map{
      "title": "Home",
      "year":  2020,
    })
  })

  // ...
}
```

### Routes groeperen in chains

📖 https://fiber.wiki/application#group  

```go
func main() {
  app := fiber.New()

  // Root API route
  api := app.Group("/api", cors())  // /api

  // API v1 routes
  v1 := api.Group("/v1", mysql())   // /api/v1
  v1.Get("/lijst", handler)         // /api/v1/lijst
  v1.Get("/gebruiker", handler)     // /api/v1/gebruiker

  // API v2 routes
  v2 := api.Group("/v2", mongodb()) // /api/v2
  v2.Get("/lijst", handler)         // /api/v2/lijst
  v2.Get("/gebruiker", handler)     // /api/v2/gebruiker

  // ...
}
```

### Middleware logger

📖 https://fiber.wiki/middleware#logger  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/logger"
)

func main() {
    app := fiber.New()

    // Optionele loggerconfiguratie
    config := logger.Config{
      Format:     "${time} - ${method} ${path}\n",
      TimeFormat: "Mon, 2 Jan 2006 15:04:05 MST",
    }

    // Logger met configuratie
    app.Use(logger.New(config))

    app.Listen(3000)
}
```

### Cross-Origin Resource Sharing (CORS)

📖 https://fiber.wiki/middleware#cors  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/cors"
)

func main() {
    app := fiber.New()

    // CORS met standaardconfiguratie
    app.Use(cors.New())

    app.Listen(3000)
}
```

Controleer CORS door een willekeurig domein in de `Origin`-header door te geven:

```bash
curl -H "Origin: http://google.nl" --verbose http://localhost:3000
```

### Custom 404 response

📖 https://fiber.wiki/application#http-methods  

```go
func main() {
  app := fiber.New()

  app.Static("/public")

  app.Get("/demo", func(c *fiber.Ctx) {
    c.Send("Dit is een demo!")
  })

  app.Post("/registreer", func(c *fiber.Ctx) {
    c.Send("Welkom!")
  })

  // Laatste middleware die bij alles past
  app.Use(func(c *fiber.Ctx) {
    c.SendStatus(404) 
    // => 404 "Not Found"
  })

  app.Listen(3000)
}
```

### JSON Response

📖 https://fiber.wiki/context#json  

```go
type Gebruiker struct {
  Naam      string  `json:"naam"`
  Leeftijd  int     `json:"leeftijd"`
}

func main() {
  app := fiber.New()

  app.Get("/gebruiker", func(c *fiber.Ctx) {
    c.JSON(&Gebruiker{"John", 20})
    // => {"naam":"John", "leeftijd":20}
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

📖 https://fiber.wiki/middleware#websocket  

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

📖 https://fiber.wiki/middleware#recover  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/recover"
)

func main() {
  app := fiber.New()

  // Optionele recover configuratie
  config := recover.Config{
    Handler: func(c *fiber.Ctx, err error) {
			c.SendString(err.Error())
			c.SendStatus(500)
		},
  }

  // Logger met aangepaste configuratie
  app.Use(recover.New(config))

  app.Listen(3000)
}
```
</details>

## 🧬 Beschikbare Middlewares

Voor _eenvoudiger_ en _duidelijker_ werk hebben we de beschikbare [middleware](https://fiber.wiki/middleware) in afzonderlijke repositories geplaatst:

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

## 👍 Bijdragen

Om de actieve ontwikkelingen van `Fiber` te ondersteunen of om een **bedankje** te geven:

1. Voeg een [GitHub Star](https://github.com/gofiber/fiber/stargazers) toe aan het project.
2. Tweet over het project [op je Twitter account](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Schrijf een recensie of tutorial op [Medium](https://medium.com/), [Dev.to](https://dev.to/) of een persoonlijke blog.
4. Help ons deze `README` naar een andere taal te vertalen.


## ☕ Coffee Supporters

<table>
  <tr>
    <td align="center">
        <a href="https://github.com/bihe">
          <img src="https://avatars1.githubusercontent.com/u/635852?s=460&v=4" width="100px"></br>
          <sub><b>Henrik Binggl</b></sub>
        </a>
    </td>
    <td align="center">
      <a href="https://github.com/koddr">
        <img src="https://avatars0.githubusercontent.com/u/11155743?s=460&v=4" width="100px"></br>
        <sub><b>Vic&nbsp;Shóstak</b></sub>
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
    <td align="center">
        <a href="https://github.com/gofiber/fiber">
          <img src="https://i.stack.imgur.com/frlIf.png" width="100px"></br>
          <sub><b>JustDave</b></sub>
        </a>
    </td>
    <td align="center">
        <a href="https://github.com/melkorm">
          <img src="https://avatars2.githubusercontent.com/u/619996?s=460&v=4" width="100px"></br>
          <sub><b>melkorm</b></sub>
        </a>
    </td>
    <td align="center">
        <a href="https://github.com/ekaputra07">
          <img src="https://avatars3.githubusercontent.com/u/1094221?s=460&v=4" width="100px"></br>
          <sub><b>ekaputra07</b></sub>
        </a>
    </td>
    <td align="center">
        <a href="https://github.com/raymayemir">
          <img src="https://avatars2.githubusercontent.com/u/5638101?s=460&v=4" width="100px"></br>
          <sub><b>Ray Mayemir</b></sub>
        </a>
    </td>
  </tr>
</table>

<a href="https://www.buymeacoffee.com/fenny" target="_blank">
  <img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" height="100" >
</a>

## ‎‍💻 Code Contributors

<img src="https://opencollective.com/fiber/contributors.svg?width=890&button=false" alt="Code Contributors" style="max-width:100%;">

## ⚠️ License

Copyright (c) 2019-present [Fenny](https://github.com/fenny) and [Fiber Contributors](https://github.com/gofiber/fiber/graphs/contributors). `Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE). Official logo was created by [Vic Shóstak](https://github.com/koddr) and distributed under [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) license (CC BY-SA 4.0 International).
