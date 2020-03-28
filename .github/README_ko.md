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
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ko.svg">
  </a>-->
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
  <b>Fiber</b>는 <a href="https://github.com/expressjs/express">Express</a>에서 영감을 받고, <a href="https://golang.org/doc/">Go</a>를 위한 <b>가장 빠른</b> HTTP 엔진인 <a ref="https://github.com/valyala/fasthttp">Fasthttp</a>를 토대로 만들어진 <b>웹 프레임워크</b> 입니다. <b>비 메모리 할당</b>과 <b>성능</b>을 고려한 <b>빠른</b> 개발을 위해 <b>손쉽게</b> 사용되도록 설계되었습니다.
</p>

## ⚡️ 빠른 시작

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

## ⚙️ 설치

우선, Go를 [다운로드](https://golang.org/dl/)하고 설치합니다. `1.11` 버전 이상이 요구됩니다.

[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) 명령어를 이용해 설치가 완료됩니다:

```bash
go get -u github.com/gofiber/fiber/...
```

## 🤖 벤치마크

이 테스트들은 [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks)와 [Go Web](https://github.com/smallnest/go-web-framework-benchmark)을 통해 측정되었습니다. 만약 모든 결과를 보고 싶다면, [Wiki](https://fiber.wiki/benchmarks)를 확인해 주세요.

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 특징

- 견고한 [라우팅](https://fiber.wiki/routing)
- [정적 파일](https://fiber.wiki/application#static) 제공
- 뛰어난 [성능](https://fiber.wiki/benchmarks)
- [적은 메모리](https://fiber.wiki/benchmarks) 공간
- [API 엔드포인트](https://fiber.wiki/context)
- 미들웨어 & [Next](https://fiber.wiki/context#next) 지원
- [빠른](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) 서버 사이드 프로그래밍
- [Template engines](https://fiber.wiki/middleware#template)
- [WebSocket support](https://fiber.wiki/middleware#websocket)
- [Rate Limiter](https://fiber.wiki/middleware#limiter)
- Available in [10 languages](https://fiber.wiki/)
- 더 알고 싶다면, [Fiber 둘러보기](https://fiber.wiki/)

## 💡 철학

[Node.js](https://nodejs.org/en/about/)에서 [Go](https://golang.org/doc/)로 전환하는 새로운 고퍼분들은 웹 어플리케이션이나 마이크로 서비스 개발을 시작할 수 있게 되기 전에 학습 곡선에 시달리고 있습니다. Fiber는 **웹 프레임워크**로서, 새로운 고퍼분들이 따뜻하고 믿음직한 환영을 가지고 빠르게 Go의 세상에 진입할 수 있게 **미니멀리즘**의 개념과 **UNIX 방식**에 따라 개발되었습니다.

Fiber는 인터넷에서 가장 인기있는 웹 프레임워크인 Express에서 **영감을 받았습니다.** 우리는 Express의 **쉬운** 사용과 Go의 **성능**을 결합하였습니다. 만약 당신이 Node.js (Express 또는 비슷한 것을 사용하여) 로 웹 어플리케이션을 개발한 경험이 있다면, 많은 메소드들과 원리들이 **매우 비슷하게** 느껴질 것 입니다.

우리는 **어떤한** 작업, **마감일정**, 개발자의 **기술**이던간에 **빠르고**, **유연하고**, **익숙한** Go 웹 프레임워크를 만들기 위해 사용자들의 [이슈들](https://github.com/gofiber/fiber/issues)을(그리고 모든 인터넷을 통해) **듣고 있습니다**! Express가 자바스크립트 세계에서 하는 것 처럼요.

## 👀 예제

다음은 일반적인 예제들 입니다. 

> 더 많은 코드 예제를 보고 싶다면, [Recipes 저장소](https://github.com/gofiber/recipes) 또는 [API 문서](https://fiber.wiki)를 방문하세요.

### Routing

📖 https://fiber.wiki/#basic-routing  


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

### Serve static files

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

  // GET /api/register
  app.Get("/api/list", func(c *fiber.Ctx) {
    fmt.Println("Last middleware")
    c.Send("Hello, World!")
  })

  app.Listen(3000)
}
```

<details>
  <summary>📚 Show more code examples</summary>

### Template engines

📖 https://fiber.wiki/application#settings  
📖 https://fiber.wiki/context#render  
📖 https://fiber.wiki/middleware#template  

Fiber supports the default [Go template engine](https://golang.org/pkg/html/template/)

But if you want to use another template engine like [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) or [pug](https://github.com/Joker/jade).

You can use our [Template Middleware](https://fiber.wiki/middleware#template).

```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/template"
)

func main() {
  // You can setup template engine before initiation app:
  app := fiber.New(&fiber.Settings{
    TemplateEngine:    template.Mustache(),
    TemplateFolder:    "./views",
    TemplateExtension: ".tmpl",
  })

  // OR after initiation app at any convenient location:
  app.Settings.TemplateEngine = template.Mustache()
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

📖 https://fiber.wiki/application#group  

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

📖 https://fiber.wiki/middleware#logger  

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/logger"
)

func main() {
    app := fiber.New()

    // Optional logger config
    config := logger.Config{
      Format:     "${time} - ${method} ${path}\n",
      TimeFormat: "Mon, 2 Jan 2006 15:04:05 MST",
    }

    // Logger with config
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

📖 https://fiber.wiki/application#http-methods  

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

📖 https://fiber.wiki/context#json  

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

## 💬 미디어

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) _by [Vic Shóstak](https://github.com/koddr), 03 Feb 2020_
- [Fiber release v1.7 is out now! 🎉 What's new and is he still fast, flexible and friendly?](https://dev.to/koddr/fiber-v2-is-out-now-what-s-new-and-is-he-still-fast-flexible-and-friendly-3ipf) (_by [Vic Shóstak](https://github.com/koddr), 21 Feb 2020_)
- [🚀 Fiber v1.8. What's new, updated and re-thinked?](https://dev.to/koddr/fiber-v1-8-what-s-new-updated-and-re-thinked-339h) (_by [Vic Shóstak](https://github.com/koddr), 03 Mar 2020_)

## 👍 기여

`Fiber`의 활발한 개발을 지원하고 감사 인사를 하고 싶다면:

1. 프로젝트에 [GitHub Star](https://github.com/gofiber/fiber/stargazers)를 추가하세요.
2. [트위터에서](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) 프로젝트에 대해 트윗하세요.
3. [Medium](https://medium.com/), [Dev.to](https://dev.to/) 또는 개인 블로그에 리뷰 또는 튜토리얼을 작성하세요.
4. 이 `README`를 다른 언어로 번역하는 것을 도와주세요.

## ☕ Coffee Supporters

<table>
  <tr>
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

<a href="https://www.buymeacoffee.com/fenny" target="_blank">
  <img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" height="100" >
</a>

## ⭐️ Stars

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ 라이센스

`Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE) Copyright (c) 2019-present [Fenny](https://github.com/fenny) and [Fiber Contributors](https://github.com/gofiber/fiber/graphs/contributors). Official logo was created by [Vic Shóstak](https://github.com/koddr) and distributed under [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) license (CC BY-SA 4.0 International).

<br>

[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/0)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/0)
[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/1)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/1)
[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/2)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/2)
[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/3)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/3)
[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/4)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/4)
[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/5)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/5)
[![](https://sourcerer.io/fame/Fenny/gofiber/fiber/images/6)](https://sourcerer.io/fame/Fenny/gofiber/fiber/links/6)