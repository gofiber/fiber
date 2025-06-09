---
id: route-handlers
title: Route Handlers
---

import Reference from '@site/src/components/reference';

Registers a route bound to a specific [HTTP method](https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods).

```go title="Signatures"
// HTTP methods – the main handler is required; additional handlers are optional
func (app *App) Get(path string, handler Handler, handlers ...Handler) Router
func (app *App) Head(path string, handler Handler, handlers ...Handler) Router
func (app *App) Post(path string, handler Handler, handlers ...Handler) Router
func (app *App) Put(path string, handler Handler, handlers ...Handler) Router
func (app *App) Delete(path string, handler Handler, handlers ...Handler) Router
func (app *App) Connect(path string, handler Handler, handlers ...Handler) Router
func (app *App) Options(path string, handler Handler, handlers ...Handler) Router
func (app *App) Trace(path string, handler Handler, handlers ...Handler) Router
func (app *App) Patch(path string, handler Handler, handlers ...Handler) Router

// Add allows you to specify multiple methods as a slice
func (app *App) Add(methods []string, path string, handler Handler, handlers ...Handler) Router

// All will register the route on all HTTP methods
func (app *App) All(path string, handler Handler, handlers ...Handler) Router
```

**Handler Execution Order:**

In Fiber v3, route handler methods clearly separate the main handler from additional handlers:

- `handler`: The main route handler (required, executed **last**)
- `handlers`: Optional additional handlers (executed **before** the main handler in left-to-right order)

```go title="Examples"
// Simple handlers (no additional handlers)
app.Get("/api/list", func(c fiber.Ctx) error {
    return c.SendString("I'm a GET request!")
})

app.Post("/api/register", func(c fiber.Ctx) error {
    return c.SendString("I'm a POST request!")
})

// Handler with additional handlers - execution order is clearly defined
app.Get("/api/users",
    func(c fiber.Ctx) error {           // Main handler (executes 3rd)
        return c.JSON(users)
    },
    authMiddleware,                     // Additional handler (executes 1st)
    rateLimitMiddleware,                // Additional handler (executes 2nd)
)

// Multiple methods example
app.Add([]string{fiber.MethodGet, fiber.MethodPost}, "/api/flexible",
    func(c fiber.Ctx) error {           // Main handler
        return c.SendString("Flexible endpoint!")
    },
    loggingMiddleware,                  // Additional handler
)
```

This separation improves type safety by ensuring at least one handler is always provided and makes the execution order explicit.

<Reference id="use">#Use</Reference>

Can be used for middleware packages and prefix catchers. These routes will only match the beginning of each path i.e. `/john` will match `/john/doe`, `/johnnnnn` etc

```go title="Signature"
func (app *App) Use(args ...any) Router

// Different usage variations
func (app *App) Use(handler Handler, handlers ...Handler) Router
func (app *App) Use(path string, handler Handler, handlers ...Handler) Router
func (app *App) Use(paths []string, handler Handler, handlers ...Handler) Router
func (app *App) Use(path string, app *App) Router
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
