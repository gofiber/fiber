---
id: route-handlers
title: Route Handlers
---

import Reference from '@site/src/components/reference';

Registers a route bound to a specific [HTTP method](https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods).

```go title="Signatures"
// HTTP methods
func (app *App) Get(path string, handler any, handlers ...any) Router
func (app *App) Head(path string, handler any, handlers ...any) Router
func (app *App) Post(path string, handler any, handlers ...any) Router
func (app *App) Put(path string, handler any, handlers ...any) Router
func (app *App) Delete(path string, handler any, handlers ...any) Router
func (app *App) Connect(path string, handler any, handlers ...any) Router
func (app *App) Options(path string, handler any, handlers ...any) Router
func (app *App) Trace(path string, handler any, handlers ...any) Router
func (app *App) Patch(path string, handler any, handlers ...any) Router

// Add allows you to specify multiple methods at once
func (app *App) Add(methods []string, path string, handler any, handlers ...any) Router

// All will register the route on all HTTP methods
// Almost the same as app.Use but not bound to prefixes
func (app *App) All(path string, handler any, handlers ...any) Router
```

Fiber's adapter converts a variety of handler shapes to native
`func(fiber.Ctx) error` callbacks. It currently recognizes nineteen cases (the
numbers below match the comments in `toFiberHandler` inside `adapter.go`). This
lets you mix Fiber-style handlers with Express-style callbacks and even reuse
`net/http` or `fasthttp` functions.

### Fiber-native handlers (cases 1–2)

1. `fiber.Handler` — the canonical `func(fiber.Ctx) error` form.
2. `func(fiber.Ctx)` — Fiber runs the function and treats it as if it returned
   `nil`.

### Express-style request handlers (cases 3–8)

3. `func(fiber.Req, fiber.Res) error`
4. `func(fiber.Req, fiber.Res)`
5. `func(fiber.Req, fiber.Res, func() error) error`
6. `func(fiber.Req, fiber.Res, func() error)`
7. `func(fiber.Req, fiber.Res, func()) error`
8. `func(fiber.Req, fiber.Res, func())`

The adapter injects a `next` callback when your signature accepts one. Fiber
propagates downstream errors from `c.Next()` back through the wrapper, so
returning those errors remains optional. If you never call the injected `next`
function, the handler chain stops, matching Express semantics.

### Express-style error middleware (cases 9–14)

9.  `func(error, fiber.Req, fiber.Res) error`
10. `func(error, fiber.Req, fiber.Res)`
11. `func(error, fiber.Req, fiber.Res, func() error) error`
12. `func(error, fiber.Req, fiber.Res, func() error)`
13. `func(error, fiber.Req, fiber.Res, func()) error`
14. `func(error, fiber.Req, fiber.Res, func())`

These handlers only run after a downstream Fiber handler returns an error via
`c.Next()`. When they accept a `next` callback, Fiber forwards the original
error. If you call `next`, the chain continues with the previously returned
error; otherwise the middleware swallows it. Unlike Express, Fiber currently
passes only the error from `c.Next()`—there is no separate `err` state for
non-error invocations.

### net/http handlers (cases 15–17)

15. `http.HandlerFunc`
16. `http.Handler`
17. `func(http.ResponseWriter, *http.Request)`

:::caution Compatibility overhead
Fiber adapts these handlers through `fasthttpadaptor`. They do not receive
`fiber.Ctx`, cannot call `c.Next()`, and therefore always terminate the handler
chain. The compatibility layer also adds more overhead than running a native
Fiber handler, so prefer the other forms when possible.
:::

### fasthttp handlers (cases 18–19)

18. `fasthttp.RequestHandler`
19. `func(*fasthttp.RequestCtx) error`

fasthttp handlers run with full access to the underlying `fasthttp.RequestCtx`.
They are expected to manage the response directly. Fiber will propagate any
error returned by the `func(*fasthttp.RequestCtx) error` variant but otherwise
does not inspect the context state.

```go title="Examples"
// Simple GET handler (Fiber accepts both func(fiber.Ctx) and func(fiber.Ctx) error)
app.Get("/api/list", func(c fiber.Ctx) error {
    return c.SendString("I'm a GET request!")
})

// Reuse an existing net/http handler without manual adaptation
httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
})

app.Get("/foo", httpHandler)

// Align with Express-style handlers using fiber.Req and fiber.Res helpers (works
// for middleware and routes alike)
app.Use(func(req fiber.Req, res fiber.Res, next func() error) error {
    if req.IP() == "192.168.1.254" {
        return res.SendStatus(fiber.StatusForbidden)
    }
    return next()
})

app.Get("/express", func(req fiber.Req, res fiber.Res) error {
    return res.SendString("Hello from Express-style handlers!")
})

// Express-style error middleware (runs after c.Next() returns an error)
app.Use(func(err error, req fiber.Req, res fiber.Res) error {
    return res.Status(fiber.StatusInternalServerError).SendString(err.Error())
})

// Mount a fasthttp.RequestHandler directly
app.Get("/bar", func(ctx *fasthttp.RequestCtx) {
    ctx.SetStatusCode(fiber.StatusAccepted)
})

// Simple POST handler
app.Post("/api/register", func(c fiber.Ctx) error {
    return c.SendString("I'm a POST request!")
})
```

<Reference id="use">#Use</Reference>

Can be used for middleware packages and prefix catchers. Prefixes now require either an exact match or a slash boundary, so `/john` matches `/john` and `/john/doe` but not `/johnnnnn`. Parameter tokens like `:name`, `:name?`, `*`, and `+` are still expanded before the boundary check runs.

```go title="Signature"
func (app *App) Use(args ...any) Router

// Fiber inspects args to support these common usage patterns:
// - app.Use(handler, handlers ...any)
// - app.Use(path string, handler, handlers ...any)
// - app.Use(paths []string, handler, handlers ...any)
// - app.Use(path string, subApp *App)
```

Each handler argument can independently be a Fiber handler (with or without an
`error` return), an Express-style callback, a `net/http` handler, or any other
supported shape including fasthttp callbacks that return errors.

```go title="Examples"
// Match any request
app.Use(func(c fiber.Ctx) error {
    return c.Next()
})

// Match request starting with /api
app.Use("/api", func(c fiber.Ctx) error {
    return c.Next()
})

// Match requests starting with /api or /home (multiple-prefix support)
app.Use([]string{"/api", "/home"}, func(c fiber.Ctx) error {
    return c.Next()
})

// Attach multiple handlers 
app.Use("/api", func(c fiber.Ctx) error {
    c.Set("X-Custom-Header", random.String(32))
    return c.Next()
}, func(c fiber.Ctx) error {
    return c.Next()
})

// Mount a sub-app
app.Use("/api", api)
```
