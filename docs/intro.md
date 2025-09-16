---
slug: /
id: welcome
title: 👋 Welcome
sidebar_position: 1
---
Welcome to Fiber's online API documentation, complete with examples to help you start building web applications right away!

**Fiber** is an [Express](https://github.com/expressjs/express)-inspired **web framework** built on top of [Fasthttp](https://github.com/valyala/fasthttp), the **fastest** HTTP engine for [Go](https://go.dev/doc/). It is designed to facilitate rapid development with **zero memory allocations** and a strong focus on **performance**.

These docs cover **Fiber v3**.

Looking to practice Fiber concepts hands-on? Check out our [Learning Resources](./extra/learning-resources) for interactive challenges and tutorials.

### Installation

First, [download](https://go.dev/dl/) and install Go. Version `1.25` or higher is required.

Install Fiber using the [`go get`](https://pkg.go.dev/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) command:

```bash
go get github.com/gofiber/fiber/v3
```

### Zero Allocation

Fiber is optimized for **high performance**, meaning values returned from **fiber.Ctx** are **not** immutable by default and **will** be reused across requests. As a rule of thumb, you should use context values only within the handler and **must not** keep any references. Once you return from the handler, any values obtained from the context will be reused in future requests. Here is an example:

```go
func handler(c fiber.Ctx) error {
    // Variable is only valid within this handler
    result := c.Params("foo")

    // ...
}
```

If you need to persist such values outside the handler, make copies of their **underlying buffer** using the [copy](https://pkg.go.dev/builtin/#copy) builtin. Here is an example of persisting a string:

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

Fiber provides `GetString` and `GetBytes` methods on the app that detach values when `Immutable` is enabled and the data isn't already read-only. If it's disabled, use `utils.CopyString` and `utils.CopyBytes` to allocate only when necessary.

```go
app.Get("/:foo", func(c fiber.Ctx) error {
    // Detach if necessary when Immutable is enabled
    result := c.App().GetString(c.Params("foo"))

    // ...
})
```

Alternatively, you can enable the `Immutable` setting. This makes all values returned from the context immutable, allowing you to persist them anywhere. Note that this comes at the cost of performance.

```go
app := fiber.New(fiber.Config{
    Immutable: true,
})
```

For more information, please refer to [#426](https://github.com/gofiber/fiber/issues/426), [#185](https://github.com/gofiber/fiber/issues/185), and [#3012](https://github.com/gofiber/fiber/issues/3012).

### Hello, World

Here is the simplest **Fiber** application you can create:

```go
package main

import "github.com/gofiber/fiber/v3"

func main() {
    app := fiber.New()

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    app.Listen(":3000")
}
```

```bash
go run server.go
```

Browse to `http://localhost:3000` and you should see `Hello, World!` displayed on the page.

### Basic Routing

Routing determines how an application responds to a client request at a particular endpoint—a combination of path and HTTP request method (`GET`, `PUT`, `POST`, etc.).

Each route can have **multiple handler functions** that are executed when the route is matched.

Route definitions follow the structure below:

```go
// Function signature
app.Method(path string, ...func(fiber.Ctx) error)
```

- `app` is an instance of **Fiber**
- `Method` is an [HTTP request method](./api/app#route-handlers): `GET`, `PUT`, `POST`, etc.
- `path` is a virtual path on the server
- `func(fiber.Ctx) error` is a callback function containing the [Context](./api/ctx) executed when the route is matched

#### Simple Route

```go
// Respond with "Hello, World!" on root path "/"
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Hello, World!")
})
```

#### Parameters

```go
// GET http://localhost:8080/hello%20world

app.Get("/:value", func(c fiber.Ctx) error {
    return c.SendString("value: " + c.Params("value"))
    // => Response: "value: hello world"
})
```

#### Optional Parameter

```go
// GET http://localhost:3000/john

app.Get("/:name?", func(c fiber.Ctx) error {
    if c.Params("name") != "" {
        return c.SendString("Hello " + c.Params("name"))
        // => Response: "Hello john"
    }
    return c.SendString("Where is john?")
    // => Response: "Where is john?"
})
```

#### Wildcards

```go
// GET http://localhost:3000/api/user/john

app.Get("/api/*", func(c fiber.Ctx) error {
    return c.SendString("API path: " + c.Params("*"))
    // => Response: "API path: user/john"
})
```

### Static Files

To serve static files such as **images**, **CSS**, and **JavaScript** files, use the [static middleware](./middleware/static.md).

Use the following code to serve files in a directory named `./public`:

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
    app := fiber.New()

    app.Use("/", static.New("./public"))

    app.Listen(":3000")
}
```

Now, you can access the files in the `./public` directory via your browser:

```bash
http://localhost:3000/hello.html
http://localhost:3000/js/jquery.js
http://localhost:3000/css/style.css
```
