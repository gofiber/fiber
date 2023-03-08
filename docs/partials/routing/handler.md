---
id: route-handlers
title: Route Handlers
---

Registers a route bound to a specific [HTTP method](https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods).

```go title="Signatures"
// HTTP methods
func (app *App) Get(path string, handlers ...Handler) Router
func (app *App) Head(path string, handlers ...Handler) Router
func (app *App) Post(path string, handlers ...Handler) Router
func (app *App) Put(path string, handlers ...Handler) Router
func (app *App) Delete(path string, handlers ...Handler) Router
func (app *App) Connect(path string, handlers ...Handler) Router
func (app *App) Options(path string, handlers ...Handler) Router
func (app *App) Trace(path string, handlers ...Handler) Router
func (app *App) Patch(path string, handlers ...Handler) Router

// Add allows you to specifiy a method as value
func (app *App) Add(method, path string, handlers ...Handler) Router

// All will register the route on all HTTP methods
// Almost the same as app.Use but not bound to prefixes
func (app *App) All(path string, handlers ...Handler) Router
```

```go title="Examples"
// Simple GET handler
app.Get("/api/list", func(c *fiber.Ctx) error {
  return c.SendString("I'm a GET request!")
})

// Simple POST handler
app.Post("/api/register", func(c *fiber.Ctx) error {
  return c.SendString("I'm a POST request!")
})
```

**Use** can be used for middleware packages and prefix catchers. These routes will only match the beginning of each path i.e. `/john` will match `/john/doe`, `/johnnnnn` etc

```go title="Signature"
func (app *App) Use(args ...interface{}) Router
```

```go title="Examples"
// Match any request
app.Use(func(c *fiber.Ctx) error {
    return c.Next()
})

// Match request starting with /api
app.Use("/api", func(c *fiber.Ctx) error {
    return c.Next()
})

// Match requests starting with /api or /home (multiple-prefix support)
app.Use([]string{"/api", "/home"}, func(c *fiber.Ctx) error {
    return c.Next()
})

// Attach multiple handlers 
app.Use("/api", func(c *fiber.Ctx) error {
  c.Set("X-Custom-Header", random.String(32))
    return c.Next()
}, func(c *fiber.Ctx) error {
    return c.Next()
})
```
