---
id: app
title: 🚀 App
description: The `App` type represents your Fiber application.
sidebar_position: 2
toc_max_heading_level: 4
---

import Reference from '@site/src/components/reference';
import MethodIndex from '@site/src/components/method-index';

Use the index to jump straight to any `App` method; filter by name or by category:

<MethodIndex itemNoun="methods" placeholder="Filter methods, e.g. route" />

## Routing

import RoutingHandler from './../partials/routing/handler.md';
import RoutingUse from './../partials/routing/use.md';

### Route Handlers

<RoutingHandler />

Beyond the native `func(fiber.Ctx)` forms, Fiber also adapts Express-style, `net/http`, and `fasthttp` handlers. See [Handler types](../guide/routing.md#handler-types) in the routing guide for the full list of supported shapes.

### Use

<RoutingUse />

### Mounting

Mount another Fiber instance with [`app.Use`](#use), similar to Express's [`router.use`](https://expressjs.com/en/api.html#router.use).

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

:::caution
Unlike Express, Fiber does not strip the mount prefix. Inside the mounted app, `c.Path()` still returns the full request path (`/john/doe`, not `/doe`). The counterpart of `req.baseUrl` is [`c.MountPath()`](./ctx.md#mountpath), which returns the prefix the running route's sub-app was mounted under (`/john`).
:::

Passing a slice of prefixes mounts the sub-app under each of them (`app.Use([]string{"/john", "/jane"}, micro)`). Mounting an app onto itself panics.

:::note
The routes of a mounted app are matched under the settings of the top-level app, at every nesting depth: its [`CaseSensitive`](./fiber.md#casesensitive), [`StrictRouting`](./fiber.md#strictrouting) and [`UnescapePath`](./fiber.md#unescapepath) apply, not the sub-app's own and not those of the app it is mounted on. A sub-app that relies on one of them for its routes needs the top-level app to set it too. The settings that handlers read at request time, such as `JSONEncoder`, `StructValidator` and `PassLocalsToViews`, and those of the server itself, such as `BodyLimit`, come from the top-level app as well.

A sub-app's own [`ErrorHandler`](#errorhandler), `Views` and `ViewsLayout` still apply to the requests under its mount path, but only when the whole mount path, from the top-level app down, is literal. Under a parameter, as in `app.Use("/t/:tenant", tenantApp)`, the routes match and run while the nearest enclosing mount with a literal path, or the top-level app, supplies the error handler and the view engine. A [`Domain`](#domain) mount matches a parameterized path as a pattern, so its sub-app keeps its own. The [`RegexHandler`](./fiber.md#regexhandler) of a sub-app mounted on the top-level app compiles the `regex()` constraints in the full path of every route below it, the mount prefix included, so an app mounted deeper does not use its own.
:::

### MountPath

`MountPath` returns the path pattern on which a sub-app was mounted, or an empty string for an app that is not mounted. An app mounted under several prefixes reports the last one.

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
    Query(handler any, handlers ...any) Register

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
        return c.Next()
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

Creates a router scoped to a specific hostname pattern. Routes registered through the returned `Router` only match requests whose hostname (from `c.Hostname()`) matches the pattern. Domain names are matched case-insensitively per [RFC 4343](https://www.rfc-editor.org/rfc/rfc4343).

When `TrustProxy` is enabled and the proxy is trusted, the hostname may be derived from the `X-Forwarded-Host` header instead of the `Host` header. To prevent header spoofing, you must both enable `TrustProxy` and configure [`TrustProxyConfig`](https://docs.gofiber.io/api/fiber#trustproxyconfig) with the IPs or ranges of your trusted proxies. See the [TrustProxy documentation](https://docs.gofiber.io/api/fiber#trustproxy) for details.

The pattern can contain parameters prefixed with `:`. Use [`DomainParam`](#domainparam) to retrieve them inside handlers. `Domain` panics when the pattern is empty, has an empty label, has more than 16 labels or 253 characters, has a constant label of more than 63 characters, or has a label with a character other than an ASCII letter, a digit or `-` (a parameter name may also contain `_`). A request hostname that breaks the same limits never matches.

Groups offer `Domain` too: `api.Domain("api.example.com")` keeps the group prefix and adds the host filter.

Routes for one path registered through different `Domain` patterns stay separate routes, each with its own name and automatic `HEAD` route, even when they are registered one after another; [`c.Endpoint()`](./ctx.md#endpoint) reports a `Domain` route only for a hostname its pattern matches.

Domain routing has **zero performance impact** on routes that don't use it — the hostname check is applied as a handler wrapper, not a change to the core router.

:::note
Because domain filtering is applied at handler-execution time (not during route matching), Fiber's `405 Method Not Allowed` logic may advertise methods from domain-scoped routes even when the requesting host does not match the domain pattern. This is a known trade-off of the handler-wrapping approach — it avoids core router changes while keeping non-domain routes unaffected.
:::

:::note
When mounting sub-applications via `Domain(...).Use(*fiber.App)`, routes are cloned from the sub-app at mount time. This means the same sub-app can safely be mounted on multiple domains without double-wrapping, but routes registered on the sub-app **after** mounting will not inherit domain filtering. Register all sub-app routes before mounting.

Apps that the sub-app has mounted itself are cloned along with it, so nested routes are domain-filtered as well. The same "register before mounting" rule applies to them.

A domain-mounted sub-app's `ErrorHandler` and `Views` are host-scoped like its routes: they apply to requests whose hostname matches the pattern, and requests on other hosts fall back to the parent's. Two sub-apps mounted at the same path on different domains therefore keep their own configuration. A sub-app that sets none of its own inherits them from the app it is mounted inside, as with a plain mount; a separately mounted app that merely covers the same paths and hosts is not inherited from.
:::

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

    // Mount sub-applications on domain routers
    subApp := fiber.New()
    subApp.Get("/users", func(c fiber.Ctx) error {
        return c.SendString("Users list")
    })
    app.Domain("api.example.com").Use("/api", subApp)

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

Returns the underlying router stack: one slice of routes for each entry of [`RequestMethods`](./fiber.md#requestmethods), in that order, and `nil` for a method that has no routes. The `HEAD` routes Fiber generates for `GET` routes only appear once the app has started or [`RebuildTree`](#rebuildtree) has run.

```go title="Signature"
func (app *App) Stack() [][]*Route
```

```go title="Example"
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
)

var handler = func(c fiber.Ctx) error { return nil }

func main() {
    app := fiber.New()

    app.Get("/john/:age", handler)
    app.Post("/register", handler)

    // Listen generates the HEAD routes when it starts the app.
    // RebuildTree generates them now, so the stack below shows them.
    app.RebuildTree()

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
      "params": [
        "age"
      ],
      "method": "GET",
      "name": "",
      "path": "/john/:age"
    }
  ],
  [
    {
      "params": [
        "age"
      ],
      "method": "HEAD",
      "name": "",
      "path": "/john/:age"
    }
  ],
  [
    {
      "params": null,
      "method": "POST",
      "name": "",
      "path": "/register"
    }
  ],
  null,
  null,
  null,
  null,
  null,
  null,
  null
]
```

</details>

### Name

This method assigns the name to the latest created route. When the latest call registered several methods at once, as `Add` or `All` do, every one of those routes receives the name. The `HEAD` route that Fiber generates for a named `GET` route carries the same name.

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

    // Generate the HEAD routes now, as Listen does when it starts the app.
    app.RebuildTree()

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
      "params": null,
      "method": "GET",
      "name": "index",
      "path": "/"
    },
    {
      "params": null,
      "method": "GET",
      "name": "home",
      "path": "/doe"
    },
    {
      "params": null,
      "method": "GET",
      "name": "fd.test",
      "path": "/a/test"
    }
  ],
  [
    {
      "params": null,
      "method": "HEAD",
      "name": "index",
      "path": "/"
    },
    {
      "params": null,
      "method": "HEAD",
      "name": "home",
      "path": "/doe"
    },
    {
      "params": null,
      "method": "HEAD",
      "name": "fd.test",
      "path": "/a/test"
    }
  ],
  null,
  null,
  [
    {
      "params": null,
      "method": "DELETE",
      "name": "delete",
      "path": "/delete"
    }
  ],
  null,
  null,
  [
    {
      "params": null,
      "method": "TRACE",
      "name": "tracert",
      "path": "/tracer"
    }
  ],
  null,
  null
]
```

</details>

### GetRoute

This method retrieves a route by its name.

The returned `Route` can be inspected or used to generate a URL directly with `route.URL(params)`. When no route has that name, `GetRoute` returns the zero `Route`, and its `URL` method returns `ErrNotFound`.

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

var handler = func(c fiber.Ctx) error { return nil }

func main() {
    app := fiber.New()

    app.Get("/", handler).Name("index")
    app.Get("/user/:name/:id", handler).Name("user")

    route := app.GetRoute("index")

    data, _ := json.MarshalIndent(route, "", "  ")
    fmt.Println(string(data))

    userRoute := app.GetRoute("user")
    location, _ := userRoute.URL(fiber.Map{"name": "john", "id": 1})
    fmt.Println(location) // /user/john/1

    log.Fatal(app.Listen(":3000"))
}
```

<details>
<summary>Click here to see the result</summary>

```json
{
  "params": null,
  "method": "GET",
  "name": "index",
  "path": "/"
}
```

</details>

:::note
`Route.URL` automatically percent-encodes parameter values with URL
path-segment rules. Greedy parameters (`*` and `+`) preserve `/` inside the
matched path tail; delimiters such as `?` and `#` remain parameter data instead
of restructuring the generated URL. [`GetRouteURL`](./ctx.md#getrouteurl) and
[`Redirect().Route`](./redirect.md#route) apply the same encoding.

These take the value as data, not as URL text. Representable values use the same
encoding under any configuration. With `UnescapePath` off (the default) `c.Params`
returns the value still percent-encoded, so forwarding it straight back encodes
the `%` a second time. Turn `UnescapePath` on, or decode with
[`url.PathUnescape`](https://pkg.go.dev/net/url#PathUnescape) first.
An app with `UnescapePath` enabled decodes `%2F` before route matching, so an
ordinary single-segment parameter holding a `/` cannot round-trip and returns
`ErrRouteNotRepresentable`. Greedy (`*` or `+`), adjacent single-byte parameters
and required parameters with a single-byte non-slash terminator retain their
existing slash-matching rules. An optional parameter with such a terminator does
not match a `/`, so a decoded `/` in its value returns `ErrRouteNotRepresentable`.
Dot-containing values are also rejected when the composed path has a `.` or
`..` segment that would be removed during normalization. Surrounding route
constants are considered: `:name.txt` with `name="."` remains representable.
Pass a different value or use a route that can represent it; do not pre-encode it.
:::

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
    "params": null,
    "method": "POST",
    "name": "index",
    "path": "/"
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

`Handler` returns the server handler that can be used to serve custom [`*fasthttp.RequestCtx`](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx) requests.

```go title="Signature"
func (app *App) Handler() fasthttp.RequestHandler
```

:::caution
When Fiber starts its own server, it closes the connection after the response to a request that carries both `Content-Length` and `Transfer-Encoding`, using a `HeaderReceived` callback it installs on that server. A `fasthttp.Server` of your own that serves `Handler()` does not get that callback and needs the same check in its own `HeaderReceived`. See [Protocol error responses](../whats_new.md#protocol-error-responses).
:::

## AcquireCtx / ReleaseCtx

`AcquireCtx` takes a context from the app's pool and binds it to a [`*fasthttp.RequestCtx`](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx); `ReleaseCtx` puts it back. Use them when you run a handler yourself instead of going through [`Handler`](#handler), as the [adaptor middleware](../middleware/adaptor.md) does.

```go title="Signature"
func (app *App) AcquireCtx(ctx *fasthttp.RequestCtx) CustomCtx
func (app *App) ReleaseCtx(c CustomCtx)
```

```go title="Example"
func serve(app *fiber.App, ctx *fasthttp.RequestCtx, handler fiber.Handler) {
    c := app.AcquireCtx(ctx)
    defer app.ReleaseCtx(c)

    if err := handler(c); err != nil {
        _ = app.ErrorHandler(c, err)
    }
}
```

Release every context you acquire, and don't use it afterwards: the next request reuses it. `ReleaseCtx` does not return an abandoned context (see [`Abandon`](./ctx.md#abandon)) to the pool, which is how the [timeout middleware](../middleware/timeout.md) keeps a context away from the pool while its handler is still running. When [`ScheduleReclaim`](./ctx.md#abandon) armed the context, the call only tells the reclaimer that the request handler is done, and the reclaimer pools it once the other goroutine finishes too.

## ErrorHandler

`ErrorHandler` executes the process defined for the application in case of errors. This is used in some cases in middlewares.

```go title="Signature"
func (app *App) ErrorHandler(ctx Ctx, err error) error
```

When the request path falls under a mounted sub-app that sets its own `ErrorHandler`, that handler runs instead of the parent's. The deepest mount covering the path wins, and a sub-app that sets none inherits the handler of the nearest mount around it. The mount path, taken from the top-level app down, is compared with the request path as literal text, so a sub-app mounted under a parameter such as `/t/:tenant` is never found, nor is an app mounted inside it: their routes run, but the nearest enclosing mount with a literal path, or the top-level app, supplies the handler. A sub-app mounted through [`Domain`](#domain) matches a parameterized path as a pattern, and contributes its handler only for requests whose hostname matches the pattern.

`ErrorHandler` also runs for a request that the HTTP server refuses while reading it, with a `*fiber.Error` carrying the status Fiber chose, such as `413` for a body over `BodyLimit` or `431` for headers over `ReadBufferSize`. See [Errors Before a Handler Runs](../guide/error-handling.md#errors-before-a-handler-runs) for the full list. When the request method is a known one, middleware registered with `Use` still runs for such a request and sees that status.

A request whose response fasthttp already holds as a timeout response, as after the [timeout middleware](../middleware/timeout.md) has answered it, is left alone: `ErrorHandler` returns `nil` without running the configured handler, since anything written now would be ignored and the handler that timed out may still be using the context. The error still reaches the outer middleware.

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

`NewWithCustomCtx` works with global and route middleware. Fiber passes your custom context through `c.Next()`, so overridden methods (for example a custom `JSON`) still run after `app.Use(...)`.

```go title="Custom JSON with middleware"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

type APICtx struct {
    fiber.DefaultCtx
}

type envelope struct {
    Code    int    `json:"code"`
    Data    any    `json:"data"`
    Message string `json:"message"`
}

func (c *APICtx) JSON(data any, ctype ...string) error {
    return c.DefaultCtx.JSON(envelope{
        Code:    fiber.StatusOK,
        Data:    data,
        Message: "OK",
    }, ctype...)
}

func main() {
    app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
        return &APICtx{DefaultCtx: *fiber.NewDefaultCtx(app)}
    })

    app.Use(func(c fiber.Ctx) error {
        return c.Next()
    })

    app.Get("/vvv", func(c fiber.Ctx) error {
        // Uses APICtx.JSON even though middleware called Next
        return c.JSON(fiber.Map{"a": "b"})
    })

    log.Fatal(app.Listen(":3000"))
}
```

:::tip
Prefer embedding `fiber.DefaultCtx` (by value or pointer) and constructing it with `fiber.NewDefaultCtx(app)`. Overridden methods are resolved on the concrete `CustomCtx` type Fiber stores for the request.
:::

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

`Listen` sets a handler up itself when it serves `CertFile` and `CertKeyFile`. With `Listener`, register a `TLSHandler` and use its `GetClientInfo` method as the `GetCertificate` callback of the `tls.Config` the listener runs with. [`Ctx.ClientHelloInfo`](./ctx.md#clienthelloinfo) then returns the hello message of the connection a request arrived on. It returns `nil` where `crypto/tls` does not call `GetCertificate`: for a resumed TLS 1.3 connection, whatever the configuration, and, with `Certificates` set, as in the example below and in the config `Listen` builds, for a client that sends no server name (SNI).

```go title="Example"
cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
if err != nil {
    log.Fatal(err)
}

handler := &fiber.TLSHandler{}
app.SetTLSHandler(handler)

ln, err := tls.Listen("tcp", ":443", &tls.Config{
    MinVersion:     tls.VersionTLS12,
    Certificates:   []tls.Certificate{cert},
    GetCertificate: handler.GetClientInfo, // records the ClientHelloInfo of each connection
})
if err != nil {
    log.Fatal(err)
}

app.Get("/", func(c fiber.Ctx) error {
    if info := c.ClientHelloInfo(); info != nil {
        return c.SendString(info.ServerName)
    }
    return c.SendStatus(fiber.StatusNoContent)
})

log.Fatal(app.Listener(ln))
```

## State / SharedState

`State()` returns in-process state (local to the current process).  
`SharedState()` returns storage-backed state intended for prefork/multi-process sharing.

```go title="Signature"
func (app *App) State() *State
func (app *App) SharedState() *SharedState
```

See [State Management](./state.md) for usage and examples.

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

When the handler has not answered within `Timeout`, `Test` returns `os.ErrDeadlineExceeded` if `FailOnTimeout` is `true`. If it is `false`, `Test` waits up to one more second and then reads whatever the handler has written by then. When the handler sends no response at all, for example after `c.Drop()`, `Test` returns `fiber.ErrTestGotEmptyResponse`.

:::caution

Calling `app.Test(req)` uses the defaults above. Supplying an empty `fiber.TestConfig{}` instead is **not** equivalent; it is the same as supplying:

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

## Route Management

Routes are normally defined before the app starts. You can also add or remove them at runtime with the methods below, but these operations are **not thread-safe** and are performance-intensive, so use them sparingly and only in development.

### RebuildTree

The `RebuildTree` method is designed to rebuild the route tree and enable dynamic route registration. It returns a pointer to the `App` instance. GET routes registered since the last rebuild also receive their [automatic HEAD route](../guide/routing.md#automatic-head-routes) here.

```go title="Signature"
func (app *App) RebuildTree() *App
```

**Note:** Use this method with caution. It is **not** thread-safe and calling it can be very performance-intensive, so it should be used sparingly and only in development mode. Avoid using it concurrently.

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

### RemoveRoute

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

### RemoveRouteByName

This method removes a route by name.
If no methods are specified, the route will be removed for all HTTP methods defined in the app. To limit removal to specific methods, provide them as additional arguments.

```go title="Signature"
func (app *App) RemoveRouteByName(name string, methods ...string)
```

### RemoveRouteFunc

This method removes a route by function having `*Route` parameter. The `Path` field the function sees is the path as it was registered, not the normalized one.
If no methods are specified, the route will be removed for all HTTP methods defined in the app. To limit removal to specific methods, provide them as additional arguments.

```go title="Signature"
func (app *App) RemoveRouteFunc(matchFunc func(r *Route) bool, methods ...string)
```

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

Reloads the configured view engine on demand by calling its `Load` method. The engines of mounted sub-apps are reloaded too. Use this helper in development workflows (e.g., file watchers or debug-only routes) to pick up template changes without restarting the server. Returns an error if reloading fails, and `ErrNoViewEngineConfigured` if neither the app nor a mounted sub-app has a view engine (a typed nil `Views` counts as none).

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

### Render

`Render` writes a template through the configured view engine without creating
an HTTP response. It applies `ViewsLayout` when no layout is supplied and is
safe to use concurrently with `ReloadViews`. It returns `ErrNoViewEngineConfigured`
when the app has no view engine, a typed nil `Views` included.

```go title="Signature"
func (app *App) Render(out io.Writer, name string, binding any, layouts ...string) error
```

Capture the app, rather than a pooled request context, when rendering from an
asynchronous stream writer:

```go
app.Get("/events", func(c fiber.Ctx) error {
    return c.SendStreamWriter(func(w *bufio.Writer) {
        if err := app.Render(w, "partial", fiber.Map{"Title": "update"}); err != nil {
            return
        }
        if err := w.Flush(); err != nil {
            return
        }
    })
})
```
