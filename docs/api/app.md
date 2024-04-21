---
id: app
title: 🚀 App
description: The app instance conventionally denotes the Fiber application.
sidebar_position: 2
---

import Reference from '@site/src/components/reference';

## Routing

import RoutingHandler from './../partials/routing/handler.md';

### Static

Use the **Static** method to serve static files such as **images**, **CSS,** and **JavaScript**.

:::info
By default, **Static** will serve `index.html` files in response to a request on a directory.
:::

```go title="Signature"
func (app *App) Static(prefix, root string, config ...Static) Router
```

Use the following code to serve files in a directory named `./public`

```go title="Examples"
// Serve files from multiple directories
app.Static("/", "./public")

// => http://localhost:3000/hello.html
// => http://localhost:3000/js/jquery.js
// => http://localhost:3000/css/style.css

// Serve files from "./files" directory:
app.Static("/", "./files")
```

You can use any virtual path prefix \(_where the path does not actually exist in the file system_\) for files that are served by the **Static** method, specify a prefix path for the static directory, as shown below:

```go title="Examples"
app.Static("/static", "./public")

// => http://localhost:3000/static/hello.html
// => http://localhost:3000/static/js/jquery.js
// => http://localhost:3000/static/css/style.css
```

#### Config

If you want to have a little bit more control regarding the settings for serving static files. You could use the `fiber.Static` struct to enable specific settings.

| Property                                                   | Type               | Description                                                                                                                                                            | Default          |
|------------------------------------------------------------|--------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------|
| <Reference id="compress">Compress</Reference>              | `bool`             | When set to true, the server tries minimizing CPU usage by caching compressed files. This works differently than the [compress](../middleware/compress.md) middleware. | false            |
| <Reference id="byte_range">ByteRange</Reference>           | `bool`             | When set to true, enables byte range requests.                                                                                                                         | false            |
| <Reference id="browse">Browse</Reference>                  | `bool`             | When set to true, enables directory browsing.                                                                                                                          | false            |
| <Reference id="download">Download</Reference>              | `bool`             | When set to true, enables direct download.                                                                                                                             | false            |
| <Reference id="index">Index</Reference>                    | `string`           | The name of the index file for serving a directory.                                                                                                                    | "index.html"     |
| <Reference id="cache_duration">CacheDuration</Reference>   | `time.Duration`    | Expiration duration for inactive file handlers. Use a negative `time.Duration` to disable it.                                                                          | 10 * time.Second |
| <Reference id="max_age">MaxAge</Reference>                 | `int`              | The value for the `Cache-Control` HTTP-header that is set on the file response. MaxAge is defined in seconds.                                                          | 0                |
| <Reference id="modify_response">ModifyResponse</Reference> | `Handler`          | ModifyResponse defines a function that allows you to alter the response.                                                                                               | nil              |
| <Reference id="next">Next</Reference>                      | `func(c Ctx) bool` | Next defines a function to skip this middleware when returned true.                                                                                                    | nil              |

```go title="Example"
// Custom config
app.Static("/", "./public", fiber.Static{
  Compress:      true,
  ByteRange:     true,
  Browse:        true,
  Index:         "john.html",
  CacheDuration: 10 * time.Second,
  MaxAge:        3600,
})
```

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

You can define routes with a common prefix inside the common function.

```go title="Signature"
func (app *App) Route(prefix string, fn func(router Router), name ...string) Router
```

```go title="Examples"
func main() {
  app := fiber.New()

  app.Route("/test", func(api fiber.Router) {
      api.Get("/foo", handler).Name("foo") // /test/foo (name: test.foo)
      api.Get("/bar", handler).Name("bar") // /test/bar (name: test.bar)
  }, "test.")

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

Testing your application is done with the **Test** method. Use this method for creating `_test.go` files or when you need to debug your routing logic. The default timeout is `1s` if you want to disable a timeout altogether, pass `-1` as a second argument.

```go title="Signature"
func (app *App) Test(req *http.Request, msTimeout ...int) (*http.Response, error)
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

## Hooks

Hooks is a method to return [hooks](./hooks.md) property.

```go title="Signature"
func (app *App) Hooks() *Hooks
```
