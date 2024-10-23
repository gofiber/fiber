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

You can Mount Fiber instance using the [`app.Use`](./app.md#use) method similar to [`express`](https://expressjs.com/en/api.html#router.use).

```go title="Examples"
func main() {
    app := fiber.New()
    micro := fiber.New()
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

```go title="Examples"
func main() {
    app := fiber.New()
    one := fiber.New()
    two := fiber.New()
    three := fiber.New()

    two.Use("/three", three)
    one.Use("/two", two)
    app.Use("/one", one)
  
    one.MountPath()   // "/one"
    two.MountPath()   // "/one/two"
    three.MountPath() // "/one/two/three"
    app.MountPath()   // ""
}
```

:::caution
Mounting order is important for MountPath. If you want to get mount paths properly, you should start mounting from the deepest app.
:::

### Group

You can group routes by creating a `*Group` struct.

```go title="Signature"
func (app *App) Group(prefix string, handlers ...Handler) Router
```

```go title="Examples"
func main() {
  app := fiber.New()

  api := app.Group("/api", handler)  // /api

  v1 := api.Group("/v1", handler)   // /api/v1
  v1.Get("/list", handler)          // /api/v1/list
  v1.Get("/user", handler)          // /api/v1/user

  v2 := api.Group("/v2", handler)   // /api/v2
  v2.Get("/list", handler)          // /api/v2/list
  v2.Get("/user", handler)          // /api/v2/user

  log.Fatal(app.Listen(":3000"))
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

```go title="Examples"
func main() {
  app := fiber.New()

  // use `Route` as chainable route declaration method
  app.Route("/test").Get(func(c fiber.Ctx) error {
    return c.SendString("GET /test")
  })
  
  app.Route("/events").all(func(c fiber.Ctx) error {
    // runs for all HTTP verbs first
    // think of it as route specific middleware!
  })
  .get(func(c fiber.Ctx) error {
    return c.SendString("GET /events")
  })
  .post(func(c fiber.Ctx) error {
    // maybe add a new event...
  })
  
  // combine multiple routes
  app.Route("/v2").Route("/user").Get(func(c fiber.Ctx) error {
    return c.SendString("GET /v2/user")
  })
  
  // use multiple methods
  app.Route("/api").Get(func(c fiber.Ctx) error {
    return c.SendString("GET /api")
  }).Post(func(c fiber.Ctx) error {
    return c.SendString("POST /api")
  })

  log.Fatal(app.Listen(":3000"))
}
```

### HandlersCount

This method returns the amount of registered handlers.

```go title="Signature"
func (app *App) HandlersCount() uint32
```

### Stack

This method returns the original router stack

```go title="Signature"
func (app *App) Stack() [][]*Route
```

```go title="Examples"
var handler = func(c fiber.Ctx) error { return nil }

func main() {
    app := fiber.New()

    app.Get("/john/:age", handler)
    app.Post("/register", handler)

    data, _ := json.MarshalIndent(app.Stack(), "", "  ")
    fmt.Println(string(data))

    app.Listen(":3000")
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

This method assigns the name of latest created route.

```go title="Signature"
func (app *App) Name(name string) Router
```

```go title="Examples"
var handler = func(c fiber.Ctx) error { return nil }

func main() {
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
    fmt.Print(string(data))

    app.Listen(":3000")

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

This method gets the route by name.

```go title="Signature"
func (app *App) GetRoute(name string) Route
```

```go title="Examples"
var handler = func(c fiber.Ctx) error { return nil }

func main() {
    app := fiber.New()

    app.Get("/", handler).Name("index")
    
    data, _ := json.MarshalIndent(app.GetRoute("index"), "", "  ")
    fmt.Print(string(data))


    app.Listen(":3000")
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

This method gets all routes.

```go title="Signature"
func (app *App) GetRoutes(filterUseOption ...bool) []Route
```

When filterUseOption equal to true, it will filter the routes registered by the middleware.

```go title="Examples"
func main() {
    app := fiber.New()
    app.Post("/", func (c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    }).Name("index")
    data, _ := json.MarshalIndent(app.GetRoutes(true), "", "  ")
    fmt.Print(string(data))
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

Config returns the [app config](./fiber.md#config) as value \( read-only \).

```go title="Signature"
func (app *App) Config() Config
```

## Handler

Handler returns the server handler that can be used to serve custom [`\*fasthttp.RequestCtx`](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx) requests.

```go title="Signature"
func (app *App) Handler() fasthttp.RequestHandler
```

## ErrorHandler

Errorhandler executes the process which was defined for the application in case of errors, this is used in some cases in middlewares.

```go title="Signature"
func (app *App) ErrorHandler(ctx Ctx, err error) error
```

## NewCtxFunc

NewCtxFunc allows to customize the ctx struct as we want.

```go title="Signature"
func (app *App) NewCtxFunc(function func(app *App) CustomCtx)
```

```go title="Examples"
type CustomCtx struct {
    DefaultCtx
}

// Custom method
func (c *CustomCtx) Params(key string, defaultValue ...string) string {
    return "prefix_" + c.DefaultCtx.Params(key)
}

app := New()
app.NewCtxFunc(func(app *fiber.App) fiber.CustomCtx {
    return &CustomCtx{
        DefaultCtx: *NewDefaultCtx(app),
    }
})
// curl http://localhost:3000/123
app.Get("/:id", func(c Ctx) error {
    // use custom method - output: prefix_123
    return c.SendString(c.Params("id"))
})
```

## RegisterCustomBinder

You can register custom binders to use as [`Bind().Custom("name")`](bind.md#custom).
They should be compatible with CustomBinder interface.

```go title="Signature"
func (app *App) RegisterCustomBinder(binder CustomBinder)
```

```go title="Examples"
app := fiber.New()

// My custom binder
customBinder := &customBinder{}
// Name of custom binder, which will be used as Bind().Custom("name")
func (*customBinder) Name() string {
    return "custom"
}
// Is used in the Body Bind method to check if the binder should be used for custom mime types
func (*customBinder) MIMETypes() []string {
    return []string{"application/yaml"}
}
// Parse the body and bind it to the out interface
func (*customBinder) Parse(c Ctx, out any) error {
    // parse yaml body
    return yaml.Unmarshal(c.Body(), out)
}
// Register custom binder
app.RegisterCustomBinder(customBinder)

// curl -X POST http://localhost:3000/custom -H "Content-Type: application/yaml" -d "name: John"
app.Post("/custom", func(c Ctx) error {
    var user User
    // output: {Name:John}
    // Custom binder is used by the name
    if err := c.Bind().Custom("custom", &user); err != nil {
        return err
    }
    // ...
    return c.JSON(user)
})
// curl -X POST http://localhost:3000/normal -H "Content-Type: application/yaml" -d "name: Doe"
app.Post("/normal", func(c Ctx) error {
    var user User
    // output: {Name:Doe}
    // Custom binder is used by the mime type
    if err := c.Bind().Body(&user); err != nil {
        return err
    }
    // ...
    return c.JSON(user)
})
```

## RegisterCustomConstraint

RegisterCustomConstraint allows to register custom constraint.

```go title="Signature"
func (app *App) RegisterCustomConstraint(constraint CustomConstraint)
```

See [Custom Constraint](../guide/routing.md#custom-constraint) section for more information.

## SetTLSHandler

Use SetTLSHandler to set [ClientHelloInfo](https://datatracker.ietf.org/doc/html/rfc8446#section-4.1.2) when using TLS with Listener.

```go title="Signature"
func (app *App) SetTLSHandler(tlsHandler *TLSHandler)
```

## Test

Testing your application is done with the **Test** method. Use this method for creating `_test.go` files or when you need to debug your routing logic. The default timeout is `1s`. If you want to disable a timeout altogether, pass a `TestConfig` struct with `Timeout: -1`.

```go title="Signature"
func (app *App) Test(req *http.Request, config ...TestConfig) (*http.Response, error)
```

```go title="Examples"
// Create route with GET method for test:
app.Get("/", func(c fiber.Ctx) error {
  fmt.Println(c.BaseURL())              // => http://google.com
  fmt.Println(c.Get("X-Custom-Header")) // => hi

  return c.SendString("hello, World!")
})

// http.Request
req := httptest.NewRequest("GET", "http://google.com", nil)
req.Header.Set("X-Custom-Header", "hi")

// http.Response
resp, _ := app.Test(req)

// Do something with results:
if resp.StatusCode == fiber.StatusOK {
  body, _ := io.ReadAll(resp.Body)
  fmt.Println(string(body)) // => Hello, World!
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

This would make a Test that instantly times out,
which would always result in a "test: empty response" error.

:::

## Hooks

Hooks is a method to return [hooks](./hooks.md) property.

```go title="Signature"
func (app *App) Hooks() *Hooks
```

## RebuildTree

The RebuildTree method is designed to rebuild the route tree and enable dynamic route registration. It returns a pointer to the App instance.

```go title="Signature"
func (app *App) RebuildTree() *App
```

**Note:** Use this method with caution. It is **not** thread-safe and calling it can be very performance-intensive, so it should be used sparingly and only in development mode. Avoid using it concurrently.

### Example Usage

Here’s an example of how to define and register routes dynamically:

```go
app.Get("/define", func(c Ctx) error {  // Define a new route dynamically
    app.Get("/dynamically-defined", func(c Ctx) error {  // Adding a dynamically defined route
        return c.SendStatus(http.StatusOK)
    })

    app.RebuildTree()  // Rebuild the route tree to register the new route

    return c.SendStatus(http.StatusOK)
})
```

In this example, a new route is defined and then `RebuildTree()` is called to make sure the new route is registered and available.
