<p align="center">
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
    <img src="https://img.shields.io/github/actions/workflow/status/gofiber/fiber/test.yml?branch=master&label=%F0%9F%A7%AA%20tests&style=flat-square&color=75C46B">
  </a>
    <a href="https://docs.gofiber.io">
    <img src="https://img.shields.io/badge/%F0%9F%92%A1%20fiber-docs-00ACD7.svg?style=flat-square">
  </a>
  <a href="https://gofiber.io/discord">
    <img src="https://img.shields.io/discord/704680098577514527?style=flat-square&label=%F0%9F%92%AC%20discord&color=00ACD7">
  </a>
</p>
<p align="center">
  <em><b>Fiber</b> is an <a href="https://github.com/expressjs/express">Express</a> inspired <b>web framework</b> built on top of <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, the <b>fastest</b> HTTP engine for <a href="https://go.dev/doc/">Go</a>. Designed to <b>ease</b> things up for <b>fast</b> development with <a href="https://docs.gofiber.io/#zero-allocation"><b>zero memory allocation</b></a> and <b>performance</b> in mind.</em>
</p>

---

## ⚠️ **Attention**

Fiber v3 is currently in beta and under active development. While it offers exciting new features, please note that it may not be stable for production use. We recommend sticking to the latest stable release (v2.x) for mission-critical applications. If you choose to use v3, be prepared for potential bugs and breaking changes. Always check the official documentation and release notes for updates and proceed with caution. Happy coding! 🚀

---

## ⚙️ Installation

