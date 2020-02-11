<img alt="Fiber" src="https://i.imgur.com/Nwvx4cu.png"><a href="https://github.com/gofiber/fiber/blob/master/README.md">
  <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/gb.svg">
</a>
<a href="https://github.com/gofiber/fiber/blob/master/.github/README_es.md">
  <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/es.svg">
</a>
<a href="https://github.com/gofiber/fiber/blob/master/.github/README_ru.md">
  <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/ru.svg">
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

[![](https://img.shields.io/github/release/gofiber/fiber?style=flat-square)](https://github.com/gofiber/fiber/releases) [![](https://img.shields.io/badge/api-documentation-blue?style=flat-square)](https://fiber.wiki) ![](https://img.shields.io/badge/goreport-A%2B-brightgreen?style=flat-square) [![](https://img.shields.io/badge/coverage-91%25-brightgreen?style=flat-square)](https://gocover.io/github.com/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=linux&style=flat-square)](https://travis-ci.org/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=windows&style=flat-square)](https://travis-ci.org/gofiber/fiber)

**Fiber** - это вдохновленная [Expressjs](https://github.com/expressjs/express) **веб-инфраструктура,** [созданная](https://github.com/valyala/fasthttp) на основе [Fasthttp](https://github.com/valyala/fasthttp) , самого **быстрого** HTTP-движка для [Go](https://golang.org/doc/) . Разработанный, чтобы **упростить** процесс **быстрой** разработки с **нулевым распределением памяти** и **производительностью** .

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

Прежде всего, [скачайте](https://golang.org/dl/) и установите Go. Требуется `1.11` или выше.

Установка выполняется с помощью команды [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) :

```bash
go get github.com/gofiber/fiber
```

## 🤖 Тесты

Эти тесты выполняются [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) и [Go Web](https://github.com/smallnest/go-web-framework-benchmark) . Если вы хотите увидеть все результаты, пожалуйста, посетите нашу [вики](https://fiber.wiki/benchmarks) .

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Особенности

- Надежная [маршрутизация](https://fiber.wiki/routing)
- Служить [статическим файлам](https://fiber.wiki/application#static)
- Экстремальная [производительность](https://fiber.wiki/benchmarks)
- [Низкий объем памяти](https://fiber.wiki/benchmarks)
- [Конечные точки](https://fiber.wiki/context) Express [API](https://fiber.wiki/context)
- Middleware & [Next](https://fiber.wiki/context#next) support
- [Быстрое](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) программирование на стороне сервера
- И многое другое, [исследовать волокна](https://fiber.wiki/)

## 💡 Философия

Новые суслики, которые переключаются с [Node.js](https://nodejs.org/en/about/) на [Go,](https://golang.org/doc/) имеют дело с кривой обучения, прежде чем они смогут начать создавать свои веб-приложения или микросервисы. Fiber, как **веб-фреймворк** , был создан с идеей **минимализма** и следовал **принципу UNIX** , так что новые суслики могут быстро войти в мир Go с теплым и надежным приемом.

Fiber **вдохновлен** Expressjs, самой популярной веб-инфраструктурой в Интернете. Мы объединили **простоту** Express и **чистую производительность** Go. Если вы когда-либо реализовывали веб-приложение на Node.js ( *с использованием Express.js или аналогичного* ), то многие методы и принципы покажутся вам **очень общими** .

## 👀 Примеры

Ниже перечислены некоторые из распространенных примеров. Если вы хотите увидеть больше примеров кода, пожалуйста, посетите наш [репозиторий рецептов](https://github.com/gofiber/recipes) или посетите нашу [документацию по API](https://fiber.wiki) .

### Статические файлы

```go
func main() {
  app := fiber.New()

  app.Static("./public")
  // => http://localhost:3000/js/script.js
  // => http://localhost:3000/css/style.css

  app.Static("/prefix", "./public")
  // => http://localhost:3000/prefix/js/script.js
  // => http://localhost:3000/prefix/css/style.css

  app.Listen(3000)
}
```

### Маршрутизация

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

### Промежуточное

```go
func main() {
  app := fiber.New()

  // Match any post route
  app.Post(func(c *fiber.Ctx) {
    user, pass, ok := c.BasicAuth()
    if !ok || user != "john" || pass != "doe" {
      c.Status(403).Send("Sorry John")
      return
    }
    c.Next()
  })

  // Match all routes starting with /api
  app.Use("/api", func(c *fiber.Ctx) {
    c.Set("Access-Control-Allow-Origin", "*")
    c.Set("Access-Control-Allow-Headers", "X-Requested-With")
    c.Next()
  })

  // Optional param
  app.Post("/api/register", func(c *fiber.Ctx) {
    username := c.Body("username")
    password := c.Body("password")
    // ..
  })

  app.Listen(3000)
}
```

### 404 Обработка

```go
func main() {
  app := fiber.New()

  // Serve static files from "public" directory
  app.Static("./public")

  // Last middleware
  app.Use(func (c *fiber.Ctx) {
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
  app.Get("/json", func (c *fiber.Ctx) {
    c.JSON(&User{"John", 20})
  })

  app.Listen(3000)
}
```

## 💬 СМИ

- [Добро пожаловать в Fiber - фреймворк в стиле Express.js, написанный на Go с ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) *[Вик Шостак](https://github.com/koddr) , 3 февраля 2020 г.*

## 👍 способствовать

Если вы хотите сказать **спасибо** и / или поддержать активное развитие `fiber` :

1. Добавьте [GitHub Star](https://github.com/gofiber/fiber/stargazers) в проект.
2. Чирикать о проекте [в вашем Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) .
3. Написать обзор или учебник на [Medium](https://medium.com/) , [Dev.to](https://dev.to/) или в личном блоге.
4. Помогите нам перевести эти документы `README` и [API](https://fiber.wiki/) на другой язык.

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" style="height: 35px !important;"></a>

### ⭐️ Звезды

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ Лицензия

`Fiber` - это бесплатное программное обеспечение с открытым исходным кодом, лицензированное по [лицензии MIT](https://github.com/gofiber/fiber/master/LICENSE) .
