---
id: routing
title: 🔌 Routing
description: >-
  Routing refers to how an application's endpoints (URIs) respond to client
  requests.
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import RoutingHandler from './../partials/routing/handler.md';

## Handlers

<RoutingHandler />

## Paths

Route paths, combined with a request method, define the endpoints at which requests can be made. Route paths can be **strings** or **string patterns**.

**Examples of route paths based on strings**

```go
// This route path will match requests to the root route, "/":
app.Get("/", func(c *fiber.Ctx) error {
      return c.SendString("root")
})

// This route path will match requests to "/about":
app.Get("/about", func(c *fiber.Ctx) error {
    return c.SendString("about")
})

// This route path will match requests to "/random.txt":
app.Get("/random.txt", func(c *fiber.Ctx) error {
    return c.SendString("random.txt")
})
```

As with the expressJs framework, the order of the route declaration plays a role.
When a request is received, the routes are checked in the order in which they are declared.

:::info
So please be careful to write routes with variable parameters after the routes that contain fixed parts, so that these variable parts do not match instead and unexpected behavior occurs.
:::

## Parameters

Route parameters are dynamic elements in the route, which are **named** or **not named segments**. This segments that are used to capture the values specified at their position in the URL. The obtained values can be retrieved using the [Params](https://fiber.wiki/context#params) function, with the name of the route parameter specified in the path as their respective keys or for unnamed parameters the character\(\*, +\) and the counter of this.

The characters :, +, and \* are characters that introduce a parameter.

Greedy parameters are indicated by wildcard\(\*\) or plus\(+\) signs.

The routing also offers the possibility to use optional parameters, for the named parameters these are marked with a final "?", unlike the plus sign which is not optional, you can use the wildcard character for a parameter range which is optional and greedy.

**Example of define routes with route parameters**

```go
// Parameters
app.Get("/user/:name/books/:title", func(c *fiber.Ctx) error {
    fmt.Fprintf(c, "%s\n", c.Params("name"))
    fmt.Fprintf(c, "%s\n", c.Params("title"))
    return nil
})
// Plus - greedy - not optional
app.Get("/user/+", func(c *fiber.Ctx) error {
    return c.SendString(c.Params("+"))
})

// Optional parameter
app.Get("/user/:name?", func(c *fiber.Ctx) error {
    return c.SendString(c.Params("name"))
})

// Wildcard - greedy - optional
app.Get("/user/*", func(c *fiber.Ctx) error {
    return c.SendString(c.Params("*"))
})

// This route path will match requests to "/v1/some/resource/name:customVerb", since the parameter character is escaped
app.Get(`/v1/some/resource/name\:customVerb`, func(c *fiber.Ctx) error {
    return c.SendString("Hello, Community")
})
```

:::info
Since the hyphen \(`-`\) and the dot \(`.`\) are interpreted literally, they can be used along with route parameters for useful purposes.
:::

:::info
All special parameter characters can also be escaped with `"\\"` and lose their value, so you can use them in the route if you want, like in the custom methods of the [google api design guide](https://cloud.google.com/apis/design/custom_methods). It's recommended to use backticks `` ` `` because in go's regex documentation, they always use backticks to make sure it is unambiguous and the escape character doesn't interfere with regex patterns in an unexpected way.
:::

```go
// http://localhost:3000/plantae/prunus.persica
app.Get("/plantae/:genus.:species", func(c *fiber.Ctx) error {
    fmt.Fprintf(c, "%s.%s\n", c.Params("genus"), c.Params("species"))
    return nil // prunus.persica
})
```

```go
// http://localhost:3000/flights/LAX-SFO
app.Get("/flights/:from-:to", func(c *fiber.Ctx) error {
    fmt.Fprintf(c, "%s-%s\n", c.Params("from"), c.Params("to"))
    return nil // LAX-SFO
})
```

Our intelligent router recognizes that the introductory parameter characters should be part of the request route in this case and can process them as such.

```go
// http://localhost:3000/shop/product/color:blue/size:xs
app.Get("/shop/product/color::color/size::size", func(c *fiber.Ctx) error {
    fmt.Fprintf(c, "%s:%s\n", c.Params("color"), c.Params("size"))
    return nil // blue:xs
})
```

In addition, several parameters in a row and several unnamed parameter characters in the route, such as the wildcard or plus character, are possible, which greatly expands the possibilities of the router for the user.

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

We have adapted the routing strongly to the express routing, but currently without the possibility of the regular expressions, because they are quite slow. The possibilities can be tested with version 0.1.7 \(express 4\) in the online [Express route tester](http://forbeslindesay.github.io/express-route-tester/).

### Constraints
Route constraints execute when a match has occurred to the incoming URL and the URL path is tokenized into route values by parameters. The feature was intorduced in `v2.37.0` and inspired by [.NET Core](https://docs.microsoft.com/en-us/aspnet/core/fundamentals/routing?view=aspnetcore-6.0#route-constraints).

:::caution
Constraints aren't validation for parameters. If constraint aren't valid for parameter value, Fiber returns **404 handler**.
:::

| Constraint        | Example                              | Example matches                                                                             |
| ----------------- | ------------------------------------ | ------------------------------------------------------------------------------------------- |
| int               | :id<int\>                            | 123456789, -123456789                                                                       |
| bool              | :active<bool\>                       | true,false                                                                                  |
| guid              | :id<guid\>                           | CD2C1638-1638-72D5-1638-DEADBEEF1638                                                        |
| float             | :weight<float\>                      | 1.234, -1,001.01e8                                                                          |
| minLen(value)     | :username<minLen(4)\>                | Test (must be at least 4 characters)                                                        |
| maxLen(value)     | :filename<maxLen(8)\>                | MyFile (must be no more than 8 characters                                                   |
| len(length)       | :filename<len(12)\>                  | somefile.txt (exactly 12 characters)                                                        |
| min(value)        | :age<min(18)\>                       | 19 (Integer value must be at least 18)                                                      |
| max(value)        | :age<max(120)\>                      | 91 (Integer value must be no more than 120)                                                 |
| range(min,max)    | :age<range(18,120)\>                 | 91 (Integer value must be at least 18 but no more than 120)                                 |
| alpha             | :name<alpha\>                        | Rick (String must consist of one or more alphabetical characters, a-z and case-insensitive) |
| datetime          | :dob<datetime(2006\\\\-01\\\\-02)\>  | 2005-11-01                                                                                  |
| regex(expression) | :date<regex(\\d{4}-\\d{2}-\\d{2})\> | 2022-08-27 (Must match regular expression)                                                  |

**Examples**

<Tabs>
<TabItem value="single-constraint" label="Single Constraint">

```go
app.Get("/:test<min(5)>", func(c *fiber.Ctx) error {
  return c.SendString(c.Params("test"))
})

// curl -X GET http://localhost:3000/12
// 12

// curl -X GET http://localhost:3000/1
// Cannot GET /1
```
</TabItem>
<TabItem value="multiple-constraints" label="Multiple Constraints">

You can use `;` for multiple constraints.
```go
app.Get("/:test<min(100);maxLen(5)>", func(c *fiber.Ctx) error {
  return c.SendString(c.Params("test"))
})

// curl -X GET http://localhost:3000/120000
// Cannot GET /120000

// curl -X GET http://localhost:3000/1
// Cannot GET /1

// curl -X GET http://localhost:3000/250
// 250
```
</TabItem>
<TabItem value="regex-constraint" label="Regex Constraint">

Fiber precompiles regex query when to register routes. So there're no performance overhead for regex constraint.
```go
app.Get(`/:date<regex(\d{4}-\d{2}-\d{2})>`, func(c *fiber.Ctx) error {
  return c.SendString(c.Params("date"))
})

// curl -X GET http://localhost:3000/125
// Cannot GET /125

// curl -X GET http://localhost:3000/test
// Cannot GET /test

// curl -X GET http://localhost:3000/2022-08-27
// 2022-08-27
```

</TabItem>
</Tabs>

:::caution
You should use `\\` before routing-specific characters when to use datetime constraint (`*`, `+`, `?`, `:`, `/`, `<`, `>`, `;`, `(`, `)`), to avoid wrong parsing.
:::

**Optional Parameter Example**

You can impose constraints on optional parameters as well.

```go
app.Get("/:test<int>?", func(c *fiber.Ctx) error {
  return c.SendString(c.Params("test"))
})
// curl -X GET http://localhost:3000/42
// 42
// curl -X GET http://localhost:3000/
//
// curl -X GET http://localhost:3000/7.0
// Cannot GET /7.0
```

## Middleware

Functions that are designed to make changes to the request or response are called **middleware functions**. The [Next](../api/ctx.md#next) is a **Fiber** router function, when called, executes the **next** function that **matches** the current route.

**Example of a middleware function**

```go
app.Use(func(c *fiber.Ctx) error {
  // Set a custom header on all responses:
  c.Set("X-Custom-Header", "Hello, World")

  // Go to next middleware:
  return c.Next()
})

app.Get("/", func(c *fiber.Ctx) error {
  return c.SendString("Hello, World!")
})
```

`Use` method path is a **mount**, or **prefix** path, and limits middleware to only apply to any paths requested that begin with it.

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
