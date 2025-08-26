---
id: grouping
title: 🎭 Grouping
sidebar_position: 2
---

:::info
In general, group functionality in Fiber behaves like Express.js. Groups are declared virtually, and all routes defined within a group are flattened into a single list with a prefix and processed in the order declared. This makes group behavior in Fiber identical to Express.js.
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

A group of paths can include an optional handler.

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
Accessing `/api`, `/v1`, or `/v2` directly results in a **404** error, so configure appropriate error handlers.
:::

## Group Handlers

Group handlers can also act as routing paths, but they must call `Next` to continue the flow.

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
