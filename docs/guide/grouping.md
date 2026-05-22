---
id: grouping
title: 🎭 Grouping
sidebar_position: 2
---

:::info
Grouping works like Express.js. Groups are virtual; routes are flattened with the group's prefix and executed in declaration order, mirroring Express.js.
:::

## Paths

Groups can use path prefixes to organize related routes.

```go
func main() {
    app := fiber.New()

    api := app.Group("/api", middleware) // /api

    v1 := api.Group("/v1", middleware)   // /api/v1
    v1.Get("/list", handler)             // /api/v1/list
    v1.Get("/user", handler)             // /api/v1/user

    v2 := api.Group("/v2", middleware)   // /api/v2
    v2.Get("/list", handler)             // /api/v2/list
    v2.Get("/user", handler)             // /api/v2/user

    log.Fatal(app.Listen(":3000"))
}
```

:::note
Group prefixes follow the same slash-boundary rule as `app.Use`. A prefix must either match the full path or stop at a `/`, so `/api` applies to `/api` and `/api/v1` but not `/apiv1`. Parameter markers (for example `:id`, `:id?`, `*`, and `+`) are processed before checking the boundary.
:::

Groups can also include an optional handler.

```go
func main() {
    app := fiber.New()

    api := app.Group("/api")      // /api

    v1 := api.Group("/v1")        // /api/v1
    v1.Get("/list", handler)      // /api/v1/list
    v1.Get("/user", handler)      // /api/v1/user

    v2 := api.Group("/v2")        // /api/v2
    v2.Get("/list", handler)      // /api/v2/list
    v2.Get("/user", handler)      // /api/v2/user

    log.Fatal(app.Listen(":3000"))
}
```

:::caution
Accessing `/api`, `/v1`, or `/v2` directly returns a **404**, so add error handlers as needed.
:::

## Group Handlers

Group handlers can act as routing paths but must call `Next` to continue the flow.

```go
func main() {
    app := fiber.New()

    handler := func(c fiber.Ctx) error {
        return c.SendStatus(fiber.StatusOK)
    }
    api := app.Group("/api") // /api

    v1 := api.Group("/v1", func(c fiber.Ctx) error { // middleware for /api/v1
        c.Set("Version", "v1")
        return c.Next()
    })
    v1.Get("/list", handler) // /api/v1/list
    v1.Get("/user", handler) // /api/v1/user

    log.Fatal(app.Listen(":3000"))
}
```

## Route

[`Route`](../api/app.md#route) groups routes under a common prefix declared inside a single callback, with an optional name prefix. It is shorthand for nesting with `Group`.

```go
app.Route("/api/v1", func(r fiber.Router) {
    r.Get("/users", handler).Name("users")   // /api/v1/users  (name: v1.users)
    r.Post("/users", handler).Name("create") // /api/v1/users  (name: v1.create)
}, "v1.")
```

## RouteChain

When several HTTP methods share the **same path**, [`RouteChain`](../api/app.md#routechain) lets you declare the path once and chain the verb handlers. An `All` in the chain runs before the verb handlers on that path, acting as route-specific middleware.

```go
app.RouteChain("/events").
    All(func(c fiber.Ctx) error { return c.Next() }). // route-local middleware
    Get(func(c fiber.Ctx) error { return c.SendString("GET /events") }).
    Post(func(c fiber.Ctx) error { return c.SendString("POST /events") })
```
