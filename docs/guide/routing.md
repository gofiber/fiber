---
id: routing
title: 🔌 Routing
description: >-
  Routing refers to how an application's endpoints (URIs) respond to client
  requests.
sidebar_position: 1
toc_max_heading_level: 4
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import RoutingHandler from './../partials/routing/handler.md';
import RoutingUse from './../partials/routing/use.md';
import RoutingHandlerTypes from './../partials/routing/handler-types.md';
import RouteAnatomy from '@site/src/components/route-anatomy';

## Anatomy of a route

A route ties together an HTTP method, a path, and one or more handlers. Hover or click any colored part to jump to the section that explains it:

<RouteAnatomy />

`Get` is the [routing method](#route-handlers), `"/users/:id"` is the [route path](#paths) (the resource, in REST terms) with `:id` a [route parameter](#parameters), and `func(c fiber.Ctx) error` is the [handler](#handler-types) (or [middleware](#middleware)) run when the route matches.

## Route Handlers

<RoutingHandler />

Here is a complete, runnable app for context:

```go title="A minimal Fiber app"
package main

import "github.com/gofiber/fiber/v3"

func main() {
    app := fiber.New()

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    app.Listen(":3000")
}
```

In the shorter examples throughout this guide, `app` is the `*fiber.App` returned by `fiber.New()`, and `handler`/`middleware` stand in for any `func(c fiber.Ctx) error`. Snippets that call `fmt.Println` or `fmt.Fprintf` also need `import "fmt"`.

Beyond the native `func(fiber.Ctx)` forms, Fiber also adapts Express-style, `net/http`, and `fasthttp` handlers. See [Handler types](#handler-types) at the end of this guide for the full list of supported shapes.

## Get vs Use vs All

`Get` (and the other method helpers like `Post` and `Put`) match a **single HTTP method** at an **exact path**. `All` matches an **exact path** across **every** HTTP method. `Use` registers **middleware** that matches by **prefix** and runs in **declaration order**, calling [`c.Next()`](../api/ctx.md#next) to continue the chain.

<Tabs>
<TabItem value="get" label="Get (one method)">

```go
app.Get("/users", func(c fiber.Ctx) error {
    return c.SendString("GET /users")
})

// GET    /users      -> "GET /users"
// POST   /users      -> 405 Method Not Allowed
// GET    /users/42   -> 404 Not Found  (exact match only)
```

</TabItem>
<TabItem value="all" label="All (every method)">

```go
app.All("/ping", func(c fiber.Ctx) error {
    return c.SendString(c.Method() + " /ping")
})

// GET    /ping        -> "GET /ping"
// POST   /ping        -> "POST /ping"
// DELETE /ping        -> "DELETE /ping"
// GET    /ping/extra  -> 404 Not Found  (still exact path)
```

</TabItem>
<TabItem value="use" label="Use (prefix middleware)">

```go
// Empty Use: no path -> matches every request, any method, any path
app.Use(func(c fiber.Ctx) error {
    c.Set("X-Powered-By", "Fiber")
    return c.Next()
})

// Prefixed Use: matches the prefix and anything below a slash boundary
app.Use("/api", func(c fiber.Ctx) error {
    return c.Next()
})

// The empty Use above runs for ALL of these. The notes below show which
// requests ALSO match the prefixed "/api" Use:
// /api        -> also matches "/api" Use   (exact prefix)
// /api/users  -> also matches "/api" Use   (slash boundary)
// /apiv2      -> empty Use only             (no slash boundary)
// /anything   -> empty Use only
```

</TabItem>
<TabItem value="chain" label="Ordered chain">

Multiple handlers that match the same request run in the order you declare them. Each must call `c.Next()` to pass control to the next; if one returns without calling it, the rest of the chain is skipped.

```go
app.Use("/api", func(c fiber.Ctx) error {
    fmt.Println("1: auth check")
    return c.Next()
})

app.Use("/api", func(c fiber.Ctx) error {
    fmt.Println("2: logging")
    return c.Next()
})

app.Get("/api/users", func(c fiber.Ctx) error {
    fmt.Println("3: handler")
    return c.SendString("users")
})

// GET /api/users prints, in order:
//   1: auth check
//   2: logging
//   3: handler
```

</TabItem>
<TabItem value="multi" label="Multiple handlers in one call">

Attach several handlers in a single registration: list the route-specific middleware before the business handler.

```go
app.Get("/users/:id",
    func(c fiber.Ctx) error { // 1: require authentication
        if c.Get("Authorization") == "" {
            return c.SendStatus(fiber.StatusUnauthorized) // returns without c.Next(): stops here
        }
        return c.Next()
    },
    func(c fiber.Ctx) error { // 2: stash data for downstream handlers
        c.Locals("userID", c.Params("id"))
        return c.Next()
    },
    func(c fiber.Ctx) error { // 3: business handler reads the stashed value
        return c.SendString("user " + c.Locals("userID").(string))
    },
)

// GET /users/42 (no Authorization header) -> 401, handlers 2 and 3 never run
// GET /users/42 (with Authorization)      -> "user 42"
```

</TabItem>
</Tabs>

| Helper         | Methods matched | Path matching                              | Typical use                   |
| -------------- | --------------- | ------------------------------------------ | ----------------------------- |
| `Get`/`Post`/… | one             | exact                                      | a specific endpoint           |
| `All`          | every method    | exact                                      | one path, any verb            |
| `Use`          | every method    | prefix (slash boundary); all paths if none given | middleware, mounting sub-apps |

A path that exists only for a different method returns **405 Method Not Allowed**; a path that matches no route at all (including one rejected by a [constraint](#constraints)) returns **404 Not Found**.

## Paths

A route path paired with an HTTP method defines an endpoint. It can be a plain **string** or a **pattern**.

```go
// This route path will match requests to the root route, "/":
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("root")
})

// This route path will match requests to "/about":
app.Get("/about", func(c fiber.Ctx) error {
    return c.SendString("about")
})

// This route path will match requests to "/random.txt":
app.Get("/random.txt", func(c fiber.Ctx) error {
    return c.SendString("random.txt")
})
```

The order in which you declare routes matters: like Express.js, routes are matched in registration order (first match wins), so declare more specific paths before those that contain parameters. Note that method helpers such as `Get` match the exact path only.

:::info
Place routes with variable parameters after fixed paths to avoid unintended matches.
:::

## Parameters

Route parameters are dynamic segments in a path, either named or unnamed, used to capture values from the URL. Retrieve them with the [Params](../api/ctx.md#params) function using the parameter name or, for unnamed parameters, the wildcard (`*`) or plus (`+`) symbol with an index.

The characters `:`, `+`, and `*` introduce parameters. Append `?` to a named segment to make it optional. `+` is a greedy, required wildcard (it must match at least one character); `*` is a greedy, optional wildcard (it can match nothing).

<Tabs>
<TabItem value="named" label="Named, optional, greedy">

```go
// Named parameters
app.Get("/user/:name/books/:title", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s\n", c.Params("name"))
    fmt.Fprintf(c, "%s\n", c.Params("title"))
    return nil
})

// Plus - greedy, required (matches at least one character)
app.Get("/user/+", func(c fiber.Ctx) error {
    return c.SendString(c.Params("+"))
})

// Optional named parameter
app.Get("/user/:name?", func(c fiber.Ctx) error {
    return c.SendString(c.Params("name"))
})

// Wildcard - greedy, optional (may match nothing)
app.Get("/user/*", func(c fiber.Ctx) error {
    return c.SendString(c.Params("*"))
})
```

</TabItem>
<TabItem value="literal" label="Literal separators">

The hyphen (`-`), dot (`.`), and colon (`:`) are treated literally between parameters, so you can combine them with route parameters. Fiber's router detects when these characters belong to the literal path.

```go
// http://localhost:3000/plantae/prunus.persica
app.Get("/plantae/:genus.:species", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s.%s\n", c.Params("genus"), c.Params("species"))
    return nil // prunus.persica
})

// http://localhost:3000/flights/LAX-SFO
app.Get("/flights/:from-:to", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s-%s\n", c.Params("from"), c.Params("to"))
    return nil // LAX-SFO
})

// http://localhost:3000/shop/product/color:blue/size:xs
app.Get("/shop/product/color::color/size::size", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s:%s\n", c.Params("color"), c.Params("size"))
    return nil // blue:xs
})
```

</TabItem>
<TabItem value="escaped" label="Escaped characters">

Escape special parameter characters with `\\` to treat them literally. This is useful for custom methods like those in the [Google API Design Guide](https://cloud.google.com/apis/design/custom_methods). Wrap routes in backticks to keep escape sequences clear.

```go
// Matches "/v1/some/resource/name:customVerb" because the colon is escaped
app.Get(`/v1/some/resource/name\:customVerb`, func(c fiber.Ctx) error {
    return c.SendString("Hello, Community")
})
```

</TabItem>
<TabItem value="multi" label="Multiple params per segment">

You can chain multiple named or unnamed parameters, including wildcard and plus segments, within a single segment.

```go
// GET /@v1
// Params: "sign" -> "@", "param" -> "v1"
app.Get("/:sign:param", handler)

// GET /api-v1
// Params: "name" -> "v1"
app.Get("/api-:name", handler)

// GET /customer/v1/cart/proxy
// Params: "*1" -> "customer/", "*2" -> "/cart"
app.Get("/*v1*/proxy", handler)

// GET /v1/brand/4/shop/blue/xs
// Params: "*1" -> "brand/4", "*2" -> "blue/xs"
app.Get("/v1/*/shop/*", handler)
```

:::info
Fiber lets multiple parameters share a single path segment, unlike routers such as Express, Gin, and Echo where `:param` always consumes a whole segment. When named parameters are adjacent, each leading one captures a single character and the last captures the rest. This does not raise an error, so an unexpected pattern silently captures differently than you might assume.
:::

</TabItem>
</Tabs>

When a route has several wildcard (`*`) or plus (`+`) segments, retrieve them positionally with a 1-based index matching the symbol: `c.Params("*1")` and `c.Params("*2")` for wildcards, `c.Params("+1")` and `c.Params("+2")` for plus segments. A single wildcard or plus is just `c.Params("*")` or `c.Params("+")`.

Fiber's routing is inspired by Express but intentionally omits regex route patterns due to their performance cost. To validate a parameter against a regular expression, use the [`regex()` constraint](#constraints) described below.

### Constraints

Route constraints execute when a match has occurred to the incoming URL and the URL path is tokenized into route values by parameters. The feature was introduced in `v2.37.0` and inspired by [.NET Core](https://docs.microsoft.com/en-us/aspnet/core/fundamentals/routing?view=aspnetcore-6.0#route-constraints).

:::caution
Constraints are matching rules, not input validation: if a value fails a constraint, the route simply does not match and Fiber returns **404 Not Found**.
:::

| Constraint        | Example                          | Example matches                                                                             |
| ----------------- | -------------------------------- | ------------------------------------------------------------------------------------------- |
| int               | `:id<int>`                       | 123456789, -123456789                                                                       |
| bool              | `:active<bool>`                  | true,false                                                                                  |
| guid              | `:id<guid>`                      | CD2C1638-1638-72D5-1638-DEADBEEF1638                                                        |
| float             | `:weight<float>`                 | 1.234, -1001.01e8, 3.14                                                                     |
| minLen(value)     | `:username<minLen(4)>`           | Test (must be at least 4 characters)                                                        |
| maxLen(value)     | `:filename<maxLen(8)>`           | MyFile (must be no more than 8 characters)                                                  |
| len(length)       | `:filename<len(12)>`             | somefile.txt (exactly 12 characters)                                                        |
| min(value)        | `:age<min(18)>`                  | 19 (Integer value must be at least 18)                                                      |
| max(value)        | `:age<max(120)>`                 | 91 (Integer value must be no more than 120)                                                 |
| range(min,max)    | `:age<range(18,120)>`            | 91 (Integer value must be at least 18 but no more than 120)                                 |
| alpha             | `:name<alpha>`                   | Rick (String must consist of one or more alphabetical characters, a-z and case-insensitive) |
| datetime          | `:dob<datetime(2006\\-01\\-02)>` | 2005-11-01                                                                                  |
| regex(expression) | `:date<regex(\d{4}-\d{2}-\d{2})>` | 2022-08-27 (Must match regular expression)                                                  |

#### Examples

<Tabs>
<TabItem value="single-constraint" label="Single Constraint">

```go
app.Get("/:test<min(5)>", func(c fiber.Ctx) error {
    return c.SendString(c.Params("test"))
})

// curl -X GET http://localhost:3000/12
// 12

// curl -X GET http://localhost:3000/1
// Not Found
```

</TabItem>
<TabItem value="multiple-constraints" label="Multiple Constraints">

You can use `;` for multiple constraints.

```go
app.Get("/:test<min(100);maxLen(5)>", func(c fiber.Ctx) error {
    return c.SendString(c.Params("test"))
})

// curl -X GET http://localhost:3000/120000
// Not Found

// curl -X GET http://localhost:3000/1
// Not Found

// curl -X GET http://localhost:3000/250
// 250
```

</TabItem>
<TabItem value="regex-constraint" label="Regex Constraint">

Fiber precompiles the regex when registering routes, so the pattern is matched (not recompiled) on each request.

```go
app.Get(`/:date<regex(\d{4}-\d{2}-\d{2})>`, func(c fiber.Ctx) error {
    return c.SendString(c.Params("date"))
})

// curl -X GET http://localhost:3000/125
// Not Found

// curl -X GET http://localhost:3000/test
// Not Found

// curl -X GET http://localhost:3000/2022-08-27
// 2022-08-27
```

</TabItem>
</Tabs>

:::caution
When using the datetime constraint, prefix routing characters (`*`, `+`, `?`, `:`, `/`, `<`, `>`, `;`, `(`, `)`) with `\\` to avoid misparsing.
:::

#### Optional Parameter Example

You can impose constraints on optional parameters as well.

```go
app.Get("/:test<int>?", func(c fiber.Ctx) error {
  return c.SendString(c.Params("test"))
})
// curl -X GET http://localhost:3000/42
// 42
// curl -X GET http://localhost:3000/
//
// curl -X GET http://localhost:3000/7.0
// Not Found
```

#### Custom Constraint

Custom constraints can be added to Fiber using the `app.RegisterCustomConstraint` method. Your constraints have to be compatible with the `CustomConstraint` interface.

:::caution
Attention, custom constraints can now override built-in constraints. If a custom constraint has the same name as a built-in constraint, the custom constraint will be used instead. This allows for more flexibility in defining route parameter constraints.
:::

Add external constraints when you need stricter rules, such as verifying that a parameter is a valid ULID.

```go
// CustomConstraint is an interface for custom constraints
type CustomConstraint interface {
    // Name returns the name of the constraint.
    // This name is used in the constraint matching.
    Name() string

    // Execute executes the constraint.
    // It returns true if the constraint is matched and right.
    // param is the parameter value to check.
    // args are the constraint arguments.
    Execute(param string, args ...string) bool
}
```

You can check the example below:

```go
type UlidConstraint struct {
    fiber.CustomConstraint
}

func (*UlidConstraint) Name() string {
    return "ulid"
}

func (*UlidConstraint) Execute(param string, args ...string) bool {
    _, err := ulid.Parse(param)
    return err == nil
}

func main() {
    app := fiber.New()
    app.RegisterCustomConstraint(&UlidConstraint{})

    app.Get("/login/:id<ulid>", func(c fiber.Ctx) error {
        return c.SendString("...")
    })

    app.Listen(":3000")

    // /login/01HK7H9ZE5BFMK348CPYP14S0Z -> 200
    // /login/12345 -> 404
}
```

## Middleware

Functions that are designed to make changes to the request or response are called **middleware functions**. [`c.Next()`](../api/ctx.md#next) passes control to the next handler in the matched chain (middleware or route handler); if a handler returns without calling it, the remaining handlers are skipped.

```go title="Example of a middleware function"
app.Use(func(c fiber.Ctx) error {
    // Set a custom header on all responses:
    c.Set("X-Custom-Header", "Hello, World")

    // Go to next middleware:
    return c.Next()
})

app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Hello, World!")
})
```

See [Get vs Use vs All](#get-vs-use-vs-all) for how `Use` prefix matching differs from exact route matching, and how multiple handlers run in order.

### Use

<RoutingUse />

### Adding or removing routes at runtime

:::caution
Defining all routes before the app starts is strongly recommended. You can still change them at runtime with [`RebuildTree`](../api/app.md#rebuildtree), [`RemoveRoute`](../api/app.md#removeroute), [`RemoveRouteByName`](../api/app.md#removeroutebyname), and [`RemoveRouteFunc`](../api/app.md#removeroutefunc), but these operations are not thread-safe and are performance-intensive, so use them sparingly and only in development.
:::

## Grouping

If you have many endpoints, you can organize your routes using `Group`.

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

More information about this in our [Grouping Guide](./grouping.md).

### Route

[`Route`](../api/app.md#route) is shorthand for [`Group`](#grouping): it scopes a set of routes under a common prefix declared inside a single callback, with an optional name prefix.

```go
app.Route("/api/v1", func(r fiber.Router) {
    r.Get("/users", handler).Name("users")   // /api/v1/users  (name: v1.users)
    r.Post("/users", handler).Name("create") // /api/v1/users  (name: v1.create)
}, "v1.")
```

### RouteChain

When several HTTP methods share the **same path**, [`RouteChain`](../api/app.md#routechain) lets you declare the path once and chain the verb handlers. An `All` in the chain runs before the verb handlers on that path, acting as route-specific middleware.

```go
app.RouteChain("/events").
    All(func(c fiber.Ctx) error { return c.Next() }). // route-local middleware
    Get(func(c fiber.Ctx) error { return c.SendString("GET /events") }).
    Post(func(c fiber.Ctx) error { return c.SendString("POST /events") })
```

:::note
Within a chain, `All` registers prefix-matched middleware (like [`Use`](#use)), not the exact-path `App.All`, so it also runs for sub-paths of the chain path.
:::

Pick the helper that fits: a single endpoint uses `Get`/`Post`/…; a fixed set of methods on one path uses [`Add`](#route-handlers); one path with many methods (fluently) uses `RouteChain`; many paths under a shared prefix use [`Group`](#grouping) or `Route`.

## Automatic HEAD routes

Fiber automatically registers a `HEAD` route for every `GET` route you add. The generated handler chain mirrors the `GET` chain, so `HEAD` requests reuse middleware, status codes, and headers while the response body is suppressed.

```go title="GET handlers automatically expose HEAD"
app := fiber.New()

app.Get("/users/:id", func(c fiber.Ctx) error {
    c.Set("X-User", c.Params("id"))
    return c.SendStatus(fiber.StatusOK)
})

// HEAD /users/:id now returns the same headers and status without a body.
```

You can still register dedicated `HEAD` handlers, even with auto-registration enabled, and Fiber replaces the generated route so your implementation wins:

```go title="Override the generated HEAD handler"
app.Head("/users/:id", func(c fiber.Ctx) error {
    return c.SendStatus(fiber.StatusNoContent)
})
```

To opt out globally, start the app with `DisableHeadAutoRegister`:

```go title="Disable automatic HEAD registration"
handler := func(c fiber.Ctx) error {
    c.Set("X-User", c.Params("id"))
    return c.SendStatus(fiber.StatusOK)
}

app := fiber.New(fiber.Config{DisableHeadAutoRegister: true})
app.Get("/users/:id", handler) // HEAD /users/:id now returns 405 unless you add it manually.
```

Auto-generated `HEAD` routes participate in every router scope, including `Group` hierarchies, mounted sub-apps, parameterized and wildcard paths, and static file helpers. They also appear in route listings such as `app.Stack()` so tooling sees both the `GET` and `HEAD` entries.

## Handler types

<RoutingHandlerTypes />
