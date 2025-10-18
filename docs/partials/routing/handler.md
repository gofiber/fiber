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

Handlers can be native Fiber handlers (`func(fiber.Ctx) error` or even
`func(fiber.Ctx)`), Express-style callbacks (`func(fiber.Req, fiber.Res)` with
optional `next` callbacks typed as `func() error` or `func()`, plus optional
`error` return values), familiar `net/http` shapes such as `http.Handler`,
`http.HandlerFunc`, or `func(http.ResponseWriter, *http.Request)`, and
fasthttp-based callbacks like `fasthttp.RequestHandler` or
`func(*fasthttp.RequestCtx) error`. Fiber automatically adapts supported
handlers for you during registration, so you can mix and match the style that
best fits your existing code.

:::caution Compatibility overhead
When you register net/http handlers, Fiber adapts them through a compatibility
layer. They don't receive
`fiber.Ctx` or gain access to Fiber-specific APIs, and the conversion adds more
overhead than running a native `fiber.Handler`. Because they cannot call `c.Next()`, they will also terminate the handler chain.
Express-style handlers are not subject to this limitation when they accept a
`next` callback (either `func() error` or `func()`). Prefer Fiber handlers when
you need the lowest latency or Fiber features.
:::

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
