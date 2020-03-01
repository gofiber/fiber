<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="125" src="https://github.com/gofiber/docs/blob/master/static/fiber_v2_logo.svg">
  </a>
  <br>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/gb.svg">
  </a>
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_ru.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/ru.svg">
  </a>-->
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
  <strong>Fiber</strong> — это <strong>веб фреймворк</strong>, который был вдохновлен <a href="https://github.com/expressjs/express">Express</a> и основан на <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, самом быстром HTTP-движке написанном на <a href="https://golang.org/doc/">Go</a>. Фреймворк был разработан с целью <strong>упростить</strong> процесс <strong>быстрой</strong> разработки <strong>высокопроизводительных</strong> веб-приложений с <strong>нулевым распределением памяти</strong>.
</p>

## ⚡️ Быстрый старт

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

## ⚙️ Установка

Прежде всего, [скачайте](https://golang.org/dl/) и установите Go. Версия **1.11** или выше.

Установка выполняется с помощью команды [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them):

```bash
go get -u github.com/gofiber/fiber/...
```

## 🤖 Бенчмарки

Тестирование проводилось с помощью [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) и [Go Web](https://github.com/smallnest/go-web-framework-benchmark). Если вы хотите увидеть все результаты, пожалуйста, посетите наш [Wiki](https://fiber.wiki/benchmarks).

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Особенности

- Надежная [маршрутизация](https://fiber.wiki/routing)
- Доступ к [статичным файлам](https://fiber.wiki/application#static)
- Экстремальная [производительность](https://fiber.wiki/benchmarks)
- [Низкий объем потребления памяти](https://fiber.wiki/benchmarks)
- [Эндпоинты](https://fiber.wiki/context), как в [API](https://fiber.wiki/context) Express
- Middleware и поддержка [Next](https://fiber.wiki/context#next)
- [Быстрое](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) программирование на стороне сервера
- Переведен на 9 других языков
- И многое другое, [посетите наш Wiki](https://fiber.wiki/)

## 💡 Философия

Новые Go-программисты, которые переключаются с [Node.js](https://nodejs.org/en/about/) на [Go](https://golang.org/doc/), имеют дело с очень извилистой кривой обучения, прежде чем они смогут начать создавать свои веб-приложения или микросервисы. Fiber, как **веб-фреймворк**, был создан с идеей **минимализма** и следовал **принципу UNIX**, так что новички смогут быстро войти в мир Go без особых проблем.

Fiber **вдохновлен** Express, самым популярным веб фреймворком в Интернете. Мы объединили **простоту** Express и **чистую производительность** Go. Если вы когда-либо реализовывали веб-приложение на Node.js (*с использованием Express или аналогичного фреймворка*), то многие методы и принципы покажутся вам **очень знакомыми**.

Мы **прислушиваемся** к нашим пользователям в [issues](https://github.com/gofiber/fiber/issues) (_и остальном Интернете_), чтобы создать **быстрый**, **гибкий** и **дружелюбный** веб фреймворк на Go для **любых** задач, **дедлайнов** и **уровней** разработчиков! Как это делает Express в мире JavaScript.

## 👀 Примеры

Ниже перечислены некоторые из распространенных примеров. Если вы хотите увидеть больше примеров кода, пожалуйста, посетите наш [репозиторий рецептов](https://github.com/gofiber/recipes) или [документацию по API](https://fiber.wiki).

### Роутинг

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

### Обслуживание статичных файлов

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

### Middleware и функция Next

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
  <summary>📚 Показать больше примеров кода</summary>

### Работа с шаблонами

Поддерживаемые движки шаблонов:

- [html](https://golang.org/pkg/html/template/)
- [amber](https://github.com/eknkc/amber)
- [handlebars](https://github.com/aymerick/raymond)
- [mustache](https://github.com/cbroglie/mustache)
- [pug](https://github.com/Joker/jade)

```go
func main() {
  // Вы можете настроить нужный движок для шаблонов 
  // перед инициализацией приложения:
  app := fiber.New(&fiber.Settings{
    TemplateEngine:    "mustache",
    TemplateFolder:    "./views",
    TemplateExtension: ".tmpl",
  })

  // ИЛИ уже после инициализации приложения,
  // в любом удобном месте:
  app.Settings.TemplateEngine = "mustache"
  app.Settings.TemplateFolder = "./views"
  app.Settings.TemplateExtension = ".tmpl"

  // Теперь, вы сможете вызывать шаблон `./views/home.tmpl` вот так:
  app.Get("/", func(c *fiber.Ctx) {
    c.Render("home", fiber.Map{
      "title": "Homepage",
      "year":  1999,
    })
  })
  
  // ...
}
```

### Группировка роутов в цепочки

```go
func main() {
  app := fiber.New()
  
  // Корневой API роут
  api := app.Group("/api", cors())  // /api
  
  // Роуты для API v1
  v1 := api.Group("/v1", mysql())   // /api/v1
  v1.Get("/list", handler)          // /api/v1/list
  v1.Get("/user", handler)          // /api/v1/user
  
  // Роуты для API v2
  v2 := api.Group("/v2", mongodb()) // /api/v2
  v2.Get("/list", handler)          // /api/v2/list
  v2.Get("/user", handler)          // /api/v2/user
  
  // ...
}
```

### Cross-Origin Resource Sharing (CORS)

[CORS](https://developer.mozilla.org/ru/docs/Web/HTTP/CORS) — это механизм, использующий дополнительные HTTP-заголовки, чтобы дать возможность агенту пользователя получать разрешения на доступ к выбранным ресурсам с сервера на источнике (домене), отличном от того, что сайт использует в данный момент.

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/fiber/middleware" // все middleware объединены в отдельный Go пакет
)

func main() {
    app := fiber.New()

    // Подключаем CORS для каждого роута в качестве middleware
    app.Use(middleware.CORS())

    app.Get("/", func(c *fiber.Ctx) {
        c.Send("CORS is enabled!")
    })

    app.Listen(3000)
}
```

Проверьте работу CORS, передав любой домен в заголовке `Origin`: 

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000`
```

### Обработка 404 ошибки

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

### Ответ в формате JSON

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

### Восстановление работы после `panic`

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

## 💬 Медиа

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) (_by [Vic Shóstak](https://github.com/koddr), 03 Feb 2020_)
- [Fiber release v1.7 is out now! 🎉 What's new and is he still fast, flexible and friendly?](https://dev.to/koddr/fiber-v2-is-out-now-what-s-new-and-is-he-still-fast-flexible-and-friendly-3ipf) (_by [Vic Shóstak](https://github.com/koddr), 21 Feb 2020_)

## 👍 Помощь проекту

Если вы хотите сказать **спасибо** и/или поддержать активное развитие `Fiber`:

1. Добавьте [GitHub Star](https://github.com/gofiber/fiber/stargazers) в проект.
2. Напишите о проекте [в вашем Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Сделайте обзор фреймворка на [Medium](https://medium.com/), [Dev.to](https://dev.to/) или в личном блоге.
4. Помогите нам перевести `README` и [API](https://fiber.wiki/) на другой язык.

## ☕ Те, кто уже поддержал проект

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
  </tr>
</table>

## ⭐️ Звезды

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ Лицензия

`Fiber` — это бесплатное программное обеспечение с открытым исходным кодом, лицензированное по [лицензии MIT](https://github.com/gofiber/fiber/blob/master/LICENSE).
