---
id: adaptor
---

# Adaptor

The `adaptor` package converts between Fiber and `net/http`, letting you reuse handlers, middleware, and requests across both frameworks.

## Features

- Convert `net/http` handlers and middleware to Fiber handlers
- Convert Fiber handlers to `net/http` handlers
- Convert a Fiber context (`fiber.Ctx`) into an `http.Request`

## API Reference

| Name                        | Signature                                                                                 | Description                                                      |
|-----------------------------|-------------------------------------------------------------------------------------------|------------------------------------------------------------------|
| `HTTPHandler`               | `HTTPHandler(h http.Handler) fiber.Handler`                                               | Converts `http.Handler` to `fiber.Handler`                       |
| `HTTPHandlerFunc`           | `HTTPHandlerFunc(h http.HandlerFunc) fiber.Handler`                                       | Converts `http.HandlerFunc` to `fiber.Handler`                   |
| `HTTPMiddleware`            | `HTTPMiddleware(mw func(http.Handler) http.Handler) fiber.Handler`                        | Converts `http.Handler` middleware to `fiber.Handler` middleware |
| `FiberHandler`              | `FiberHandler(h fiber.Handler) http.Handler`                                              | Converts `fiber.Handler` to `http.Handler`                       |
| `FiberHandlerFunc`          | `FiberHandlerFunc(h fiber.Handler) http.HandlerFunc`                                      | Converts `fiber.Handler` to `http.HandlerFunc`                   |
| `FiberApp`                  | `FiberApp(app *fiber.App) http.HandlerFunc`                                               | Converts an entire Fiber app to a `http.HandlerFunc`             |
| `ConvertRequest`            | `ConvertRequest(c fiber.Ctx, forServer bool) (*http.Request, error)`                      | Converts `fiber.Ctx` into a `http.Request`                       |
| `CopyContextToFiberContext` | `CopyContextToFiberContext(context context.Context, requestContext *fasthttp.RequestCtx)` | Copies `context.Context` to `fasthttp.RequestCtx`                |

---

## Usage Examples

### 1. Using `net/http` handlers in Fiber

This example shows how to run a standard `net/http` handler within a Fiber app:

```go
package main

import (
    "fmt"
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    app := fiber.New()

    // Convert an http.Handler to a Fiber handler
    app.Get("/", adaptor.HTTPHandler(http.HandlerFunc(helloHandler)))

    app.Listen(":3000")
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Hello from net/http!")
}
```

### 2. Using `net/http` middleware with Fiber

Middleware written for `net/http` can run inside Fiber:

```go
package main

import (
    "log"
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    app := fiber.New()

    // Apply an http middleware in Fiber
    app.Use(adaptor.HTTPMiddleware(loggingMiddleware))

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello Fiber!")
    })

    app.Listen(":3000")
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("Request received")
        next.ServeHTTP(w, r)
    })
}
```

### 3. Using Fiber handlers in `net/http`

You can use Fiber handlers from `net/http`:

```go
package main

import (
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    // Convert a Fiber handler to an http.Handler
    http.Handle("/", adaptor.FiberHandler(helloFiber))

    // Convert a Fiber handler to an http.HandlerFunc
    http.HandleFunc("/func", adaptor.FiberHandlerFunc(helloFiber))

    http.ListenAndServe(":3000", nil)
}

func helloFiber(c fiber.Ctx) error {
    return c.SendString("Hello from Fiber!")
}
```

### 4. Running a Fiber app in `net/http`

You can wrap a full Fiber app inside `net/http`:

```go
package main

import (
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    app := fiber.New()
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello from Fiber!")
    })

    // Run Fiber inside an http server
    http.ListenAndServe(":3000", adaptor.FiberApp(app))
}
```

### 5. Converting a Fiber context (`fiber.Ctx`) to `http.Request`

To access an `http.Request` within a Fiber handler:

```go
package main

import (
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    app := fiber.New()
    app.Get("/request", handleRequest)
    app.Listen(":3000")
}

func handleRequest(c fiber.Ctx) error {
    httpReq, err := adaptor.ConvertRequest(c, false)
    if err != nil {
        return err
    }
    return c.SendString("Converted Request URL: " + httpReq.URL.String())
}
```

---

## Summary

The `adaptor` package lets Fiber and `net/http` interoperate so you can:

- Convert handlers and middleware in both directions
- Run Fiber apps inside `net/http`
- Convert `fiber.Ctx` to `http.Request`

This makes it straightforward to integrate Fiber with existing Go projects or migrate between frameworks.
