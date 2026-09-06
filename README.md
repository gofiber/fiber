<h1 align="center">
  <a href="https://gofiber.io">
    <picture>
      <source height="125" media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/gofiber/docs/master/static/img/logo-dark.svg">
      <img height="125" alt="Fiber" src="https://raw.githubusercontent.com/gofiber/docs/master/static/img/logo.svg">
    </picture>
  </a>
  <br>
  <a href="https://pkg.go.dev/github.com/gofiber/fiber/v3#pkg-overview">
    <img src="https://img.shields.io/badge/%F0%9F%93%9A%20godoc-pkg-00ACD7.svg?color=00ACD7&style=flat-square">
  </a>
  <a href="https://goreportcard.com/report/github.com/gofiber/fiber/v3">
    <img src="https://img.shields.io/badge/%F0%9F%93%9D%20goreport-A%2B-75C46B?style=flat-square">
  </a>
  <a href="https://codecov.io/gh/gofiber/fiber" >
   <img alt="Codecov" src="https://img.shields.io/codecov/c/github/gofiber/fiber?token=3Cr92CwaPQ&style=flat-square&logo=codecov&label=codecov">
 </a>
  <a href="https://github.com/gofiber/fiber/actions?query=workflow%3ATest">
    <img src="https://img.shields.io/github/actions/workflow/status/gofiber/fiber/test.yml?branch=main&label=%F0%9F%A7%AA%20tests&style=flat-square&color=75C46B">
  </a>
  <a href="https://gofiber.github.io/fiber/benchmarks/">
    <img src="https://img.shields.io/badge/%F0%9F%93%8A%20benchmarks-charts-00ACD7.svg?style=flat-square">
  </a>
    <a href="https://docs.gofiber.io">
    <img src="https://img.shields.io/badge/%F0%9F%92%A1%20fiber-docs-00ACD7.svg?style=flat-square">
  </a>
  <a href="https://gofiber.io/discord">
    <img src="https://img.shields.io/discord/704680098577514527?style=flat-square&label=%F0%9F%92%AC%20discord&color=00ACD7">
  </a>
  <a href="https://github.com/sponsors/gofiber">
    <img src="https://img.shields.io/github/sponsors/gofiber?style=flat-square&label=%E2%AD%90%20sponsors&color=ec6cb9">
  </a>
</h1>

<p align="center">
  <em><b>Fiber</b> is an <a href="https://github.com/expressjs/express">Express</a> inspired <b>web framework</b> built on top of <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, the <b>fastest</b> HTTP engine for <a href="https://go.dev/doc/">Go</a>. Designed to <b>ease</b> things up for <b>fast</b> development with <a href="https://docs.gofiber.io/#zero-allocation"><b>zero memory allocation</b></a> and <b>performance</b> in mind.</em>
</p>

---
<!-- skip-docs -->
<p align="center">
  <a href="https://github.com/sponsors/gofiber">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/OFFICIAL%20SPONSORS.png?variant=outline&color=ffffff&logo=github&logoColor=ffffff">
      <img alt="Official Sponsors" src="https://shieldcn.dev/badge/OFFICIAL%20SPONSORS.png?variant=outline&color=000000&logo=github&logoColor=000000">
    </picture>
  </a>
</p>
<p align="center">
  <a href="https://www.coderabbit.ai/?utm_source=gofiber&utm_medium=sponsor&utm_content=homepage">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://www.coderabbit.ai/images/logo-dark.svg">
      <img width="280" height="52" alt="CodeRabbit" src="https://www.coderabbit.ai/images/logo-orange.svg">
    </picture>
  </a>
</p>
<p align="center">
  <a href="https://blacksmith.sh/?utm_source=gofiber&utm_medium=sponsor&utm_content=readme">
    <img width="280" height="96" alt="Blacksmith" src="https://raw.githubusercontent.com/gofiber/.github/main/assets/sponsors/blacksmith.png">
  </a>
</p>
<p align="center">
  <sub><b>Tool Sponsors</b> - supporting Fiber with free IDE licenses and AI credits</sub>
</p>
<p align="center">
  <a href="https://www.jetbrains.com/?from=gofiber" title="JetBrains - IDE licenses"><img width="36" height="36" alt="JetBrains" src="https://github.com/JetBrains.png?size=72"></a>
  &nbsp;
  <a href="https://openai.com/?utm_source=gofiber&utm_medium=sponsor&utm_content=readme" title="OpenAI - AI credits"><img width="36" height="36" alt="OpenAI" src="https://github.com/openai.png?size=72"></a>
  &nbsp;
  <a href="https://www.anthropic.com/?utm_source=gofiber&utm_medium=sponsor&utm_content=readme" title="Anthropic - AI credits"><img width="36" height="36" alt="Anthropic" src="https://github.com/anthropics.png?size=72"></a>
</p>

---
<!-- skip-docs -->
## ⚙️ Installation

