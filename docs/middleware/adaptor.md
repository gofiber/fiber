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
or `FiberApp`, the adaptor enforces the app's configured `BodyLimit`. The app's `BodyLimit` defaults to **4 MiB** if a non-positive value is provided during configuration. `FiberHandler` and `FiberHandlerFunc` run the handler on a fresh app with the default configuration, so they always apply the 4 MiB default; pass your own app to `FiberApp` to use a different `BodyLimit`. Requests exceeding the active limit receive `413 Request Entity Too Large`.
:::

## API Reference

| Name                          | Signature                                                                     | Description                                                                   |
|-------------------------------|-------------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| `HTTPHandler`                 | `HTTPHandler(h http.Handler) fiber.Handler`                                   | Converts `http.Handler` to `fiber.Handler`                                    |
| `HTTPHandlerWithContext`      | `HTTPHandlerWithContext(h http.Handler) fiber.Handler`                        | Converts `http.Handler` to `fiber.Handler`, propagating Fiber's local context |
| `HTTPHandlerFunc`             | `HTTPHandlerFunc(h http.HandlerFunc) fiber.Handler`                           | Converts `http.HandlerFunc` to `fiber.Handler`                                |
| `HTTPMiddleware`              | `HTTPMiddleware(mw func(http.Handler) http.Handler) fiber.Handler`            | Converts `http.Handler` middleware to `fiber.Handler` middleware              |
| `FiberHandler`                | `FiberHandler(h fiber.Handler) http.Handler`                                  | Converts `fiber.Handler` to `http.Handler`                                    |
| `FiberHandlerFunc`            | `FiberHandlerFunc(h fiber.Handler) http.HandlerFunc`                          | Converts `fiber.Handler` to `http.HandlerFunc`                                |
| `FiberApp`                    | `FiberApp(app *fiber.App) http.HandlerFunc`                                   | Converts an entire Fiber app to a `http.HandlerFunc`                          |
| `ConvertRequest`              | `ConvertRequest(c fiber.Ctx, forServer bool) (*http.Request, error)`          | Converts `fiber.Ctx` into a `http.Request`                                    |
| `LocalContextFromHTTPRequest` | `LocalContextFromHTTPRequest(r *http.Request) (context.Context, bool)`        | Extracts the propagated `context.Context` from an adapted `http.Request`      |
| `CopyContextToFiberContext`   | `CopyContextToFiberContext(src any, requestContext *fasthttp.RequestCtx)`     | Copies `context.Context` to `fasthttp.RequestCtx`                             |

---

## Examples

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

:::caution Two parsers read one request
The `net/http` server and the Fiber app behind it each parse the request's headers, and they do not read a repeated header alike. `Header.Get` returns the first field line even when it is empty, while `c.Get` returns the first line that holds a value. A request carrying `X-Internal:` on one line and `X-Internal: true` on the next therefore reads as empty in a `net/http` guard and as `true` in the Fiber handler behind it. A guard that must keep clients from setting a header should strip it with `r.Header.Del(name)` or test `len(r.Header.Values(name)) > 0`, never compare `r.Header.Get(name)` with the empty string. Fiber's `basicauth` and `csrf` middleware refuse a request that repeats the single-value fields they read, and so does `keyauth` with its default `Authorization` extractor, so a credential split across two `Authorization` lines is rejected rather than authenticated; a custom `extractors.FromHeader` extractor combines repeated lines into one value, which the validator then judges.
:::

### 6. Converting `fiber.Ctx` to `*http.Request` (`ConvertRequest`)

Create an `*http.Request` from a `fiber.Ctx`. The `forServer` parameter decides whether
the request carries a `RequestURI`; `RemoteAddr` and `TLS` are filled in either way:

- Use `forServer = true` when the converted request will be passed into a `net/http` handler
  (sets `RequestURI`, which a server-side request carries)
- Use `forServer = false` when creating a request for client-side use (e.g., making an
  outbound HTTP request with `http.Client`, which refuses a request that has `RequestURI` set)

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

### 7. Passing Fiber user context into `net/http`

This example shows a realistic flow: a Fiber middleware sets a request-scoped `context.Context` (with a `request_id`) on the Fiber context, then an adapted `net/http` handler retrieves it via `LocalContextFromHTTPRequest`.

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/adaptor"
)

type ctxKey string
const requestIDKey ctxKey = "request_id"

