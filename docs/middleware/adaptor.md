---
id: adaptor
---

# Adaptor

The `adaptor` package provides utilities for converting between Fiber and `net/http`. It allows seamless integration of `net/http` handlers, middleware, and requests into Fiber applications, and vice versa.

## Features

- Convert `net/http` handlers and middleware to Fiber handlers.
- Convert Fiber handlers to `net/http` handlers.
- Convert Fiber context (`fiber.Ctx`) into an `http.Request`.

## API Reference

| Name                        | Signature                                                                     | Description                                                      |
|-----------------------------|-------------------------------------------------------------------------------|------------------------------------------------------------------|
| `HTTPHandler`               | `HTTPHandler(h http.Handler) fiber.Handler`                                   | Converts `http.Handler` to `fiber.Handler`                       |
| `HTTPHandlerFunc`           | `HTTPHandlerFunc(h http.HandlerFunc) fiber.Handler`                           | Converts `http.HandlerFunc` to `fiber.Handler`                   |
| `HTTPMiddleware`            | `HTTPMiddleware(mw func(http.Handler) http.Handler) fiber.Handler`            | Converts `http.Handler` middleware to `fiber.Handler` middleware |
| `FiberHandler`              | `FiberHandler(h fiber.Handler) http.Handler`                                  | Converts `fiber.Handler` to `http.Handler`                       |
| `FiberHandlerFunc`          | `FiberHandlerFunc(h fiber.Handler) http.HandlerFunc`                          | Converts `fiber.Handler` to `http.HandlerFunc`                   |
| `FiberApp`                  | `FiberApp(app *fiber.App) http.HandlerFunc`                                   | Converts an entire Fiber app to a `http.HandlerFunc`             |
| `ConvertRequest`            | `ConvertRequest(c fiber.Ctx, forServer bool) (*http.Request, error)`          | Converts `fiber.Ctx` into a `http.Request`                       |
| `CopyContextToFiberContext` | `CopyContextToFiberContext(context any, requestContext *fasthttp.RequestCtx)` | Copies `context.Context` to `fasthttp.RequestCtx`                |

---

## Usage Examples

### 1. Using `net/http` Handlers in Fiber

This example demonstrates how to use standard `net/http` handlers inside a Fiber application:

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

    // Convert a http.Handler to a Fiber handler
    app.Get("/", adaptor.HTTPHandler(http.HandlerFunc(helloHandler)))

    app.Listen(":3000")
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Hello from net/http!")
}
```

### 2. Using `net/http` Middleware with Fiber

Middleware written for `net/http` can be used in Fiber:

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

    // Apply a http middleware in Fiber
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

### 3. Using Fiber Handlers in `net/http`

You can embed Fiber handlers inside `net/http`:

```go
package main

import (
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    // Convert Fiber handler to an http.Handler
    http.Handle("/", adaptor.FiberHandler(helloFiber))
    
    // Convert Fiber handler to http.HandlerFunc
    http.HandleFunc("/func", adaptor.FiberHandlerFunc(helloFiber))
    
    http.ListenAndServe(":3000", nil)
}

func helloFiber(c fiber.Ctx) error {
    return c.SendString("Hello from Fiber!")
}
```

### 4. Running a Fiber App in `net/http`

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

### 5. Converting Fiber Context (`fiber.Ctx`) to `http.Request`

If you need to use a `http.Request` inside a Fiber handler:

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

The `adaptor` package allows easy interoperation between Fiber and `net/http`. You can:

- Convert handlers and middleware in both directions.
- Run Fiber apps inside `net/http`.
- Convert `fiber.Ctx` to `http.Request`.

This makes it simple to integrate Fiber with existing Go projects or migrate between frameworks as needed.
