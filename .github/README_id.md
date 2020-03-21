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
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ko.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/fr.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_tr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/tr.svg">
  </a>
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_id.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/id.svg">
  </a>-->
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
	<b>Fiber</b> adalah <b>web framework</b> yang terinspirasi dari <a href="https://github.com/expressjs/express">Express</a> yang berbasiskan <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, HTTP engine paling <b>cepat</b> untuk <a href="https://golang.org/doc/">Go</a>. Dirancang untuk <b>mempermudah</b>, <b>mempercepat</b> pengembangan aplikasi dengan <b>alokasi memori nol-nya</b> serta <b>kinerja</b> yang selalu diperhatikan.
</p>

## ⚡️ Cara Mulai

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

## ⚙️ Instalasi

Pertama, [unduh](https://golang.org/dl/) dan instal Go di komputer anda. Versi `1.11` atau yang lebih tinggi diperlukan.

Instalasi dilakukkan dengan perintah [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them):

```bash
go get -u github.com/gofiber/fiber/...
```

## 🤖 Pengukuran Kinerja

Pengukuran ini dilakukan oleh [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) dan [Go Web](https://github.com/smallnest/go-web-framework-benchmark). Apabila anda ingin melihat hasil lengkapnya, silahkan kunjungi halaman [Wiki](https://fiber.wiki/benchmarks) kami.

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Fitur

- Sistem [Routing](https://fiber.wiki/routing) yang solid
- Serve [file statis](https://fiber.wiki/application#static)
- [Kinerja](https://fiber.wiki/benchmarks) ekstrim
- [Penggunaan memori](https://fiber.wiki/benchmarks) yang kecil
- Cocok untuk [API](https://fiber.wiki/context)
- Mendukung Middleware & [Next](https://fiber.wiki/context#next) seperti Express
- Kembangkan aplikasi dengan [Cepat](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497)
- Tersedia di 10 bahasa lainnya
- Dan masih banyak lagi, [kunjungi Fiber](https://fiber.wiki/)

## 💡 Filosofi

Bagi yang baru yang beralih dari [Node.js](https://nodejs.org/en/about/) ke [Go](https://golang.org/doc/) terkadang perlu waktu yang cukup lama sebelum mereka mampu membuat aplikasi web dengan Go. Fiber, sebagai **web framework** dirancang secara **minimalis** dan mengikuti filosofi dari **UNIX**, sehingga pengguna baru dengan cepat memasuki dunia Go dengan sambutan yang hangat dan dapat diandalkan.

Fiber terinspirasi dari Express, salah satu web framework paling terkenal di Internet. Kami menggabungkan **kemudahan** dari Express dan **kinerja luar biasa** dari Go. Apabila anda pernah membuat aplikasi dengan Node.js (_dengan Express atau yang lainnya_), maka banyak metode dan prinsip yang akan terasa **sangat umum** bagi anda.

Kami **mendengarkan** para pengguna di [GitHub Issues](https://github.com/gofiber/fiber/issues) (_dan berbagai platform lainnya_) untuk menciptakan web framework yang **cepat**, **fleksibel** dan **bersahabat** untuk berbagai macam keperluan, **tenggat waktu** dan **keahlian** para pengguna! Sama halnya seperti yang dilakukkan Express di dunia JavaScript.

## 👀 Contoh

Dibawah ini terdapat beberapa contoh penggunaan. Jika anda ingin contoh lainnya, silahkan kunjungi [Gudang resep](https://github.com/gofiber/recipes) atau kunjungi [Dokumentasi API](https://fiber.wiki) kami.

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

### Menyajikan file statis
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

Docs:

- 📖 https://fiber.wiki/application#settings
- 📖 https://fiber.wiki/context#render

Supported engines:

- [html](https://golang.org/pkg/html/template/)
- [amber](https://github.com/eknkc/amber)
- [handlebars](https://github.com/aymerick/raymond)
- [mustache](https://github.com/cbroglie/mustache)
- [pug](https://github.com/Joker/jade)

Example:

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

Docs:

- 📖 https://fiber.wiki/application#group

Example:

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

Docs:

- 📖 https://fiber.wiki/middleware#logger

Example:

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/logger"
)

func main() {
    app := fiber.New()

    // Optional logger config
    config := logger.LoggerConfig{
      Format:     "${time} - ${method} ${path}\n",
      TimeFormat: "Mon, 2 Jan 2006 15:04:05 MST",
    }

    // Logger with config
    app.Use(logger.New(config))

    app.Listen(3000)
}
```

### Cross-Origin Resource Sharing (CORS)

Docs:

- 📖 https://fiber.wiki/middleware#cors

Example:

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

Docs:

- 📖 https://fiber.wiki/application#http-methods

Example:

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

Docs:

- 📖 https://fiber.wiki/context#json

Example:

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

### WebSocket middleware

Docs:

- 📖 https://fiber.wiki/middleware#websocket

Example:

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

Docs:

- 📖 https://fiber.wiki/middleware#recover

Example:

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/recover"
)

func main() {
  app := fiber.New()

  // Optional recover config
  config := recover.LoggerConfig{
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

## 💬 Media

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) (_by [Vic Shóstak](https://github.com/koddr), 03 Feb 2020_)
- [Fiber release v1.7 is out now! 🎉 What's new and is he still fast, flexible and friendly?](https://dev.to/koddr/fiber-v2-is-out-now-what-s-new-and-is-he-still-fast-flexible-and-friendly-3ipf) (_by [Vic Shóstak](https://github.com/koddr), 21 Feb 2020_)

## 👍 Berkontribusi

Apabila anda ingin mengucapkan **terima kasih** dan/atau mendukung pengembangan `Fiber`:

1. Berikan bintang atau [GitHub Star](https://github.com/gofiber/fiber/stargazers) ke proyek ini.
2. Bagikan [di Twitter anda](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Buat ulasan atau tutorial di [Medium](https://medium.com/), [Dev.to](https://dev.to/) atau blog pribadi anda.
4. Bantu kami menerjemahkan `README` ini ke bahasa lainnya.


## ☕ Para Pendukung

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
  </tr>
</table>

## ⭐️ Bintang

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ Lisensi

`Fiber` adalah perangkat lunak gratis dengan sumber terbuka berlisensi [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE).

[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/0)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/0)[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/1)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/1)[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/2)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/2)[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/3)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/3)[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/4)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/4)[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/5)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/5)[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/6)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/6)[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/7)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/7)
