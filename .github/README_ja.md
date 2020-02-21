<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="125" src="https://github.com/gofiber/docs/blob/master/static/fiber_v2_logo.svg">
  </a>
  <br>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/gb.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ru.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/ru.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_es.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/es.svg">
  </a>
  <!--<a href="https://github.com/gofiber/fiber/blob/master/.github/README_ja.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/jp.svg">
  </a>-->
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
<strong>FIber</strong>は、<a href="https://github.com/expressjs/express">Express</a>に触発された<strong>Webフレームワーク</strong>です。<a href="https://golang.org/doc/">Go</a><strong> 最速</strong>のHTTPエンジンである<a href="https://github.com/valyala/fasthttp">Fasthttp</a>で作られています。<strong>ゼロメモリアロケーション</strong>と<strong>パフォーマンス</strong>を念頭に置いて設計されており、<strong>迅速</strong>な開発をサポートします。

</p>

## ⚡️ クイックスタート

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

## ⚙️ インストール

まず、Goを[ダウンロード](https://golang.org/dl/)してください。 `1.11`以降が必要です。

そして、[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them)コマンドを使用してインストールしてください。

```bash
go get github.com/gofiber/fiber
```

## 🤖 ベンチマーク

これらのテストは[TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks)および[Go Web](https://github.com/smallnest/go-web-framework-benchmark)によって計測を行っています 。すべての結果を表示するには、 [Wiki](https://fiber.wiki/benchmarks)にアクセスしてください。

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 機能

- 堅牢な[ルーティング](https://fiber.wiki/routing)
- [静的ファイル](https://fiber.wiki/application#static)のサポート
- 究極の[パフォーマンス](https://fiber.wiki/benchmarks)
- [低メモリ](https://fiber.wiki/benchmarks)フットプリント
- Express [APIエンドポイント](https://fiber.wiki/context)
- Middlewareと[Next](https://fiber.wiki/context#next)のサポート
- [迅速](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497)なサーバーサイドプログラミング
- [５ヶ国語](https://fiber.wiki/)に対応
- [Fiber](https://fiber.wiki/)をもっと知る

## 💡 哲学

[Node.js](https://nodejs.org/en/about/)から[Go](https://golang.org/doc/) に乗り換えようとしている新しいGopherはWebフレームワークやマイクロサービスの構築を始める前に多くを学ばなければなりません。
しかし、 **Webフレームワーク**であるFiberは**ミニマリズム**と**UNIX哲学**をもとに作られているため、新しいGopherはスムーズにGoの世界に入ることができます。

Fiberは人気の高いWebフレームワークであるExpressjsに**インスパイア**されています。
わたしたちは Expressの**手軽さ**とGoの**パフォーマンス**を組み合わせました。
もしも、WebフレームワークをExpress等のNode.jsフレームワークで実装した経験があれば、多くの方法や原理がとても**馴染み深い**でしょう。

## 👀 例

以下に一般的な例をいくつか示します。他のコード例をご覧になりたい場合は、 [Recipesリポジトリ](https://github.com/gofiber/recipes)または[APIドキュメント](https://fiber.wiki)にアクセスしてください。

### Serve static files

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

### Middleware & Next

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
  <summary>📜 Show more code examples</summary>

### Custom 404 response

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

### JSON Response

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


### Recover from panic

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
## 💬 メディア

- [ファイバーへようこそ— Go with❤️で記述されたExpress.jsスタイルのWebフレームワーク](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) *[ヴィック・ショースタク](https://github.com/koddr) 、2020年2月3日*

## 👍 貢献する

`Fiber`に開発支援してくださるなら:

1. [GitHub Star](https://github.com/gofiber/fiber/stargazers)をつけてください 。
2. [あなたのTwitterで](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber)プロジェクトについてツイートしてください。
3. [Medium](https://medium.com/) 、 [Dev.to、](https://dev.to/)または個人のブログでレビューまたはチュートリアルを書いてください。
4. この`README`と[APIドキュメント](https://fiber.wiki/)を別の言語に翻訳するためにご協力ください。

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" height="100" ></a>

## ☕ Supporters

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
        <sub><b>koddr</b></sub>
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

## ⭐️ スター

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ ライセンス

`Fiber`は、 [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE)に基づいてライセンスされた無料のオープンソースソフトウェアです。
