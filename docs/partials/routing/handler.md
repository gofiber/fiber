---
id: route-handlers
title: Route Handlers
---

import Reference from '@site/src/components/reference';

Registers a route bound to a specific [HTTP method](https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods).

```go title="Signatures"
// HTTP methods
func (app *App[TCtx]) Get(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Head(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Post(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Put(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Delete(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Connect(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Options(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Trace(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Patch(path string, handler Handler, middlewares ...Handler) Router

// Add allows you to specify a method as value
func (app *App[TCtx]) Add(method, path string, handler Handler, middlewares ...Handler) Router

// All will register the route on all HTTP methods
// Almost the same as app.Use but not bound to prefixes
func (app *App[TCtx]) All(path string, handler Handler, middlewares ...Handler) Router
```

```go title="Examples"
// Simple GET handler
app.Get("/api/list", func(c fiber.Ctx) error {
    return c.SendString("I'm a GET request!")
})

// Simple POST handler
app.Post("/api/register", func(c fiber.Ctx) error {
    return c.SendString("I'm a POST request!")
})
```

<Reference id="use">**Use**</Reference>

Can be used for middleware packages and prefix catchers. These routes will only match the beginning of each path i.e. `/john` will match `/john/doe`, `/johnnnnn` etc

```go title="Signature"
func (app *App[TCtx]) Use(args ...any) Router

// Different usage variations
func (app *App[TCtx]) Use(handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Use(path string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Use(paths []string, handler Handler, middlewares ...Handler) Router
func (app *App[TCtx]) Use(path string, app *App) Router
```

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
