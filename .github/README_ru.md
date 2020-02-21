<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="100" src="https://github.com/gofiber/docs/blob/master/static/logo.svg">
  </a>
  <br><br>
  <a href="https://github.com/gofiber/fiber/blob/master/README.md">
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

Прежде всего, [скачайте](https://golang.org/dl/) и установите Go.

> Go **1.11** (с включенными [модулями Go](https://golang.org/doc/go1.11#modules)) или выше.

Установка выполняется с помощью команды [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) :

```bash
go get -u github.com/gofiber/fiber
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
- [Эндпоинты](https://fiber.wiki/context) Express [API](https://fiber.wiki/context)
- Middleware и поддержка [Next](https://fiber.wiki/context#next)
- [Быстрое](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) программирование на стороне сервера
- И многое другое, [посетите наш Wiki](https://fiber.wiki/)

## 💡 Философия

Новые Go-программисты, которые переключаются с [Node.js](https://nodejs.org/en/about/) на [Go](https://golang.org/doc/), имеют дело с очень извилистой кривой обучения, прежде чем они смогут начать создавать свои веб-приложения или микросервисы. Fiber, как **веб-фреймворк**, был создан с идеей **минимализма** и следовал **принципу UNIX**, так что новички смогут быстро войти в мир Go без особых проблем.

Fiber **вдохновлен** Express, самым популярным веб фреймворком в Интернете. Мы объединили **простоту** Express и **чистую производительность** Go. Если вы когда-либо реализовывали веб-приложение на Node.js (*с использованием Express или аналогичного фреймворка*), то многие методы и принципы покажутся вам **очень знакомыми**.

## 👀 Примеры

Ниже перечислены некоторые из распространенных примеров. Если вы хотите увидеть больше примеров кода, пожалуйста, посетите наш [репозиторий рецептов](https://github.com/gofiber/recipes) или [документацию по API](https://fiber.wiki).

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

### Middleware

```go
func main() {
  app := fiber.New()

  // Принимает POST запрос с любого адреса
  app.Post(func(c *fiber.Ctx) {
    user, pass, ok := c.BasicAuth()
    if !ok || user != "john" || pass != "doe" {
      c.Status(403).Send("Sorry John")
      return
    }
    c.Next()
  })

  // Разрешает все адреса, начинающиеся на "/api"
  app.Use("/api", func(c *fiber.Ctx) {
    c.Set("Access-Control-Allow-Origin", "*")
    c.Set("Access-Control-Allow-Headers", "X-Requested-With")
    c.Next()
  })

  // Опциональный параметр
  app.Post("/api/register", func(c *fiber.Ctx) {
    username := c.Body("username")
    password := c.Body("password")
    // ..
  })

  app.Listen(3000)
}
```

### Обработка ошибки 404

```go
func main() {
  app := fiber.New()

  // Доступ к файлам в директории "./public":
  app.Static("./public")

  // Последний middleware
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

  // Сериализация JSON
  app.Get("/json", func (c *fiber.Ctx) {
    c.JSON(&User{"John", 20})
  })

  app.Listen(3000)
}
```

## 💬 Медиа

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) *[Vic Shóstak](https://github.com/koddr), 3 февраля 2020 г.*

## 👍 Помощь проекту

Если вы хотите сказать **спасибо** и/или поддержать активное развитие `Fiber`:

1. Добавьте [GitHub Star](https://github.com/gofiber/fiber/stargazers) в проект.
2. Напишите о проекте [в вашем Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Сделайте обзор фреймворка на [Medium](https://medium.com/), [Dev.to](https://dev.to/) или в личном блоге.
4. Помогите нам перевести `README` и [API](https://fiber.wiki/) на другой язык.

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" height="100" ></a>

## ☕ Supporters

<table>
  <tr>
    <td align="center">
      <a href="https://www.buymeacoffee.com/fenny">
        <img src="https://img.buymeacoffee.com/api/?name=ToishY&size=300&bg-image=bmc" width="100px;" style="border-radius:50%"></br>
        <b>ToishY</b>
        </a>
      </td>
  </tr>
</table>

## ⭐️ Звезды

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ Лицензия

`Fiber` — это бесплатное программное обеспечение с открытым исходным кодом, лицензированное по [лицензии MIT](https://github.com/gofiber/fiber/blob/master/LICENSE).
