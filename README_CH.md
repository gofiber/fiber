# 🔌 Fiber Web Framework <a href="README.md"><img width="20px" src="docs/static/flags/en.svg" alt="en"/></a> <a href="README_RU.md"><img width="20px" src="docs/static/flags/ru.svg" alt="ru"/></a>

[![](https://img.shields.io/github/release/gofiber/fiber)](https://github.com/gofiber/fiber/releases) ![](https://img.shields.io/github/languages/top/gofiber/fiber) [![](https://godoc.org/github.com/gofiber/fiber?status.svg)](https://godoc.org/github.com/gofiber/fiber) ![](https://goreportcard.com/badge/github.com/gofiber/fiber) [![GitHub license](https://img.shields.io/github/license/gofiber/fiber.svg)](https://github.com/gofiber/fiber/blob/master/LICENSE) [![Join the chat at https://gitter.im/gofiber/community](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/gofiber/community)

<img align="right" height="180px" src="docs/static/logo_320px_trans.png" alt="Fiber logo" />

**[Fiber](https://github.com/gofiber/fiber)** 是一个 [Express.js](https://expressjs.com/en/4x/api.html) 运行的样式化HTTP Web框架实现 [Fasthttp](https://github.com/valyala/fasthttp), **最快的** HTTP引擎 Go (Golang). 该软件包使用了**相似的框架约定** Express.

人们从 [Node.js](https://nodejs.org/en/about/) 至 [Go](https://golang.org/doc/) 通常会遇到学习曲线不好的问题，从而开始构建他们的Web应用程序, 这个项目是为了 **缓解** 事情准备 **快速** 发展，但与 **零内存分配** 和 **性能** 心里.

## API Documentation

📚 我们创建了一个扩展我们创建了一个扩展 **API documentation** (_包括例子_), **[点击这里](https://gofiber.github.io/fiber/)**.

## Benchmark

[![](https://gofiber.github.io/fiber/static/benchmarks/benchmark.png)](https://gofiber.github.io/fiber/#/benchmarks)

👉 **[点击这里](https://gofiber.github.io/fiber/#/benchmarks)** 查看所有基准测试结果.

## Features

- 针对速度和低内存使用进行了优化
- 快速的服务器端编程
- 通过参数轻松路由
- 具有自定义前缀的静态文件
- 具有Next支持的中间件
- Express API端点
- [Extended documentation](https://gofiber.github.io/fiber/)

## Installing

假设您已经安装 Go `1.11+` 😉

安装 [Fiber](https://github.com/gofiber/fiber) 通过调用以下命令来打包:

```bash
go get -u github.com/gofiber/fiber
```

## Hello, world!

本质上，下面嵌入是您可以创建的最简单的Fiber应用程序:

```go
// server.go

package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Create new route with GET method
  app.Get("/", func(c *fiber.Ctx) {
    c.Send("Hello, World!")
  })

  // Start server on http://localhost:8080
  app.Listen(8080)
}
```

转到控制台并运行:

```bash
go run server.go
```

现在，浏览至 `http://localhost:8080` 你应该看到 `Hello, World!` 在页面上！ 🎉

## Static files

要提供静态文件，请使用 [Static](https://gofiber.github.io/fiber/#/?id=static-files) 方法:

```go
package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Serve all static files on ./public folder
  app.Static("./public")

  // Start server on http://localhost:8080
  app.Listen(8080)
}
```

现在，您可以加载公共目录中的文件：

```bash
http://localhost:8080/hello.html
http://localhost:8080/js/script.js
http://localhost:8080/css/style.css
```

## Middleware

中间件从未如此简单！就像Express，您致电 `Next()` 匹配路线功能:

```go
package main

import "github.com/gofiber/fiber"

func main() {
  // Create new Fiber instance
  app := fiber.New()

  // Define all used middlewares in Use()

  app.Use(func(c *fiber.Ctx) {
    c.Write("Match anything!\n")
    c.Next()
  })

  app.Use("/api", func(c *fiber.Ctx) {
    c.Write("Match starting with /api\n")
    c.Next()
  })

  app.Get("/api/user", func(c *fiber.Ctx) {
    c.Write("Match exact path /api/user\n")
  })

  // Start server on http://localhost:8080
  app.Listen(8080)
}
```

## Project assistance

如果您要说声谢谢或/并且支持积极的发展 `gofiber/fiber`:

1. 将GitHub Star添加到项目中。
2. 关于项目的推文 [on your Twitter](https://twitter.com/intent/tweet?text=%F0%9F%94%8C%20Fiber%20is%20an%20Express.js%20inspired%20Go%20web%20framework%20build%20on%20%F0%9F%9A%80%20Fasthttp%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. 帮助我们翻译 `README` 和 [API Docs](https://gofiber.github.io/fiber/) 换一种语言.

谢谢你的支持! 😘 我们在一起 `Fiber Web Framework` 每天都好.

## Stargazers over time

[![Stargazers over time](https://starchart.cc/gofiber/fiber.svg)](https://starchart.cc/gofiber/fiber)

## License

⚠️ _请注意:_ `gofiber/fiber` 是根据以下条款获得许可的免费开源软件 [MIT License](https://github.com/gofiber/fiber/edit/master/LICENSE).