func main() {
    app := fiber.New()

    // Create a request-scoped context in Fiber (e.g., request id, auth claims, trace span).
    app.Use(func(c fiber.Ctx) error {
        reqID := c.Get("X-Request-ID")

        ctx := context.WithValue(context.Background(), requestIDKey, reqID)

        // Fiber stores request-scoped context as "user context".
        c.SetContext(ctx)
        return c.Next()
    })

    // 2) Run a standard net/http handler that includes Fiber's user context propagated.
    app.Get("/hello", adaptor.HTTPHandlerWithContext(http.HandlerFunc(handleRequest)))

    app.Listen(":3000")
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx, ok := adaptor.LocalContextFromHTTPRequest(r)
    if !ok || ctx == nil {
        http.Error(w, "missing propagated context", http.StatusInternalServerError)
        return
    }

    reqID, _ := ctx.Value(requestIDKey).(string)
    fmt.Fprintf(w, "Hello from net/http (request_id=%s)\n", reqID)
}
```

### 8. Copying context values onto `fasthttp.RequestCtx` (`CopyContextToFiberContext`)

`CopyContextToFiberContext` copies values stored in a `context.Context` onto a
`fasthttp.RequestCtx`. The function is marked deprecated in code because it uses
reflection and unsafe operations—prefer explicit parameter passing when possible.
When you do need it, call it immediately after you add values to the `net/http`
context so Fiber can read them via `c.Locals()`. Values are copied from the
chain of derived contexts (`WithValue`, `WithCancel`, `WithTimeout`,
`WithDeadline`, `WithoutCancel`), the most recently derived context winning for
a key set twice, as `Value` reports it. The walk follows the chain to its end
and stops only where it would repeat itself, so a deep middleware stack keeps
its outermost values and a context pointing back at itself still terminates:

```go
package main

