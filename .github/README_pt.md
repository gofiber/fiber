<p align="center">
  <a href="https://fiber.wiki">
    <img alt="Fiber" height="100" src="https://github.com/gofiber/docs/blob/master/static/logo.svg">
  </a>
  <br><br>
  <a href="https://github.com/gofiber/fiber/blob/master/README.md">
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
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_de.md">
    <img height="20px" src="https://cdnjs.cloudflare.com/ajax/libs/flag-icon-css/3.4.6/flags/4x3/de.svg">
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
<strong>Fiber</strong> é uma <a href="https://github.com/expressjs/express">estrutura da</a> <strong>Web</strong> inspirada no <a href="https://github.com/valyala/fasthttp">Expressjs</a> , construída sobre o <a href="https://github.com/valyala/fasthttp">Fasthttp</a> , o mecanismo HTTP <strong>mais rápido</strong> do <a href="https://golang.org/doc/">Go</a> . Projetado para <strong>facilitar</strong> o desenvolvimento <strong>rápido</strong> , com <strong>zero de alocação de memória</strong> e <strong>desempenho</strong> em mente.
</p>

## ⚡️ Início rápido

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

## ⚙️ Instalação

Primeiro de tudo, faça o [download](https://golang.org/dl/) e instale o Go. `1.11` ou superior é necessário.

A instalação é feita usando o comando [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) :

```bash
go get github.com/gofiber/fiber
```

## 🤖 Benchmarks

Esses testes são realizados pelo [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) e [Go Web](https://github.com/smallnest/go-web-framework-benchmark) . Se você quiser ver todos os resultados, visite nosso [Wiki](https://fiber.wiki/benchmarks) .

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 Recursos

- [Roteamento](https://fiber.wiki/routing) robusto
- Servir [arquivos estáticos](https://fiber.wiki/application#static)
- [Desempenho](https://fiber.wiki/benchmarks) extremo
- [Baixo](https://fiber.wiki/benchmarks) consumo de [memória](https://fiber.wiki/benchmarks)
- [Pontos de extremidade da API](https://fiber.wiki/context) Express
- Suporte para Middleware e [Next](https://fiber.wiki/context#next)
- Programação [rápida](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) do lado do servidor
- E muito mais, [explore o Fiber](https://fiber.wiki/)

## 💡 Filosofia

Os novos esquilos que mudam do [Node.js](https://nodejs.org/en/about/) para o [Go](https://golang.org/doc/) estão lidando com uma curva de aprendizado antes que possam começar a criar seus aplicativos da web ou microsserviços. O Fiber, como uma **estrutura da Web** , foi criado com a ideia de **minimalismo** e segue o **caminho UNIX** , para que novos esquilos possam entrar rapidamente no mundo do Go com uma recepção calorosa e confiável.

O Fiber é **inspirado** no Expressjs, a estrutura da web mais popular da Internet. Combinamos a **facilidade** do Express e **o desempenho bruto** do Go. Se você já implementou um aplicativo Web no Node.js. ( *usando Express.js ou similar* ), muitos métodos e princípios parecerão **muito comuns** para você.

## 👀 Exemplos

Listados abaixo estão alguns exemplos comuns. Se você quiser ver mais exemplos de código, visite nosso [repositório de receitas](https://github.com/gofiber/recipes) ou nossa [documentação da API](https://fiber.wiki) .

### Arquivos estáticos

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

### Encaminhamento

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

### 404 Manuseio

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

### Resposta JSON

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

## 💬 Mídia

- [Bem-vindo ao Fiber - uma estrutura da Web com estilo Express.js, escrita em Ir com ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) *por [Vic Shóstak](https://github.com/koddr) , 03 fev 2020*

## 👍 Contribuir

Se você quer **agradecer** e / ou apoiar o desenvolvimento ativo da `fiber` :

1. Adicione uma [estrela do GitHub](https://github.com/gofiber/fiber/stargazers) ao projeto.
2. Tweet sobre o projeto [no seu Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) .
3. Escreva uma crítica ou tutorial sobre [Medium](https://medium.com/) , [Dev.to](https://dev.to/) ou blog pessoal.
4. Ajude-nos a traduzir esses documentos `README` - `README` e [API](https://fiber.wiki/) para outro idioma.

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" style="height: 35px !important;"></a>

### ⭐️ Estrelas

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## ⚠️ Licença

`Fiber` é um software gratuito e de código aberto licenciado sob a [Licença MIT](https://github.com/gofiber/fiber/master/LICENSE) .
