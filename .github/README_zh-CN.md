<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="100" src="https://github.com/gofiber/docs/blob/master/static/logo.svg">
  </a>
  <br><br>
  <a href="https://github.com/gofiber/fiber/blob/master/README.md">
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
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_zh-CN.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/cn.svg">
  </a>-->
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
</p>
<p align="center">
  <strong>Fiber</strong>是一个基于<a href="https://github.com/expressjs/express">Express的</a> <strong>Web框架，<strong>建立在<a href="https://github.com/valyala/fasthttp">Fasthttp</a> （ <a href="https://golang.org/doc/">Go</a> <strong>最快的</strong> HTTP引擎）的基础上。皆在</strong>简化</strong> <strong>零内存分配</strong>和<strong>提高性能</strong>，以便<strong>快速</strong>开发。
</p>

## ⚡️ 快速入门

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

## ⚙️ 安装

首先， [下载](https://golang.org/dl/)并安装Go。 `1.11`或更高。

使用[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them)命令完成安装：

```bash
go get github.com/gofiber/fiber
```

## 🤖 性能

这些测试由[TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks)和[Go Web执行](https://github.com/smallnest/go-web-framework-benchmark) 。如果要查看所有结果，请访问我们的[Wiki](https://fiber.wiki/benchmarks) 。

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets/benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets/benchmark_alloc.png" width="49%">
</p>

## 🎯 特点

- 强大的[路由](https://fiber.wiki/routing)
- [静态文件](https://fiber.wiki/application#static)服务
- 极限[表现](https://fiber.wiki/benchmarks)
- [内存占用低](https://fiber.wiki/benchmarks)
- Express [API端点](https://fiber.wiki/context)
- 中间件和[Next](https://fiber.wiki/context#next)支持
- [快速的](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497)服务器端编程
- 以及更多[文档](https://fiber.wiki/)

## 💡 哲学

从[Node.js](https://nodejs.org/en/about/)切换到[Go的](https://golang.org/doc/)新gopher在开始构建Web应用程序或微服务之前正在应对学习过程。 Fiber作为一个**Web框架** ，是按照**极简主义**的思想并遵循**UNIX方式创建的** ，因此新的gopher可以以热烈和可信赖的欢迎**方式**迅速进入Go的世界。

Fiber **受** Internet上最流行的Web框架Expressjs的**启发** 。我们结合了Express的**易用**性和Go的**原始性能** 。如果您曾经在Node.js上实现过Web应用程序（ *使用Express.js或类似工具* ），那么许多方法和原理对您来说似乎**非常易懂** 。

## 👀 例子

下面列出了一些常见示例。如果您想查看更多代码示例，请访问我们的[Recipes存储库](https://github.com/gofiber/recipes)或访问我们的[API文档](https://fiber.wiki) 。

### 静态文件

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

### 路由

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

### 中间件

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

### 404处理

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

### JSON响应

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

## 💬 媒体

- [欢迎使用Fiber —用Go语言编写的Express.js风格的Web框架](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) *作者[维克·肖斯塔克（VicShóstak）](https://github.com/koddr)，2020年2月3日*

## 👍 贡献

如果您要说声**谢谢**或支持`Fiber`的积极发展：

1. 将[GitHub Star](https://github.com/gofiber/fiber/stargazers)添加到项目中。
2. [在Twitter上](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber)发布有关项目[的推文](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) 。
3. 在[Medium](https://medium.com/) ， [Dev.to](https://dev.to/)或个人博客上写评论或教程。
4. 帮助我们将此`README` [文件](https://fiber.wiki/)和[API文档](https://fiber.wiki/)翻译成另一种语言。

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" height="100" ></a>

### ⭐️ 星星

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ 许可证

`Fiber`是根据[MIT许可证许可的](https://github.com/gofiber/fiber/blob/master/LICENSE)免费开源软件。
