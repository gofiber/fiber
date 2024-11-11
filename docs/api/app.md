---
id: app
title: 🚀 App
description: The app instance conventionally denotes the Fiber application.
sidebar_position: 2
---

import Reference from '@site/src/components/reference';

## Routing

import RoutingHandler from './../partials/routing/handler.md';

### Route Handlers

<RoutingHandler />

### Mounting

You can mount a Fiber instance using the [`app.Use`](./app.md#use) method, similar to [`Express`](https://expressjs.com/en/api.html#router.use).

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
func (app *App) Group(prefix string, handlers ...Handler) Router
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

### Route

Returns an instance of a single route, which you can then use to handle HTTP verbs with optional middleware.

Similar to [`Express`](https://expressjs.com/de/api.html#app.route).

```go title="Signature"
func (app *App) Route(path string) Register
```

<details>
<summary>Click here to see the `Register` interface</summary>

```go
type Register interface {
    All(handler Handler, middleware ...Handler) Register
    Get(handler Handler, middleware ...Handler) Register
    Head(handler Handler, middleware ...Handler) Register
    Post(handler Handler, middleware ...Handler) Register
    Put(handler Handler, middleware ...Handler) Register
    Delete(handler Handler, middleware ...Handler) Register
    Connect(handler Handler, middleware ...Handler) Register
    Options(handler Handler, middleware ...Handler) Register
    Trace(handler Handler, middleware ...Handler) Register
    Patch(handler Handler, middleware ...Handler) Register

    Add(methods []string, handler Handler, middleware ...Handler) Register

    Route(path string) Register
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

    // Use `Route` as a chainable route declaration method
    app.Route("/test").Get(func(c fiber.Ctx) error {
        return c.SendString("GET /test")
    })

    app.Route("/events").All(func(c fiber.Ctx) error {
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
    app.Route("/v2").Route("/user").Get(func(c fiber.Ctx) error {
        return c.SendString("GET /v2/user")
    })

    // Use multiple methods
    app.Route("/api").Get(func(c fiber.Ctx) error {
        return c.SendString("GET /api")
    }).Post(func(c fiber.Ctx) error {
        return c.SendString("POST /api")
    })

    log.Fatal(app.Listen(":3000"))
}
```

### HandlersCount

This method returns the number of registered handlers.

```go title="Signature"
func (app *App) HandlersCount() uint32
```

### Stack

This method returns the original router stack.

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

## NewCtxFunc

`NewCtxFunc` allows you to customize the `ctx` struct as needed.

```go title="Signature"
func (app *App) NewCtxFunc(function func(app *App) CustomCtx)
```

```go title="Example"
package main

import (
    "fmt"

    "github.com/gofiber/fiber/v3"
)

type CustomCtx struct {
    fiber.DefaultCtx
}

// Custom method
func (c *CustomCtx) Params(key string, defaultValue ...string) string {
    return "prefix_" + c.DefaultCtx.Params(key)
}

func main() {
    app := fiber.New()

    app.NewCtxFunc(func(app *fiber.App) fiber.CustomCtx {
        return &CustomCtx{
            DefaultCtx: *fiber.NewDefaultCtx(app),
        }
    })

    app.Get("/:id", func(c fiber.Ctx) error {
        // Use custom method - output: prefix_123
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
  Timeout:      time.Second(),
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

This method removes a route by path.  You must call the `RebuildTree()` method after the remove in to ensure the route is removed.

```go title="Signature"
func (app *App) RemoveRoute(path string, methods ...string)
```

This method removes a route by name
```go title="Signature"
func (app *App) RemoveRouteByName(name string, methods ...string)
```

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/api/feature-a", func(c *fiber.Ctx) error {
           app.RemoveRoute("/api/feature", fiber.MethodGet)
           app.RebuildTree()
           // Redefine route
           app.Get("/api/feature", func(c *fiber.Ctx) error {
                   return c.SendString("Testing feature-a")
           })

           app.RebuildTree()
           return c.SendStatus(fiber.StatusOK)
    })
    app.Get("/api/feature-b", func(c *fiber.Ctx) error {
           app.RemoveRoute("/api/feature", fiber.MethodGet)
           app.RebuildTree()
           // Redefine route
           app.Get("/api/feature", func(c *fiber.Ctx) error {
                   return c.SendString("Testing feature-b")
           })

           app.RebuildTree()
           return c.SendStatus(fiber.StatusOK)
    })

    log.Fatal(app.Listen(":3000"))
}
```
