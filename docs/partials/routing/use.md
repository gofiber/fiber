---
id: route-use
title: Use
---

`Use` mounts middleware on a **prefix** (or **mount**) path: it runs for every request whose path begins with that prefix, on any HTTP method. Prefixes require either an exact match or a slash boundary, so `/john` matches `/john` and `/john/doe` but not `/johnnnnn`. Parameter tokens like `:name`, `:name?`, `*`, and `+` are still expanded before the boundary check runs. Called without a path, `Use` matches every request.

```go title="Signature"
func (app *App) Use(args ...any) Router

// Fiber inspects args to support these common usage patterns:
// - app.Use(handler, handlers ...any)
// - app.Use(path string, handler, handlers ...any)
// - app.Use(paths []string, handler, handlers ...any)
// - app.Use(path string, subApp *App)
```

Each handler argument can independently be a Fiber handler (with or without an `error` return), an Express-style callback, a `net/http` handler, or any other supported shape including fasthttp callbacks that return errors.

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

// Attach multiple handlers (they run in order; each must call c.Next() to continue)
app.Use("/api", func(c fiber.Ctx) error {
    c.Set("X-Custom-Header", random.String(32))
    return c.Next()
}, func(c fiber.Ctx) error {
    return c.Next()
})

// Mount a sub-app
app.Use("/api", api)
```