Fiber requires **Go version `1.26` or higher** to run. If you need to install or upgrade Go, visit the [official Go download page](https://go.dev/dl/). To start setting up your project, create a new directory for your project and navigate into it. Then, initialize your project with Go modules by executing the following command in your terminal:

```bash
go mod init github.com/your/repo
```

To learn more about Go modules and how they work, you can check out the [Using Go Modules](https://go.dev/blog/using-go-modules) blog post.

After setting up your project, you can install Fiber with the `go get` command:

```bash
go get -u github.com/gofiber/fiber/v3
```

This command fetches the Fiber package and adds it to your project's dependencies, allowing you to start building your web applications with Fiber.

## ⚡️ Quickstart

Getting started with Fiber is easy. Here's a basic example to create a simple web server that responds with "Hello, World 👋!" on the root path. This example demonstrates initializing a new Fiber app, setting up a route, and starting the server.

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    // Initialize a new Fiber app
    app := fiber.New()

    // Define a route for the GET method on the root path '/'
    app.Get("/", func(c fiber.Ctx) error {
        // Send a string response to the client
        return c.SendString("Hello, World 👋!")
    })

    // Start the server on port 3000
    log.Fatal(app.Listen(":3000"))
}
```

This simple server is easy to set up and run. It introduces the core concepts of Fiber: app initialization, route definition, and starting the server. Just run this Go program, and visit `http://localhost:3000` in your browser to see the message.

## Zero Allocation

Fiber is optimized for **high-performance**, meaning values returned from **fiber.Ctx** are **not** immutable by default and **will** be reused across requests. As a rule of thumb, you **must** only use context values within the handler and **must not** keep any references. Once you return from the handler, any values obtained from the context will be reused in future requests. Visit our [documentation](https://docs.gofiber.io/#zero-allocation) to learn more.

## 🤖 Benchmarks

These tests are performed by [TechEmpower](https://www.techempower.com/benchmarks/#section=data-r19&hw=ph&test=plaintext). If you want to see all the results, please visit our [Wiki](https://docs.gofiber.io/extra/benchmarks).

<p float="left" align="middle">
  <img src="https://raw.githubusercontent.com/gofiber/docs/master/static/img/v3/plaintext.png" width="49%">
  <img src="https://raw.githubusercontent.com/gofiber/docs/master/static/img/v3/json.png" width="49%">
</p>

## 🎯 Features

- Robust [Routing](https://docs.gofiber.io/guide/routing)
- Serve [Static Files](https://docs.gofiber.io/api/app#static)
- Extreme [Performance](https://docs.gofiber.io/extra/benchmarks)
- [Low Memory](https://docs.gofiber.io/extra/benchmarks) footprint
- [API Endpoints](https://docs.gofiber.io/api/ctx)
- [Middleware](https://docs.gofiber.io/category/-middleware) & [Next](https://docs.gofiber.io/api/ctx#next) support
- [Rapid](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) server-side programming
- [Template Engines](https://github.com/gofiber/template)
- [WebSocket Support](https://github.com/gofiber/contrib/tree/main/websocket)
- [Socket.io Support](https://github.com/gofiber/contrib/tree/main/socketio)
- [Server-Sent Events](https://github.com/gofiber/recipes/tree/master/sse)
- [Rate Limiter](https://docs.gofiber.io/api/middleware/limiter)
- And much more, [explore Fiber](https://docs.gofiber.io/)

## 💡 Philosophy

New gophers that make the switch from [Node.js](https://nodejs.org/en/about/) to [Go](https://go.dev/doc/) are dealing with a learning curve before they can start building their web applications or microservices. Fiber, as a **web framework**, was created with the idea of **minimalism** and follows the **UNIX way**, so that new gophers can quickly enter the world of Go with a warm and trusted welcome.

Fiber is **inspired** by Express, the most popular web framework on the Internet. We combined the **ease** of Express and **raw performance** of Go. If you have ever implemented a web application in Node.js (_using Express or similar_), then many methods and principles will seem **very common** to you.

We **listen** to our users in [issues](https://github.com/gofiber/fiber/issues), Discord [channel](https://gofiber.io/discord) _and all over the Internet_ to create a **fast**, **flexible** and **friendly** Go web framework for **any** task, **deadline** and developer **skill**! Just like Express does in the JavaScript world.

## ⚠️ Limitations

- Due to Fiber's usage of unsafe, the library may not always be compatible with the latest Go version. Fiber v3 has been tested with Go version 1.26 or higher.
- Fiber automatically adapts common `net/http` handler shapes when you register them on the router, and you can still use the [adaptor middleware](https://docs.gofiber.io/next/middleware/adaptor/) when you need to bridge entire apps or `net/http` middleware.

### net/http compatibility

Fiber can run side by side with the standard library. The router accepts existing `net/http` handlers directly and even works with native `fasthttp.RequestHandler` callbacks, so you can plug in legacy endpoints without wrapping them manually:

```go
package main

import (
    "log"
    "net/http"

    "github.com/gofiber/fiber/v3"
)

func main() {
    httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if _, err := w.Write([]byte("served by net/http")); err != nil {
            panic(err)
        }
    })

    app := fiber.New()
    app.Get("/", httpHandler)

    // Start the server on port 3000
    log.Fatal(app.Listen(":3000"))
}
```

When you need to convert entire applications or reuse `net/http` middleware chains, rely on the [adaptor middleware](https://docs.gofiber.io/next/middleware/adaptor/). It converts handlers and middlewares in both directions and even lets you mount a Fiber app in a `net/http` server.

### Express-style handlers

Fiber also adapts Express-style callbacks that operate on the lightweight `fiber.Req` and `fiber.Res` helper interfaces. This lets you port middleware and route handlers from Express-inspired codebases while keeping Fiber's router features:

```go
// Request/response handlers (2-argument)
app.Get("/", func(req fiber.Req, res fiber.Res) error {
    return res.SendString("Hello from Express-style handlers!")
})

// Middleware with an error-returning next callback (3-argument)
app.Use(func(req fiber.Req, res fiber.Res, next func() error) error {
    if req.IP() == "192.168.1.254" {
        return res.SendStatus(fiber.StatusForbidden)
    }
    return next()
})

// Middleware with a no-arg next callback (3-argument)
app.Use(func(req fiber.Req, res fiber.Res, next func()) {
    if req.Get("X-Skip") == "true" {
        return // stop the chain without calling next
    }
    next()
})
```

> **Note:** Adapted `net/http` handlers continue to operate with the standard-library semantics. They don't get access to `fiber.Ctx` features and incur the overhead of the compatibility layer, so native `fiber.Handler` callbacks still provide the best performance.

## 👀 Examples

Listed below are some of the common examples. If you want to see more code examples, please visit our [Recipes repository](https://github.com/gofiber/recipes) or visit our hosted [API documentation](https://docs.gofiber.io).

### 📖 [**Basic Routing**](https://docs.gofiber.io/#basic-routing)

```go title="Example"
package main

import (
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // GET /api/register
    app.Get("/api/*", func(c fiber.Ctx) error {
        msg := fmt.Sprintf("✋ %s", c.Params("*"))
        return c.SendString(msg) // => ✋ register
    })

    // GET /flights/LAX-SFO
    app.Get("/flights/:from-:to", func(c fiber.Ctx) error {
        msg := fmt.Sprintf("💸 From: %s, To: %s", c.Params("from"), c.Params("to"))
        return c.SendString(msg) // => 💸 From: LAX, To: SFO
    })

    // GET /dictionary.txt
    app.Get("/:file.:ext", func(c fiber.Ctx) error {
        msg := fmt.Sprintf("📃 %s.%s", c.Params("file"), c.Params("ext"))
        return c.SendString(msg) // => 📃 dictionary.txt
    })

    // GET /john/75
    app.Get("/:name/:age/:gender?", func(c fiber.Ctx) error {
        msg := fmt.Sprintf("👴 %s is %s years old", c.Params("name"), c.Params("age"))
        return c.SendString(msg) // => 👴 john is 75 years old
    })

    // GET /john
    app.Get("/:name", func(c fiber.Ctx) error {
        msg := fmt.Sprintf("Hello, %s 👋!", c.Params("name"))
        return c.SendString(msg) // => Hello john 👋!
    })

    log.Fatal(app.Listen(":3000"))
}
```

#### 📖 [**Route Naming**](https://docs.gofiber.io/api/app#name)

```go title="Example"
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/api/*", func(c fiber.Ctx) error {
        msg := fmt.Sprintf("✋ %s", c.Params("*"))
        return c.SendString(msg) // => ✋ register
    }).Name("api")

    route := app.GetRoute("api")

    data, _ := json.MarshalIndent(route, "", "  ")
    fmt.Println(string(data))
    // Prints:
    // {
    //    "method": "GET",
    //    "name": "api",
    //    "path": "/api/*",
    //    "params": [
    //      "*1"
    //    ]
    // }

    log.Fatal(app.Listen(":3000"))
}
```

#### 📖 [**Serving Static Files**](https://docs.gofiber.io/api/app#static)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
    app := fiber.New()

    // Serve static files from the "./public" directory
    app.Get("/*", static.New("./public"))
    // => http://localhost:3000/js/script.js
    // => http://localhost:3000/css/style.css

    app.Get("/prefix*", static.New("./public"))
    // => http://localhost:3000/prefix/js/script.js
    // => http://localhost:3000/prefix/css/style.css

    // Serve a single file for any unmatched routes
    app.Get("*", static.New("./public/index.html"))
    // => http://localhost:3000/any/path/shows/index.html

    log.Fatal(app.Listen(":3000"))
}
```

#### 📖 [**Middleware & Next**](https://docs.gofiber.io/api/ctx#next)

```go title="Example"
package main

import (
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Middleware that matches any route
    app.Use(func(c fiber.Ctx) error {
        fmt.Println("🥇 First handler")
        return c.Next()
    })

    // Middleware that matches all routes starting with /api
    app.Use("/api", func(c fiber.Ctx) error {
        fmt.Println("🥈 Second handler")
        return c.Next()
    })

    // GET /api/list
    app.Get("/api/list", func(c fiber.Ctx) error {
        fmt.Println("🥉 Last handler")
        return c.SendString("Hello, World 👋!")
    })

    log.Fatal(app.Listen(":3000"))
}
```

<details>
  <summary>📚 Show more code examples</summary>

### Views Engines

📖 [Config](https://docs.gofiber.io/api/fiber#config)
📖 [Engines](https://github.com/gofiber/template)
📖 [Render](https://docs.gofiber.io/api/ctx#render)

Fiber defaults to the [html/template](https://pkg.go.dev/html/template/) when no view engine is set.

If you want to execute partials or use a different engine like [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache), or [pug](https://github.com/Joker/jade), etc., check out our [Template](https://github.com/gofiber/template) package that supports multiple view engines.

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/template/pug"
)

func main() {
    // Initialize a new Fiber app with Pug template engine
    app := fiber.New(fiber.Config{
        Views: pug.New("./views", ".pug"),
    })

    // Define a route that renders the "home.pug" template
    app.Get("/", func(c fiber.Ctx) error {
        return c.Render("home", fiber.Map{
            "title": "Homepage",
            "year":  1999,
        })
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Grouping Routes into Chains

📖 [Group](https://docs.gofiber.io/api/app#group)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func middleware(c fiber.Ctx) error {
    log.Println("Middleware executed")
    return c.Next()
}

func handler(c fiber.Ctx) error {
    return c.SendString("Handler response")
}

func main() {
    app := fiber.New()

    // Root API group with middleware
    api := app.Group("/api", middleware) // /api

    // API v1 routes
    v1 := api.Group("/v1", middleware) // /api/v1
    v1.Get("/list", handler)           // /api/v1/list
    v1.Get("/user", handler)           // /api/v1/user

    // API v2 routes
    v2 := api.Group("/v2", middleware) // /api/v2
    v2.Get("/list", handler)           // /api/v2/list
    v2.Get("/user", handler)           // /api/v2/user

    log.Fatal(app.Listen(":3000"))
}
```

### Middleware Logger

📖 [Logger](https://docs.gofiber.io/api/middleware/logger)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
)

func main() {
    app := fiber.New()

    // Use Logger middleware
    app.Use(logger.New())

    // Define routes
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, Logger!")
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Cross-Origin Resource Sharing (CORS)

📖 [CORS](https://docs.gofiber.io/api/middleware/cors)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/cors"
)

func main() {
    app := fiber.New()

    // Use CORS middleware with default settings
    app.Use(cors.New())

    // Define routes
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("CORS enabled!")
    })

    log.Fatal(app.Listen(":3000"))
}
```

Check CORS by passing any domain in `Origin` header:

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```

### Custom 404 Response

📖 [HTTP Methods](https://docs.gofiber.io/api/ctx#status)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Define routes
    app.Get("/", static.New("./public"))

    app.Get("/demo", func(c fiber.Ctx) error {
        return c.SendString("This is a demo page!")
    })

    app.Post("/register", func(c fiber.Ctx) error {
        return c.SendString("Registration successful!")
    })

    // Middleware to handle 404 Not Found
    app.Use(func(c fiber.Ctx) error {
        return c.SendStatus(fiber.StatusNotFound) // => 404 "Not Found"
    })

    log.Fatal(app.Listen(":3000"))
}
```

### JSON Response

📖 [JSON](https://docs.gofiber.io/api/ctx#json)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func main() {
    app := fiber.New()

    // Route that returns a JSON object
    app.Get("/user", func(c fiber.Ctx) error {
        return c.JSON(&User{"John", 20})
        // => {"name":"John", "age":20}
    })

    // Route that returns a JSON map
    app.Get("/json", func(c fiber.Ctx) error {
        return c.JSON(fiber.Map{
            "success": true,
            "message": "Hi John!",
        })
        // => {"success":true, "message":"Hi John!"}
    })

    log.Fatal(app.Listen(":3000"))
}
```

### WebSocket Upgrade

📖 [Websocket](https://github.com/gofiber/websocket)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/websocket"
)

func main() {
    app := fiber.New()

    // WebSocket route
    app.Get("/ws", websocket.New(func(c *websocket.Conn) {
        defer c.Close()
        for {
            // Read message from client
            mt, msg, err := c.ReadMessage()
            if err != nil {
                log.Println("read:", err)
                break
            }
            log.Printf("recv: %s", msg)

            // Write message back to client
            err = c.WriteMessage(mt, msg)
            if err != nil {
                log.Println("write:", err)
                break
            }
        }
    }))

    log.Fatal(app.Listen(":3000"))
    // Connect via WebSocket at ws://localhost:3000/ws
}
```

### Server-Sent Events

📖 [More Info](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)

```go title="Example"
package main

import (
    "bufio"
    "fmt"
    "log"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/valyala/fasthttp"
)

func main() {
    app := fiber.New()

    // Server-Sent Events route
    app.Get("/sse", func(c fiber.Ctx) error {
        c.Set("Content-Type", "text/event-stream")
        c.Set("Cache-Control", "no-cache")
        c.Set("Connection", "keep-alive")
        c.Set("Transfer-Encoding", "chunked")

        c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
            var i int
            for {
                i++
                msg := fmt.Sprintf("%d - the time is %v", i, time.Now())
                fmt.Fprintf(w, "data: Message: %s\n\n", msg)
                fmt.Println(msg)

                w.Flush()
                time.Sleep(5 * time.Second)
            }
        })

        return nil
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Recover Middleware

📖 [Recover](https://docs.gofiber.io/api/middleware/recover)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/recover"
)

func main() {
    app := fiber.New()

    // Use Recover middleware to handle panics gracefully
    app.Use(recover.New())

    // Route that intentionally panics
    app.Get("/", func(c fiber.Ctx) error {
        panic("normally this would crash your app")
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Using Trusted Proxy

📖 [Config](https://docs.gofiber.io/api/fiber#config)

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New(fiber.Config{
        // Configure trusted proxies - WARNING: Only trust proxies you control
        // Using TrustProxy: true with unrestricted IPs can lead to IP spoofing
        TrustProxy: true,
        TrustProxyConfig: fiber.TrustProxyConfig{
            Proxies: []string{"10.0.0.0/8", "172.16.0.0/12"}, // Example: Internal network ranges only
        },
        ProxyHeader: fiber.HeaderXForwardedFor,
    })

    // Define routes
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Trusted Proxy Configured!")
    })

    log.Fatal(app.Listen(":3000"))
}
```

</details>

## 🧬 Internal Middleware

Here is a list of middleware that are included within the Fiber framework.

| Middleware                                                                           | Description                                                                                                                                                             |
|--------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [adaptor](https://github.com/gofiber/fiber/tree/main/middleware/adaptor)             | Converter for net/http handlers to/from Fiber request handlers.                                                                                                         |
| [basicauth](https://github.com/gofiber/fiber/tree/main/middleware/basicauth)         | Provides HTTP basic authentication. It calls the next handler for valid credentials and 401 Unauthorized for missing or invalid credentials.                            |
| [cache](https://github.com/gofiber/fiber/tree/main/middleware/cache)                 | Intercept and cache HTTP responses.                                                                                                                                     |
| [compress](https://github.com/gofiber/fiber/tree/main/middleware/compress)           | Compression middleware for Fiber, with support for `deflate`, `gzip`, `brotli` and `zstd`.                                                                             |
| [cors](https://github.com/gofiber/fiber/tree/main/middleware/cors)                   | Enable cross-origin resource sharing (CORS) with various options.                                                                                                       |
| [csrf](https://github.com/gofiber/fiber/tree/main/middleware/csrf)                   | Protect from CSRF exploits.                                                                                                                                             |
| [earlydata](https://github.com/gofiber/fiber/tree/main/middleware/earlydata)         | Adds support for TLS 1.3's early data ("0-RTT") feature.                                                                                                                |
| [encryptcookie](https://github.com/gofiber/fiber/tree/main/middleware/encryptcookie) | Encrypt middleware which encrypts cookie values.                                                                                                                        |
| [envvar](https://github.com/gofiber/fiber/tree/main/middleware/envvar)               | Expose environment variables with providing an optional config.                                                                                                         |
| [etag](https://github.com/gofiber/fiber/tree/main/middleware/etag)                   | Allows for caches to be more efficient and save bandwidth, as a web server does not need to resend a full response if the content has not changed.                      |
| [expvar](https://github.com/gofiber/fiber/tree/main/middleware/expvar)               | Serves via its HTTP server runtime exposed variables in the JSON format.                                                                                                 |
| [favicon](https://github.com/gofiber/fiber/tree/main/middleware/favicon)             | Ignore favicon from logs or serve from memory if a file path is provided.                                                                                               |
| [healthcheck](https://github.com/gofiber/fiber/tree/main/middleware/healthcheck)     | Liveness and Readiness probes for Fiber.                                                                                                                                |
| [helmet](https://github.com/gofiber/fiber/tree/main/middleware/helmet)               | Helps secure your apps by setting various HTTP headers.                                                                                                                 |
| [hostauthorization](https://github.com/gofiber/fiber/tree/main/middleware/hostauthorization) | Validates the Host header against a configurable allowlist, protecting against DNS rebinding attacks.                                                            |
| [idempotency](https://github.com/gofiber/fiber/tree/main/middleware/idempotency)     | Allows for fault-tolerant APIs where duplicate requests do not erroneously cause the same action performed multiple times on the server-side.                           |
| [keyauth](https://github.com/gofiber/fiber/tree/main/middleware/keyauth)             | Adds support for key based authentication.                                                                                                                              |
| [limiter](https://github.com/gofiber/fiber/tree/main/middleware/limiter)             | Adds Rate-limiting support to Fiber. Use to limit repeated requests to public APIs and/or endpoints such as password reset.                                             |
| [logger](https://github.com/gofiber/fiber/tree/main/middleware/logger)               | HTTP request/response logger.                                                                                                                                           |
| [paginate](https://github.com/gofiber/fiber/tree/main/middleware/paginate)           | Extracts pagination parameters from query strings. Supports page-based, offset-based, and cursor-based pagination with multi-field sorting.                             |
| [pprof](https://github.com/gofiber/fiber/tree/main/middleware/pprof)                 | Serves runtime profiling data in pprof format.                                                                                                                          |
| [proxy](https://github.com/gofiber/fiber/tree/main/middleware/proxy)                 | Allows you to proxy requests to multiple servers.                                                                                                                       |
| [recover](https://github.com/gofiber/fiber/tree/main/middleware/recover)             | Recovers from panics anywhere in the stack chain and handles the control to the centralized ErrorHandler.                                                               |
| [redirect](https://github.com/gofiber/fiber/tree/main/middleware/redirect)           | Redirect middleware.                                                                                                                                                    |
| [requestid](https://github.com/gofiber/fiber/tree/main/middleware/requestid)         | Adds a request ID to every request.                                                                                                                                     |
| [responsetime](https://github.com/gofiber/fiber/tree/main/middleware/responsetime)   | Measures request handling duration and writes it to a configurable response header.                          |
| [rewrite](https://github.com/gofiber/fiber/tree/main/middleware/rewrite)             | Rewrites the URL path based on provided rules. It can be helpful for backward compatibility or just creating cleaner and more descriptive links.                        |
| [session](https://github.com/gofiber/fiber/tree/main/middleware/session)             | Session middleware. NOTE: This middleware uses our Storage package.                                                                                                     |
| [skip](https://github.com/gofiber/fiber/tree/main/middleware/skip)                   | Skip middleware that skips a wrapped handler if a predicate is true.                                                                                                    |
| [static](https://github.com/gofiber/fiber/tree/main/middleware/static)               | Static middleware for Fiber that serves static files such as **images**, **CSS**, and **JavaScript**.                                                                    |
| [timeout](https://github.com/gofiber/fiber/tree/main/middleware/timeout)             | Adds a max time for a request and forwards to ErrorHandler if it is exceeded.                                                                                           |

## 🧬 External Middleware

List of externally hosted middleware modules and maintained by the [Fiber team](https://github.com/orgs/gofiber/people).

| Middleware                                        | Description                                                                                                           |
| :------------------------------------------------ | :-------------------------------------------------------------------------------------------------------------------- |
| [contrib](https://github.com/gofiber/contrib)   | Third-party middlewares                                                                                               |
| [storage](https://github.com/gofiber/storage)   | Premade storage drivers that implement the Storage interface, designed to be used with various Fiber middlewares.     |
| [template](https://github.com/gofiber/template) | This package contains 9 template engines that can be used with Fiber.      |

## 🕶️ Awesome List

For more articles, middlewares, examples, or tools, check our [awesome list](https://github.com/gofiber/awesome-fiber).

## 👍 Contribute

If you want to say **Thank You** and/or support the active development of `Fiber`:

1. Add a [GitHub Star](https://github.com/gofiber/fiber/stargazers) to the project.
2. Tweet about the project [on your 𝕏 (Twitter)](https://x.com/intent/tweet?text=Fiber%20is%20an%20Express%20inspired%20%23web%20%23framework%20built%20on%20top%20of%20Fasthttp%2C%20the%20fastest%20HTTP%20engine%20for%20%23Go.%20Designed%20to%20ease%20things%20up%20for%20%23fast%20development%20with%20zero%20memory%20allocation%20and%20%23performance%20in%20mind%20%F0%9F%9A%80%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Write a review or tutorial on [Medium](https://medium.com/), [Dev.to](https://dev.to/) or your personal blog.
4. Support the project by becoming a [GitHub Sponsor](https://github.com/sponsors/gofiber).

## 💻 Development

To ensure your contributions are ready for a Pull Request, please use the following `Makefile` commands. These tools help maintain code quality and consistency.

- **make help**: Display available commands.
- **make audit**: Conduct quality checks.
- **make benchmark**: Benchmark code performance.
- **make coverage**: Generate test coverage report.
- **make format**: Automatically format code.
- **make lint**: Run lint checks.
- **make test**: Execute all tests.
- **make tidy**: Tidy dependencies.

Run these commands to ensure your code adheres to project standards and best practices.

<!-- skip-docs -->
## ☕ Supporters

Fiber is an open-source project that runs on donations to pay the bills, e.g., our domain name, GitBook, Netlify, and serverless hosting. If you want to support Fiber, please sponsor the organization via ⭐ [**GitHub Sponsors**](https://github.com/sponsors/gofiber).

<!-- sponsors -->

### 📅 Monthly Sponsors

<table>
<tr><td valign="top"><strong>🔥 Fiber Guardian</strong></td><td><a href="https://www.coderabbit.ai/?utm_source=cr_org&amp;utm_medium=github" title="@coderabbitai"><img src="https://github.com/coderabbitai.png" width="50" alt="@coderabbitai" /></a></td></tr>
<tr><td valign="top"><strong>☕ Fiber Supporter</strong></td><td><a href="https://ndole.studio" title="@NdoleStudio"><img src="https://github.com/NdoleStudio.png" width="34" alt="@NdoleStudio" /></a></td></tr>
<tr><td valign="top"><strong>🪴 Fiber Friend</strong></td><td><a href="https://github.com/simonheisstpeter" title="@simonheisstpeter"><img src="https://github.com/simonheisstpeter.png" width="32" alt="@simonheisstpeter" /></a></td></tr>
</table>

### 🎁 One-time Sponsors

<table>
<tr><td valign="top"><strong>🚀 Fiber Hero</strong></td><td><a href="https://www.thanks.dev" title="@thnxdev"><img src="https://github.com/thnxdev.png" width="40" alt="@thnxdev" /></a></td></tr>
<tr><td valign="top"><strong>🪴 Fiber Friend</strong></td><td><a href="https://github.com/Gl1tchedPixzl" title="@Gl1tchedPixzl"><img src="https://github.com/Gl1tchedPixzl.png" width="26" alt="@Gl1tchedPixzl" /></a></td></tr>
</table>
<!-- sponsors -->
<!-- skip-docs -->

## 💻 Code Contributors

<a href="https://github.com/gofiber/fiber/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=gofiber/fiber" alt="Code Contributors" style="max-width:100%;">
</a>

## ⭐️ Stargazers

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://next.ossinsight.io/widgets/official/analyze-repo-stars-history/thumbnail.png?repo_id=234231371&image_size=auto&color_scheme=dark" />
  <source media="(prefers-color-scheme: light)" srcset="https://next.ossinsight.io/widgets/official/analyze-repo-stars-history/thumbnail.png?repo_id=234231371&image_size=auto&color_scheme=light" />
  <img src="https://next.ossinsight.io/widgets/official/analyze-repo-stars-history/thumbnail.png?repo_id=234231371&image_size=auto&color_scheme=light" alt="Stargazers over time" />
</picture>

## 🧾 License

Copyright (c) 2019-present [Fenny](https://github.com/fenny) and [Contributors](https://github.com/gofiber/fiber/graphs/contributors). `Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/blob/main/LICENSE). Official logo was created by [Vic Shóstak](https://github.com/koddr) and distributed under [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) license (CC BY-SA 4.0 International).


## 🌐 Web Resources & Interactive Index
- [BACKGAMMON DUEL](https://themindzone.pages.dev/backgammon-duel.html)
- [COOKING FESTIVAL](https://quizverses.github.io/cooking-festival.html)
- [CATEGORY TITANIUM NETWORK](https://studyquests.github.io/category-titanium-network.html)
- [NUBIK COURIER AN OPEN WORLD](https://quizverses.pages.dev/nubik-courier-an-open-world.html)
- [PYRAMIDZ](https://quizverses.github.io/pyramidz.html)
- [ARROW SLIDE PUZZLE](https://studyquesthub.web.app/arrow-slide-puzzle.html)
- [RAGDOLL JUMP](https://studyplayings.web.app/ragdoll-jump.html)
- [COLOR SCREW RESCUE PUZZLE](https://studyquests.pages.dev/color-screw-rescue-puzzle.html)
- [QUACKVENTURE](https://studyquests.github.io/quackventure.html)
- [ANIMAL TRANSFORM RACE](https://studyquests.pages.dev/animal-transform-race.html)
- [CATEGORY AVOID295](https://quizverses.github.io/category-avoid295.html)
- [PHOTO BLOCK JOURNEY](https://studyquests.pages.dev/photo-block-journey.html)
- [WAVE CHIC OCEAN FASHION FRENZY](https://studyquests.pages.dev/wave-chic-ocean-fashion-frenzy.html)
- [CONNECT BALLS NEW YEAR PUZZLES](https://studyquests.github.io/connect-balls-new-year-puzzles.html)
- [BUSY BEE HIVE](https://studyquests.pages.dev/busy-bee-hive.html)
- [THE BIG HIT RUN](https://quizverses-9d2f2.web.app/the-big-hit-run.html)
- [RIDDLEMATH](https://studyquests.pages.dev/riddlemath.html)
- [PUSH PUSH CAT](https://studyquesthub.web.app/push-push-cat.html)
- [WATERPARK SORT](https://studyquests.github.io/waterpark-sort.html)
- [FOOD JAM](https://quizverses.pages.dev/food-jam.html)
- [BRICK BLAZE](https://studyquests.github.io/brick-blaze.html)
- [CLEAN THE FLOOR](https://quizverses-9d2f2.web.app/clean-the-floor.html)
- [BANK ROBBERY 3](https://studyquests.github.io/bank-robbery-3.html)
- [ITALIAN BRAINROT NEURO BEASTS](https://studyquests.github.io/italian-brainrot-neuro-beasts.html)
- [WOOD NUTS MASTER SCREW PUZZLE](https://quizverses.github.io/wood-nuts-master-screw-puzzle.html)
- [CATEGORY 3D1 371](https://studyquests.pages.dev/category-3d1-371.html)
- [MINETAP](https://quizverses.github.io/minetap.html)
- [SITEMAP](https://studyquests.pages.dev/sitemap.html)
- [PLANET MERGE](https://studyquesthub.web.app/planet-merge.html)
- [CARDS 2048](https://studyquests.github.io/cards-2048.html)
- [CHICKEN WILD RUN](https://quizverses.pages.dev/chicken-wild-run.html)
- [CHICKEN SCREAM RACE](https://quizverses.pages.dev/chicken-scream-race.html)
- [MR LONG HAND](https://studyquests.pages.dev/mr-long-hand.html)
- [ARROW COUNT MASTER](https://quizverses-9d2f2.web.app/arrow-count-master.html)
- [STICKMAN THE FLASH](https://studyquests.pages.dev/stickman-the-flash.html)
- [LADDER MASTER COLOR RUN](https://quizverses-9d2f2.web.app/ladder-master-color-run.html)
- [CATEGORY AGILITY 2](https://quizverses.pages.dev/category-agility-2.html)
- [CAT RESCUE](https://studyquests.pages.dev/cat-rescue.html)
- [JEWEL COLORING](https://studyquests.github.io/jewel-coloring.html)
- [FLAG PUZZLE JAM COLLECT FLAGS](https://studyquests.github.io/flag-puzzle-jam-collect-flags.html)
- [GRID BLAST](https://thelearnquester.web.app/grid-blast.html)
- [DIVINEX](https://quizverses.pages.dev/divinex.html)
- [HIGH HEELS COLLECT RUN](https://studyquests.github.io/high-heels-collect-run.html)
- [HOTEL MANAGER](https://studyplaying.github.io/hotel-manager.html)
- [SNAKE KING](https://studyquests.github.io/snake-king.html)
- [SOCCER DUEL](https://quizverses-9d2f2.web.app/soccer-duel.html)
- [BACTERIA LIFE DEATH](https://studyplayings.web.app/bacteria-life-death.html)
- [AVATAR MASTER FIX UP FACE](https://studyplaying.github.io/avatar-master-fix-up-face.html)
- [CATEGORY TOWER DEFENSE](https://quizverses.pages.dev/category-tower-defense.html)
- [INDEX9](https://quizverses.github.io/index9.html)
- [CATEGORY INCREMENTAL388](https://studyquests.github.io/category-incremental388.html)
- [SUMMER MAZE](https://studyplayings.pages.dev/summer-maze.html)
- [CATEGORY SKILL256](https://studyplayings.pages.dev/category-skill256.html)
- [MERGE CUBE CHALLENGE](https://quizverses.pages.dev/merge-cube-challenge.html)
- [WATER SORT](https://quizverses.pages.dev/water-sort.html)
- [DEAD ZONE MECH OPS](https://studyplaying.github.io/dead-zone-mech-ops.html)
- [INDEX10](https://studyplayings.pages.dev/index10.html)
- [CATEGORY TITANIUM NETWORK](https://quizverses.pages.dev/category-titanium-network.html)
- [LAST PLAY RAGDOLL SANDBOX KQB](https://studyquests.github.io/last-play-ragdoll-sandbox-kqb.html)
- [ROBOTS GONE WILD](https://quizverses-9d2f2.web.app/robots-gone-wild.html)
- [SMART DOTS RELOADED](https://quizverses.pages.dev/smart-dots-reloaded.html)
- [FIGHT FOR THE TREE](https://studyplaying.github.io/fight-for-the-tree.html)
- [MAX CRUSHER CRAZY DESTRUCTION AND CAR CRASHES](https://studyquests.pages.dev/max-crusher-crazy-destruction-and-car-crashes.html)
- [CATEGORY WEB PROXY](https://studyquests.github.io/category-web-proxy.html)
- [PANDA RUNNING](https://studyquests.github.io/panda-running.html)
- [MERGE BRICK BREAKER](https://studyquests.github.io/merge-brick-breaker.html)
- [SPRUNKI JIGSAW PUZZLE](https://quizverses-9d2f2.web.app/sprunki-jigsaw-puzzle.html)
- [SLIDE THEM AWAY](https://studyquesthub.web.app/slide-them-away.html)
- [CRY ISLANDS](https://quizverses.github.io/cry-islands.html)
- [DESERT ROVER SURVIVAL](https://quizverses-9d2f2.web.app/desert-rover-survival.html)
- [SID GINNY Y2K GLAM CLASH](https://studyplaying.github.io/sid-ginny-y2k-glam-clash.html)
- [MAHJONG CONNECT SPOOKY](https://studyquests.pages.dev/mahjong-connect-spooky.html)
- [DUCK HUNTING OPEN SEASON](https://quizverses.github.io/duck-hunting-open-season.html)
- [ADDICTION MINI SOLITAIRE](https://studyplayings.pages.dev/addiction-mini-solitaire.html)
- [NUTS STACK SORT NUTS BOLTS](https://studyplayings.web.app/nuts-stack-sort-nuts-bolts.html)
- [GRAVITY SPEED RUN](https://studyquests.github.io/gravity-speed-run.html)
- [SUPER TANK WRESTLE](https://studyquests.pages.dev/super-tank-wrestle.html)
- [PIZZA MAKER COOKING GAMES FOR KIDS](https://studyplayings.pages.dev/pizza-maker-cooking-games-for-kids.html)
- [LIGHT LINE](https://studyplayings.pages.dev/light-line.html)
- [ONLINE PORTAL](https://cryptotify.github.io/)
- [GUN CRAFT RUN WEAPON FIRE](https://studyplayings.pages.dev/gun-craft-run-weapon-fire.html)
- [CATEGORY BIKE63](https://studyplayings.pages.dev/category-bike63.html)
- [HUGGY WUGGY ESCAPE](https://quizverses-9d2f2.web.app/huggy-wuggy-escape.html)
- [IDLE PINBALL MERGE CLICKER](https://quizverses.github.io/idle-pinball-merge-clicker.html)
- [CATEGORY SPACE](https://studyquesthub.web.app/category-space.html)
- [DRAW CLIMBER](https://studyquesthub.web.app/draw-climber.html)
- [CARD SOLITAIRE WORD GAME](https://quizverses-9d2f2.web.app/card-solitaire-word-game.html)
- [CLEAN THE FLOOR](https://quizverses.pages.dev/clean-the-floor.html)
- [CATEGORY CONTROLLER](https://studyplayings.pages.dev/category-controller.html)
- [WORLD SOCCER](https://studyquests.github.io/world-soccer.html)
- [BULL RUNNER](https://studyquesthub.web.app/bull-runner.html)
- [SPRUNKI COLORING BOOK](https://studyplayings.pages.dev/sprunki-coloring-book.html)
- [CATEGORY MAKEUP51](https://quizverses.pages.dev/category-makeup51.html)
- [PIRATE PARADISE](https://studyplayings.pages.dev/pirate-paradise.html)
- [IDLE MERGE CAR AND RACE](https://quizverses.pages.dev/idle-merge-car-and-race.html)
- [SUPER TANK WRESTLE](https://studyplaying.github.io/super-tank-wrestle.html)
- [MURDER STONE AGE](https://studyquests.github.io/murder-stone-age.html)
- [SQUID ESCAPE BUT BLOCKWORLD](https://studyplaying.github.io/squid-escape-but-blockworld.html)
- [CATEGORY COLLECT565](https://studyplayings.web.app/category-collect565.html)
- [CINEMA EMPIRE IDLE TYCOON](https://studyplaying.github.io/cinema-empire-idle-tycoon.html)
- [CATEGORY HERO72](https://studyplayings.pages.dev/category-hero72.html)
- [BLOOM SORT 2 BEE PUZZLE](https://studyplayings.web.app/bloom-sort-2-bee-puzzle.html)
- [ARROW ESCAPE](https://studyquests.github.io/arrow-escape.html)
- [CATEGORY SURVIVAL366](https://studyquests.pages.dev/category-survival366.html)
- [JIGSORT PUZZLES](https://studyplayings.pages.dev/jigsort-puzzles.html)
- [CATEGORY RACING DRIVING 2](https://studyquesthub.web.app/category-racing-driving-2.html)
- [AGENT ZERO INFILTRATION](https://quizverses.github.io/agent-zero-infiltration.html)
- [CROWN CANNON](https://studyquests.github.io/crown-cannon.html)
- [SERIOUS HEAD 2](https://quizverses-9d2f2.web.app/serious-head-2.html)
- [BRAIN DRAW LINE](https://studyplayings.pages.dev/brain-draw-line.html)
- [GEOMETRY TOWER DEFENSE](https://quizverses.github.io/geometry-tower-defense.html)
- [KINGDOM PUZZLES](https://quizverses-9d2f2.web.app/kingdom-puzzles.html)
- [CAT MATCH 3](https://quizverses.github.io/cat-match-3.html)
- [CATEGORY ADVENTURE 3](https://studyquests.pages.dev/category-adventure-3.html)
- [MAHJONG CLASSIC WEBGL](https://studyplayings.web.app/mahjong-classic-webgl.html)
- [CHICKEN WILD RUN](https://studyplayings.pages.dev/chicken-wild-run.html)
- [SITEMAP](https://cryptotify.web.app/sitemap.html)
- [LOL SURPRISE OMG BB DRIVER](https://studyquesthub.web.app/lol-surprise-omg-bb-driver.html)
- [CATEGORY CASUAL 6](https://studyplayings.web.app/category-casual-6.html)
- [TERMS](https://cryptotify.vercel.app/terms.html)
- [CATEGORY BIKE63](https://studyquests.pages.dev/category-bike63.html)
- [SAVE BABY CAPYBARAS PULL PIN](https://quizverses-9d2f2.web.app/save-baby-capybaras-pull-pin.html)
- [NUBIK IN THE MONSTER WORLD](https://studyplayings.web.app/nubik-in-the-monster-world.html)
- [ACOX RUNNER](https://quizverses.pages.dev/acox-runner.html)
- [GROW A GARDEN 3D](https://studyplayings.web.app/grow-a-garden-3d.html)
- [MEMORY MATCH MAGIC](https://learnquester.github.io/memory-match-magic.html)
- [CATEGORY BRAIN261](https://studyplayings.web.app/category-brain261.html)
- [CHICKEN STRIKE](https://quizverses.pages.dev/chicken-strike.html)
- [GT FORMULA CHAMPIONSHIP](https://studyplaying.github.io/gt-formula-championship.html)
- [WIPE INSIGHT MASTER](https://studyquests.pages.dev/wipe-insight-master.html)
