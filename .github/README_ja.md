<img alt="Fiber" src="https://i.imgur.com/Nwvx4cu.png" width="250"><a href="https://github.com/gofiber/fiber/blob/master/.github/README_RU.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ru.svg" alt="ru"></a> <a href="https://github.com/gofiber/fiber/blob/master/.github/README_CH.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ch.svg" alt="ch"></a>

[![](https://img.shields.io/github/release/gofiber/fiber?style=flat-square)](https://github.com/gofiber/fiber/releases) [![](https://img.shields.io/badge/api-documentation-blue?style=flat-square)](https://fiber.wiki) ![](https://img.shields.io/badge/goreport-A%2B-brightgreen?style=flat-square) [![](https://img.shields.io/badge/coverage-91%25-brightgreen?style=flat-square)](https://gocover.io/github.com/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=linux&style=flat-square)](https://travis-ci.org/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=windows&style=flat-square)](https://travis-ci.org/gofiber/fiber)

**Fiber**は、 [Go](https://golang.org/doc/)用の**最速の** HTTPエンジンである[Fasthttpの](https://github.com/valyala/fasthttp)上に構築された[Expressjsに](https://github.com/expressjs/express)ヒントを得た**Webフレームワーク**です。 **ゼロのメモリ割り当て**と**パフォーマンス**を念頭に置いて、開発を**迅速**に**行える**ように設計されてい**ます** 。

## ⚡️クイックスタート

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

## Installation️インストール

まず、Goを[ダウンロード](https://golang.org/dl/)してインストールします。 `1.11`以降が必要です。

インストールは[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them)コマンドを使用して行われ[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) 。

```bash
go get github.com/gofiber/fiber
```

## 🤖ベンチマーク

これらのテストは[TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks)および[Go Web](https://github.com/smallnest/go-web-framework-benchmark)によって実行され[ます](https://github.com/smallnest/go-web-framework-benchmark) 。すべての結果を表示するには、 [Wikiに](https://fiber.wiki/benchmarks)アクセスしてください。

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/static/benchmarks/benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/static/benchmarks/benchmark_alloc.png" width="49%">
</p>

## 🎯機能

- 堅牢な[ルーティング](https://fiber.wiki/routing)
- [静的ファイルを提供する](https://fiber.wiki/application#static)
- 究極の[パフォーマンス](https://fiber.wiki/benchmarks)
- [低メモリ](https://fiber.wiki/benchmarks)フットプリント
- Express [APIエンドポイント](https://fiber.wiki/context)
- ミドルウェアと[次の](https://fiber.wiki/context#next)サポート
- [迅速な](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497)サーバー側プログラミング
- さらに、 [Fiberを探索する](https://fiber.wiki/)

## 💡哲学

[Node.js](https://nodejs.org/en/about/)から[Go](https://golang.org/doc/)への切り替えを行う新しいgopherは、Webアプリケーションまたはマイクロサービスの構築を開始する前に、学習曲線に対処しています。 **Webフレームワーク**としてのFiberは、 **ミニマリズム**と**UNIXの方法**に基づいて作成されたため、新しいgopherがGoの世界にすばやく入ることができます。

Fiberは、インターネットで最も人気のあるWebフレームワークであるExpressjsに**触発さ**れています。 Expressの**使いやすさ**とGoの**生のパフォーマンス**を組み合わせました。 （ *Express.jsなどを使用*して）Node.jsにWebアプリケーションを実装したことがある場合、多くの方法と原則が**非常に一般的**です。

## 👀例

以下に一般的な例をいくつか示します。他のコード例をご覧になりたい場合は、 [Recipesリポジトリ](https://github.com/gofiber/recipes)または[APIドキュメントを](https://fiber.wiki)ご覧ください。

### 静的ファイル

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

### ルーティング

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

### ミドルウェア

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

### 404ハンドリング

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

### JSONレスポンス

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

## 💬メディア

- [ファイバーへようこそ— Go with❤️で記述されたExpress.jsスタイルのWebフレームワーク](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) *[ヴィック・ショースタク](https://github.com/koddr) 、2020年2月3日*

## 貢献する

**ありがとう、**および/または`fiber`積極的な開発をサポートしたい場合：

1. [GitHub Star](https://github.com/gofiber/fiber/stargazers)をプロジェクトに追加し[ます](https://github.com/gofiber/fiber/stargazers) 。
2. [あなたのTwitterで](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber)プロジェクトについてツイート[してください](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) 。
3. [Medium](https://medium.com/) 、 [Dev.to、](https://dev.to/)または個人のブログでレビューまたはチュートリアルを書いてください。
4. この`README`と[APIドキュメント](https://fiber.wiki/)を別の言語に翻訳するためにご協力ください。

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" style="height: 35px !important;"></a>

### ⭐️スター

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## License️ライセンス

`Fiber`は、 [MIT Licenseに](https://github.com/gofiber/fiber/master/LICENSE)基づいてライセンスされた無料のオープンソースソフトウェアです。
