<p align="center">
  <a href="https://gofiber.io">
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
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_nl.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/nl.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ko.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/fr.svg">
  </a>
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_tr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/tr.svg">
  </a>-->
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_id.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/id.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_he.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/il.svg">
  </a>
  <br><br>
  <a href="https://github.com/gofiber/fiber/releases">
    <img src="https://img.shields.io/github/release/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://pkg.go.dev/github.com/gofiber/fiber?tab=doc">
    <img src="https://img.shields.io/badge/go.dev-007d9c?logo=go&logoColor=white&style=flat-square">
  </a>
  <a href="https://docs.gofiber.io">
    <img src="https://img.shields.io/badge/api-docs-blue?style=flat-square">
  </a>
  <a href="https://goreportcard.com/report/github.com/gofiber/fiber">
    <img src="https://goreportcard.com/badge/github.com/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://gocover.io/github.com/gofiber/fiber">
    <img src="https://img.shields.io/badge/coverage-91%25-brightgreen?style=flat-square">
  </a>
  <a href="https://github.com/gofiber/fiber/actions?query=workflow%3ATest">
    <img src="https://img.shields.io/github/workflow/status/gofiber/fiber/Test?label=tests&style=flat-square">
  </a>
  <a href="https://github.com/gofiber/fiber/actions?query=workflow%3AGosec">
    <img src="https://img.shields.io/github/workflow/status/gofiber/fiber/Gosec?label=gosec&style=flat-square">
  </a>
  <a href="https://t.me/gofiber">
    <img src="https://img.shields.io/badge/telegram-join%20chat-0088cc?style=flat-square">
  </a>
</p>
<p align="center">
    <b>Fiber</b>, <a href="https://golang.org/doc/">Go</a> için <b>en hızlı</b> HTTP motoru olan <a href="https://github.com/valyala/fasthttp">Fasthttp</a> üzerine inşa edilmiş, <a href="https://github.com/expressjs/express">Express</a> den ilham alan bir <b>web çatısıdır</b>. <b>Sıfır bellek ayırma</b> ve <b>performans</b> göz önünde bulundurularak <b>hızlı</b> geliştirme için işleri <b>kolaylaştırmak</b> üzere tasarlandı.
</p>

## ⚡️ Hızlı Başlangıç

```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()

  app.Get("/", func(c *fiber.Ctx) {
    c.Send("Merhaba dünya!")
  })

  app.Listen(3000)
}
```

## ⚙️ Kurulum

