<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="100" src="https://github.com/gofiber/docs/blob/master/static/logo.svg">
  </a>
  <br><br>
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
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/fr.svg">
  </a>
  <br><br>
  <a href="https://github.com/gofiber/fiber/releases">
    <img src="https://img.shields.io/github/release/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://fiber.wiki">
    <img src="https://img.shields.io/badge/api-documentation-blue?style=flat-square">
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
go get github.com/gofiber/fiber
```

## 🤖 Performans Ölçümleri

Bu testler [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) ve [Go Web](https://github.com/smallnest/go-web-framework-benchmark) ile koşuldu. Bütün sonuçları görmek için lütfen [Wiki](https://fiber.wiki/benchmarks) sayfasını ziyaret ediniz.

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Özellikler

- Güçlü [rotalar](https://fiber.wiki/routing)
- [Statik dosya](https://fiber.wiki/application#static) yönetimi
- Olağanüstü [performans](https://fiber.wiki/benchmarks)
- [Düşük bellek](https://fiber.wiki/benchmarks) tüketimi
- [API uç noktaları](https://fiber.wiki/context)
- Ara katman & [Sonraki](https://fiber.wiki/context#next) desteği
- [Hızlı](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) sunucu taraflı programlama
- [5 dilde](https://fiber.wiki/) mevcut
- Ve daha fazlası, [Fiber ı keşfet](https://fiber.wiki/)

## 💡 Felsefe

[Node.js](https://nodejs.org/en/about/) den [Go](https://golang.org/doc/) ya geçen yeni gopher lar kendi web uygulamalarını ve mikroservislerini yazmaya başlamadan önce dili öğrenmek ile uğraşıyorlar. Fiber, bir **web çatısı** olarak, **minimalizm** ve **UNIX yolu**nu izlemek fikri ile oluşturuldu. Böylece yeni gopher lar sıcak ve güvenilir bir hoşgeldin ile Go dünyasına giriş yapabilirler.

Fiber internet üzerinde en popüler olan Express web çatısından **esinlenmiştir**. Biz Express in **kolaylığını** ve Go nun **ham performansını** birleştirdik. Daha önce Node.js üzerinde (Express veya benzerini kullanarak) bir web uygulaması geliştirdiyseniz, pek çok metod ve prensip size **çok tanıdık** gelecektir.

## 👀 Örnekler

Aşağıda yaygın örneklerden bazıları listelenmiştir. Daha fazla kod örneği görmek için, lütfen [Kod depomuzu](https://github.com/gofiber/recipes) veya [API dökümantasyonunu](https://fiber.wiki) ziyaret ediniz.

### Rotalar

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

### Statik dosya yönetimi

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

### Ara Katman & Sonraki

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
  <summary>📚 Daha fazla kod örneği görüntüle</summary>

### Özel 404 Cevabı

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

### JSON Cevabı

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


### Panikten Kurtarma

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

## 💬 Medya

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) , [Vic Shóstak](https://github.com/koddr) tarafından, 03 Şub 2020

## 👍 Destek

Eğer  **teşekkür etmek** ve/veya `Fiber` ın aktif geliştirilmesini desteklemek istiyorsanız:

1. Projeye [GitHub Yıldızı](https://github.com/gofiber/fiber/stargazers) verin.
2. [Twitter hesabınızdan](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) proje hakkında tweet atın.
3. [Medium](https://medium.com/), [Dev.to](https://dev.to/) veya kişisel blog üzerinden bir inceleme veya eğitici yazı yazın.
4. Bu `BENİOKU` sayfasını başka bir dile tercüme etmek için bize yardım edin.


## ☕ Destekleyenler

<a href="https://www.buymeacoffee.com/fenny" target="_blank">
  <img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Bir Kahve Ismarla" height="100" >
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
        <sub><b>Vic Shóstak</b></sub>
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

## ⭐️ Yıldızlar

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Zamana göre yıldız sayısı" style="max-width:100%;"></a>

## ⚠️ Lisans

`Fiber` [MIT Lisansı](https://github.com/gofiber/fiber/blob/master/LICENSE) kapsamında ücretsiz ve açık kaynak kodlu bir yazılımdır.
