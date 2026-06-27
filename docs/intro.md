---
slug: /
id: welcome
title: 👋 Welcome
sidebar_position: 1
---

Welcome to Fiber's online API documentation, complete with examples to help you start building web applications right away!

**Fiber** is an [Express](https://github.com/expressjs/express)-inspired **web framework** built on top of [Fasthttp](https://github.com/valyala/fasthttp), the **fastest** HTTP engine for [Go](https://go.dev/doc/). It is designed to facilitate rapid development with **zero memory allocations** and a strong focus on **performance**. Fiber also ships batteries included: built-in middleware, officially maintained integrations, storage drivers, and template engines cover most production needs (see [Explore the Ecosystem](#explore-the-ecosystem) below).

These docs cover **Fiber v3**.

:::tip
Coming from Fiber v2? See [What's New in v3](./whats_new.md) for the migration guide and the CLI migration tool.
:::

## Installation

First, [download](https://go.dev/dl/) and install Go. Version `1.25` or higher is required.

Install Fiber using the [`go get`](https://pkg.go.dev/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) command:

```bash
go get github.com/gofiber/fiber/v3
```

## Hello, World

Create a file named `server.go` with the simplest **Fiber** application you can write:

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    log.Fatal(app.Listen(":3000"))
}
```

Run it:

```bash
go run server.go
```

Browse to `http://localhost:3000` and you should see `Hello, World!` displayed on the page.

## Basic Routing

Routing determines how an application responds to a client request at a particular endpoint, a combination of path and HTTP request method (`GET`, `PUT`, `POST`, etc.).

Route definitions follow the structure below:

```go
// Function signature
func (app *App) Get(path string, handler any, handlers ...any) Router
```

- `app` is an instance of **Fiber**
- `Get` is an [HTTP request method](./api/app.md#route-handlers); `Post`, `Put`, `Delete`, and the other methods work the same way
- `path` is a virtual path on the server
- `handler` is a function executed when the route is matched; the canonical form is `func(fiber.Ctx) error`, and a route can register multiple handlers

For an interactive breakdown of every part of a route definition, see the [anatomy of a route](./guide/routing.md#anatomy-of-a-route).

A simple route and a route with a parameter:

```go
// Respond with "Hello, World!" on root path "/"
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Hello, World!")
})
```

```go
// GET http://localhost:3000/hello%20world

app.Get("/:value", func(c fiber.Ctx) error {
    return c.SendString("value: " + c.Params("value"))
    // => Response: "value: hello world"
})
```

See the [routing guide](./guide/routing.md) for optional parameters, wildcards, constraints, route groups, and the full list of supported handler types.

## Static Files

To serve static files such as **images**, **CSS**, and **JavaScript** files, register the [static middleware](./middleware/static.md):

```go
import "github.com/gofiber/fiber/v3/middleware/static"

app.Use("/", static.New("./public"))
```

Files in the `./public` directory are now reachable in the browser, for example at `http://localhost:3000/css/style.css`.

## Using Middleware

Middleware runs before or after your handlers and takes care of cross-cutting concerns. Registering one is a single `app.Use` call; here is the [logger](./middleware/logger.md) middleware printing every request:

```go
import "github.com/gofiber/fiber/v3/middleware/logger"

app.Use(logger.New())
```

Middleware for logging, CORS, rate limiting, sessions, compression, panic [recovery](./middleware/recover.md), and much more ships with Fiber itself; the ecosystem section below shows where to find it all.

## Zero Allocation

:::caution
Fiber is optimized for **high performance**, so values returned from **fiber.Ctx** are **not** immutable by default and **will** be reused across requests. Use context values only within the handler, and do not keep any references after the handler returns.
:::

If you need to persist a context value beyond the handler, make a copy of its **underlying buffer** using the [copy](https://pkg.go.dev/builtin/#copy) builtin:

```go
func handler(c fiber.Ctx) error {
    // Variable is only valid within this handler
    result := c.Params("foo")

    // Make a copy
    buffer := make([]byte, len(result))
    copy(buffer, result)
    resultCopy := string(buffer)
    // Variable is now valid indefinitely

    // ...
}
```

Alternatively, you can enable the `Immutable` setting. This makes all values returned from the context immutable, allowing you to persist them anywhere, at the cost of some performance:

```go
app := fiber.New(fiber.Config{
    Immutable: true,
})
```

For details, see [Immutable](./api/fiber.md#immutable) in the configuration reference and the [GetString and GetBytes](./api/app.md#getstring) helpers.

## Explore the Ecosystem

Fiber is more than the core module. When your application grows, these officially maintained building blocks are one import away:

- **Built-in middleware**: 30+ middleware for logging, CORS, security headers, caching, rate limiting, and more live in the core module; browse the [middleware overview](https://docs.gofiber.io/category/-middleware).
- **[Contrib packages](https://docs.gofiber.io/contrib/)**: officially maintained integrations with external dependencies, such as JWT, WebSocket, OpenTelemetry, Casbin, and structured logging adapters.
- **[Storage drivers](https://docs.gofiber.io/storage/)**: a growing list of backends (Redis, Postgres, MongoDB, S3, and more) behind one interface, ready to plug into the session, limiter, cache, and idempotency middleware.
- **[Template engines](https://docs.gofiber.io/template/)**: server-side rendering through the Views interface, with engines like html, django, handlebars, and pug.
- **[HTTP client](./client/rest.md)**: a built-in client, also built on Fasthttp, for calling other services with the same performance philosophy.
- **[Recipes](https://docs.gofiber.io/recipes/)**: runnable example projects (Docker, GORM, JWT auth, clean architecture, and more) to copy a working starting point from.

## Next Steps

Ready to go deeper? These guides and references cover the everyday tasks:

- [Routing](./guide/routing.md): parameters, wildcards, constraints, and route groups
- [Context](./api/ctx.md): reading requests and sending responses
- [Error handling](./guide/error-handling.md): custom error handlers and status codes
- [Request binding](./api/bind.md) and [validation](./guide/validation.md): map request data onto structs safely
- [Templates](./guide/templates.md): render views with your favorite template engine
- [Configuration](./api/fiber.md): every option accepted by `fiber.New`
- [Testing](./api/app.md#test): test handlers without a running server using `app.Test`
- [Learning resources](./extra/learning-resources.md): tutorials and hands-on challenges

## Community and Help

Stuck or have questions? Join the [Discord](https://gofiber.io/discord) server or check the [FAQ](./extra/faq.md). Fiber is developed in the open on [GitHub](https://github.com/gofiber/fiber); issues, discussions, and contributions are welcome.
