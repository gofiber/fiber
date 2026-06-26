---
id: route-handlers
title: Route Handlers
---

Registers a route bound to a specific [HTTP method](https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods). The canonical handler is `func(fiber.Ctx) error`; Fiber also accepts `func(fiber.Ctx)` and runs it as if it returned `nil`.

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
func (app *App) Query(path string, handler any, handlers ...any) Router

// Add registers the same handlers on multiple methods at once.
// The handlers run in order, starting with `handler` and then the variadic `handlers`.
func (app *App) Add(methods []string, path string, handler any, handlers ...any) Router

// All registers the route on every HTTP method at the EXACT path
// (unlike Use, which is prefix-matched).
func (app *App) All(path string, handler any, handlers ...any) Router
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
