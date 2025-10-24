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

## Handlers

<RoutingHandler />

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

If you register a dedicated `HEAD` handler later, Fiber replaces the generated route so your implementation wins:

```go title="Override the generated HEAD handler"
app.Head("/users/:id", func(c fiber.Ctx) error {
    return c.SendStatus(fiber.StatusNoContent)
})
```

To opt out globally, start the app with `DisableAutoRegister`:

```go title="Disable automatic HEAD registration"
app := fiber.New(fiber.Config{DisableAutoRegister: true})
app.Get("/users/:id", handler) // HEAD /users/:id now returns 405 unless you add it manually.
```

Auto-generated `HEAD` routes participate in every router scope, including `Group` hierarchies, mounted sub-apps, parameterized and wildcard paths, and static file helpers. They also appear in route listings such as `app.Stack()` so tooling sees both the `GET` and `HEAD` entries.

## Paths

A route path paired with an HTTP method defines an endpoint. It can be a plain **string** or a **pattern**.

### Examples of route paths based on strings

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

As with the Express.js framework, the order in which routes are declared matters.
Routes are evaluated sequentially, so more specific paths should appear before those with variables.

:::info
Place routes with variable parameters after fixed paths to avoid unintended matches.
:::

## Parameters

Route parameters are dynamic segments in a path, either named or unnamed, used to capture values from the URL. Retrieve them with the [Params](https://fiber.wiki/context#params) function using the parameter name or, for unnamed parameters, the wildcard (`*`) or plus (`+`) symbol with an index.

The characters `:`, `+`, and `*` introduce parameters.

Use `*` or `+` to capture segments greedily.

You can define optional parameters by appending `?` to a named segment. The `+` sign is greedy and required, while `*` acts as an optional greedy wildcard.

### Example of defining routes with route parameters

```go
// Parameters
app.Get("/user/:name/books/:title", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s\n", c.Params("name"))
    fmt.Fprintf(c, "%s\n", c.Params("title"))
    return nil
})
// Plus - greedy - not optional
app.Get("/user/+", func(c fiber.Ctx) error {
    return c.SendString(c.Params("+"))
})

// Optional parameter
app.Get("/user/:name?", func(c fiber.Ctx) error {
    return c.SendString(c.Params("name"))
})

// Wildcard - greedy - optional
app.Get("/user/*", func(c fiber.Ctx) error {
    return c.SendString(c.Params("*"))
})

// This route path will match requests to "/v1/some/resource/name:customVerb", since the parameter character is escaped
app.Get(`/v1/some/resource/name\:customVerb`, func(c fiber.Ctx) error {
    return c.SendString("Hello, Community")
})
```

:::info
The hyphen \(`-`\) and dot \(`.`\) are treated literally, so you can combine them with route parameters.
:::

:::info
Escape special parameter characters with `\\` to treat them literally. This technique is useful for custom methods like those in the [Google API Design Guide](https://cloud.google.com/apis/design/custom_methods). Wrap routes in backticks to keep escape sequences clear.
:::

```go
// http://localhost:3000/plantae/prunus.persica
app.Get("/plantae/:genus.:species", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s.%s\n", c.Params("genus"), c.Params("species"))
    return nil // prunus.persica
})
```

```go
// http://localhost:3000/flights/LAX-SFO
app.Get("/flights/:from-:to", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s-%s\n", c.Params("from"), c.Params("to"))
    return nil // LAX-SFO
})
```

Fiber's router detects when these characters belong to the literal path and handles them accordingly.

```go
// http://localhost:3000/shop/product/color:blue/size:xs
app.Get("/shop/product/color::color/size::size", func(c fiber.Ctx) error {
    fmt.Fprintf(c, "%s:%s\n", c.Params("color"), c.Params("size"))
    return nil // blue:xs
})
```

You can chain multiple named or unnamed parameters—including wildcard and plus segments—giving the router greater flexibility.

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

Fiber's routing is inspired by Express but intentionally omits regular expression routes due to their performance cost. You can try similar patterns using the Express route tester (v0.1.7).

### Constraints

Route constraints execute when a match has occurred to the incoming URL and the URL path is tokenized into route values by parameters. The feature was introduced in `v2.37.0` and inspired by [.NET Core](https://docs.microsoft.com/en-us/aspnet/core/fundamentals/routing?view=aspnetcore-6.0#route-constraints).

:::caution
Constraints aren't validation for parameters. If constraints aren't valid for a parameter value, Fiber returns **404 handler**.
:::

| Constraint        | Example                          | Example matches                                                                             |
| ----------------- | -------------------------------- | ------------------------------------------------------------------------------------------- |
| int               | `:id<int>`                       | 123456789, -123456789                                                                       |
| bool              | `:active<bool>`                  | true,false                                                                                  |
| guid              | `:id<guid>`                      | CD2C1638-1638-72D5-1638-DEADBEEF1638                                                        |
| float             | `:weight<float>`                 | 1.234, -1,001.01e8                                                                          |
| minLen(value)     | `:username<minLen(4)>`           | Test (must be at least 4 characters)                                                        |
| maxLen(value)     | `:filename<maxLen(8)>`           | MyFile (must be no more than 8 characters                                                   |
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

Fiber precompiles the regex when registering routes, so regex constraints add no runtime overhead.

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
Prefix routing characters with `\\` when using the datetime constraint (`*`, `+`, `?`, `:`, `/`, `<`, `>`, `;`, `(`, `)`), to avoid misparsing.
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

Functions that are designed to make changes to the request or response are called **middleware functions**. The [Next](../api/ctx.md#next) is a **Fiber** router function, when called, executes the **next** function that **matches** the current route.

### Example of a middleware function

```go
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

`Use` method path is a **mount**, or **prefix** path, and limits middleware to only apply to any paths requested that begin with it.

:::note
Prefix matches must now end at a slash boundary (or be an exact match). For example, `/api` runs for `/api` and `/api/users` but no longer for `/apiv2`. Parameter tokens such as `:name`, `:name?`, `*`, and `+` are still expanded before this boundary check runs.
:::

### Constraints on Adding Routes Dynamically

:::caution
Adding routes dynamically after the application has started is not supported due to design and performance considerations. Make sure to define all your routes before the application starts.
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

More information about this in our [Grouping Guide](./grouping.md)
