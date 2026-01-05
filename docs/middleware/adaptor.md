---
id: adaptor
---

# Adaptor

The `adaptor` package converts between Fiber and `net/http`, letting you reuse handlers, middleware, and requests across both frameworks.

:::tip
Fiber can register plain `net/http` handlers directly—just pass an `http.Handler`,
`http.HandlerFunc`, or `func(http.ResponseWriter, *http.Request)` to any router
method and it will be adapted automatically. The adaptor helpers remain valuable
when you need to convert middleware, swap handler directions, or transform
requests explicitly.
:::

:::caution Fiber features are unavailable
Even when you register them directly, adapted `net/http` handlers still run with standard
library semantics. They don't have access to `fiber.Ctx`, and the compatibility layer comes
with additional overhead compared to native Fiber handlers. Use them for interop and legacy
scenarios, but prefer Fiber handlers when performance or Fiber-specific APIs matter.
:::

## Features

- Convert `net/http` handlers and middleware to Fiber handlers
- Convert Fiber handlers to `net/http` handlers
- Convert a Fiber context (`fiber.Ctx`) into an `http.Request`
- Copy values stored in a `context.Context` onto a `fasthttp.RequestCtx`

:::note Body size limits when running Fiber from net/http
When Fiber is executed from a `net/http` server through `FiberHandler`, `FiberHandlerFunc`,
or `FiberApp`, the adaptor enforces the app's configured `BodyLimit`. If the app's
configuration sets a non-positive `BodyLimit`, the adaptor falls back to Fiber's
default of **4 MiB**. Requests exceeding the active limit receive `413 Request Entity Too Large`.
:::

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

### 1. Using `net/http` handlers in Fiber (`HTTPHandler`, `HTTPHandlerFunc`)

Run standard `net/http` handlers inside Fiber. Fiber can auto-adapt them, or you can
explicitly convert them when you want to cache or share the converted handler.

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

    // Fiber adapts net/http handlers for you during registration.
    app.Get("/", http.HandlerFunc(helloHandler))

    // You can also convert and reuse the handler manually.
    cached := adaptor.HTTPHandler(http.HandlerFunc(helloHandler))
    app.Get("/cached", cached)

    // When you already have an http.HandlerFunc, convert it directly.
    app.Get("/func", adaptor.HTTPHandlerFunc(helloHandler))

    app.Listen(":3000")
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Hello from net/http!")
}
```

### 2. Using `net/http` middleware with Fiber (`HTTPMiddleware`)

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

### 3. Using Fiber handlers in `net/http` (`FiberHandler`)

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

### 4. Converting Fiber handlers to `http.HandlerFunc` (`FiberHandlerFunc`)

When you specifically need an `http.HandlerFunc`, wrap the Fiber handler directly:

```go
package main

import (
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    http.HandleFunc("/func-only", adaptor.FiberHandlerFunc(helloFiber))
    http.ListenAndServe(":3000", nil)
}

func helloFiber(c fiber.Ctx) error {
    return c.SendString("Hello from Fiber!")
}
```

### 5. Running a full Fiber app inside `net/http` (`FiberApp`)

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

### 6. Converting `fiber.Ctx` to `*http.Request` (`ConvertRequest`)

Create an `*http.Request` from a `fiber.Ctx`. The `forServer` parameter determines how
server-oriented fields are populated:

- Use `forServer = true` when the converted request will be passed into a `net/http` handler
  (sets `RequestURI`, `RemoteAddr`, and `TLS` fields for server-side handling)
- Use `forServer = false` when creating a request for client-side use (e.g., making an
  outbound HTTP request with `http.Client`)

```go
package main

import (
    "net/http"
    "net/http/httptest"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
    app := fiber.New()
    app.Get("/request", handleRequest)
    app.Listen(":3000")
}

func handleRequest(c fiber.Ctx) error {
    // Use forServer = true when passing to a net/http handler
    httpReq, err := adaptor.ConvertRequest(c, true)
    if err != nil {
        return err
    }

    // Pass the request to a net/http handler.
    recorder := httptest.NewRecorder()
    http.DefaultServeMux.ServeHTTP(recorder, httpReq)

    return c.SendString("Converted Request URL: " + httpReq.URL.String())
}
```

### 7. Copying context values onto `fasthttp.RequestCtx` (`CopyContextToFiberContext`)

`CopyContextToFiberContext` copies values stored in a `context.Context` onto a
`fasthttp.RequestCtx`. The function is marked deprecated in code because it uses
reflection and unsafe operations—prefer explicit parameter passing when possible.
When you do need it, call it immediately after you add values to the `net/http`
context so Fiber can read them via `c.Context()`:

```go
package main

import (
    "context"
    "net/http"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

type contextKey string

func main() {
    app := fiber.New()

    app.Use(func(c fiber.Ctx) error {
        // Convert the Fiber context to an http.Request so we can attach context values.
        httpReq, err := adaptor.ConvertRequest(c, true)
        if err != nil {
            return err
        }

        // Add context data and push it back to the Fiber context.
        enriched := httpReq.WithContext(context.WithValue(httpReq.Context(), contextKey("requestID"), "req-123"))
        adaptor.CopyContextToFiberContext(enriched.Context(), c.RequestCtx())

        return c.Next()
    })

    app.Get("/", func(c fiber.Ctx) error {
        if id, ok := c.Context().Value(contextKey("requestID")).(string); ok {
            return c.SendString("Request ID: " + id)
        }
        return c.SendStatus(fiber.StatusNotFound)
    })

    app.Listen(":3000")
}
```

---

## Summary

The `adaptor` package lets Fiber and `net/http` interoperate so you can:

- Convert handlers and middleware in both directions
- Run Fiber apps inside `net/http`
- Convert `fiber.Ctx` to `http.Request`

This makes it straightforward to integrate Fiber with existing Go projects or migrate between frameworks.