İlk önce, Go yu [indirip](https://golang.org/dl/) kuruyoruz. `1.11` veya daha yeni sürüm gereklidir.

[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) komutunu kullanarak kurulumu tamamlıyoruz:

```bash
go get -u github.com/gofiber/fiber/...
```

## 🤖 Performans Ölçümleri

Bu testler [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) ve [Go Web](https://github.com/smallnest/go-web-framework-benchmark) ile koşuldu. Bütün sonuçları görmek için lütfen [Wiki](https://docs.gofiber.io/benchmarks) sayfasını ziyaret ediniz.

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Özellikler

- Güçlü [rotalar](https://docs.gofiber.io/routing)
- [Statik dosya](https://docs.gofiber.io/application#static) yönetimi
- Olağanüstü [performans](https://docs.gofiber.io/benchmarks)
- [Düşük bellek](https://docs.gofiber.io/benchmarks) tüketimi
- [API uç noktaları](https://docs.gofiber.io/context)
- Ara katman & [Sonraki](https://docs.gofiber.io/context#next) desteği
- [Hızlı](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) sunucu taraflı programlama
- [Template engines](https://docs.gofiber.io/middleware#template)
- [WebSocket support](https://docs.gofiber.io/middleware#websocket)
- [Rate Limiter](https://docs.gofiber.io/middleware#limiter)
- Available in [12 languages](https://docs.gofiber.io/)
- Ve daha fazlası, [Fiber ı keşfet](https://docs.gofiber.io/)

## 💡 Felsefe

[Node.js](https://nodejs.org/en/about/) den [Go](https://golang.org/doc/) ya geçen yeni gopher lar kendi web uygulamalarını ve mikroservislerini yazmaya başlamadan önce dili öğrenmek ile uğraşıyorlar. Fiber, bir **web çatısı** olarak, **minimalizm** ve **UNIX yolu**nu izlemek fikri ile oluşturuldu. Böylece yeni gopher lar sıcak ve güvenilir bir hoşgeldin ile Go dünyasına giriş yapabilirler.

Fiber internet üzerinde en popüler olan Express web çatısından **esinlenmiştir**. Biz Express in **kolaylığını** ve Go nun **ham performansını** birleştirdik. Daha önce Node.js üzerinde (Express veya benzerini kullanarak) bir web uygulaması geliştirdiyseniz, pek çok metod ve prensip size **çok tanıdık** gelecektir.

## 👀 Örnekler

Aşağıda yaygın örneklerden bazıları listelenmiştir. Daha fazla kod örneği görmek için, lütfen [Kod deposunu](https://github.com/gofiber/recipes) veya [API dökümantasyonunu](https://docs.gofiber.io) ziyaret ediniz.

### Rotalama

📖 [Rotalama](https://docs.gofiber.io/#basic-routing)


```go
func main() {
  app := fiber.New()

  // GET /john http methodunu çağır
  app.Get("/:name", func(c *fiber.Ctx) {
    fmt.Printf("Hello %s!", c.Params("name"))
    // => Hello john!
  })

  // GET /john http methodunu çağır
  app.Get("/:name/:age?", func(c *fiber.Ctx) {
    fmt.Printf("Name: %s, Age: %s", c.Params("name"), c.Params("age"))
    // => Name: john, Age:
  })

  // GET /api/register http methodunu çağır
  app.Get("/api/*", func(c *fiber.Ctx) {
    fmt.Printf("/api/%s", c.Params("*"))
    // => /api/register
  })

  app.Listen(3000)
}
```

### Statik Dosyaları Servis Etmek

📖 [Statik](https://docs.gofiber.io/application#static)

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

### Ara Katman ve İleri(Middleware & Next)

📖 [Ara Katman](https://docs.gofiber.io/routing#middleware)
📖 [İleri](https://docs.gofiber.io/context#next)

```go
func main() {
  app := fiber.New()

  // Bütün rotalarla eşleş
  app.Use(func(c *fiber.Ctx) {
    fmt.Println("First middleware")
    c.Next()
  })

  // /api ile başlayan tüm rotalarla eşleş
  app.Use("/api", func(c *fiber.Ctx) {
    fmt.Println("Second middleware")
    c.Next()
  })

  // GET /api/register http methodunu çağır
  app.Get("/api/list", func(c *fiber.Ctx) {
    fmt.Println("Last middleware")
    c.Send("Hello, World!")
  })

  app.Listen(3000)
}
```

<details>
  <summary>📚 Daha fazla kod örneği göster</summary>

### Şablon Motorları

📖 [Ayarlar](https://docs.gofiber.io/application#settings)
📖 [Tasvir et(Render)](https://docs.gofiber.io/context#render)
📖 [Şablonlar](https://docs.gofiber.io/middleware#template)

Fiber varsayılan olarak [Go şablon motoru](https://golang.org/pkg/html/template/)'nu destekler.

Eğer başka bir şablon motoru kullanmak isterseniz, mesela [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) yada [pug](https://github.com/Joker/jade) gibi, bizim [Şablon Ara Katmanımızı](https://docs.gofiber.io/middleware#template) da kullanabilirsiniz.

```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/template"
)

func main() {
  //Uygulamayı başlatmadan önce şablon motorunu kurabilirsiniz:
  app := fiber.New(&fiber.Settings{
    TemplateEngine:    template.Mustache(),
    TemplateFolder:    "./views",
    TemplateExtension: ".tmpl",
  })

  // YADA uygulamayı başlattıktan sonra uygun yere koyabilirsiniz:
  app.Settings.TemplateEngine = template.Mustache()
  app.Settings.TemplateFolder = "./views"
  app.Settings.TemplateExtension = ".tmpl"

  // Ve şimdi, bu şekide `./views/home.tmpl` şablonunu çağırabilirsiniz:
  app.Get("/", func(c *fiber.Ctx) {
    c.Render("home", fiber.Map{
      "title": "Homepage",
      "year":  1999,
    })
  })

  // ...
}
```

### Rotaları Zincirlere Gruplama

📖 [Grup](https://docs.gofiber.io/application#group)

```go
func main() {
  app := fiber.New()

  // Kök API rotası
  api := app.Group("/api", cors())  // /api

  // API v1 rotası
  v1 := api.Group("/v1", mysql())   // /api/v1
  v1.Get("/list", handler)          // /api/v1/list
  v1.Get("/user", handler)          // /api/v1/user

  // API v2 rotası
  v2 := api.Group("/v2", mongodb()) // /api/v2
  v2.Get("/list", handler)          // /api/v2/list
  v2.Get("/user", handler)          // /api/v2/user

  // ...
}
```

### Ara Katman Günlükcüsü(Logger)

📖 [Günlükcü](https://docs.gofiber.io/middleware#logger)

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/logger"
)

func main() {
    app := fiber.New()

    // Tercihe bağlı günlük ayarları
    config := logger.Config{
      Format:     "${time} - ${method} ${path}\n",
      TimeFormat: "Mon, 2 Jan 2006 15:04:05 MST",
    }

    // Günlükcüyü ayarla
    app.Use(logger.New(config))

    app.Listen(3000)
}
```

### Farklı Merkezler Arası Kaynak Paylaşımı (CORS)

📖 [CORS](https://docs.gofiber.io/middleware#cors)

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/cors"
)

func main() {
    app := fiber.New()

    // Varsayılan ayarlarla CORS
    app.Use(cors.New())

    app.Listen(3000)
}
```

`Origin` başlığı içinde herhangı bir alan adı kullanarak CORS'u kontrol et:

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```

### Özelleştirilebilir 404 yanıtları

📖 [HTTP Methodlari](https://docs.gofiber.io/application#http-methods)

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

  // Herhangi bir şeyle eşleşen son ara katman
  app.Use(func(c *fiber.Ctx) {
    c.SendStatus(404)
    // => 404 "Not Found"
  })

  app.Listen(3000)
}
```

### JSON Yanıtları

📖 [JSON](https://docs.gofiber.io/context#json)

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

### WebSocket Yükseltmesi

📖 [Websocket](https://docs.gofiber.io/middleware#websocket)

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

### Ara Katman'dan Kurtarma

📖 [Kurtar](https://docs.gofiber.io/middleware#recover)

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/recover"
)

func main() {
  app := fiber.New()

  // Özelleştirilebilir kurtarma ayarı
  config := recover.Config{
    Handler: func(c *fiber.Ctx, err error) {
			c.SendString(err.Error())
			c.SendStatus(500)
		},
  }

  // Özelleştrilebilir günlükleme
  app.Use(recover.New(config))

  app.Listen(3000)
}
```
</details>

## 🧬 Official Middlewares

For an more _maintainable_ middleware _ecosystem_, we've put official [middlewares](https://docs.gofiber.io/middleware) into separate repositories:

- [gofiber/basicauth](https://github.com/gofiber/basicauth)
- [gofiber/keyauth](https://github.com/gofiber/keyauth)
- [gofiber/compression](https://github.com/gofiber/compression)
- [gofiber/requestid](https://github.com/gofiber/requestid)
- [gofiber/websocket](https://github.com/gofiber/websocket)
- [gofiber/rewrite](https://github.com/gofiber/rewrite)
- [gofiber/recover](https://github.com/gofiber/recover)
- [gofiber/limiter](https://github.com/gofiber/limiter)
- [gofiber/session](https://github.com/gofiber/session)
- [gofiber/logger](https://github.com/gofiber/logger)
- [gofiber/helmet](https://github.com/gofiber/helmet)
- [gofiber/embed](https://github.com/gofiber/embed)
- [gofiber/pprof](https://github.com/gofiber/pprof)
- [gofiber/cors](https://github.com/gofiber/cors)
- [gofiber/csrf](https://github.com/gofiber/csrf)
- [gofiber/jwt](https://github.com/gofiber/jwt)

## 🌱 Third Party Middlewares

This is a list of middlewares that are created by the Fiber community, please create a PR if you want to see yours!
- [arsmn/fiber-swagger](https://github.com/arsmn/fiber-swagger)
- [arsmn/fiber-casbin](https://github.com/arsmn/fiber-casbin)
- [arsmn/fiber-introspect](https://github.com/arsmn/fiber-introspect)
- [shareed2k/fiber_tracing](https://github.com/shareed2k/fiber_tracing)
- [shareed2k/fiber_limiter](https://github.com/shareed2k/fiber_limiter)

## 💬 Medya

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) — _03 Şubat 2020_
- [Fiber released v1.7! 🎉 What's new and is it still fast, flexible and friendly?](https://dev.to/koddr/fiber-v2-is-out-now-what-s-new-and-is-he-still-fast-flexible-and-friendly-3ipf) — _21 Şubat 2020_
- [🚀 Fiber v1.8. What's new, updated and re-thinked?](https://dev.to/koddr/fiber-v1-8-what-s-new-updated-and-re-thinked-339h) — _03 Mart 2020_
- [Is switching from Express to Fiber worth it? 🤔](https://dev.to/koddr/are-sure-what-your-lovely-web-framework-running-so-fast-2jl1) — _01 Nisan 2020_
- [Creating Fast APIs In Go Using Fiber](https://dev.to/jozsefsallai/creating-fast-apis-in-go-using-fiber-59m9) — _07 Nisan 2020_
- [Building a Basic REST API in Go using Fiber](https://tutorialedge.net/golang/basic-rest-api-go-fiber/) - _23 Nisan 2020_
- [📺 Building a REST API using GORM and Fiber](https://youtu.be/Iq2qT0fRhAA) - _25 Nisan 2020_
- [🌎 Create a travel list app with Go, Fiber, Angular, MongoDB and Google Cloud Secret Manager](https://blog.yongweilun.me/create-a-travel-list-app-with-go-fiber-angular-mongodb-and-google-cloud-secret-manager-ck9fgxy0p061pcss1xt1ubu8t) - _25 Apr 2020_

## 👍 Destek

Eğer  **teşekkür etmek** ve/veya `Fiber`'in aktif geliştirilmesini desteklemek istiyorsanız:

1. Projeye [GitHub Yıldızı](https://github.com/gofiber/fiber/stargazers) verin.
2. [Twitter hesabınızdan](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) proje hakkında tweet atın.
3. [Medium](https://medium.com/), [Dev.to](https://dev.to/) veya kişisel blog üzerinden bir inceleme veya eğitici yazı yazın.
4. API dökümantasyonunu çevirerek destek olabilirsiniz [Crowdin](https://crowdin.com/project/gofiber) [![Crowdin](https://badges.crowdin.net/gofiber/localized.svg)](https://crowdin.com/project/gofiber)
5. Projeye [bir fincan kahve] ısmarlayarak projeye destek olabilirsiniz(https://buymeacoff.ee/fenny).

## ☕ Destekçiler
Fiber, alan adı, gitbook, netlify, serverless yer sağlayıcısı giderleri ve benzeri şeyleri ödemek için bağışlarla yaşayan bir açık kaynaklı projedir. Eğer Fiber'e destek olmak isterseniz, ☕ [**buradan kahve ısmarlayabilirsiniz.**](https://buymeacoff.ee/fenny)

|                                                             | User                                            | Donation |
| :---------------------------------------------------------- | :---------------------------------------------- | :------- |
| ![](https://avatars.githubusercontent.com/u/59947262?s=25 ) | [@thomasvvugt](https://github.com/thomasvvugt)  | ☕ x 5    |
| ![](https://avatars.githubusercontent.com/u/1094221?s=25 )  | [@ekaputra07](https://github.com/ekaputra07)    | ☕ x 5    |
| ![](https://avatars.githubusercontent.com/u/186637?s=25 )   | [@candidosales](https://github.com/candidosales)| ☕ x 5    |
| ![](https://avatars.githubusercontent.com/u/635852?s=25 )   | [@bihe](https://github.com/bihe)                | ☕ x 3    |
| ![](https://avatars.githubusercontent.com/u/307334?s=25 )   | [@justdave](https://github.com/justdave)        | ☕ x 3    |
| ![](https://avatars.githubusercontent.com/u/11155743?s=25 ) | [@koddr](https://github.com/koddr)              | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/2978730?s=25 )  | [@diegowifi](https://github.com/diegowifi)      | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/44171355?s=25 ) | [@ssimk0](https://github.com/ssimk0)            | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/5638101?s=25 )  | [@raymayemir](https://github.com/raymayemir)    | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/619996?s=25 )   | [@melkorm](https://github.com/melkorm)          | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/31022056?s=25 ) | [@marvinjwendt](https://github.com/thomasvvugt) | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/31921460?s=25 ) | [@toishy](https://github.com/toishy)            | ☕ x 1    |

## ‎‍💻 Koda Katkı Sağlayanlar

<img src="https://opencollective.com/fiber/contributors.svg?width=890&button=false" alt="Code Contributors" style="max-width:100%;">

## ⚠️ Lisans

Telif (c) 2019-günümüz [Fenny](https://github.com/fenny) ve [Contributors](https://github.com/gofiber/fiber/graphs/contributors). `Fiber`, [MIT Lisansı](https://github.com/gofiber/fiber/blob/master/LICENSE) altında özgür ve açık kaynaklı bir yazılımdır. Resmi logosu [Vic Shóstak](https://github.com/koddr) tarafında tasarlanmıştır ve [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) lisansı altında dağıtımı yapılır. (CC BY-SA 4.0 International).

**3. Parti yazılım lisanları**
- [FastHTTP](https://github.com/valyala/fasthttp/blob/master/LICENSE)
- [Schema](https://github.com/gorilla/schema/blob/master/LICENSE)
