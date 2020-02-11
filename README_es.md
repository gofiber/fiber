<img alt="Fiber" src="https://i.imgur.com/Nwvx4cu.png" width="250"><a href="https://github.com/gofiber/fiber/blob/master/.github/README_RU.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ru.svg" alt="ru"></a> <a href="https://github.com/gofiber/fiber/blob/master/.github/README_CH.md"><img width="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ch.svg" alt="ch"></a>

[![](https://img.shields.io/github/release/gofiber/fiber?style=flat-square)](https://github.com/gofiber/fiber/releases) [![](https://img.shields.io/badge/api-documentation-blue?style=flat-square)](https://fiber.wiki) ![](https://img.shields.io/badge/goreport-A%2B-brightgreen?style=flat-square) [![](https://img.shields.io/badge/coverage-91%25-brightgreen?style=flat-square)](https://gocover.io/github.com/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=linux&style=flat-square)](https://travis-ci.org/gofiber/fiber) [![](https://img.shields.io/travis/gofiber/fiber/master.svg?label=windows&style=flat-square)](https://travis-ci.org/gofiber/fiber)

**Fiber** es un **framework web** inspirado en [Expressjs](https://github.com/expressjs/express) construido sobre [Fasthttp](https://github.com/valyala/fasthttp) , el motor HTTP **más rápido** para [Go](https://golang.org/doc/) . Diseñado para **facilitar las** cosas para **un** desarrollo **rápido** con **cero asignación de memoria** y **rendimiento** en mente.

## ⚡️ Inicio rápido

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

## ⚙️ Instalación

En primer lugar, [descargue](https://golang.org/dl/) e instale Go. Se requiere `1.11` o superior.

La instalación se realiza con el comando [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) :

```bash
go get github.com/gofiber/fiber
```

## 🤖 Puntos de referencia

Estas pruebas son realizadas por [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) y [Go Web](https://github.com/smallnest/go-web-framework-benchmark) . Si desea ver todos los resultados, visite nuestro [Wiki](https://fiber.wiki/benchmarks) .

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/static/benchmarks/benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/static/benchmarks/benchmark_alloc.png" width="49%">
</p>

## 🎯 Características

- [Enrutamiento](https://fiber.wiki/routing) robusto
- Servir [archivos estáticos](https://fiber.wiki/application#static)
- [Rendimiento](https://fiber.wiki/benchmarks) extremo
- [Poca](https://fiber.wiki/benchmarks) huella de [memoria](https://fiber.wiki/benchmarks)
- [Puntos finales de API](https://fiber.wiki/context) Express
- Middleware y [próximo](https://fiber.wiki/context#next) soporte
- Programación [rápida](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) del lado del servidor
- Y mucho más, [explore Fiber](https://fiber.wiki/)

## 💡 Filosofía

Los nuevos gophers que hacen el cambio de [Node.js](https://nodejs.org/en/about/) a [Go](https://golang.org/doc/) están lidiando con una curva de aprendizaje antes de que puedan comenzar a construir sus aplicaciones web o microservicios. Fiber, como un **marco web** , fue creado con la idea del **minimalismo** y sigue el **camino de UNIX** , para que los nuevos gophers puedan ingresar rápidamente al mundo de Go con una cálida y confiable bienvenida.

Fiber está **inspirado** en Expressjs, el framework web más popular en Internet. Combinamos la **facilidad** de Express y **el rendimiento bruto** de Go. Si alguna vez ha implementado una aplicación web en Node.js ( *utilizando Express.js o similar* ), muchos métodos y principios le parecerán **muy comunes** .

## 👀 Ejemplos

A continuación se enumeran algunos de los ejemplos comunes. Si desea ver más ejemplos de código, visite nuestro [repositorio de Recetas](https://github.com/gofiber/recipes) o nuestra [documentación de API](https://fiber.wiki) .

### Archivos estáticos

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

### Enrutamiento

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

### Manejo 404

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

### Respuesta JSON

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

## 💬 Medios

- [Bienvenido a Fiber: un marco web con estilo Express.js escrito en Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) *por [Vic Shóstak](https://github.com/koddr) , 03 feb 2020*

## 👍 Contribuir

Si quiere **agradecer** y / o apoyar el desarrollo activo de la `fiber` :

1. Agregue una [estrella de GitHub](https://github.com/gofiber/fiber/stargazers) al proyecto.
2. Tuitea sobre el proyecto [en tu Twitter](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber) .
3. Escriba una reseña o tutorial en [Medium](https://medium.com/) , [Dev.to](https://dev.to/) o blog personal.
4. Ayúdanos a traducir este `README` y [API Docs](https://fiber.wiki/) a otro idioma.

<a href="https://www.buymeacoffee.com/fenny" target="_blank"><img src="https://github.com/gofiber/docs/blob/master/static/buy-morning-coffee-3x.gif" alt="Buy Me A Coffee" style="height: 35px !important;"></a>

### ⭐️ estrellas

<a href="https://starchart.cc/gofiber/fiber" rel="nofollow"><img src="https://starchart.cc/gofiber/fiber.svg" alt="Stars over time" style="max-width:100%;"></a>

## License️ Licencia

`Fiber` es un software gratuito y de código abierto licenciado bajo la [Licencia MIT](https://github.com/gofiber/fiber/master/LICENSE) .