Fiber requires **Go version `1.21` or higher** to run. If you need to install or upgrade Go, visit the [official Go download page](https://go.dev/dl/). To start setting up your project. Create a new directory for your project and navigate into it. Then, initialize your project with Go modules by executing the following command in your terminal:

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

```go
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

Fiber is optimized for **high-performance**, meaning values returned from **fiber.Ctx** are **not** immutable by default and **will** be re-used across requests. As a rule of thumb, you **must** only use context values within the handler and **must not** keep any references. Once you return from the handler, any values obtained from the context will be re-used in future requests. Visit our [documentation](https://docs.gofiber.io/#zero-allocation) to learn more.

## 🤖 Benchmarks

These tests are performed by [TechEmpower](https://www.techempower.com/benchmarks/#section=data-r19&hw=ph&test=plaintext) and [Go Web](https://github.com/smallnest/go-web-framework-benchmark). If you want to see all the results, please visit our [Wiki](https://docs.gofiber.io/extra/benchmarks).

<p float="left" align="middle">
  <img src="https://raw.githubusercontent.com/gofiber/docs/master/static/img/benchmark-pipeline.png" width="49%">
  <img src="https://raw.githubusercontent.com/gofiber/docs/master/static/img/benchmark_alloc.png" width="49%">
</p>

## 🎯 Features

-   Robust [Routing](https://docs.gofiber.io/guide/routing)
-   Serve [Static Files](https://docs.gofiber.io/api/app#static)
-   Extreme [Performance](https://docs.gofiber.io/extra/benchmarks)
-   [Low Memory](https://docs.gofiber.io/extra/benchmarks) footprint
-   [API Endpoints](https://docs.gofiber.io/api/ctx)
-   [Middleware](https://docs.gofiber.io/category/-middleware) & [Next](https://docs.gofiber.io/api/ctx#next) support
-   [Rapid](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) server-side programming
-   [Template Engines](https://github.com/gofiber/template)
-   [WebSocket Support](https://github.com/gofiber/contrib/tree/main/websocket)
-   [Socket.io Support](https://github.com/gofiber/contrib/tree/main/socketio)
-   [Server-Sent Events](https://github.com/gofiber/recipes/tree/master/sse)
-   [Rate Limiter](https://docs.gofiber.io/api/middleware/limiter)
-   And much more, [explore Fiber](https://docs.gofiber.io/)

## 💡 Philosophy

New gophers that make the switch from [Node.js](https://nodejs.org/en/about/) to [Go](https://go.dev/doc/) are dealing with a learning curve before they can start building their web applications or microservices. Fiber, as a **web framework**, was created with the idea of **minimalism** and follows the **UNIX way**, so that new gophers can quickly enter the world of Go with a warm and trusted welcome.

Fiber is **inspired** by Express, the most popular web framework on the Internet. We combined the **ease** of Express and **raw performance** of Go. If you have ever implemented a web application in Node.js (_using Express or similar_), then many methods and principles will seem **very common** to you.

We **listen** to our users in [issues](https://github.com/gofiber/fiber/issues), Discord [channel](https://gofiber.io/discord) _and all over the Internet_ to create a **fast**, **flexible** and **friendly** Go web framework for **any** task, **deadline** and developer **skill**! Just like Express does in the JavaScript world.

## ⚠️ Limitations

-   Due to Fiber's usage of unsafe, the library may not always be compatible with the latest Go version. Fiber v3 has been tested with Go versions 1.21 and 1.22.
-   Fiber is not compatible with net/http interfaces. This means you will not be able to use projects like gqlgen, go-swagger, or any others which are part of the net/http ecosystem.

## 👀 Examples

Listed below are some of the common examples. If you want to see more code examples, please visit our [Recipes repository](https://github.com/gofiber/recipes) or visit our hosted [API documentation](https://docs.gofiber.io).

#### 📖 [**Basic Routing**](https://docs.gofiber.io/#basic-routing)

```go
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

```go
func main() {
    app := fiber.New()

    // GET /api/register
    app.Get("/api/*", func(c fiber.Ctx) error {
        msg := fmt.Sprintf("✋ %s", c.Params("*"))
        return c.SendString(msg) // => ✋ register
    }).Name("api")

    data, _ := json.MarshalIndent(app.GetRoute("api"), "", "  ")
    fmt.Print(string(data))
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

```go
func main() {
    app := fiber.New()

    app.Get("/*", static.New("./public"))
    // => http://localhost:3000/js/script.js
    // => http://localhost:3000/css/style.css

    app.Get("/prefix*", static.New("./public"))
    // => http://localhost:3000/prefix/js/script.js
    // => http://localhost:3000/prefix/css/style.css

    app.Get("*", static.New("./public/index.html"))
    // => http://localhost:3000/any/path/shows/index/html

    log.Fatal(app.Listen(":3000"))
}

```

#### 📖 [**Middleware & Next**](https://docs.gofiber.io/api/ctx#next)

```go
func main() {
    app := fiber.New()

    // Match any route
    app.Use(func(c fiber.Ctx) error {
        fmt.Println("🥇 First handler")
        return c.Next()
    })

    // Match all routes starting with /api
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

### Views engines

📖 [Config](https://docs.gofiber.io/api/fiber#config)
📖 [Engines](https://github.com/gofiber/template)
📖 [Render](https://docs.gofiber.io/api/ctx#render)

Fiber defaults to the [html/template](https://pkg.go.dev/html/template/) when no view engine is set.

If you want to execute partials or use a different engine like [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) or [pug](https://github.com/Joker/jade) etc..

Checkout our [Template](https://github.com/gofiber/template) package that support multiple view engines.

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/template/pug"
)

func main() {
    // You can setup Views engine before initiation app:
    app := fiber.New(fiber.Config{
        Views: pug.New("./views", ".pug"),
    })

    // And now, you can call template `./views/home.pug` like this:
    app.Get("/", func(c fiber.Ctx) error {
        return c.Render("home", fiber.Map{
            "title": "Homepage",
            "year":  1999,
        })
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Grouping routes into chains

📖 [Group](https://docs.gofiber.io/api/app#group)

```go
func middleware(c fiber.Ctx) error {
    fmt.Println("Don't mind me!")
    return c.Next()
}

func handler(c fiber.Ctx) error {
    return c.SendString(c.Path())
}

func main() {
    app := fiber.New()

    // Root API route
    api := app.Group("/api", middleware) // /api

    // API v1 routes
    v1 := api.Group("/v1", middleware) // /api/v1
    v1.Get("/list", handler)           // /api/v1/list
    v1.Get("/user", handler)           // /api/v1/user

    // API v2 routes
    v2 := api.Group("/v2", middleware) // /api/v2
    v2.Get("/list", handler)           // /api/v2/list
    v2.Get("/user", handler)           // /api/v2/user

    // ...
}

```

### Middleware logger

📖 [Logger](https://docs.gofiber.io/api/middleware/logger)

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
)

func main() {
    app := fiber.New()

    app.Use(logger.New())

    // ...

    log.Fatal(app.Listen(":3000"))
}
```

### Cross-Origin Resource Sharing (CORS)

📖 [CORS](https://docs.gofiber.io/api/middleware/cors)

```go
import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/cors"
)

func main() {
    app := fiber.New()

    app.Use(cors.New())

    // ...

    log.Fatal(app.Listen(":3000"))
}
```

Check CORS by passing any domain in `Origin` header:

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```

### Custom 404 response

📖 [HTTP Methods](https://docs.gofiber.io/api/ctx#status)

```go
func main() {
    app := fiber.New()

    app.Get("/", static.New("./public"))

    app.Get("/demo", func(c fiber.Ctx) error {
        return c.SendString("This is a demo!")
    })

    app.Post("/register", func(c fiber.Ctx) error {
        return c.SendString("Welcome!")
    })

    // Last middleware to match anything
    app.Use(func(c fiber.Ctx) error {
        return c.SendStatus(404)
        // => 404 "Not Found"
    })

    log.Fatal(app.Listen(":3000"))
}
```

### JSON Response

📖 [JSON](https://docs.gofiber.io/api/ctx#json)

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func main() {
    app := fiber.New()

    app.Get("/user", func(c fiber.Ctx) error {
        return c.JSON(&User{"John", 20})
        // => {"name":"John", "age":20}
    })

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

```go
import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/websocket"
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

  log.Fatal(app.Listen(":3000"))
  // ws://localhost:3000/ws
}
```

### Server-Sent Events

📖 [More Info](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)

```go
import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/valyala/fasthttp"
)

func main() {
  app := fiber.New()

  app.Get("/sse", func(c fiber.Ctx) error {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")

    c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
      fmt.Println("WRITER")
      var i int

      for {
        i++
        msg := fmt.Sprintf("%d - the time is %v", i, time.Now())
        fmt.Fprintf(w, "data: Message: %s\n\n", msg)
        fmt.Println(msg)

        w.Flush()
        time.Sleep(5 * time.Second)
      }
    }))

    return nil
  })

  log.Fatal(app.Listen(":3000"))
}
```

### Recover Middleware

📖 [Recover](https://docs.gofiber.io/api/middleware/recover)

```go
import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/recover"
)

func main() {
    app := fiber.New()

    app.Use(recover.New())

    app.Get("/", func(c fiber.Ctx) error {
        panic("normally this would crash your app")
    })

    log.Fatal(app.Listen(":3000"))
}
```

</details>

### Using Trusted Proxy

📖 [Config](https://docs.gofiber.io/api/fiber#config)

```go
import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New(fiber.Config{
        EnableTrustedProxyCheck: true,
        TrustedProxies: []string{"0.0.0.0", "1.1.1.1/30"}, // IP address or IP address range
        ProxyHeader: fiber.HeaderXForwardedFor,
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
| [compress](https://github.com/gofiber/fiber/tree/main/middleware/compress)           | Compression middleware for Fiber, with support for `deflate`, `gzip`, `brotli` and `zstd`.                                                                                      |
| [cors](https://github.com/gofiber/fiber/tree/main/middleware/cors)                   | Enable cross-origin resource sharing (CORS) with various options.                                                                                                       |
| [csrf](https://github.com/gofiber/fiber/tree/main/middleware/csrf)                   | Protect from CSRF exploits.                                                                                                                                             |
| [earlydata](https://github.com/gofiber/fiber/tree/main/middleware/earlydata)         | Adds support for TLS 1.3's early data ("0-RTT") feature.                                                                                                                |
| [encryptcookie](https://github.com/gofiber/fiber/tree/main/middleware/encryptcookie) | Encrypt middleware which encrypts cookie values.                                                                                                                        |
| [envvar](https://github.com/gofiber/fiber/tree/main/middleware/envvar)               | Expose environment variables with providing an optional config.                                                                                                         |
| [etag](https://github.com/gofiber/fiber/tree/main/middleware/etag)                   | Allows for caches to be more efficient and save bandwidth, as a web server does not need to resend a full response if the content has not changed.                      |
| [expvar](https://github.com/gofiber/fiber/tree/main/middleware/expvar)               | Serves via its HTTP server runtime exposed variants in the JSON format.                                                                                                 |
| [favicon](https://github.com/gofiber/fiber/tree/main/middleware/favicon)             | Ignore favicon from logs or serve from memory if a file path is provided.                                                                                               |
| [healthcheck](https://github.com/gofiber/fiber/tree/main/middleware/healthcheck)     | Liveness and Readiness probes for Fiber.                                                                                                                                |
| [helmet](https://github.com/gofiber/fiber/tree/main/middleware/helmet)               | Helps secure your apps by setting various HTTP headers.                                                                                                                 |
| [idempotency](https://github.com/gofiber/fiber/tree/main/middleware/idempotency)     | Allows for fault-tolerant APIs where duplicate requests do not erroneously cause the same action performed multiple times on the server-side.                           |
| [keyauth](https://github.com/gofiber/fiber/tree/main/middleware/keyauth)             | Adds support for key based authentication.                                                                                                                              |
| [limiter](https://github.com/gofiber/fiber/tree/main/middleware/limiter)             | Adds Rate-limiting support to Fiber. Use to limit repeated requests to public APIs and/or endpoints such as password reset.                                             |
| [logger](https://github.com/gofiber/fiber/tree/main/middleware/logger)               | HTTP request/response logger.                                                                                                                                           |
| [pprof](https://github.com/gofiber/fiber/tree/main/middleware/pprof)                 | Serves runtime profiling data in pprof format.                                                                                                                          |
| [proxy](https://github.com/gofiber/fiber/tree/main/middleware/proxy)                 | Allows you to proxy requests to multiple servers.                                                                                                                       |
| [recover](https://github.com/gofiber/fiber/tree/main/middleware/recover)             | Recovers from panics anywhere in the stack chain and handles the control to the centralized ErrorHandler.                                                               |
| [redirect](https://github.com/gofiber/fiber/tree/main/middleware/redirect)           | Redirect middleware.                                                                                                                                                    |
| [requestid](https://github.com/gofiber/fiber/tree/main/middleware/requestid)         | Adds a request ID to every request.                                                                                                                                     |
| [rewrite](https://github.com/gofiber/fiber/tree/main/middleware/rewrite)             | Rewrites the URL path based on provided rules. It can be helpful for backward compatibility or just creating cleaner and more descriptive links.                        |
| [session](https://github.com/gofiber/fiber/tree/main/middleware/session)             | Session middleware. NOTE: This middleware uses our Storage package.                                                                                                     |
| [skip](https://github.com/gofiber/fiber/tree/main/middleware/skip)                   | Skip middleware that skips a wrapped handler if a predicate is true.                                                                                                    |
| [static](https://github.com/gofiber/fiber/tree/main/middleware/static)                   | Static middleware for Fiber that serves static files such as **images**, **CSS,** and **JavaScript**.                                                                                                   |
| [timeout](https://github.com/gofiber/fiber/tree/main/middleware/timeout)             | Adds a max time for a request and forwards to ErrorHandler if it is exceeded.                                                                                           |

## 🧬 External Middleware

List of externally hosted middleware modules and maintained by the [Fiber team](https://github.com/orgs/gofiber/people).

| Middleware                                        | Description                                                                                                           |
| :------------------------------------------------ | :-------------------------------------------------------------------------------------------------------------------- |
| [contrib](https://github.com/gofiber/contrib)     | Third party middlewares                                                                                               |
| [storage](https://github.com/gofiber/storage)     | Premade storage drivers that implement the Storage interface, designed to be used with various Fiber middlewares.     |
| [template](https://github.com/gofiber/template)   | This package contains 9 template engines that can be used with Fiber `v3` Go version 1.21 or higher is required.      |

## 🕶️ Awesome List

For more articles, middlewares, examples or tools check our [awesome list](https://github.com/gofiber/awesome-fiber).

## 👍 Contribute

If you want to say **Thank You** and/or support the active development of `Fiber`:

1. Add a [GitHub Star](https://github.com/gofiber/fiber/stargazers) to the project.
2. Tweet about the project [on your 𝕏 (Twitter)](https://x.com/intent/tweet?text=Fiber%20is%20an%20Express%20inspired%20%23web%20%23framework%20built%20on%20top%20of%20Fasthttp%2C%20the%20fastest%20HTTP%20engine%20for%20%23Go.%20Designed%20to%20ease%20things%20up%20for%20%23fast%20development%20with%20zero%20memory%20allocation%20and%20%23performance%20in%20mind%20%F0%9F%9A%80%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Write a review or tutorial on [Medium](https://medium.com/), [Dev.to](https://dev.to/) or personal blog.
4. Support the project by donating a [cup of coffee](https://buymeacoff.ee/fenny).

## 🖥️ Development

To ensure your contributions are ready for a Pull Request, please use the following `Makefile` commands. These tools help maintain code quality, consistency.

* **make help**: Display available commands.
* **make audit**: Conduct quality checks.
* **make benchmark**: Benchmark code performance.
* **make coverage**: Generate test coverage report.
* **make format**: Automatically format code.
* **make lint**: Run lint checks.
* **make test**: Execute all tests.
* **make tidy**: Tidy dependencies.

Run these commands to ensure your code adheres to project standards and best practices.

## ☕ Supporters

Fiber is an open source project that runs on donations to pay the bills e.g. our domain name, gitbook, netlify and serverless hosting. If you want to support Fiber, you can ☕ [**buy a coffee here**](https://buymeacoff.ee/fenny).

|                                                            | User                                             | Donation |
| :--------------------------------------------------------- | :----------------------------------------------- | :------- |
| ![](https://avatars.githubusercontent.com/u/204341?s=25)   | [@destari](https://github.com/destari)           | ☕ x 10  |
| ![](https://avatars.githubusercontent.com/u/63164982?s=25) | [@dembygenesis](https://github.com/dembygenesis) | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/56607882?s=25) | [@thomasvvugt](https://github.com/thomasvvugt)   | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/27820675?s=25) | [@hendratommy](https://github.com/hendratommy)   | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/1094221?s=25)  | [@ekaputra07](https://github.com/ekaputra07)     | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/194590?s=25)   | [@jorgefuertes](https://github.com/jorgefuertes) | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/186637?s=25)   | [@candidosales](https://github.com/candidosales) | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/29659953?s=25) | [@l0nax](https://github.com/l0nax)               | ☕ x 3   |
| ![](https://avatars.githubusercontent.com/u/635852?s=25)   | [@bihe](https://github.com/bihe)                 | ☕ x 3   |
| ![](https://avatars.githubusercontent.com/u/307334?s=25)   | [@justdave](https://github.com/justdave)         | ☕ x 3   |
| ![](https://avatars.githubusercontent.com/u/11155743?s=25) | [@koddr](https://github.com/koddr)               | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/29042462?s=25) | [@lapolinar](https://github.com/lapolinar)       | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/2978730?s=25)  | [@diegowifi](https://github.com/diegowifi)       | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/44171355?s=25) | [@ssimk0](https://github.com/ssimk0)             | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/5638101?s=25)  | [@raymayemir](https://github.com/raymayemir)     | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/619996?s=25)   | [@melkorm](https://github.com/melkorm)           | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/31022056?s=25) | [@marvinjwendt](https://github.com/marvinjwendt) | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/31921460?s=25) | [@toishy](https://github.com/toishy)             | ☕ x 1   |

## 💻 Code Contributors

<img src="https://opencollective.com/fiber/contributors.svg?width=890&button=false" alt="Code Contributors" style="max-width:100%;">

## ⭐️ Stargazers

<img src="https://starchart.cc/gofiber/fiber.svg" alt="Stargazers over time" style="max-width: 100%">

## 🧾 License

Copyright (c) 2019-present [Fenny](https://github.com/fenny) and [Contributors](https://github.com/gofiber/fiber/graphs/contributors). `Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE). Official logo was created by [Vic Shóstak](https://github.com/koddr) and distributed under [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) license (CC BY-SA 4.0 International).
