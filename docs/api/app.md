---
id: app
title: 🚀 App
description: The `App` type represents your Fiber application.
sidebar_position: 2
---

import Reference from '@site/src/components/reference';

## Helpers

### GetString

Returns `s` unchanged when [`Immutable`](./fiber.md#immutable) is disabled or `s` resides in read-only memory. Otherwise, it returns a detached copy using `strings.Clone`.

```go title="Signature"
func (app *App) GetString(s string) string
```

### GetBytes

Returns `b` unchanged when [`Immutable`](./fiber.md#immutable) is disabled or `b` resides in read-only memory. Otherwise, it returns a detached copy.

```go title="Signature"
func (app *App) GetBytes(b []byte) []byte
```

### ReloadViews

Reloads the configured view engine on demand by calling its `Load` method. Use this helper in development workflows (e.g., file watchers or debug-only routes) to pick up template changes without restarting the server. Returns an error if no view engine is configured or reloading fails.

```go title="Signature"
func (app *App) ReloadViews() error
```

```go title="Example"
app := fiber.New(fiber.Config{Views: engine})

app.Get("/dev/reload", func(c fiber.Ctx) error {
    if err := app.ReloadViews(); err != nil {
        return err
    }
    return c.SendString("Templates reloaded")
})
```

## Routing

import RoutingHandler from './../partials/routing/handler.md';

### Route Handlers

<RoutingHandler />

### Mounting

Mount another Fiber instance with [`app.Use`](./app.md#use), similar to Express's [`router.use`](https://expressjs.com/en/api.html#router.use).

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()
    micro := fiber.New()

    // Mount the micro app on the "/john" route
    app.Use("/john", micro) // GET /john/doe -> 200 OK

    micro.Get("/doe", func(c fiber.Ctx) error {
        return c.SendStatus(fiber.StatusOK)
    })

    log.Fatal(app.Listen(":3000"))
}
```

### MountPath

The `MountPath` property contains one or more path patterns on which a sub-app was mounted.

```go title="Signature"
func (app *App) MountPath() string
```

```go title="Example"
package main

import (
    "fmt"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()
    one := fiber.New()
    two := fiber.New()
    three := fiber.New()

    two.Use("/three", three)
    one.Use("/two", two)
    app.Use("/one", one)

    fmt.Println("Mount paths:")
    fmt.Println("one.MountPath():", one.MountPath())       // "/one"
    fmt.Println("two.MountPath():", two.MountPath())       // "/one/two"
    fmt.Println("three.MountPath():", three.MountPath())   // "/one/two/three"
    fmt.Println("app.MountPath():", app.MountPath())       // ""
}
```

:::caution
Mounting order is important for `MountPath`. To get mount paths properly, you should start mounting from the deepest app.
:::

### Group

You can group routes by creating a `*Group` struct.

```go title="Signature"
func (app *App) Group(prefix string, handlers ...any) Router
```

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    api := app.Group("/api", handler)  // /api

    v1 := api.Group("/v1", handler)    // /api/v1
    v1.Get("/list", handler)           // /api/v1/list
    v1.Get("/user", handler)           // /api/v1/user

    v2 := api.Group("/v2", handler)    // /api/v2
    v2.Get("/list", handler)           // /api/v2/list
    v2.Get("/user", handler)           // /api/v2/user

    log.Fatal(app.Listen(":3000"))
}

func handler(c fiber.Ctx) error {
    return c.SendString("Handler response")
}
```

### RouteChain

Returns an instance of a single route, which you can then use to handle HTTP verbs with optional middleware.

Similar to [`Express`](https://expressjs.com/en/api.html#app.route).

```go title="Signature"
func (app *App) RouteChain(path string) Register
```

<details>
<summary>Click here to see the `Register` interface</summary>

```go
type Register interface {
    All(handler any, handlers ...any) Register
    Get(handler any, handlers ...any) Register
    Head(handler any, handlers ...any) Register
    Post(handler any, handlers ...any) Register
    Put(handler any, handlers ...any) Register
    Delete(handler any, handlers ...any) Register
    Connect(handler any, handlers ...any) Register
    Options(handler any, handlers ...any) Register
    Trace(handler any, handlers ...any) Register
    Patch(handler any, handlers ...any) Register

    Add(methods []string, handler any, handlers ...any) Register

    RouteChain(path string) Register
}
```

</details>

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Use `RouteChain` as a chainable route declaration method
    app.RouteChain("/test").Get(func(c fiber.Ctx) error {
        return c.SendString("GET /test")
    })

    app.RouteChain("/events").All(func(c fiber.Ctx) error {
        // Runs for all HTTP verbs first
        // Think of it as route-specific middleware!
    }).
    Get(func(c fiber.Ctx) error {
        return c.SendString("GET /events")
    }).
    Post(func(c fiber.Ctx) error {
        // Maybe add a new event...
        return c.SendString("POST /events")
    })

    // Combine multiple routes
    app.RouteChain("/reports").RouteChain("/daily").Get(func(c fiber.Ctx) error {
        return c.SendString("GET /reports/daily")
    })

    // Use multiple methods
    app.RouteChain("/api").Get(func(c fiber.Ctx) error {
        return c.SendString("GET /api")
    }).Post(func(c fiber.Ctx) error {
        return c.SendString("POST /api")
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Route

Defines routes with a common prefix inside the supplied function. Internally it uses [`Group`](#group) to create a sub-router and accepts an optional name prefix.

```go title="Signature"
func (app *App) Route(prefix string, fn func(router Router), name ...string) Router
```

```go title="Example"
app.Route("/test", func(api fiber.Router) {
    api.Get("/foo", handler).Name("foo") // /test/foo (name: test.foo)
    api.Get("/bar", handler).Name("bar") // /test/bar (name: test.bar)
}, "test.")
```

### Domain

Creates a router scoped to a specific hostname pattern. Routes registered through the returned `Router` only match requests whose `Host` header matches the pattern. Domain names are matched case-insensitively per [RFC 4343](https://www.rfc-editor.org/rfc/rfc4343).

The pattern can contain parameters prefixed with `:`. Use [`DomainParam`](#domainparam) to retrieve them inside handlers.

Domain routing has **zero performance impact** on routes that don't use it — the hostname check is applied as a handler wrapper, not a change to the core router.

```go title="Signature"
func (app *App) Domain(host string) Router
```

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Static domain — only matches requests to api.example.com
    app.Domain("api.example.com").Get("/users", func(c fiber.Ctx) error {
        return c.SendString("API users list")
    })

    // Domain with parameter
    app.Domain(":user.blog.example.com").Get("/", func(c fiber.Ctx) error {
        user := fiber.DomainParam(c, "user")
        return c.SendString(user + "'s blog")
    })

    // Composable with groups and middleware
    admin := app.Domain("admin.example.com")
    admin.Use(func(c fiber.Ctx) error {
        // Only runs for admin.example.com
        c.Set("X-Admin", "true")
        return c.Next()
    })
    admin.Get("/dashboard", func(c fiber.Ctx) error {
        return c.SendString("Admin Dashboard")
    })

    // Fallback for unmatched domains
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Default site")
    })

    log.Fatal(app.Listen(":3000"))
}
```

#### DomainParam

Returns the value of a domain parameter captured by a [`Domain`](#domain) pattern. If the key is not found, the optional default value is returned.

```go title="Signature"
func DomainParam(c Ctx, key string, defaultValue ...string) string
```

```go title="Example"
// Pattern: ":tenant.example.com"
// Request Host: acme.example.com

app.Domain(":tenant.example.com").Get("/", func(c fiber.Ctx) error {
    tenant := fiber.DomainParam(c, "tenant")           // "acme"
    missing := fiber.DomainParam(c, "missing", "none") // "none"
    return c.SendString(tenant + " " + missing)
})
```

### HandlersCount

Returns the number of registered handlers.

```go title="Signature"
func (app *App) HandlersCount() uint32
```

### Stack

Returns the underlying router stack.

```go title="Signature"
func (app *App) Stack() [][]*Route
```

```go title="Example"
package main

import (
    "encoding/json"
    "log"

    "github.com/gofiber/fiber/v3"
)

var handler = func(c fiber.Ctx) error { return nil }

func main() {
    app := fiber.New()

    app.Get("/john/:age", handler)
    app.Post("/register", handler)

    data, _ := json.MarshalIndent(app.Stack(), "", "  ")
    fmt.Println(string(data))

    log.Fatal(app.Listen(":3000"))
}
```

<details>
<summary>Click here to see the result</summary>

```json
[
  [
    {
      "method": "GET",
      "path": "/john/:age",
      "params": [
        "age"
      ]
    }
  ],
  [
    {
      "method": "HEAD",
      "path": "/john/:age",
      "params": [
        "age"
      ]
    }
  ],
  [
    {
      "method": "POST",
      "path": "/register",
      "params": null
    }
  ]
]
```

</details>

### Name

This method assigns the name to the latest created route.

```go title="Signature"
func (app *App) Name(name string) Router
```

```go title="Example"
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    var handler = func(c fiber.Ctx) error { return nil }

    app := fiber.New()

    app.Get("/", handler)
    app.Name("index")
    app.Get("/doe", handler).Name("home")
    app.Trace("/tracer", handler).Name("tracert")
    app.Delete("/delete", handler).Name("delete")

    a := app.Group("/a")
    a.Name("fd.")

    a.Get("/test", handler).Name("test")

    data, _ := json.MarshalIndent(app.Stack(), "", "  ")
    fmt.Println(string(data))

    log.Fatal(app.Listen(":3000"))
}
```

<details>
<summary>Click here to see the result</summary>

```json
[
  [
    {
      "method": "GET",
      "name": "index",
      "path": "/",
      "params": null
    },
    {
      "method": "GET",
      "name": "home",
      "path": "/doe",
      "params": null
    },
    {
      "method": "GET",
      "name": "fd.test",
      "path": "/a/test",
      "params": null
    }
  ],
  [
    {
      "method": "HEAD",
      "name": "",
      "path": "/",
      "params": null
    },
    {
      "method": "HEAD",
      "name": "",
      "path": "/doe",
      "params": null
    },
    {
      "method": "HEAD",
      "name": "",
      "path": "/a/test",
      "params": null
    }
  ],
  null,
  null,
  [
    {
      "method": "DELETE",
      "name": "delete",
      "path": "/delete",
      "params": null
    }
  ],
  null,
  null,
  [
    {
      "method": "TRACE",
      "name": "tracert",
      "path": "/tracer",
      "params": null
    }
  ],
  null
]
```

</details>

### GetRoute

This method retrieves a route by its name.

```go title="Signature"
func (app *App) GetRoute(name string) Route
```

```go title="Example"
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/", handler).Name("index")

    route := app.GetRoute("index")

    data, _ := json.MarshalIndent(route, "", "  ")
    fmt.Println(string(data))

    log.Fatal(app.Listen(":3000"))
}
```

<details>
<summary>Click here to see the result</summary>

```json
{
  "method": "GET",
  "name": "index",
  "path": "/",
  "params": null
}
```

</details>

### GetRoutes

This method retrieves all routes.

```go title="Signature"
func (app *App) GetRoutes(filterUseOption ...bool) []Route
```

When `filterUseOption` is set to `true`, it filters out routes registered by middleware.

```go title="Example"
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Post("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    }).Name("index")

    routes := app.GetRoutes(true)

    data, _ := json.MarshalIndent(routes, "", "  ")
    fmt.Println(string(data))

    log.Fatal(app.Listen(":3000"))
}
```

<details>
<summary>Click here to see the result</summary>

```json
[
    {
        "method": "POST",
        "name": "index",
        "path": "/",
        "params": null
    }
]
```

</details>

## Config

`Config` returns the [app config](./fiber.md#config) as a value (read-only).

```go title="Signature"
func (app *App) Config() Config
```

## Handler

`Handler` returns the server handler that can be used to serve custom [`\*fasthttp.RequestCtx`](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx) requests.

```go title="Signature"
func (app *App) Handler() fasthttp.RequestHandler
```

## ErrorHandler

`ErrorHandler` executes the process defined for the application in case of errors. This is used in some cases in middlewares.

```go title="Signature"
func (app *App) ErrorHandler(ctx Ctx, err error) error
```

## NewWithCustomCtx

`NewWithCustomCtx` creates a new `*App` and sets the custom context factory
function at construction time.

```go title="Signature"
func NewWithCustomCtx(fn func(app *App) CustomCtx, config ...Config) *App
```

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

type CustomCtx struct {
    fiber.DefaultCtx
}

func (c *CustomCtx) Params(key string, defaultValue ...string) string {
    return "prefix_" + c.DefaultCtx.Params(key)
}

func main() {
    app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
        return &CustomCtx{
            DefaultCtx: *fiber.NewDefaultCtx(app),
        }
    })

    app.Get("/:id", func(c fiber.Ctx) error {
        return c.SendString(c.Params("id"))
    })

    log.Fatal(app.Listen(":3000"))
}
```

## RegisterCustomBinder

You can register custom binders to use with [`Bind().Custom("name")`](bind.md#custom). They should be compatible with the `CustomBinder` interface.

```go title="Signature"
func (app *App) RegisterCustomBinder(binder CustomBinder)
```

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "gopkg.in/yaml.v2"
)

type User struct {
    Name string `yaml:"name"`
}

type customBinder struct{}

func (*customBinder) Name() string {
    return "custom"
}

func (*customBinder) MIMETypes() []string {
    return []string{"application/yaml"}
}

func (*customBinder) Parse(c fiber.Ctx, out any) error {
    // Parse YAML body
    return yaml.Unmarshal(c.Body(), out)
}

func main() {
    app := fiber.New()

    // Register custom binder
    app.RegisterCustomBinder(&customBinder{})

    app.Post("/custom", func(c fiber.Ctx) error {
        var user User
        // Use Custom binder by name
        if err := c.Bind().Custom("custom", &user); err != nil {
            return err
        }
        return c.JSON(user)
    })

    app.Post("/normal", func(c fiber.Ctx) error {
        var user User
        // Custom binder is used by the MIME type
        if err := c.Bind().Body(&user); err != nil {
            return err
        }
        return c.JSON(user)
    })

    log.Fatal(app.Listen(":3000"))
}
```

## RegisterCustomConstraint

`RegisterCustomConstraint` allows you to register custom constraints.

```go title="Signature"
func (app *App) RegisterCustomConstraint(constraint CustomConstraint)
```

See the [Custom Constraint](../guide/routing.md#custom-constraint) section for more information.

## SetTLSHandler

Use `SetTLSHandler` to set [`ClientHelloInfo`](https://datatracker.ietf.org/doc/html/rfc8446#section-4.1.2) when using TLS with a `Listener`.

```go title="Signature"
func (app *App) SetTLSHandler(tlsHandler *TLSHandler)
```

## Test

Testing your application is done with the `Test` method. Use this method for creating `_test.go` files or when you need to debug your routing logic. The default timeout is `1s`; to disable a timeout altogether, pass a `TestConfig` struct with `Timeout: 0`.

```go title="Signature"
func (app *App) Test(req *http.Request, config ...TestConfig) (*http.Response, error)
```

```go title="Example"
package main

import (
    "fmt"
    "io"
    "log"
    "net/http"
    "net/http/httptest"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Create route with GET method for test:
    app.Get("/", func(c fiber.Ctx) error {
        fmt.Println(c.BaseURL())              // => http://google.com
        fmt.Println(c.Get("X-Custom-Header")) // => hi
        return c.SendString("hello, World!")
    })

    // Create http.Request
    req := httptest.NewRequest("GET", "http://google.com", nil)
    req.Header.Set("X-Custom-Header", "hi")

    // Perform the test
    resp, _ := app.Test(req)

    // Do something with the results:
    if resp.StatusCode == fiber.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        fmt.Println(string(body)) // => hello, World!
    }
}
```

If not provided, TestConfig is set to the following defaults:

```go title="Default TestConfig"
config := fiber.TestConfig{
  Timeout:      time.Second,
  FailOnTimeout: true,
}
```

:::caution

This is **not** the same as supplying an empty `TestConfig{}` to
`app.Test(), but rather be the equivalent of supplying:

```go title="Empty TestConfig"
cfg := fiber.TestConfig{
  Timeout:      0,
  FailOnTimeout: false,
}
```

This would make a Test that has no timeout.

:::

## Hooks

`Hooks` is a method to return the [hooks](./hooks.md) property.

```go title="Signature"
func (app *App) Hooks() *Hooks
```

## RebuildTree

The `RebuildTree` method is designed to rebuild the route tree and enable dynamic route registration. It returns a pointer to the `App` instance.

```go title="Signature"
func (app *App) RebuildTree() *App
```

**Note:** Use this method with caution. It is **not** thread-safe and calling it can be very performance-intensive, so it should be used sparingly and only in development mode. Avoid using it concurrently.

### Example Usage

Here’s an example of how to define and register routes dynamically:

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/define", func(c fiber.Ctx) error {
        // Define a new route dynamically
        app.Get("/dynamically-defined", func(c fiber.Ctx) error {
            return c.SendStatus(fiber.StatusOK)
        })

        // Rebuild the route tree to register the new route
        app.RebuildTree()

        return c.SendStatus(fiber.StatusOK)
    })

    log.Fatal(app.Listen(":3000"))
}
```

In this example, a new route is defined and then `RebuildTree()` is called to ensure the new route is registered and available.

## RemoveRoute

This method removes a route by path. You must call the `RebuildTree()` method after the removal to finalize the update and rebuild the routing tree.
If no methods are specified, the route will be removed for all HTTP methods defined in the app. To limit removal to specific methods, provide them as additional arguments.

```go title="Signature"
func (app *App) RemoveRoute(path string, methods ...string)
```

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/api/feature-a", func(c fiber.Ctx) error {
           app.RemoveRoute("/api/feature", fiber.MethodGet)
           app.RebuildTree()
           // Redefine route
           app.Get("/api/feature", func(c fiber.Ctx) error {
                   return c.SendString("Testing feature-a")
           })

           app.RebuildTree()
           return c.SendStatus(fiber.StatusOK)
    })
    app.Get("/api/feature-b", func(c fiber.Ctx) error {
           app.RemoveRoute("/api/feature", fiber.MethodGet)
           app.RebuildTree()
           // Redefine route
           app.Get("/api/feature", func(c fiber.Ctx) error {
                   return c.SendString("Testing feature-b")
           })

           app.RebuildTree()
           return c.SendStatus(fiber.StatusOK)
    })

    log.Fatal(app.Listen(":3000"))
}
```

## RemoveRouteByName

This method removes a route by name.
If no methods are specified, the route will be removed for all HTTP methods defined in the app. To limit removal to specific methods, provide them as additional arguments.

```go title="Signature"
func (app *App) RemoveRouteByName(name string, methods ...string)
```

## RemoveRouteFunc

This method removes a route by function having `*Route` parameter.
If no methods are specified, the route will be removed for all HTTP methods defined in the app. To limit removal to specific methods, provide them as additional arguments.

```go title="Signature"
func (app *App) RemoveRouteFunc(matchFunc func(r *Route) bool, methods ...string)
```