import (
    "context"

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
        if id, ok := c.Locals(contextKey("requestID")).(string); ok {
            return c.SendString("Request ID: " + id)
        }
        return c.SendStatus(fiber.StatusNotFound)
    })

    app.Listen(":3000")
}
```

---

## Notes and limitations

- An adapted `net/http` handler, middleware or `ConvertRequest` request is built from the path the router matched, `c.Path()`, followed by the query as the client sent it, not from the request line as it arrived. The two differ for an escaped or non-canonical request, and `net/http` reads the raw line its own way: `/public/..%2Fadmin/x` decodes into a `URL.Path` that `http.FileServer` cleans to `/admin/x`, although no middleware mounted on `/admin` has run for it. Routed, `/a/../admin/x` reaches the handler as `/admin/x` (after the `/admin` middleware ran), `%41` as `A`, a stray `%` as `%25`, a byte a path may not carry raw, such as `|` or a byte of UTF-8, as its escape, and with `UnescapePath` the decoded path is escaped again segment by segment. The query keeps its bytes unless a Fiber handler changed the arguments through `QueryArgs()`.
- The adapted request keeps the host Fiber saw. For a request in the absolute form, such as `GET http://public.example/x` with `Host: admin.internal`, that is the host the request line names ([RFC 9112 Section 3.2.2](https://www.rfc-editor.org/rfc/rfc9112#section-3.2.2)): `r.Host` is `public.example`, as are `c.Hostname()` and the host `app.Domain` matches.
- `HTTPHandler`, `HTTPHandlerFunc` and `HTTPHandlerWithContext` answer `404 Not Found` for a routed path with an empty segment (`//admin/x`) or an escaped slash (`/public/..%2Fadmin/x`): `net/http` decodes `%2F` into a separator in `URL.Path` and its handlers clean `//` and `..` away, so the handler would serve a path the router never matched. They answer `404` as well for a backslash, raw or escaped as `%5C`, and for a `.` or `..` segment carrying parameters, such as `..;`, or a segment of parameters alone before another, such as `/;x/admin`, which a handler that passes the request on, such as `httputil.ReverseProxy`, would hand to an upstream that reads them as a separator (WHATWG URL parsers, IIS), as a dot segment or as an empty one (Tomcat, Jetty). A path parameter on a named segment, such as `/a;b/c`, is handed on, since `net/http` reads it as part of the name. A middleware mounted on `/admin` does not run for `/admin;x/secret`, so put an upstream that strips parameters, such as Tomcat or Jetty, behind the [proxy middleware](./proxy.md#request-target), which refuses it, rather than an adapted `httputil.ReverseProxy`. `HTTPMiddleware` and `ConvertRequest` hand all of these on, with an escaped slash kept in `URL.RawPath`; a Fiber route behind the middleware still receives its `%2F`. To serve names that hold one, such as a GitLab-style `group%2Fproject` id, wrap the handler with `HTTPMiddleware` as a middleware that does not call `next`, and read the name from `r.URL.EscapedPath()`.
- `HTTPMiddleware` answers `400 Bad Request`, and `ConvertRequest` returns `fiber.ErrBadRequest`, for a routed path fasthttp would read as an authority rather than a path: one that begins with `//` and holds `://`, which `..` resolution builds out of `//u:pw@evil.example/a:/x/..//b`, or any path beginning with `//` on a request without a `Host` header. Set as the request line, it would become the host of both the `net/http` request and the Fiber request behind the middleware.
- A routed path holding an escape of an unreserved character, such as `%2e` or `%41`, is answered with `404 Not Found` by `HTTPHandler`, `HTTPHandlerFunc` and `HTTPHandlerWithContext` and with `400 Bad Request` by `HTTPMiddleware` and `ConvertRequest`. The router decodes every such escape when it normalizes a path, so one still in `c.Path()` can only have been forged by a stray `%`, and `URL.Path` would read a forged `%2e%2e` as the `..` segment an `http.FileServer` cleans away.
- `HTTPHandler`, `HTTPHandlerFunc` and `HTTPHandlerWithContext` put the original request line back once a buffered handler has returned, or panicked, so `c.OriginalURL()` in a middleware that runs afterwards is unchanged. A handler that flushed or hijacked is still running with a request whose URL aliases the request line, so the line is then left as the handler read it; `HTTPMiddleware` and `ConvertRequest` leave it as well.
- A request that `net/http` served over TLS is seen as TLS by Fiber: `c.Scheme()` is `https`, `c.Secure()` is true and `c.RequestCtx().TLSConnectionState()` carries the state from `r.TLS`.
- For a request that reaches Fiber through `FiberHandler`, `FiberHandlerFunc` or `FiberApp`, `c.StartTime()` marks the moment the adaptor took the request over, not the moment the `net/http` server began reading it, so `c.Elapsed()` leaves out the time spent before the handler was called.
- Both parsers read the headers: `Header.Get` returns the first field line even when it is empty, `c.Get` the first line that holds a value. A `net/http` guard should strip a header or check every value rather than compare `Header.Get` with the empty string; see the caution in the `FiberApp` section above.
- `HTTPMiddleware` routes the request the wrapped middleware hands to `next`, including a rewritten `r.URL` such as the one `http.StripPrefix` produces. The middleware must call `next` before it flushes or hijacks the response; after that the response has left Fiber's hands and the call is ignored.
- `HTTPMiddleware` copies `r.Method`, `r.Host` and `r.Header`, as the wrapped middleware hands them to `next`, back onto the Fiber request, so a header it replaces or deletes (`r.Header.Set`, `r.Header.Del`) is replaced or gone for the handlers behind it, `c.IPs()` included. A changed `r.Method` reaches only the raw request (`c.Request().Header.Method()`): `c.Method()` and the route that is running keep the method the request was routed with, so a rewrite does not move it to another method's handlers. `Content-Length` and `Transfer-Encoding` are not copied back, so the body framing stays as received. A `Connection` field written as several entries is joined with commas, and a `close` token anywhere in it closes the connection once the response is sent.
- `FiberHandler`, `FiberHandlerFunc` and `FiberApp` route `r.RequestURI`, the request line as `net/http` received it, so a raw `/über` or `/a|b` is routed and its params are read as a Fiber server would read them. When `r.URL` no longer says what that line says, or there is no such line, they route the request line `net/http` would write for `r.URL` instead: the path an `http.StripPrefix` in front of them rewrote, the URL of a request built in code, which has no `RequestURI` at all, and, for a `CONNECT` request in the authority form, the authority rather than the `/` that `r.URL.RequestURI()` reads for it: `r.URL.Opaque` when set, else `r.Host`, else `r.URL.Host`, the order in which `net/http` itself writes such a request.
- `FiberHandler`, `FiberHandlerFunc` and `FiberApp` pass repeated lines of a header such as `X-Forwarded-For` on as repeated lines, and `c.Protocol()` reports the version the request arrived with (`HTTP/1.0`, `HTTP/1.1`, `HTTP/2` or `HTTP/3`). The client address comes from `r.RemoteAddr`: a host without a port gets port `80`, and behind a Unix socket listener it is the listener's own address, which holds no client IP. An address that is empty or cannot be resolved does not fail the request; `c.IP()` is then an empty string. `c.SendEarlyHints` does not panic, but its `103` interim response is discarded because there is no client connection to write it to; the `Link` headers still reach the final response.

## Summary

The `adaptor` package lets Fiber and `net/http` interoperate so you can:

- Convert handlers and middleware in both directions
- Run Fiber apps inside `net/http`
- Convert `fiber.Ctx` to `http.Request`
- Propagate Fiber's user context into adapted `net/http` handlers

This makes it straightforward to integrate Fiber with existing Go projects or migrate between frameworks.
