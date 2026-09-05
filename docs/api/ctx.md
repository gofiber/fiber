---
id: ctx
title: 🧠 Ctx
description: >-
  The Ctx interface represents the Context which holds the HTTP request and
  response. It has methods for the request query string, parameters, body, HTTP
  headers, and so on.
sidebar_position: 3
---

import MethodIndex from '@site/src/components/method-index';

Use the index to jump straight to any `Ctx` method; filter by name or by category:

<MethodIndex defaultCategory="Context" itemNoun="methods" placeholder="Filter methods, e.g. cookie" />

### Abandon

Marks the context as abandoned. An abandoned context will not be returned to the pool when `ReleaseCtx` is called. This is used internally by the [timeout middleware](../middleware/timeout.md) to return immediately while the handler goroutine continues safely.

```go title="Signature"
func (c fiber.Ctx) Abandon()
func (c fiber.Ctx) IsAbandoned() bool
func (c fiber.Ctx) ForceRelease()
```

| Method         | Description                                                                 |
|:---------------|:----------------------------------------------------------------------------|
| `Abandon()`    | Marks the context as abandoned. ReleaseCtx becomes a no-op for this context. |
| `IsAbandoned()`| Returns `true` if `Abandon()` was called on this context.                   |
| `ForceRelease()`| Releases an abandoned context back to the pool. Must only be called after the handler has completely finished. |

:::caution
These methods are primarily for internal use and advanced middleware development. Most applications should not need to call them directly.
:::

### App

Returns the [\*App](app.md) reference so you can easily access all application settings.

```go title="Signature"
func (c fiber.Ctx) App() *App
```

```go title="Example"
app.Get("/stack", func(c fiber.Ctx) error {
  return c.JSON(c.App().Stack())
})
```

### Bind

Bind returns a helper for decoding the request body, query string, headers, cookies, and more.

For full details, see the [Bind](./bind.md) documentation.

```go title="Signature"
func (c fiber.Ctx) Bind() *Bind
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  user := new(User)
  // Bind the request body to a struct:
  return c.Bind().Body(user)
})
```

### Context

Returns a `context.Context` that was previously set with [`SetContext`](#setcontext).
If no context was set, it returns `context.Background()`. Unlike `fiber.Ctx` itself,
the returned context is safe to use after the handler completes.

```go title="Signature"
func (c fiber.Ctx) Context() context.Context
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  ctx := c.Context()
  go doWork(ctx)
  return nil
})
```

:::info Graceful shutdown
The default `Context()` is **not** canceled when the app shuts down. In-flight handlers that pass `c.Context()` to database clients or other cancellation-aware work keep running until they finish (or until you cancel a derived context yourself).

[`RequestCtx`](#requestctx) implements `context.Context` differently: its `Done()` channel is closed when the underlying fasthttp server starts shutting down. Prefer `c.Context()` (or a context you derive with `context.WithCancel` / timeouts) for work that must outlive the start of graceful shutdown.
:::

### context.Context

`Ctx` implements `context.Context`, but as a context that can never be canceled: `Deadline()` reports no deadline, `Done()` returns `nil` and `Err()` returns `nil`, regardless of what you pass to [`SetContext`](#setcontext). The `fiber.Ctx` instance is pooled and reused after the handler returns, which is why it cannot carry cancellation of its own. Call [`Context`](#context) within the handler to obtain a real `context.Context`, and pass that to anything that is cancellation-aware or that outlives the handler.

```go title="Signature"
func (c fiber.Ctx) Deadline() (deadline time.Time, ok bool)
func (c fiber.Ctx) Done() <-chan struct{}
func (c fiber.Ctx) Err() error
func (c fiber.Ctx) Value(key any) any
```

```go title="Example"

func doSomething(ctx context.Context) {
  // ...
}

app.Get("/", func(c fiber.Ctx) error {
  doSomething(c)
  return nil
})
```

:::caution
Passing `c` satisfies the compiler but carries no cancellation, so a call that honors `context.Context` will never be interrupted. Derive from [`Context`](#context) and pass that instead:
:::

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
  defer cancel()

  // The driver respects the 5s timeout. Passing c would never time out.
  rows, err := db.QueryContext(ctx, "SELECT ...")
  if err != nil {
    return err
  }
  defer rows.Close()
  // ...
  return nil
})
```

[`SetContext`](#setcontext) replaces what `Context()` returns for the rest of the request, so downstream middleware and handlers observe it. It does not make `c` itself cancelable.

#### Value

Value can be used to retrieve [**`Locals`**](#locals).

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Locals(userKey, "admin")
  user := c.Value(userKey) // returns "admin"
})
```

### Elapsed

Returns how long this request has been handled so far, measured from [`StartTime`](#starttime). Called after the handler chain has run, it is the request latency.

```go title="Signature"
func (c fiber.Ctx) Elapsed() time.Duration
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  err := c.Next()
  metrics.Observe(c.Route().Path, c.Elapsed())
  return err
})
```

### Endpoint

Returns the route that will handle this request, without advancing the handler chain. Useful inside global middleware for access control or logging when you need the target route's `Path` or `Name` before calling `Next`.

Returns `nil` when no endpoint will run: on `404` and `405`, and while the error handler replays the chain for a request rejected at the protocol level, such as one over `BodyLimit`.

```go title="Signature"
func (c fiber.Ctx) Endpoint() *Route
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  route := c.Endpoint() // e.g. "/api/users/:id" named "user.show"
  if route == nil {
    return c.Next()
  }
  // enforce access control using route.Path or route.Name
  return c.Next()
})

app.Get("/api/users/:id", handler).Name("user.show")
```

:::info
`Endpoint` looks ahead, while its neighbors look back: [`Route`](#route) is the route currently executing, which inside middleware is the middleware itself, and [`Matched`](#matched) reports whether an endpoint has been selected yet.

Mounted sub-apps report the flattened, prefixed route. The result is the route the router *would* reach, not a promise that it runs: an earlier middleware can still end the request first.

It scans the remaining routes in the request's tree bucket, so calling it from global middleware costs a second router scan per request. That is cheap for routes spread over many prefixes and noticeable when a hundred or more share one, as with everything under `/api/v1`.
:::

### Error

Returns an [`*fiber.Error`](./fiber.md#newerror) carrying the given status code, so a handler can reject a request in one line. The message defaults to the status text when omitted.

Returning the error hands it to the app's `ErrorHandler`, which is what writes the response; `Error` itself sets nothing on the response.

```go title="Signature"
func (c fiber.Ctx) Error(status int, message ...string) error
```

```go title="Example"
app.Get("/user/:id", func(c fiber.Ctx) error {
  id, err := strconv.Atoi(c.Params("id"))
  if err != nil {
    return c.Error(fiber.StatusBadRequest, "id must be numeric")
  }

  if !exists(id) {
    return c.Error(fiber.StatusNotFound) // => "Not Found"
  }

  return c.JSON(load(id))
})
```

### FullPath

Returns the full path of the matched route. This includes any prefixes that were added by [groups](../guide/routing.md#grouping) or mounts.

```go title="Signature"
func (c fiber.Ctx) FullPath() string
```

```go title="Example"
api := app.Group("/api")
api.Get("/users/:id", func(c fiber.Ctx) error {
  return c.JSON(fiber.Map{
    "route": c.FullPath(), // "/api/users/:id"
  })
})

app.Use(func(c fiber.Ctx) error {
  beforeNext := c.FullPath() // "/"

  if err := c.Next(); err != nil {
    return err
  }

  afterNext := c.FullPath() // "/api/users/:id"
  // ... react to the downstream handler's route path
  return nil
})
```

### GetReqHeaders

Returns the HTTP request headers as a map. Because a header can appear multiple times in a request, each key maps to a slice with all values for that header.

```go title="Signature"
func (c fiber.Ctx) GetReqHeaders() map[string][]string
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### GetRespHeader

Returns the HTTP response header specified by the field.

:::tip
The match is **case-insensitive**.
:::

```go title="Signature"
func (c fiber.Ctx) GetRespHeader(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.GetRespHeader("X-Request-Id")       // "8d7ad5e3-aaf3-450b-a241-2beb887efd54"
  c.GetRespHeader("Content-Type")       // "text/plain"
  c.GetRespHeader("something", "john")  // "john"
  // ..
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### GetRespHeaders

Returns the HTTP response headers as a map. Since a header can be set multiple times in a single request, the values of the map are slices of strings containing all the different values of the header.

```go title="Signature"
func (c fiber.Ctx) GetRespHeaders() map[string][]string
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### GetRouteURL

Generates URLs to named routes, with parameters. URLs are relative, for example: "/user/1831"

```go title="Signature"
func (c fiber.Ctx) GetRouteURL(routeName string, params Map) (string, error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Home page")
}).Name("home")

app.Get("/user/:id", func(c fiber.Ctx) error {
    return c.SendString(c.Params("id"))
}).Name("user.show")

app.Get("/test", func(c fiber.Ctx) error {
    location, _ := c.GetRouteURL("user.show", fiber.Map{"id": 1})
    return c.SendString(location)
})

// /test returns "/user/1"
```

:::note
A named route belongs to this application, so what comes back is always a path
on this origin: a `params` value that would open an authority — `"/evil.com"`
or `"\evil.com"` under a `/*` route — is kept as the path segment the route
asked for. [`Route.URL`](./app.md#getroute) and
[`Redirect().Route`](./redirect.md#route) return the same answer for the same
input, so it does not matter which one puts it in a `Location` header or an
`href`.

The values themselves are still written into the path as given. Where they come
from the request, escape them with [`url.PathEscape`](https://pkg.go.dev/net/url#PathEscape)
if the route expects one segment per parameter.
:::

### Hijack

Registers a handler that takes over the connection once the current response is sent, for protocols Fiber does not speak itself. The handler runs on the connection's own goroutine, after which the connection is closed unless the server has `KeepHijackedConns` set.

:::caution
The `Ctx` is pooled and reused, so the hijack handler must not touch it.
:::

```go title="Signature"
func (c fiber.Ctx) Hijack(handler fasthttp.HijackHandler)
```

```go title="Example"
app.Get("/raw", func(c fiber.Ctx) error {
  c.Hijack(func(conn net.Conn) {
    _, _ = conn.Write([]byte("speaking something else now"))
  })

  return c.SendStatus(fiber.StatusSwitchingProtocols)
})
```

### Hijacked

Returns `true` if [`Hijack`](#hijack) has been called on this request, so a later handler can tell that the connection is already spoken for and leave the response alone.

```go title="Signature"
func (c fiber.Ctx) Hijacked() bool
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  if c.Hijacked() {
    return nil // Nothing to add to a hijacked connection.
  }

  c.Set("X-Served-By", "fiber")
  return nil
})
```

### ID

Returns the connection-unique identifier assigned to this request. It is cheap and always present, unlike [`RequestID`](#requestid), which reads a header a client or proxy has to have set.

:::note
The value is not unique across processes or restarts, so it is for correlating log lines within one server run.
:::

```go title="Signature"
func (c fiber.Ctx) ID() uint64
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  log.Printf("[%d] %s %s", c.ID(), c.Method(), c.Path())
  return c.Next()
})
```

### IsFinal

Returns `true` only for the last handler of a matched, non-middleware route, so nothing further on that route runs. It is `false` when no route matched at all — as [`IsMiddleware`](#ismiddleware) also is, so the two are not exact complements.

:::caution
This describes the route, not the whole request. A route registered with `Use` is never final even when it is the last thing that runs, and another route can still match after a final handler calls [`Next`](#next) — a specific path followed by a catch-all is the ordinary case. Use it to tell an endpoint handler from a middleware one, not to decide that the response is finished.
:::

```go title="Signature"
func (c fiber.Ctx) IsFinal() bool
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  c.IsFinal() // => false, there is always a c.Next() from here
  return c.Next()
})

app.Get("/", func(c fiber.Ctx) error {
  c.IsFinal() // => true
  return nil
})
```

### IsMiddleware

Returns `true` if the current request handler was registered as middleware.

```go title="Signature"
func (c fiber.Ctx) IsMiddleware() bool
```

```go title="Example"
app.Get("/route", func(c fiber.Ctx) error {
  fmt.Println(c.IsMiddleware()) // true
  return c.Next()
}, func(c fiber.Ctx) error {
  fmt.Println(c.IsMiddleware()) // false
  return c.SendStatus(fiber.StatusOK)
})
```

### LocalAddr

Returns the server-side address of the connection this request arrived on. [`IP`](#ip) returns the client address as a string; this is the full `net.Addr`, so the port, network, and unix socket path survive.

```go title="Signature"
func (c fiber.Ctx) LocalAddr() net.Addr
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.LocalAddr().String() // => "127.0.0.1:3000"

  // ...
})
```

### Locals

Stores variables scoped to the request, making them available only to matching routes. The variables are removed after the request completes. If a stored value implements `io.Closer`, Fiber calls its `Close` method before removal.

:::tip
This is useful if you want to pass some **specific** data to the next middleware. Remember to perform type assertions when retrieving the data to ensure it is of the expected type. You can also use a non-exported type as a key to avoid collisions.
:::

```go title="Signature"
func (c fiber.Ctx) Locals(key any, value ...any) any
```

```go title="Example"

// keyType is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type keyType int

// userKey is the key for user.User values in Contexts. It is
// unexported; clients use user.NewContext and user.FromContext
// instead of using this key directly.
var userKey keyType

app.Use(func(c fiber.Ctx) error {
  c.Locals(userKey, "admin") // Stores the string "admin" under a non-exported type key
  return c.Next()
})

app.Get("/admin", func(c fiber.Ctx) error {
  user, ok := c.Locals(userKey).(string) // Retrieves the data stored under the key and performs a type assertion
  if ok && user == "admin" {
    return c.Status(fiber.StatusOK).SendString("Welcome, admin!")
  }
  return c.SendStatus(fiber.StatusForbidden)
})
```

An alternative version of the `Locals` method that takes advantage of Go's generics feature is also available. This version allows for the manipulation and retrieval of local values within a request's context with a more specific data type.

```go title="Signature"
func Locals[V any](c fiber.Ctx, key any, value ...V) V
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  fiber.Locals[string](c, "john", "doe")
  fiber.Locals[int](c, "age", 18)
  fiber.Locals[bool](c, "isHuman", true)
  return c.Next()
})

app.Get("/test", func(c fiber.Ctx) error {
  fiber.Locals[string](c, "john")    // "doe"
  fiber.Locals[int](c, "age")        // 18
  fiber.Locals[bool](c, "isHuman")   // true
  return nil
})
```

Make sure to understand and correctly implement the `Locals` method in both its standard and generic form for better control over route-specific data within your application.

### Matched

Returns `true` if the current request path was matched by the router.

```go title="Signature"
func (c fiber.Ctx) Matched() bool
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if c.Matched() {
    return c.Next()
  }
  return c.Status(fiber.StatusNotFound).SendString("Not Found")
})
```

### MountPath

Returns the prefix the sub-app owning the current route was mounted under, or an empty string when the route belongs to the top-level app.

Fiber mounts by cloning a sub-app's routes into the app that serves them with the prefix already baked in, so [`Path`](#path) inside a mounted handler is the whole requested path, not one relative to the mount. `MountPath` is what tells such a handler which prefix it is living under, to build links back into its own app or to strip the prefix itself.

:::note
It answers from the route that is running, unlike [`App.MountPath`](./app.md#mountpath), which describes the app it is called on however that app is being served — a sub-app that is mounted *and* listened on directly reports its mount prefix there even for its own traffic, where this reports `""`.
:::

:::caution
The prefix is the owning app's, so mounting one `*fiber.App` at two prefixes reports the one it was mounted at last — the same limitation `App.MountPath` has, since that is where the prefix is recorded. Mount separate instances when each needs to know its own prefix.
:::

```go title="Signature"
func (c fiber.Ctx) MountPath() string
```

```go title="Example"
micro := fiber.New()
micro.Get("/doe", func(c fiber.Ctx) error {
  c.MountPath() // => "/john"
  c.Path()      // => "/john/doe"

  // ...
})

app := fiber.New()
app.Use("/john", micro)
```

### Next

When **Next** is called, it executes the next method in the stack that matches the current route. You can pass an error struct within the method that will end the chaining and call the [error handler](../guide/error-handling).

```go title="Signature"
func (c fiber.Ctx) Next() error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  fmt.Println("1st route!")
  return c.Next()
})

app.Get("*", func(c fiber.Ctx) error {
  fmt.Println("2nd route!")
  return c.Next()
})

app.Get("/", func(c fiber.Ctx) error {
  fmt.Println("3rd route!")
  return c.SendString("Hello, World!")
})
```

### OverrideParam

Overwrites the value of an existing route parameter.

:::note
If the parameter does not exist, this method does nothing.
:::

```go title="Signature"
func (c fiber.Ctx) OverrideParam(name, value string)
```

```go title="Example"
// GET http://example.com/user
app.Get("/user/:name", func(c fiber.Ctx) error {
  // mutate parameter
  c.OverrideParam("name", "new value")
  return c.SendString(c.Params("name")) // sends "new value"
})
// GET http://example.com/shop/tech/1
app.Get("/shop/*", func(c fiber.Ctx) error {
  // mutate parameter
  c.OverrideParam("*", "new tech") // replaces "tech/1" with "new tech"
  return c.SendString(c.Params("*")) // sends "new tech"
})

```

Unnamed route parameters can be accessed by their character (`*` or `+`) followed by their position index (e.g., `*1` for the first wildcard, `*2` for the second).

```go title="Example"
// GET /v1/brand/4/shop/blue/xs
app.Get("/v1/*/shop/*", func(c fiber.Ctx) error {
  // mutate parameter
  c.OverrideParam("*1", "updated brand")
  c.OverrideParam("*2", "updated data")

  param1 := c.Params("*1") // "updated brand"
  param2 := c.Params("*2") // "updated data"

  // ...
})
```

### Redirect

Returns the Redirect reference.

For detailed information, check the [Redirect](./redirect.md) documentation.

```go title="Signature"
func (c fiber.Ctx) Redirect() *Redirect
```

```go title="Example"
app.Get("/coffee", func(c fiber.Ctx) error {
    return c.Redirect().To("/teapot")
})

app.Get("/teapot", func(c fiber.Ctx) error {
    return c.Status(fiber.StatusTeapot).Send("🍵 short and stout 🍵")
})
```

### RemoteAddr

Returns the address of the immediate peer, which is the proxy rather than the client when the app sits behind one. Use [`IP`](#ip) or [`IPs`](#ips) for the client address a trusted proxy forwarded.

```go title="Signature"
func (c fiber.Ctx) RemoteAddr() net.Addr
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.RemoteAddr().String() // => "192.168.1.10:54321"

  // ...
})
```

### Request

Returns the [*fasthttp.Request](https://pkg.go.dev/github.com/valyala/fasthttp#Request) pointer.

```go title="Signature"
func (c fiber.Ctx) Request() *fasthttp.Request
```

:::info
Returns `nil` if the context has been released (e.g., after the handler completes and the context is returned to the pool).
:::

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Request().Header.Method()
  // => []byte("GET")
})
```

### RequestCtx

Returns [\*fasthttp.RequestCtx](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx) that is compatible with the `context.Context` interface that requires a deadline, a cancellation signal, and other values across API boundaries.

```go title="Signature"
func (c fiber.Ctx) RequestCtx() *fasthttp.RequestCtx
```

:::info
Please read the [Fasthttp Documentation](https://pkg.go.dev/github.com/valyala/fasthttp?tab=doc) for more information.
:::

### Reset

Resets the context fields by the given request when using server handlers.

```go title="Signature"
func (c fiber.Ctx) Reset(fctx *fasthttp.RequestCtx)
```

It is used outside of the Fiber Handlers to reset the context for the next request.

### Response

Returns the [\*fasthttp.Response](https://pkg.go.dev/github.com/valyala/fasthttp#Response) pointer.

```go title="Signature"
func (c fiber.Ctx) Response() *fasthttp.Response
```

:::info
Returns `nil` if the context has been released (e.g., after the handler completes and the context is returned to the pool).
:::

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Response().BodyWriter().Write([]byte("Hello, World!"))
  // => "Hello, World!"
  return nil
})
```

### RestartRouting

Instead of executing the next method when calling [Next](ctx.md#next), **RestartRouting** restarts execution from the first method that matches the current route. This may be helpful after overriding the path, i.e., an internal redirect. Note that handlers might be executed again, which could result in an infinite loop.

```go title="Signature"
func (c fiber.Ctx) RestartRouting() error
```

```go title="Example"
app.Get("/new", func(c fiber.Ctx) error {
  return c.SendString("From /new")
})

app.Get("/old", func(c fiber.Ctx) error {
  c.Path("/new")
  return c.RestartRouting()
})
```

### Route

Returns the matched [Route](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc#Route) struct.

```go title="Signature"
func (c fiber.Ctx) Route() *Route
```

```go title="Example"
// http://localhost:8080/hello

app.Get("/hello/:name", func(c fiber.Ctx) error {
  r := c.Route()
  fmt.Println(r.Method, r.Path, r.Params, r.Handlers)
  // GET /hello/:name handler [name]

  // ...
})
```

:::caution
`c.Route()` returns the **last executed route**. Inside middleware that runs before your handler it reflects the middleware route itself. Use [`c.Endpoint()`](#endpoint) to look up the downstream handler without advancing the chain.
:::

```go title="Example"
func MyMiddleware() fiber.Handler {
  return func(c fiber.Ctx) error {
    beforeNext := c.Route().Path // Will be '/'
    err := c.Next()
    afterNext := c.Route().Path // Will be '/hello/:name'
    return err
  }
}
```

### RouteName

Returns the name of the route currently executing, or an empty string when the route is unnamed. Inside middleware this is the middleware's own route; use `Endpoint().Name` to look ahead to the route that will handle the request.

```go title="Signature"
func (c fiber.Ctx) RouteName() string
```

```go title="Example"
app.Get("/home", func(c fiber.Ctx) error {
  c.RouteName() // => "home"

  // ...
}).Name("home")
```

### SetContext

Sets the base `context.Context` used by [`Context`](#context). Use this to
propagate deadlines, cancellation signals, or values to asynchronous operations.

```go title="Signature"
func (c fiber.Ctx) SetContext(ctx context.Context)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.SetContext(context.WithValue(context.Background(), "user", "alice"))
  ctx := c.Context()
  go doWork(ctx)
  return nil
})
```

### StartTime

Returns the time the server began handling this request. It is the reference point [`Elapsed`](#elapsed) measures from.

```go title="Signature"
func (c fiber.Ctx) StartTime() time.Time
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  deadline := c.StartTime().Add(2 * time.Second)

  // ...
})
```

### String

Returns a unique string representation of the context.

```go title="Signature"
func (c fiber.Ctx) String() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.String() // => "#0000000100000001 - 127.0.0.1:3000 <-> 127.0.0.1:61516 - GET http://localhost:3000/"

  // ...
})
```

### ViewBind

Adds variables to the default view variable map binding to the template engine.
Variables are read by the `Render` method and may be overwritten.

```go title="Signature"
func (c fiber.Ctx) ViewBind(vars Map) error
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  c.ViewBind(fiber.Map{
    "Title": "Hello, World!",
  })
  return c.Next()
})

app.Get("/", func(c fiber.Ctx) error {
  return c.Render("xxx.tmpl", fiber.Map{}) // Render will use the Title variable
})
```

## Request

Methods which operate on the incoming request.

:::tip
Use `c.Req()` to limit gopls suggestions to only these methods!
:::

Each entry lists both forms it can be called in. `DefaultCtx` embeds `DefaultReq`, so the method is promoted: `c.UserAgent()` and `c.Req().UserAgent()` are the same call, and the second is what the `Req` interface exposes on its own.

Four entries here are request-related but live on `Ctx` alone and so list only the `fiber.Ctx` form: [`ClientHelloInfo`](#clienthelloinfo), [`RequestID`](#requestid), [`SaveFile`](#savefile) and [`SaveFileToStorage`](#savefiletostorage).

### AcceptEncoding

Returns the `Accept-Encoding` request header.

```go title="Signature"
func (c fiber.Ctx) AcceptEncoding() string
func (r fiber.Req) AcceptEncoding() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.AcceptEncoding() // "gzip, br"
  return nil
})
```

### AcceptLanguage

Returns the `Accept-Language` request header.

```go title="Signature"
func (c fiber.Ctx) AcceptLanguage() string
func (r fiber.Req) AcceptLanguage() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.AcceptLanguage() // "en-US,en;q=0.9"
  return nil
})
```

### Accepts

Checks if the specified **extensions** or **content** **types** are acceptable.

:::info
Based on the request’s [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header.
:::

```go title="Signature"
func (c fiber.Ctx) Accepts(offers ...string) string
func (c fiber.Ctx) AcceptsCharsets(offers ...string) string
func (c fiber.Ctx) AcceptsEncodings(offers ...string) string
func (c fiber.Ctx) AcceptsLanguages(offers ...string) string
func (c fiber.Ctx) AcceptsLanguagesExtended(offers ...string) string
func (r fiber.Req) Accepts(offers ...string) string
func (r fiber.Req) AcceptsCharsets(offers ...string) string
func (r fiber.Req) AcceptsEncodings(offers ...string) string
func (r fiber.Req) AcceptsLanguages(offers ...string) string
func (r fiber.Req) AcceptsLanguagesExtended(offers ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Accepts("html")             // "html"
  c.Accepts("text/html")        // "text/html"
  c.Accepts("json", "text")     // "json"
  c.Accepts("application/json") // "application/json"
  c.Accepts("text/plain", "application/json") // "application/json", due to quality
  c.Accepts("image/png")        // ""
  c.Accepts("png")              // ""
  // ...
})
```

```go title="Example 2"
// Accept: text/html, text/*, application/json, */*; q=0

app.Get("/", func(c fiber.Ctx) error {
  c.Accepts("text/plain", "application/json") // "application/json", due to specificity
  c.Accepts("application/json", "text/html") // "text/html", due to first match
  c.Accepts("image/png")                      // "", due to */* with q=0 is Not Acceptable
  // ...
})
```

The weight of an offer is the one of the most specific range matching it (RFC 9110 §12.5.1), so a broad range with a higher weight never overrides a more specific range that lowers or rejects that offer:

```go title="Example 3"
// Accept: text/*;q=1, text/html;q=0.5

app.Get("/", func(c fiber.Ctx) error {
  c.Accepts("text/html", "text/plain") // "text/plain": text/html weighs 0.5, text/plain 1
  c.Accepts("text/html")               // "text/html": a lowered weight is still acceptable
  // ...
})
```

Media-Type parameters are supported.

```go title="Example 3"
// Accept: text/plain, application/json; version=1; foo=bar

app.Get("/", func(c fiber.Ctx) error {
  // Extra parameters in the accept are ignored
  c.Accepts("text/plain;format=flowed") // "text/plain;format=flowed"

  // An offer must contain all parameters present in the Accept type
  c.Accepts("application/json") // ""

  // Parameter order and capitalization do not matter. Quotes on values are stripped.
  c.Accepts(`application/json;foo="bar";VERSION=1`) // "application/json;foo="bar";VERSION=1"
})
```

```go title="Example 4"
// Accept: text/plain;format=flowed;q=0.9, text/plain
// i.e., "I prefer text/plain;format=flowed less than other forms of text/plain"

app.Get("/", func(c fiber.Ctx) error {
  // Beware: the order in which offers are listed matters.
  // Although the client specified they prefer not to receive format=flowed,
  // the text/plain Accept matches with "text/plain;format=flowed" first, so it is returned.
  c.Accepts("text/plain;format=flowed", "text/plain") // "text/plain;format=flowed"

  // Here, things behave as expected:
  c.Accepts("text/plain", "text/plain;format=flowed") // "text/plain"
})
```

Fiber provides similar functions for the other accept headers.

For `Accept-Language`, Fiber uses the [Basic Filtering](https://www.rfc-editor.org/rfc/rfc4647#section-3.3.1) algorithm. A language range matches an offer only if it exactly equals the tag or is a prefix followed by a hyphen. For example, the range `en` matches `en-US`, but `en-US` does not match `en`.

`AcceptsLanguagesExtended` applies [Extended Filtering](https://www.rfc-editor.org/rfc/rfc4647#section-3.3.2) where `*` may match zero or more subtags and wildcard matches can slide across subtags unless blocked by a singleton like `x`.

```go
// Accept-Charset: utf-8, iso-8859-1;q=0.2
// Accept-Encoding: gzip, compress;q=0.2
// Accept-Language: en;q=0.8, nl, ru

app.Get("/", func(c fiber.Ctx) error {
  c.AcceptsCharsets("utf-16", "iso-8859-1")
  // "iso-8859-1"

  c.AcceptsEncodings("compress", "br")
  // "compress"

  c.AcceptsLanguages("pt", "nl", "ru")
  // "nl"

  c.AcceptsLanguagesExtended("en-US", "fr-CA")
  // depends on extended ranges in the request header
  // ...
})
```

### AcceptsEventStream

Returns `true` when the `Accept` header allows `text/event-stream`.

```go title="Signature"
func (c fiber.Ctx) AcceptsEventStream() bool
func (r fiber.Req) AcceptsEventStream() bool
```

```go title="Example"
// Accept: text/html, application/json;q=0.9

app.Get("/", func(c fiber.Ctx) error {
  c.AcceptsEventStream() // false
  return nil
})
```

### AcceptsHTML

Returns `true` when the `Accept` header allows HTML.

```go title="Signature"
func (c fiber.Ctx) AcceptsHTML() bool
func (r fiber.Req) AcceptsHTML() bool
```

```go title="Example"
// Accept: text/html, application/json;q=0.9

app.Get("/", func(c fiber.Ctx) error {
  c.AcceptsHTML() // true
  return nil
})
```

### AcceptsJSON

Returns `true` when the `Accept` header allows JSON.

```go title="Signature"
func (c fiber.Ctx) AcceptsJSON() bool
func (r fiber.Req) AcceptsJSON() bool
```

```go title="Example"
// Accept: text/html, application/json;q=0.9

app.Get("/", func(c fiber.Ctx) error {
  c.AcceptsJSON() // true
  return nil
})
```

### AcceptsXML

Returns `true` when the `Accept` header allows XML.

```go title="Signature"
func (c fiber.Ctx) AcceptsXML() bool
func (r fiber.Req) AcceptsXML() bool
```

```go title="Example"
// Accept: text/html, application/json;q=0.9

app.Get("/", func(c fiber.Ctx) error {
  c.AcceptsXML() // false
  return nil
})
```

### AllCookies

Returns the cookies sent with the request as a name/value map, or `nil` when the client sent none.

A repeated name resolves to its **first** occurrence, which is the one [`Cookies`](#cookies) answers with too. The two have to agree: a client can shadow a cookie by sending the name twice, and code that validated one value while consuming the other would validate the wrong one. Use [`CookieNames`](#cookienames) to see that a name was repeated at all.

:::note
The keys and values are copies rather than views into the request buffer, so they stay valid past the handler. fasthttp rewrites those bytes in place when a handler edits a request cookie, which would not merely stale the map but rehash it into one whose own keys no longer find their entries.
:::

```go title="Signature"
func (c fiber.Ctx) AllCookies() map[string]string
func (r fiber.Req) AllCookies() map[string]string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Cookie: session=abc; theme=dark
  c.AllCookies() // => map[session:abc theme:dark]

  // ...
})
```

### Authorization

Splits the `Authorization` request header into its auth-scheme and the credentials that follow (RFC 9110, Section 11.6.2). Both are empty when the header is absent.

:::caution
The credentials are returned verbatim — `token68` or an `auth-param` list — and are neither decoded nor validated.

Returned values are only valid within the handler. Do not store any references: they view the request buffer, which the next request on the connection overwrites, so a credential cached past the handler silently becomes another request's. Make copies or use the [**`Immutable`**](./fiber.md#config) setting instead.
:::

```go title="Signature"
func (c fiber.Ctx) Authorization() (scheme, credentials string)
func (r fiber.Req) Authorization() (scheme, credentials string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Authorization: Basic dXNlcjpwYXNz
  scheme, credentials := c.Authorization()
  // scheme      => "Basic"
  // credentials => "dXNlcjpwYXNz"

  // ...
})
```

### BaseURL

Returns the base URL (**protocol** + **host**) as a `string`.

```go title="Signature"
func (c fiber.Ctx) BaseURL() string
func (r fiber.Req) BaseURL() string
```

```go title="Example"
// GET https://example.com/page#chapter-1

app.Get("/", func(c fiber.Ctx) error {
  c.BaseURL() // "https://example.com"
  // ...
})
```

### Bearer

Returns the credentials of a `Bearer` `Authorization` header (RFC 6750, Section 2.1), or an empty string when the header is absent or names a different auth-scheme.

:::caution
The token is returned verbatim: it is not validated, decoded, or verified, so it still has to be authenticated before it is trusted. Use the [keyauth](../middleware/keyauth.md) middleware when you want that done for you.

Returned value is only valid within the handler. Do not store any references: it views the request buffer, which the next request on the connection overwrites, so a token cached past the handler — or used as a key in a shared map — silently becomes another request's.
:::

```go title="Signature"
func (c fiber.Ctx) Bearer() string
func (r fiber.Req) Bearer() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Authorization: Bearer abc123
  c.Bearer() // => "abc123"

  // Authorization: Basic dXNlcjpwYXNz
  c.Bearer() // => ""

  // ...
})
```

### Body

As per the header `Content-Encoding`, this method will try to perform a file decompression from the **body** bytes. In case no `Content-Encoding` header is sent (or when it is set to `identity`), it will perform as [BodyRaw](#bodyraw). If an unknown or unsupported encoding is encountered, the response status will be `415 Unsupported Media Type` or `501 Not Implemented`. Decompression is bounded by the app [BodyLimit](./fiber.md#bodylimit).

```go title="Signature"
func (c fiber.Ctx) Body() []byte
func (r fiber.Req) Body() []byte
```

```go title="Example"
// echo 'user=john' | gzip | curl -v -i --data-binary @- -H "Content-Encoding: gzip" http://localhost:8080

app.Post("/", func(c fiber.Ctx) error {
  // Decompress body from POST request based on the Content-Encoding and return the raw content:
  return c.Send(c.Body()) // []byte("user=john")
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### BodyRaw

Returns the raw request **body**.

```go title="Signature"
func (c fiber.Ctx) BodyRaw() []byte
func (r fiber.Req) BodyRaw() []byte
```

```go title="Example"
// curl -X POST http://localhost:8080 -d user=john

app.Post("/", func(c fiber.Ctx) error {
  // Get raw body from POST request:
  return c.Send(c.BodyRaw()) // []byte("user=john")
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### BodyStream

Returns the request body as a stream. It is only non-`nil` when [`StreamRequestBody`](./fiber.md#config) is enabled and the body has not already been buffered.

:::caution
Reading from the returned reader consumes the body, so a later [`Body`](#body) call will not see it.
:::

```go title="Signature"
func (c fiber.Ctx) BodyStream() io.Reader
func (r fiber.Req) BodyStream() io.Reader
```

```go title="Example"
app := fiber.New(fiber.Config{StreamRequestBody: true})

app.Post("/upload", func(c fiber.Ctx) error {
  stream := c.BodyStream()
  if stream == nil {
    return c.Send(c.Body()) // Already buffered.
  }

  written, err := io.Copy(dst, stream)
  if err != nil {
    return err
  }

  return c.JSON(fiber.Map{"written": written})
})
```

### Charset

Returns the `charset` parameter from the `Content-Type` header.

```go title="Signature"
func (c fiber.Ctx) Charset() string
func (r fiber.Req) Charset() string
```

```go title="Example"
// Content-Type: application/json; charset=utf-8

app.Post("/", func(c fiber.Ctx) error {
  c.Charset() // "utf-8"
  return nil
})
```

### ClientHelloInfo

`ClientHelloInfo` contains information from the ClientHello message of the TLS connection the request arrived on, to guide application logic in the `GetCertificate` and `GetConfigForClient` callbacks. It is recorded per connection, so concurrent connections never see each other's handshake, and it is `nil` when the app has no TLS handler or the request did not come in over a connection it negotiated. With `Server.MaxConnsPerIP` set, the record for a connection is only dropped once the same connection object is seen again rather than when it closes, because fasthttp recycles the wrapper it reports the close on.
Refer to the [ClientHelloInfo](https://golang.org/pkg/crypto/tls/#ClientHelloInfo) struct documentation for details on the returned struct.

```go title="Signature"
func (c fiber.Ctx) ClientHelloInfo() *tls.ClientHelloInfo
```

```go title="Example"
// GET http://example.com/hello
app.Get("/hello", func(c fiber.Ctx) error {
  chi := c.ClientHelloInfo()
  // ...
})
```

### ContentLength

Returns the value of the `Content-Length` request header.

A negative result is **not a length**: `-1` is reported for a chunked body and `-2` for `Transfer-Encoding: identity`, which is also what an ordinary `GET` with no body reports. So `-2`, not `0`, is the common case for a bodyless request — use [`HasBody`](#hasbody) to test whether there is a body at all, and read this only to find out how large a declared one is.

:::note
`Req` and `Res` both carry a `ContentLength`, so on `Ctx` the request wins, as it does for [`Get`](#get). Use `c.Res().ContentLength()` for the length of the response.
:::

```go title="Signature"
func (c fiber.Ctx) ContentLength() int
func (r fiber.Req) ContentLength() int
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  if c.ContentLength() > maxUpload {
    return c.Error(fiber.StatusRequestEntityTooLarge)
  }

  // ...
})
```

### ContentType

Returns the `Content-Type` request header, parameters included. [`MediaType`](#mediatype) returns the same header with the parameters stripped, and [`Charset`](#charset) returns just the charset one.

:::note
`Req` and `Res` both carry a `ContentType`, so on `Ctx` the request wins, as it does for [`Get`](#get). Use `c.Res().ContentType()` for the type the response will send.
:::

:::caution
Returned value is only valid within the handler. Do not store any references. Make copies or use the [**`Immutable`**](./fiber.md#config) setting to use the value outside the handler.
:::

```go title="Signature"
func (c fiber.Ctx) ContentType() string
func (r fiber.Req) ContentType() string
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Content-Type: application/json; charset=utf-8
  c.ContentType() // => "application/json; charset=utf-8"
  c.MediaType()   // => "application/json"

  // ...
})
```

### CookieNames

Returns the names of the cookies sent with the request, in the order they appear in the `Cookie` header, or `nil` when the client sent none. A name the client repeated is returned once per occurrence, which is how a caller detects the shadowing [`AllCookies`](#allcookies) collapses.

:::note
Unlike [`Cookies`](#cookies), the returned strings are copies rather than views into the request buffer, so they stay valid past the handler.
:::

```go title="Signature"
func (c fiber.Ctx) CookieNames() []string
func (r fiber.Req) CookieNames() []string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Cookie: session=abc; theme=dark
  c.CookieNames() // => ["session", "theme"]

  // ...
})
```

### Cookies

Gets a cookie value by key. You can pass an optional default value that will be returned if the cookie key does not exist.

```go title="Signature"
func (c fiber.Ctx) Cookies(key string, defaultValue ...string) string
func (r fiber.Req) Cookies(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Get cookie by key:
  c.Cookies("name")         // "john"
  c.Cookies("empty", "doe") // "doe"
  // ...
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Use [`App.GetString`](./app.md#getstring) or [`App.GetBytes`](./app.md#getbytes) when immutability is enabled, or manually copy values (for example with [`utils.CopyString`](https://github.com/gofiber/utils) / `utils.CopyBytes`) when it's disabled. [Read more...](../#zero-allocation)
:::

### FormFile

MultipartForm files can be retrieved by name, the **first** file from the given key is returned.

```go title="Signature"
func (c fiber.Ctx) FormFile(key string) (*multipart.FileHeader, error)
func (r fiber.Req) FormFile(key string) (*multipart.FileHeader, error)
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Get first file from form field "document":
  file, err := c.FormFile("document")

  // Save file to root directory:
  return c.SaveFile(file, fmt.Sprintf("./%s", file.Filename))
})
```

### FormValue

Form values can be retrieved by name, the **first** value for the given key is returned.

:::caution

On a form request this lowercases the case-insensitive parts of the request's
own `Content-Type`, so a value obtained earlier from `Get(HeaderContentType)` —
which aliases those bytes unless [Immutable](./fiber.md#immutable) is set — can
change during the call. Copy it first if you need it to outlive one.

:::

```go title="Signature"
func (c fiber.Ctx) FormValue(key string, defaultValue ...string) string
func (r fiber.Req) FormValue(key string, defaultValue ...string) string
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Get first value from form field "name":
  c.FormValue("name")
  // => "john" or "" if not exist

  // ..
})
```

:::info

The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)

:::

### Fresh

When the response is still **fresh** in the client's cache **true** is returned; otherwise, **false** is returned to indicate that the client cache is now stale and the full response should be sent.

When a client sends the Cache-Control: no-cache request header to indicate an end-to-end reload request, `Fresh` will return false to make handling these requests transparent.

`Fresh` only applies to GET and HEAD requests and returns false for any other method, since a 304 Not Modified response is only defined for those methods and RFC 9110 requires If-Modified-Since to be ignored otherwise.

Read more on [https://expressjs.com/en/4x/api.html\#req.fresh](https://expressjs.com/en/4x/api.html#req.fresh)

```go title="Signature"
func (c fiber.Ctx) Fresh() bool
func (r fiber.Req) Fresh() bool
```

### FullURL

Returns the full request URL (protocol + host + original URL).

```go title="Signature"
func (c fiber.Ctx) FullURL() string
func (r fiber.Req) FullURL() string
```

```go title="Example"
// GET http://example.com/search?q=fiber

app.Get("/", func(c fiber.Ctx) error {
  c.FullURL() // "http://example.com/search?q=fiber"
  return nil
})
```

### Get

Returns the HTTP request header specified by the field.

:::tip
The match is **case-insensitive**.
:::

```go title="Signature"
func (c fiber.Ctx) Get(key string, defaultValue ...string) string
func (r fiber.Req) Get(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Get("Content-Type")       // "text/plain"
  c.Get("CoNtEnT-TypE")       // "text/plain"
  c.Get("something", "john")  // "john"
  // ..
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### GetAll

Returns every field line of the request header specified by `key`. Field names are case-insensitive.

A header repeated across field lines is semantically one comma-joined list (RFC 9110, Section 5.2). `GetAll` keeps the lines apart so a caller can inspect them individually, where [`Get`](#get) returns only the first and [`GetReqHeaders`](#getreqheaders) builds a map of the whole header block. It returns `nil` when the header is absent.

:::note
Empty field lines are skipped, so `len(c.GetAll(k)) > 0` answers the same question as [`HasHeader`](#hasheader). Without that, the headers fasthttp keeps in a slot of their own (`Content-Length`, `Trailer`) report one empty line when they are not present at all.
:::

:::caution
Returned values are only valid within the handler. Do not store any references. Make copies or use the [**`Immutable`**](./fiber.md#config) setting instead.
:::

```go title="Signature"
func (c fiber.Ctx) GetAll(key string) []string
func (r fiber.Req) GetAll(key string) []string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // X-Forwarded-For: 10.0.0.1
  // X-Forwarded-For: 10.0.0.2
  c.GetAll("X-Forwarded-For") // => ["10.0.0.1", "10.0.0.2"]
  c.Get("X-Forwarded-For")    // => "10.0.0.1"

  // ...
})
```

### HasBody

Returns `true` if the incoming request contains a body or a `Content-Length` header greater than zero.

```go title="Signature"
func (c fiber.Ctx) HasBody() bool
func (r fiber.Req) HasBody() bool
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  if !c.HasBody() {
    return c.SendStatus(fiber.StatusBadRequest)
  }
  return c.SendString("OK")
})
```

### HasHeader

Reports whether the request includes a header with the given key.

```go title="Signature"
func (c fiber.Ctx) HasHeader(key string) bool
func (r fiber.Req) HasHeader(key string) bool
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.HasHeader("X-Trace-Id")
  return nil
})
```

### Host

Returns the host derived from the [Host](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Host) HTTP header.

In a network context, [`Host`](#host) refers to the combination of a hostname and potentially a port number used for connecting, while [`Hostname`](#hostname) refers specifically to the name assigned to a device on a network, excluding any port information.

```go title="Signature"
func (c fiber.Ctx) Host() string
func (r fiber.Req) Host() string
```

```go title="Example"
// GET http://google.com:8080/search

app.Get("/", func(c fiber.Ctx) error {
  c.Host()      // "google.com:8080"
  c.Hostname()  // "google.com"

  // ...
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### Hostname

Returns the hostname derived from the [Host](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Host) HTTP header.

```go title="Signature"
func (c fiber.Ctx) Hostname() string
func (r fiber.Req) Hostname() string
```

```go title="Example"
// GET http://google.com/search

app.Get("/", func(c fiber.Ctx) error {
  c.Hostname() // "google.com"

  // ...
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### IfModifiedSince

Returns the time carried by the `If-Modified-Since` request header. It returns `fiber.ErrHeaderNotFound` when the header is absent, and a parse error when the value is none of the three HTTP-date formats RFC 9110, Section 5.6.7 requires recipients to accept.

```go title="Signature"
func (c fiber.Ctx) IfModifiedSince() (time.Time, error)
func (r fiber.Req) IfModifiedSince() (time.Time, error)
```

```go title="Example"
app.Get("/report", func(c fiber.Ctx) error {
  since, err := c.IfModifiedSince()
  switch {
  case errors.Is(err, fiber.ErrHeaderNotFound):
    // Unconditional request.
  case err != nil:
    return c.Error(fiber.StatusBadRequest, "malformed If-Modified-Since")
  case !report.ModTime().After(since):
    return c.SendStatus(fiber.StatusNotModified)
  }

  return c.JSON(report)
})
```

### IfNoneMatch

Returns the entity tags listed in the `If-None-Match` request header. Repeated field lines are combined into one list (RFC 9110, Section 5.2), and a comma inside a quoted opaque-tag does not split it, so `"v1,v2"` stays one tag. A wildcard header returns a single `*` element.

:::note
Tags are returned verbatim, weak `W/` prefix included, and are not validated; empty list elements are dropped as RFC 9110, Section 5.6.1 requires, so a trailing comma is not an extra tag. [`Fresh`](#fresh) already applies these tags to the response `ETag`; reach for this only to implement a comparison of your own.

Returned values are only valid within the handler. Do not store any references.
:::

```go title="Signature"
func (c fiber.Ctx) IfNoneMatch() []string
func (r fiber.Req) IfNoneMatch() []string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // If-None-Match: W/"a", "b,c"
  c.IfNoneMatch() // => [`W/"a"`, `"b,c"`]

  // ...
})
```

### IP

Returns the remote IP address of the request.

```go title="Signature"
func (c fiber.Ctx) IP() string
func (r fiber.Req) IP() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.IP() // "127.0.0.1"

  // ...
})
```

:::info
By default, `c.IP()` returns the remote IP address from the TCP connection. When your Fiber app is behind a reverse proxy (like Nginx, Traefik, or a load balancer), you need to configure **both** [`TrustProxy`](fiber.md#trustproxy) and [`ProxyHeader`](fiber.md#proxyheader) to read the client IP from proxy headers like `X-Forwarded-For`.

**Important:** You must enable `TrustProxy` and configure trusted proxy IPs to prevent header spoofing. Simply setting `ProxyHeader` alone will not work.

**Note:** When using a proxy header such as `X-Forwarded-For`, `c.IP()` returns the raw header value unless [`EnableIPValidation`](fiber.md#enableipvalidation) is enabled.

**Chain parsing with `EnableIPValidation`:** For `X-Forwarded-For`, the raw value is a comma-separated chain that grows from left to right as the request passes through each proxy. With validation enabled, `c.IP()` walks the chain from right to left, skipping every IP that matches the configured `TrustProxyConfig` (exact IPs, CIDR ranges, loopback, private or link-local) and returns the first non-trusted IP it finds. This matches the behavior recommended by [MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/X-Forwarded-For#selecting_an_ip_address) and the convention used by Nginx (`set_real_ip_from` + `real_ip_recursive`), Apache `mod_remoteip`, and Envoy (`xff_num_trusted_hops`).

If every IP in the chain matches the trusted set, the leftmost IP is returned as a fallback. If the chain is empty, `c.IP()` falls back to the TCP remote address.
:::

#### Configuration for apps behind a reverse proxy

```go title="Example - Basic Configuration"
app := fiber.New(fiber.Config{
  // Enable proxy support
  TrustProxy: true,
  // Specify which header contains the real client IP
  ProxyHeader: fiber.HeaderXForwardedFor,
  // Configure which proxy IPs to trust
  TrustProxyConfig: fiber.TrustProxyConfig{
    // Trust private IP ranges (for internal load balancers)
    Private: true,
    // Or specify exact proxy IPs/ranges
    // Proxies: []string{"10.10.0.58", "192.168.0.0/24"},
  },
})
```

```go title="Example - Specific Proxy IPs"
app := fiber.New(fiber.Config{
  TrustProxy: true,
  ProxyHeader: fiber.HeaderXForwardedFor,
  TrustProxyConfig: fiber.TrustProxyConfig{
    // Trust only specific proxy IP addresses
    Proxies: []string{"10.10.0.58", "192.168.1.0/24"},
  },
})
```

See [`TrustProxy`](fiber.md#trustproxy) and [`TrustProxyConfig`](fiber.md#trustproxyconfig) for more details on security considerations and configuration options.

### IPs

Returns an array of IP addresses specified in the [X-Forwarded-For](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For) request header. With `EnableIPValidation`, IPv4, IPv6 and IPv4-mapped IPv6 addresses (`::ffff:203.0.113.5`, as dual-stack proxies forward IPv4 clients) are all accepted.

```go title="Signature"
func (c fiber.Ctx) IPs() []string
func (r fiber.Req) IPs() []string
```

```go title="Example"
// X-Forwarded-For: proxy1, 127.0.0.1, proxy3

app.Get("/", func(c fiber.Ctx) error {
  c.IPs() // ["proxy1", "127.0.0.1", "proxy3"]

  // ...
})
```

:::caution
Improper use of the X-Forwarded-For header can be a security risk. For details, see the [Security and privacy concerns](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For#security_and_privacy_concerns) section.
:::

### Is

Returns the matching **content type**, if the incoming request’s [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) HTTP header field matches the [MIME type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types) specified by the type parameter.

:::info
If the request has **no** body, it returns **false**.
:::

```go title="Signature"
func (c fiber.Ctx) Is(extension string) bool
func (r fiber.Req) Is(extension string) bool
```

```go title="Example"
// Content-Type: text/html; charset=utf-8

app.Get("/", func(c fiber.Ctx) error {
  c.Is("html")  // true
  c.Is(".html") // true
  c.Is("json")  // false

  // ...
})
```

### IsForm

Reports whether the `Content-Type` header is form-encoded.

```go title="Signature"
func (c fiber.Ctx) IsForm() bool
func (r fiber.Req) IsForm() bool
```

```go title="Example"
// Content-Type: application/x-www-form-urlencoded

app.Post("/", func(c fiber.Ctx) error {
  c.IsForm() // true
  return nil
})
```

### IsFromLocal

Returns `true` if the request came from localhost.

```go title="Signature"
func (c fiber.Ctx) IsFromLocal() bool
func (r fiber.Req) IsFromLocal() bool
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // If request came from localhost, return true; else return false
  c.IsFromLocal()

  // ...
})
```

### IsFromUnixSocket

Returns `true` if the request came in over a Unix domain socket.

```go title="Signature"
func (c fiber.Ctx) IsFromUnixSocket() bool
func (r fiber.Req) IsFromUnixSocket() bool
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  if c.IsFromUnixSocket() {
    return c.SendString("Connected via Unix socket")
  }
  return c.SendString("Connected via TCP")
})
```

### IsIdempotent

Reports whether the request method is idempotent, meaning repeating it has the same intended effect as making it once (RFC 9110, Section 9.2.2). Every safe method is also idempotent.

```go title="Signature"
func (c fiber.Ctx) IsIdempotent() bool
func (r fiber.Req) IsIdempotent() bool
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  // GET, HEAD, OPTIONS, TRACE, PUT, DELETE => true
  // POST, PATCH                            => false
  if c.IsIdempotent() {
    return retryable(c)
  }

  return c.Next()
})
```

### IsJSON

Reports whether the `Content-Type` header is JSON.

```go title="Signature"
func (c fiber.Ctx) IsJSON() bool
func (r fiber.Req) IsJSON() bool
```

```go title="Example"
// Content-Type: application/json; charset=utf-8

app.Post("/", func(c fiber.Ctx) error {
  c.IsJSON() // true
  return nil
})
```

### IsMultipart

Reports whether the `Content-Type` header is multipart form data.

```go title="Signature"
func (c fiber.Ctx) IsMultipart() bool
func (r fiber.Req) IsMultipart() bool
```

```go title="Example"
// Content-Type: multipart/form-data; boundary=abc123

app.Post("/", func(c fiber.Ctx) error {
  c.IsMultipart() // true
  return nil
})
```

### IsPreflight

Returns `true` if the request is a CORS preflight (`OPTIONS` + `Access-Control-Request-Method` + `Origin`).

```go title="Signature"
func (c fiber.Ctx) IsPreflight() bool
func (r fiber.Req) IsPreflight() bool
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if c.IsPreflight() {
    return c.SendStatus(fiber.StatusNoContent)
  }
  return c.Next()
})
```

### IsProxyTrusted

Checks the trustworthiness of the remote IP.
If [`TrustProxy`](fiber.md#trustproxy) is `false`, it returns `false`.
`IsProxyTrusted` can check the remote IP by proxy ranges and IP map.

```go title="Signature"
func (c fiber.Ctx) IsProxyTrusted() bool
func (r fiber.Req) IsProxyTrusted() bool
```

```go title="Example"
app := fiber.New(fiber.Config{
  // TrustProxy enables the trusted proxy check
  TrustProxy: true,
  // TrustProxyConfig allows for configuring trusted proxies.
  // Proxies is a list of trusted proxy IP ranges/addresses
  TrustProxyConfig: fiber.TrustProxyConfig{
    Proxies: []string{"0.8.0.0", "1.1.1.1/30"}, // IP address or IP address range
    Loopback: true,   // Trust loopback addresses (127.0.0.0/8, ::1/128)
    UnixSocket: true, // Trust Unix domain socket connections
  },
})

app.Get("/", func(c fiber.Ctx) error {
  // If request came from trusted proxy, return true; else return false
  c.IsProxyTrusted()

  // ...
})
```

### IsSafe

Reports whether the request method is safe, meaning it is not expected to change server state (RFC 9110, Section 9.2.1).

```go title="Signature"
func (c fiber.Ctx) IsSafe() bool
func (r fiber.Req) IsSafe() bool
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  // GET, HEAD, OPTIONS, TRACE => true
  // POST, PUT, PATCH, DELETE  => false
  if !c.IsSafe() {
    return requireCSRFToken(c)
  }

  return c.Next()
})
```

### IsWebSocket

Returns `true` if the request includes a WebSocket upgrade handshake.

```go title="Signature"
func (c fiber.Ctx) IsWebSocket() bool
func (r fiber.Req) IsWebSocket() bool
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  if c.IsWebSocket() {
    // handle websocket
  }
  return c.Next()
})
```

### MediaType

Returns the MIME type from the `Content-Type` header without parameters.

```go title="Signature"
func (c fiber.Ctx) MediaType() string
func (r fiber.Req) MediaType() string
```

```go title="Example"
// Content-Type: application/json; charset=utf-8

app.Post("/", func(c fiber.Ctx) error {
  c.MediaType() // "application/json"
  return nil
})
```

### Method

Returns a string corresponding to the HTTP method of the request: `GET`, `POST`, `PUT`, and so on.
Optionally, you can override the method by passing a string.

Method tokens are case-sensitive (RFC 9110): the override is first matched exactly against the methods registered in [`Config.RequestMethods`](./fiber.md#requestmethods), and only falls back to the uppercase form as a convenience for the standard methods (e.g. `"get"` → `GET`). An unregistered override is ignored.

:::caution
Route registration (`app.Get`, `app.Add`, …) uppercases method names before validating them, so custom methods you want to **route** must be registered in `Config.RequestMethods` in uppercase. A mixed-case entry can be set via `c.Method(...)` but cannot have routes registered for it.
:::

```go title="Signature"
func (c fiber.Ctx) Method(override ...string) string
func (r fiber.Req) Method(override ...string) string
```

```go title="Example"
app.Post("/override", func(c fiber.Ctx) error {
  c.Method()          // "POST"

  c.Method("GET")
  c.Method()          // "GET"

  // ...
})
```

### MultipartForm

To access multipart form entries, you can parse the binary with `MultipartForm()`. This returns a `*multipart.Form`, allowing you to access form values and files. Parsing is bounded by the app [BodyLimit](./fiber.md#bodylimit).

:::caution

On a form request this lowercases the case-insensitive parts of the request's
own `Content-Type`, so a value obtained earlier from `Get(HeaderContentType)` —
which aliases those bytes unless [Immutable](./fiber.md#immutable) is set — can
change during the call. Copy it first if you need it to outlive one.

:::

```go title="Signature"
func (c fiber.Ctx) MultipartForm() (*multipart.Form, error)
func (r fiber.Req) MultipartForm() (*multipart.Form, error)
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Parse the multipart form:
  if form, err := c.MultipartForm(); err == nil {
    // => *multipart.Form

    if token := form.Value["token"]; len(token) > 0 {
      // Get key value:
      fmt.Println(token[0])
    }

    // Get all files from "documents" key:
    files := form.File["documents"]
    // => []*multipart.FileHeader

    // Loop through files:
    for _, file := range files {
      fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
      // => "tutorial.pdf" 360641 "application/pdf"

      // Save the files to disk:
      if err := c.SaveFile(file, fmt.Sprintf("./%s", file.Filename)); err != nil {
        return err
      }
    }
  }

  return nil
})
```

### Origin

Returns the `Origin` request header.

:::caution
Returned value is only valid within the handler. Do not store any references. Make copies or use the [**`Immutable`**](./fiber.md#config) setting to use the value outside the handler.
:::

```go title="Signature"
func (c fiber.Ctx) Origin() string
func (r fiber.Req) Origin() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Origin: https://example.com
  c.Origin() // => "https://example.com"

  // ...
})
```

### OriginalURL

Returns the original request URL.

```go title="Signature"
func (c fiber.Ctx) OriginalURL() string
func (r fiber.Req) OriginalURL() string
```

```go title="Example"
// GET http://example.com/search?q=something

app.Get("/", func(c fiber.Ctx) error {
  c.OriginalURL() // "/search?q=something"

  // ...
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

### Params

This method can be used to get the route parameters. You can pass an optional default value that will be returned if the param key does not exist.

:::info
Defaults to an empty string \(`""`\) if the param **doesn't** exist.
:::

```go title="Signature"
func (c fiber.Ctx) Params(key string, defaultValue ...string) string
func (r fiber.Req) Params(key string, defaultValue ...string) string
```

```go title="Example"
// GET http://example.com/user/fenny
app.Get("/user/:name", func(c fiber.Ctx) error {
  c.Params("name") // "fenny"

  // ...
})

// GET http://example.com/user/fenny/123
app.Get("/user/*", func(c fiber.Ctx) error {
  c.Params("*")  // "fenny/123"
  c.Params("*1") // "fenny/123"

  // ...
})
```

Unnamed route parameters \(\*, +\) can be fetched by the **character** and the **counter** in the route.

```go title="Example"
// ROUTE: /v1/*/shop/*
// GET:   /v1/brand/4/shop/blue/xs
c.Params("*1")  // "brand/4"
c.Params("*2")  // "blue/xs"
```

For reasons of **downward compatibility**, the first parameter segment for the parameter character can also be accessed without the counter.

```go title="Example"
app.Get("/v1/*/shop/*", func(c fiber.Ctx) error {
  c.Params("*") // outputs the value of the first wildcard segment
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

In certain scenarios, it can be useful to have an alternative approach to handle different types of parameters, not
just strings. This can be achieved using a generic `Params` function known as `Params[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V`.
This function is capable of parsing a route parameter and returning a value of a type that is assumed and specified by `V GenericType`.

```go title="Signature"
func Params[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V
```

```go title="Example"
// GET http://example.com/user/114
app.Get("/user/:id", func(c fiber.Ctx) error{
  fiber.Params[string](c, "id") // returns "114" as string.
  fiber.Params[int](c, "id")    // returns 114 as integer
  fiber.Params[string](c, "number") // returns "" (default string type)
  fiber.Params[int](c, "number")    // returns 0 (default integer value type)
})
```

The generic `Params` function supports returning the following data types based on `V GenericType`:

- Integer: `int`, `int8`, `int16`, `int32`, `int64`
- Unsigned integer: `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- Floating-point numbers: `float32`, `float64`
- Boolean: `bool`
- String: `string`
- Byte array: `[]byte`

### Path

Contains the path part of the request URL. Optionally, you can override the path by passing a string. For internal redirects, you might want to call [RestartRouting](ctx.md#restartrouting) instead of [Next](ctx.md#next).

```go title="Signature"
func (c fiber.Ctx) Path(override ...string) string
func (r fiber.Req) Path(override ...string) string
```

```go title="Example"
// GET http://example.com/users?sort=desc

app.Get("/users", func(c fiber.Ctx) error {
  c.Path()       // "/users"

  c.Path("/john")
  c.Path()       // "/john"

  // ...
})
```

### Port

Returns the remote port of the request.

```go title="Signature"
func (c fiber.Ctx) Port() string
func (r fiber.Req) Port() string
```

```go title="Example"
// GET http://example.com:8080

app.Get("/", func(c fiber.Ctx) error {
  c.Port() // "8080"

  // ...
})
```

### Protocol

Returns the HTTP protocol version of the request: `HTTP/1.1` or `HTTP/2`.

:::info
To get the request scheme (`http` or `https`), use [`Scheme`](#scheme) instead.
:::

```go title="Signature"
func (c fiber.Ctx) Protocol() string
func (r fiber.Req) Protocol() string
```

```go title="Example"
// GET http://example.com

app.Get("/", func(c fiber.Ctx) error {
  c.Protocol() // "HTTP/1.1"

  // ...
})
```

### Queries

`Queries` is a function that returns an object containing a property for each query string parameter in the route.

```go title="Signature"
func (c fiber.Ctx) Queries() map[string]string
func (r fiber.Req) Queries() map[string]string
```

```go title="Example"
// GET http://example.com/?name=alex&want_pizza=false&id=

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["name"]        // "alex"
    m["want_pizza"]  // "false"
    m["id"]          // ""
    // ...
})
```

```go title="Example"
// GET http://example.com/?field1=value1&field1=value2&field2=value3

app.Get("/", func (c fiber.Ctx) error {
    m := c.Queries()
    m["field1"] // "value2"
    m["field2"] // "value3"
})
```

```go title="Example"
// GET http://example.com/?list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["list_a"] // "3"
    m["list_b[]"] // "3"
    m["list_c"] // "1,2,3"
})
```

```go title="Example"
// GET /api/posts?filters.author.name=John&filters.category.name=Technology

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["filters.author.name"] // John
    m["filters.category.name"] // Technology
})
```

```go title="Example"
// GET /api/posts?tags=apple,orange,banana&filters[tags]=apple,orange,banana&filters[category][name]=fruits&filters.tags=apple,orange,banana&filters.category.name=fruits

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["tags"] // apple,orange,banana
    m["filters[tags]"] // apple,orange,banana
    m["filters[category][name]"] // fruits
    m["filters.tags"] // apple,orange,banana
    m["filters.category.name"] // fruits
})
```

### Query

This method returns a string corresponding to a query string parameter by name. You can pass an optional default value that will be returned if the query key does not exist.

:::info
If there is **no** query string, it returns an **empty string**.
:::

```go title="Signature"
func (c fiber.Ctx) Query(key string, defaultValue ...string) string
func (r fiber.Req) Query(key string, defaultValue ...string) string
```

```go title="Example"
// GET http://example.com/?order=desc&brand=nike

app.Get("/", func(c fiber.Ctx) error {
  c.Query("order")         // "desc"
  c.Query("brand")         // "nike"
  c.Query("empty", "nike") // "nike"

  // ...
})
```

:::info
The returned value is valid only within the handler. Do not store references.
Make copies or use the [**`Immutable`**](./fiber.md#immutable) setting instead. [Read more...](../#zero-allocation)
:::

In certain scenarios, it can be useful to have an alternative approach to handle different types of query parameters, not
just strings. This can be achieved using a generic `Query` function known as `Query[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V`.
This function is capable of parsing a query string and returning a value of a type that is assumed and specified by `V GenericType`.

Here is the signature for the generic `Query` function:

```go title="Signature"
func Query[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V
```

```go title="Example"
// GET http://example.com/?page=1&brand=nike&new=true

app.Get("/", func(c fiber.Ctx) error {
  fiber.Query[int](c, "page")     // 1
  fiber.Query[string](c, "brand") // "nike"
  fiber.Query[bool](c, "new")     // true

  // ...
})
```

In this case, `Query[V GenericType](c Ctx, key string, defaultValue ...V) V` can retrieve `page` as an integer, `brand` as a string, and `new` as a boolean. The function uses the appropriate parsing function for each specified type to ensure the correct type is returned. This simplifies the retrieval process of different types of query parameters, making your controller actions cleaner.
The generic `Query` function supports returning the following data types based on `V GenericType`:

- Integer: `int`, `int8`, `int16`, `int32`, `int64`
- Unsigned integer: `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- Floating-point numbers: `float32`, `float64`
- Boolean: `bool`
- String: `string`
- Byte array: `[]byte`

### Range

Returns a struct containing the type and a slice of ranges.
Only the canonical `bytes` unit is recognized and any optional
whitespace around range specifiers will be ignored, as specified
in RFC 9110. Empty list elements (e.g. `bytes=,0-5`) are ignored, though they
still count toward `Config.MaxRanges`.
A range with a non-numeric bound or a last position smaller than the first
position invalidates the whole header and `ErrRangeMalformed` (carrying a
**400 Bad Request** status) is returned, per RFC 9110.
A grammatically valid range unit other than `bytes` (e.g. `pages=1-3`) returns
`ErrRangeUnsupported`; RFC 9110 requires servers to **ignore** such a Range
header, so treat this error as "serve the full representation", not as a
failure. As a safety net the error carries a **400 Bad Request** status so
that blindly propagating it does not surface as a 500.
If the requested ranges are valid but none of them are satisfiable, the method
automatically sets the HTTP status code to **416 Range Not Satisfiable** and
populates the `Content-Range` header with the current representation size.

```go title="Signature"
func (c fiber.Ctx) Range(size int64) (Range, error)
func (r fiber.Req) Range(size int64) (Range, error)
```

```go title="Example"
// Range: bytes=500-700, 700-900
app.Get("/", func(c fiber.Ctx) error {
  r := c.Range(1000)
  if r.Type == "bytes" {
      for _, rng := range r.Ranges {
      fmt.Println(rng)
      // [500, 700]
    }
  }
})
```

### Referer

Returns the `Referer` request header.

```go title="Signature"
func (c fiber.Ctx) Referer() string
func (r fiber.Req) Referer() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Referer() // "https://example.com"
  return nil
})
```

### RequestID

```go title="Signature"
func (c fiber.Ctx) RequestID() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.RequestID() // "8d7ad5e3-aaf3-450b-a241-2beb887efd54"
  return nil
})
```

### SaveFile

Method is used to save **any** multipart file to disk.

```go title="Signature"
func (c fiber.Ctx) SaveFile(fh *multipart.FileHeader, path string) error
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Parse the multipart form:
  if form, err := c.MultipartForm(); err == nil {
    // => *multipart.Form

    // Get all files from "documents" key:
    files := form.File["documents"]
    // => []*multipart.FileHeader

    // Loop through files:
    for _, file := range files {
      fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
      // => "tutorial.pdf" 360641 "application/pdf"

      // Save the files to disk:
      if err := c.SaveFile(file, fmt.Sprintf("./%s", file.Filename)); err != nil {
        return err
      }
    }
    return err
  }
})
```

### SaveFileToStorage

Method is used to save **any** multipart file to an external storage system.

```go title="Signature"
func (c fiber.Ctx) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error
```

```go title="Example"
storage := memory.New()

app.Post("/", func(c fiber.Ctx) error {
  // Parse the multipart form:
  if form, err := c.MultipartForm(); err == nil {
    // => *multipart.Form

    // Get all files from "documents" key:
    files := form.File["documents"]
    // => []*multipart.FileHeader

    // Loop through files:
    for _, file := range files {
      fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
      // => "tutorial.pdf" 360641 "application/pdf"

      // Save the files to storage:
      if err := c.SaveFileToStorage(file, fmt.Sprintf("./%s", file.Filename), storage); err != nil {
        return err
      }
    }
    return err
  }
})
```

### Scheme

Contains the request protocol string: `http` or `https` for TLS requests.

:::info
Please use [`Config.TrustProxy`](fiber.md#trustproxy) to prevent header spoofing if your app is behind a proxy.
:::

:::note

Only `http` and `https` are ever returned. When the proxy is trusted, the
forwarding headers (`X-Forwarded-Proto`, `X-Forwarded-Protocol`,
`X-Forwarded-Ssl`, `X-Url-Scheme`) are read, but a value naming anything else is
ignored rather than passed through — the result is spliced into
[`BaseURL`](#baseurl) and compared for origin equality by CSRF and
`Redirect().Back()`, so a header announcing, say, `javascript` must not become
part of a URL. A proxy that terminates a different protocol should send the
scheme the client used to reach it.

:::

```go title="Signature"
func (c fiber.Ctx) Scheme() string
func (r fiber.Req) Scheme() string
```

```go title="Example"
// GET http://example.com

app.Get("/", func(c fiber.Ctx) error {
  c.Scheme() // "http"

  // ...
})
```

### Secure

A boolean property that is `true` if a **TLS** connection is established.

```go title="Signature"
func (c fiber.Ctx) Secure() bool
func (r fiber.Req) Secure() bool
```

```go title="Example"
// Secure() method is equivalent to:
c.Scheme() == "https"
```

### Stale

When the client's cached response is **stale**, this method returns **true**. It
is the logical complement of [`Fresh`](#fresh), which checks whether the cached
representation is still valid.

[https://expressjs.com/en/4x/api.html#req.stale](https://expressjs.com/en/4x/api.html#req.stale)

```go title="Signature"
func (c fiber.Ctx) Stale() bool
func (r fiber.Req) Stale() bool
```

### Subdomains

Returns a slice with the host’s sub-domain labels. The dot-separated parts that precede the registrable domain (`example`) and the top-level domain (ex: `com`).

The `subdomain offset` (default `2`) tells Fiber how many labels, counting from the right-hand side, are always discarded.
Passing an `offset` argument lets you override that value for a single call.

```go
func (c fiber.Ctx) Subdomains(offset ...int) []string
```

| `offset`               | Result                                  | Meaning                                       |
| ---------------------- | --------------------------------------- | --------------------------------------------- |
| *omitted* → **2**      | trim 2 right-most labels                | drop the registrable domain **and** the TLD   |
| `1` to `len(labels)-1` | trim exactly `offset` right-most labels | custom trimming of available labels           |
| `>= len(labels)`       | **return `[]`**                         | offset exceeds available labels → empty slice |
| `0`                    | **return every label**                  | keep the entire host unchanged                |
| `< 0`                  | **return `[]`**                         | negative offsets are invalid → empty slice    |

#### Example

```go
// Host: "tobi.ferrets.example.com"

app.Get("/", func(c fiber.Ctx) error {
  c.Subdomains()    // ["tobi", "ferrets"]
  c.Subdomains(1)   // ["tobi", "ferrets", "example"]
  c.Subdomains(0)   // ["tobi", "ferrets", "example", "com"]
  c.Subdomains(-1)  // []
  // ...
})
```

### URI

Returns the parsed [`*fasthttp.URI`](https://pkg.go.dev/github.com/valyala/fasthttp#URI) of the request, which gives access to every fasthttp URI method.

:::caution
The returned value is owned by the request and is rewritten by a [`Path`](#path) override, so it is only valid within the handler. Do not store any references.
:::

```go title="Signature"
func (c fiber.Ctx) URI() *fasthttp.URI
func (r fiber.Req) URI() *fasthttp.URI
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // GET http://example.com/search?q=fiber#results
  uri := c.URI()
  uri.QueryString() // => "q=fiber"
  uri.Hash()        // => "results"

  // ...
})
```

### UserAgent

Returns the `User-Agent` request header.

```go title="Signature"
func (c fiber.Ctx) UserAgent() string
func (r fiber.Req) UserAgent() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.UserAgent() // "Mozilla/5.0 ..."
  return nil
})
```

### XHR

A boolean property that is `true` if the request’s [X-Requested-With](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers) header field is [XMLHttpRequest](https://developer.mozilla.org/en-US/docs/Web/API/XMLHttpRequest), indicating that the request was issued by a client library (such as [jQuery](https://api.jquery.com/jQuery.ajax/)).

```go title="Signature"
func (c fiber.Ctx) XHR() bool
func (r fiber.Req) XHR() bool
```

```go title="Example"
// X-Requested-With: XMLHttpRequest

app.Get("/", func(c fiber.Ctx) error {
  c.XHR() // true

  // ...
})
```

## Response

Methods which modify the response object.

:::tip
Use `c.Res()` to limit gopls suggestions to only these methods!
:::

Each entry lists both forms it can be called in. `DefaultCtx` embeds `DefaultRes`, so the method is promoted: `c.Del(key)` and `c.Res().Del(key)` are the same call.

:::caution
Three names are defined on both `Req` and `Res`: `Body`, `ContentLength` and `ContentType`. On `Ctx` the **request wins**, the way [`Get`](#get) already does, so `c.Body()` reads the request body. The response counterparts are listed below as [`Body (Res)`](#body-res), [`ContentLength (Res)`](#contentlength-res) and [`ContentType (Res)`](#contenttype-res), and are reachable only through `c.Res()` — which is why they list a single `fiber.Res` signature. The response cookies are [`GetCookies`](#getcookies): named apart from `Req.Cookies` so that a `Ctx` still satisfies `fiber.Res`, which a same-name/different-signature pair would prevent.
:::

### Add

Appends the given value to the response header field as a **new field line**, leaving any existing lines untouched.

This differs from [`Append`](#append), which folds values into a single comma-separated line: headers whose values may themselves contain commas — `WWW-Authenticate`, `Link` — have to be sent as separate lines to stay unambiguous (RFC 9110, Section 5.3).

:::caution
`Add` does not append for most of the headers fasthttp stores in a slot of their own: `Content-Type`, `Content-Encoding`, `Content-Length`, `Connection`, `Server` and `Trailer` are **replaced**, and `Transfer-Encoding` and `Date` are **ignored** because fasthttp writes them itself. Use [`Set`](#set) for those. `Set-Cookie` is slotted too but does repeat, so `Add` appends a line — prefer [`Cookie`](#cookie), which builds the field value for you.
:::

```go title="Signature"
func (c fiber.Ctx) Add(key, val string)
func (r fiber.Res) Add(key, val string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Add(fiber.HeaderWWWAuthenticate, `Basic realm="api"`)
  c.Add(fiber.HeaderWWWAuthenticate, `Bearer realm="api"`)
  // => WWW-Authenticate: Basic realm="api"
  // => WWW-Authenticate: Bearer realm="api"

  return c.SendStatus(fiber.StatusUnauthorized)
})
```

### Append

Appends the specified **value** to the HTTP response header field.

:::caution
If the header is **not** already set, it creates the header with the specified value.
:::

Empty values are skipped, since a sender must not generate empty list elements (RFC 9110).

```go title="Signature"
func (c fiber.Ctx) Append(field string, values ...string)
func (r fiber.Res) Append(field string, values ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Append("Link", "http://google.com", "http://localhost")
  // => Link: http://google.com, http://localhost

  c.Append("Link", "Test")
  // => Link: http://google.com, http://localhost, Test

  // ...
})
```

### Attachment

Sets the HTTP response [Content-Disposition](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition) header field to `attachment`.

```go title="Signature"
func (c fiber.Ctx) Attachment(filename ...string)
func (r fiber.Res) Attachment(filename ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Attachment()
  // => Content-Disposition: attachment

  c.Attachment("./upload/images/logo.png")
  // => Content-Disposition: attachment; filename="logo.png"
  // => Content-Type: image/png

  // ...
})
```

The `filename` parameter is emitted as an RFC 9110 quoted-string: spaces and
punctuation stay literal, and quotes/backslashes are escaped with a backslash
(no URL encoding). Non-ASCII filenames additionally carry the `filename*`
parameter as defined in
[RFC 6266](https://www.rfc-editor.org/rfc/rfc6266) and
[RFC 8187](https://www.rfc-editor.org/rfc/rfc8187):

```go title="Example"
app.Get("/non-ascii", func(c fiber.Ctx) error {
  c.Attachment("./files/文件.txt")
  // => Content-Disposition: attachment; filename="文件.txt"; filename*=UTF-8''%E6%96%87%E4%BB%B6.txt
  return nil
})
```

### AutoFormat

Performs content-negotiation on the [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header. It uses [Accepts](ctx.md#accepts) to select a proper format.
The supported content types are `text/html`, `text/plain`, `application/json`, `application/vnd.msgpack`, `application/xml`, and `application/cbor`.
Because the representation is selected from the Accept header, `Vary: Accept` is added to the response.
For more flexible content negotiation, use [Format](ctx.md#format).

:::info
If the header is **not** specified or there is **no** proper format, **text/plain** is used.
:::

```go title="Signature"
func (c fiber.Ctx) AutoFormat(body any) error
func (r fiber.Res) AutoFormat(body any) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Accept: text/plain
  c.AutoFormat("Hello, World!")
  // => Hello, World!

  // Accept: text/html
  c.AutoFormat("Hello, World!")
  // => <p>Hello, World!</p>

  type User struct {
    Name string
  }
  user := User{"John Doe"}

  // Accept: application/json
  c.AutoFormat(user)
  // => {"Name":"John Doe"}

  // Accept: application/vnd.msgpack
  c.AutoFormat(user)
  // => 82 a4 6e 61 6d 65 a4 6a 6f 68 6e a4 70 61 73 73 a3 64 6f 65

  // Accept: application/cbor
  c.AutoFormat(user)
  // => a1 64 4e 61 6d 65 68 4a 6f 68 6e 20 44 6f 65

  // Accept: application/xml
  c.AutoFormat(user)
  // => <User><Name>John Doe</Name></User>
  // ..
})
```

### Body (Res)

Returns the response body buffered so far, which lets middleware inspect or checksum what a handler produced before it is written out.

Reached through `c.Res()`: `Req` and `Res` both carry a `Body`, so `c.Body()` is the **request** body.

:::note
A streamed body returns `nil` rather than being drained. Reading it would pull the whole stream into memory and turn the response into a buffered one, which would hang an SSE route and defeat a large [`SendFile`](#sendfile). Use [`Written`](#written) to tell a streaming response from one that produced nothing, and the underlying `c.Response().Body()` if you really do want to materialize the stream.
:::

:::caution
The returned slice is the response's live buffer, not a copy — writing through it writes to the response, as it does for the request-side [`Body`](#body). It is only valid within the handler and is invalidated by the next write to the response. Do not store any references; copy it instead.
:::

```go title="Signature"
func (r fiber.Res) Body() []byte
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  c.Set(fiber.HeaderETag, etag.Generate(c.Res().Body()))
  return nil
})
```

### CBOR

CBOR converts any interface or string to CBOR encoded bytes.

> **Note:** Before using any CBOR-related features, make sure to follow the [CBOR setup instructions](../guide/advance-format.md#cbor).

:::info
CBOR also sets the content header to the `ctype` parameter. If no `ctype` is passed in, the header is set to `application/cbor`.
:::

```go title="Signature"
func (c fiber.Ctx) CBOR(data any, ctype ...string) error
func (r fiber.Res) CBOR(data any, ctype ...string) error
```

```go title="Example"
type SomeStruct struct {
  Name string `cbor:"name"`
  Age  uint8 `cbor:"age"`
}

app.Get("/cbor", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.CBOR(data)
  // => Content-Type: application/cbor
  // => \xa2dnameeGramecage\x14

  return c.CBOR(fiber.Map{
    "name": "Grame",
    "age":  20,
  })
  // => Content-Type: application/cbor
  // => \xa2dnameeGramecage\x14

  return c.CBOR(fiber.Map{
    "type":     "https://example.com/probs/out-of-credit",
    "title":    "You do not have enough credit.",
    "status":   403,
    "detail":   "Your current balance is 30, but that costs 50.",
    "instance": "/account/12345/msgs/abc",
  })
  // => Content-Type: application/cbor
  // => \xa5dtypex'https://example.com/probs/out-of-creditetitlex\x1eYou do not have enough credit.fstatus\x19\x01\x93fdetailx.Your current balance is 30, but that costs 50.hinstancew/account/12345/msgs/abc
})
```

### ClearCookie

Expires a client cookie (or all cookies if left empty).

```go title="Signature"
func (c fiber.Ctx) ClearCookie(key ...string)
func (r fiber.Res) ClearCookie(key ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Clears all cookies:
  c.ClearCookie()

  // Expire specific cookie by name:
  c.ClearCookie("user")

  // Expire multiple cookies by names:
  c.ClearCookie("token", "session", "track_id", "version")
  // ...
})
```

:::caution
Web browsers and other compliant clients will only clear the cookie if the given options are identical to those when creating the cookie, excluding `Expires` and `MaxAge`. `ClearCookie` will not set these values for you - a technique similar to the one shown below should be used to ensure your cookie is deleted.
:::

```go title="Example"
app.Get("/set", func(c fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        Value:    "randomvalue",
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: true,
        SameSite: "Lax",
    })

    // ...
})

app.Get("/delete", func(c fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        Expires:  fasthttp.CookieExpireDelete, // Use fasthttp's built-in constant
        HTTPOnly: true,
        SameSite: "Lax",
    })

    // ...
})
```

You can also use `c.Cookie()` to expire cookies with specific `Path` or `Domain` attributes:

```go title="Example"
app.Get("/logout", func(c fiber.Ctx) error {
    // Expire a cookie with path and domain
    c.Cookie(&fiber.Cookie{
        Name:    "token",
        Path:    "/api",
        Domain:  "example.com",
        Expires: fasthttp.CookieExpireDelete,
    })

    return c.SendStatus(fiber.StatusOK)
})
```

### ContentLength (Res)

Returns the value of the `Content-Length` response header.

Reached through `c.Res()`: `Req` and `Res` both carry a `ContentLength`, so `c.ContentLength()` is the **request** header.

:::note
It reports what the header **declares** — a length a handler, middleware or upstream response set, and `-1` once the body is a stream of unknown length. It is not a count of buffered bytes: fasthttp fills `Content-Length` in as it serializes the response, so this is `0` inside a handler unless something set it explicitly. Use `len(c.Res().Body())` for the bytes buffered so far.
:::

```go title="Signature"
func (r fiber.Res) ContentLength() int
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  log.Printf("declared %d bytes, buffered %d", c.Res().ContentLength(), len(c.Res().Body()))
  return nil
})
```

### ContentType (Res)

Returns the `Content-Type` response header, parameters included. It is the read side of [`Type`](#type), and of the content type Fiber sets for you when a `JSON`, `XML`, or `SendFile` response goes out.

:::note
When nothing has set one it reports what would be sent: fasthttp's default, `text/plain; charset=utf-8`, or an empty string under [`DisableDefaultContentType`](./fiber.md#config).

`Req` and `Res` both carry a `ContentType`, so on `Ctx` the request wins, as it does for [`Get`](#get). This section is the response side, reached through `c.Res()`.
:::

```go title="Signature"
func (r fiber.Res) ContentType() string
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  if strings.HasPrefix(c.Res().ContentType(), fiber.MIMEApplicationJSON) {
    c.Set("X-Content-Type-Options", "nosniff")
  }

  return nil
})
```

### Cookie

Sets a cookie.

```go title="Signature"
func (c fiber.Ctx) Cookie(cookie *Cookie)
func (r fiber.Res) Cookie(cookie *Cookie)
```

```go
type Cookie struct {
    Name        string    `json:"name"`         // The name of the cookie
    Value       string    `json:"value"`        // The value of the cookie
    Path        string    `json:"path"`         // Specifies a URL path which is allowed to receive the cookie
    Domain      string    `json:"domain"`       // Specifies the domain which is allowed to receive the cookie
    MaxAge      int       `json:"max_age"`      // The maximum age (in seconds) of the cookie
    Expires     time.Time `json:"expires"`      // The expiration date of the cookie
    Secure      bool      `json:"secure"`       // Indicates that the cookie should only be transmitted over a secure HTTPS connection
    HTTPOnly    bool      `json:"http_only"`    // Indicates that the cookie is accessible only through the HTTP protocol
    SameSite    string    `json:"same_site"`    // Controls whether or not a cookie is sent with cross-site requests
    Partitioned bool      `json:"partitioned"`  // Indicates if the cookie is stored in a partitioned cookie jar
    SessionOnly bool      `json:"session_only"` // Indicates if the cookie is a session-only cookie
}
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Create cookie
  cookie := new(fiber.Cookie)
  cookie.Name = "john"
  cookie.Value = "doe"
  cookie.Expires = time.Now().Add(24 * time.Hour)

  // Set cookie
  c.Cookie(cookie)
  // ...
})
```

:::info
When setting a cookie with `SameSite=None`, Fiber automatically sets `Secure=true` as required by RFC 6265bis and modern browsers. This ensures compliance with the "None" SameSite policy which mandates that cookies must be sent over secure connections.

For more information, see:

- [Mozilla Documentation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#none)
- [Chrome Documentation](https://developers.google.com/search/blog/2020/01/get-ready-for-new-samesitenone-secure)

:::

:::info
Partitioned cookies allow partitioning the cookie jar by top-level site, enhancing user privacy by preventing cookies from being shared across different sites. This feature is particularly useful in scenarios where a user interacts with embedded third-party services that should not have access to the main site's cookies. You can check out [CHIPS](https://developers.google.com/privacy-sandbox/3pcd/chips) for more information.
:::

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Create a new partitioned cookie
  cookie := new(fiber.Cookie)
  cookie.Name = "user_session"
  cookie.Value = "abc123"
  cookie.Partitioned = true  // This cookie will be stored in a separate jar when it's embedded into another website

  // Set the cookie in the response
  c.Cookie(cookie)
  return c.SendString("Partitioned cookie set")
})
```

### Del

Removes every field line of the response header specified by `key`. Field names are case-insensitive. Deleting a header that was never set is a no-op.

:::caution
`Del(fiber.HeaderSetCookie)` withdraws every cookie this response was going to set, which is not what [`ClearCookie`](#clearcookie) does: `ClearCookie` adds a `Set-Cookie` that expires the cookie already in the client's jar.
:::

```go title="Signature"
func (c fiber.Ctx) Del(key string)
func (r fiber.Res) Del(key string)
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  c.Del("X-Powered-By")
  return nil
})
```

### Download

Transfers the file from the given path as an `attachment`.

Typically, browsers will prompt the user to download. By default, the [Content-Disposition](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition) header `filename=` parameter is the file path (this typically appears in the browser dialog).
Override this default with the `filename` parameter.

```go title="Signature"
func (c fiber.Ctx) Download(file string, filename ...string) error
func (r fiber.Res) Download(file string, filename ...string) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.Download("./files/report-12345.pdf")
  // => Download report-12345.pdf

  return c.Download("./files/report-12345.pdf", "report.pdf")
  // => Download report.pdf
})
```

The `filename` parameter is emitted as an RFC 9110 quoted-string (no URL
encoding). For filenames containing non-ASCII characters, a `filename*`
parameter is added according to
[RFC 6266](https://www.rfc-editor.org/rfc/rfc6266) and
[RFC 8187](https://www.rfc-editor.org/rfc/rfc8187):

```go title="Example"
app.Get("/non-ascii", func(c fiber.Ctx) error {
  return c.Download("./files/文件.txt")
  // => Content-Disposition: attachment; filename="文件.txt"; filename*=UTF-8''%E6%96%87%E4%BB%B6.txt
})
```

### Drop

Terminates the client connection silently without sending any HTTP headers or response body.

This can be used for scenarios where you want to block certain requests without notifying the client, such as mitigating
DDoS attacks or protecting sensitive endpoints from unauthorized access.

```go title="Signature"
func (c fiber.Ctx) Drop() error
func (r fiber.Res) Drop() error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  if c.IP() == "192.168.1.1" {
    return c.Drop()
  }

  return c.SendString("Hello World!")
})
```

### End

End immediately flushes the current response and closes the underlying connection.

```go title="Signature"
func (c fiber.Ctx) End() error
func (r fiber.Res) End() error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    c.SendString("Hello World!")
    return c.End()
})
```

:::caution
Calling `c.End()` will disallow further writes to the underlying connection.
:::

:::warning
`c.End()` does **not** work in streaming mode (e.g. when using `fasthttp`'s `HijackConn` or `SendStream`).
In streaming mode the connection is managed asynchronously and `ctx.Conn()` may return `nil`,
so `c.End()` will return `nil` without flushing or closing the connection.
:::

End can be used to stop a middleware from modifying a response of a handler/other middleware down the method chain
when they regain control after calling `c.Next()`.

```go title="Example"
// Error Logging/Responding middleware
app.Use(func(c fiber.Ctx) error {
    err := c.Next()

    // Log errors & write the error to the response
    if err != nil {
        log.Printf("Got error in middleware: %v", err)
        return c.Writef("(got error %v)", err)
    }

    // No errors occurred
    return nil
})

// Handler with simulated error
app.Get("/", func(c fiber.Ctx) error {
    // Closes the connection instantly after writing from this handler
    // and disallow further modification of its response
    defer c.End()

    c.SendString("Hello, ... I forgot what comes next!")
    return errors.New("some error")
})
```

### Format

Performs content-negotiation on the [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header. It uses [Accepts](ctx.md#accepts) to select a proper format from the supplied offers. A default handler can be provided by setting the `MediaType` to `"default"`. If no offers match and no default is provided, a 406 (Not Acceptable) response is sent. The Content-Type is automatically set when a handler is selected.

:::info
If the Accept header is **not** specified, the first handler with a real media type will be used (entries with the `"default"` media type are skipped, since `"default"` is not a valid Content-Type value). If only a `"default"` handler is supplied, it is called without setting the Content-Type.
:::

```go title="Signature"
func (c fiber.Ctx) Format(handlers ...ResFmt) error
func (r fiber.Res) Format(handlers ...ResFmt) error
```

```go title="Example"
// Accept: application/json => {"command":"eat","subject":"fruit"}
// Accept: text/plain => Eat Fruit!
// Accept: application/xml => Not Acceptable
app.Get("/no-default", func(c fiber.Ctx) error {
  return c.Format(
    fiber.ResFmt{"application/json", func(c fiber.Ctx) error {
      return c.JSON(fiber.Map{
        "command": "eat",
        "subject": "fruit",
      })
    }},
    fiber.ResFmt{"text/plain", func(c fiber.Ctx) error {
      return c.SendString("Eat Fruit!")
    }},
  )
})

// Accept: application/json => {"command":"eat","subject":"fruit"}
// Accept: text/plain => Eat Fruit!
// Accept: application/xml => Eat Fruit!
app.Get("/default", func(c fiber.Ctx) error {
  textHandler := func(c fiber.Ctx) error {
    return c.SendString("Eat Fruit!")
  }

  handlers := []fiber.ResFmt{
    {"application/json", func(c fiber.Ctx) error {
      return c.JSON(fiber.Map{
        "command": "eat",
        "subject": "fruit",
      })
    }},
    {"text/plain", textHandler},
    {"default", textHandler},
  }

  return c.Format(handlers...)
})
```

### GetCookie

Reads back a cookie this response is already set to send, so a later handler or middleware can inspect or re-emit what an earlier one wrote. The second result is `false` when no cookie of that name has been set, or when its `Set-Cookie` field value does not parse.

Cookie names are case-sensitive (RFC 6265, Section 4.1.1). A name written more than once resolves to the **first** occurrence; use [`GetCookies`](#getcookies) to see them all.

:::note
The returned `Cookie` is a copy: changing it does not change the response. Pass it to [`Cookie`](#cookie) to write the change back.
:::

:::caution
A `Set-Cookie` that carried no `Path` comes back with `Path` empty, and [`Cookie`](#cookie) writes an empty `Path` as `/`. Re-emitting such a cookie therefore widens it from the browser's RFC 6265 default-path — the directory of the request URI — to the whole origin. Set `Path` explicitly before writing one back. Cookies written through [`Cookie`](#cookie) always carry a `Path`, so this only arises for a `Set-Cookie` added with [`Add`](#add) or copied from an upstream response.
:::

```go title="Signature"
func (c fiber.Ctx) GetCookie(name string) (*fiber.Cookie, bool)
func (r fiber.Res) GetCookie(name string) (*fiber.Cookie, bool)
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  // Encrypt a session cookie an inner handler set, keeping its attributes.
  if cookie, ok := c.GetCookie("session"); ok {
    cookie.Value = encrypt(cookie.Value)
    c.Cookie(cookie)
  }

  return nil
})
```

### GetCookies

Returns a copy of every cookie this response is set to send, in the order they were added. It returns `nil` when none have been set.

Repeated names are kept apart, so a response that sets one name at two paths yields both. A `Set-Cookie` whose field value does not parse is skipped.

Named to sit beside [`GetCookie`](#getcookie) rather than mirroring `Req.Cookies`: the same name under a different signature would stop `fiber.Ctx` satisfying `fiber.Res`. `c.Cookies(key)` reads what the **client** sent.

```go title="Signature"
func (c fiber.Ctx) GetCookies() []*fiber.Cookie
func (r fiber.Res) GetCookies() []*fiber.Cookie
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  for _, cookie := range c.Res().GetCookies() {
    log.Printf("setting %s on %s", cookie.Name, cookie.Path)
  }

  return nil
})
```

### JSON

Converts any **interface** or **string** to JSON using the [encoding/json](https://pkg.go.dev/encoding/json) package.

:::info
JSON also sets the content header to the `ctype` parameter. If no `ctype` is passed in, the header is set to `application/json; charset=utf-8` by default.
:::

```go title="Signature"
func (c fiber.Ctx) JSON(data any, ctype ...string) error
func (r fiber.Res) JSON(data any, ctype ...string) error
```

```go title="Example"
type SomeStruct struct {
  Name string
  Age  uint8
}

app.Get("/json", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.JSON(data)
  // => Content-Type: application/json; charset=utf-8
  // => {"Name": "Grame", "Age": 20}

  return c.JSON(fiber.Map{
    "name": "Grame",
    "age":  20,
  })
  // => Content-Type: application/json; charset=utf-8
  // => {"name": "Grame", "age": 20}

  return c.JSON(fiber.Map{
    "type":     "https://example.com/probs/out-of-credit",
    "title":    "You do not have enough credit.",
    "status":   403,
    "detail":   "Your current balance is 30, but that costs 50.",
    "instance": "/account/12345/msgs/abc",
  }, "application/problem+json")
  // => Content-Type: application/problem+json
  // => "{
  // =>     "type": "https://example.com/probs/out-of-credit",
  // =>     "title": "You do not have enough credit.",
  // =>     "status": 403,
  // =>     "detail": "Your current balance is 30, but that costs 50.",
  // =>     "instance": "/account/12345/msgs/abc",
  // => }"
})
```

### JSONP

Sends a JSON response with JSONP support. This method is identical to [JSON](ctx.md#json), except that it opts-in to JSONP callback support. By default, the callback name is simply `callback`.

Override this by passing a **named string** in the method.

The callback name is reduced to a JavaScript member expression: every character outside `[A-Za-z0-9_$.[]]` is dropped, so names like `window.cb`, `ns.cb[0]`, and `$.jsonp_1` pass through unchanged. Because the name is written straight into a same-origin `text/javascript` body — and JSONP callers normally take it from the query string — leaving it unfiltered would let a request supply arbitrary script for your own origin. If what survives is not a valid member expression, the default `callback` is used: `cb[0]` is emitted, `cb[0x]` is not.

```go title="Signature"
func (c fiber.Ctx) JSONP(data any, callback ...string) error
func (r fiber.Res) JSONP(data any, callback ...string) error
```

```go title="Example"
type SomeStruct struct {
  Name string
  Age  uint8
}

app.Get("/", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.JSONP(data)
  // => callback({"Name": "Grame", "Age": 20})

  return c.JSONP(data, "customFunc")
  // => customFunc({"Name": "Grame", "Age": 20})
})
```

### Links

Joins the links followed by the property to populate the response’s [Link HTTP header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Link) field.
Quotes and backslashes in the `rel` value are escaped so the emitted quoted-string stays grammar-valid per RFC 9110.

```go title="Signature"
func (c fiber.Ctx) Links(link ...string)
func (r fiber.Res) Links(link ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Links(
    "http://api.example.com/users?page=2", "next",
    "http://api.example.com/users?page=5", "last",
  )
  // Link: <http://api.example.com/users?page=2>; rel="next",
  //       <http://api.example.com/users?page=5>; rel="last"

  // ...
})
```

### Location

Sets the response [Location](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Location) HTTP header to the specified path parameter.

```go title="Signature"
func (c fiber.Ctx) Location(path string)
func (r fiber.Res) Location(path string)
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  c.Location("http://example.com")

  c.Location("/foo/bar")

  return nil
})
```

### MsgPack

> **Note:** Before using any MsgPack-related features, make sure to follow the [MsgPack setup instructions](../guide/advance-format.md#msgpack).

A compact binary alternative to [JSON](#json) for efficient data transfer between micro-services or from server to client. MessagePack serializes faster and yields smaller payloads than plain JSON.

Converts any **interface** or **string** to MsgPack using the [shamaton/msgpack](https://pkg.go.dev/github.com/shamaton/msgpack/v3) package.

:::info
MsgPack also sets the content header to the `ctype` parameter. If no `ctype` is passed in, the header is set to `application/vnd.msgpack`.
:::

```go title="Signature"
func (c fiber.Ctx) MsgPack(data any, ctype ...string) error
func (r fiber.Res) MsgPack(data any, ctype ...string) error
```

```go title="Example"
type SomeStruct struct {
  Name string
  Age  uint8
}

app.Get("/msgpack", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.MsgPack(data)
  // => Content-Type: application/vnd.msgpack
  // => 82 A4 4E 61 6D 65 A5 47 72 61 6D 65 A3 41 67 65 14

  return c.MsgPack(fiber.Map{
    "name": "Grame",
    "age":  20,
  })
  // => Content-Type: application/vnd.msgpack
  // => 82 A4 6E 61 6D 65 A5 47 72 61 6D 65 A3 61 67 65 14

  return c.MsgPack(fiber.Map{
    "type":     "https://example.com/probs/out-of-credit",
    "title":    "You do not have enough credit.",
    "status":   403,
    "detail":   "Your current balance is 30, but that costs 50.",
    "instance": "/account/12345/msgs/abc",
  }, "application/problem+msgpack")
})

// => Content-Type: application/problem+msgpack
// 85 A4 74 79 70 65 D9 27 68 74 74 70 73 3A 2F 2F 65 78 61 6D 70 6C 65 2E 63 6F 6D 2F 70 72 6F 62 73 2F 6F 75 74 2D 6F 66 2D 63 72 65 64 69 74 A5 74 69 74 6C 65 BE 59 6F 75 20 64 6F 20 6E 6F 74 20 68 61 76 65 20 65 6E 6F 75 67 68 20 63 72 65 64 69 74 2E A6 73 74 61 74 75 73 CD 01 93 A6 64 65 74 61 69 6C D9 2E 59 6F 75 72 20 63 75 72 72 65 6E 74 20 62 61 6C 61 6E 63 65 20 69 73 20 33 30 2C 20 62 75 74 20 74 68 61 74 20 63 6F 73 74 73 20 35 30 2E A8 69 6E 73 74 61 6E 63 65 B7 2F 61 63 63 6F 75 6E 74 2F 31 32 33 34 35 2F 6D 73 67 73 2F 61 62 63
```

### NoContent

Replies `204 No Content`. [`SendStatus`](#sendstatus) already discards the body for every status that disallows one; this drops the `Content-Type` as well, since RFC 9110, Section 6.4.1 gives a `204` no content to describe.

:::note
`c.SendStatus(fiber.StatusNoContent)` leaves a `Content-Type` a handler had set, so the two are not interchangeable — this is the one that sends nothing about content.
:::

```go title="Signature"
func (c fiber.Ctx) NoContent() error
func (r fiber.Res) NoContent() error
```

```go title="Example"
app.Delete("/item/:id", func(c fiber.Ctx) error {
  if err := delete(c.Params("id")); err != nil {
    return err
  }

  return c.NoContent() // => 204, no body
})
```

### Render

Renders a view with data and sends a `text/html` response. By default, `Render` uses the default [**Go Template engine**](https://pkg.go.dev/html/template/). If you want to use another view engine, please take a look at our [**Template middleware**](https://docs.gofiber.io/template).

```go title="Signature"
func (c fiber.Ctx) Render(name string, bind any, layouts ...string) error
func (r fiber.Res) Render(name string, bind any, layouts ...string) error
```

### ResetBody

Discards the response body, keeping the status and headers. Use it before replacing a partially written body — an error page over a half-rendered view, a cached body over a fresh one.

```go title="Signature"
func (c fiber.Ctx) ResetBody()
func (r fiber.Res) ResetBody()
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err == nil {
    return nil
  }

  c.ResetBody() // Drop whatever the handler managed to write.
  return c.Status(fiber.StatusInternalServerError).SendString("something went wrong")
})
```

### Send

Sets the HTTP response body.

```go title="Signature"
func (c fiber.Ctx) Send(body []byte) error
func (r fiber.Res) Send(body []byte) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.Send([]byte("Hello, World!")) // => "Hello, World!"
})
```

Fiber also provides `SendString` and `SendStream` methods for raw inputs.

:::tip
Use this if you **don't need** type assertion, recommended for **faster** performance.
:::

```go title="Signature"
func (c fiber.Ctx) SendString(body string) error
func (c fiber.Ctx) SendStream(stream io.Reader, size ...int) error
func (r fiber.Res) SendString(body string) error
func (r fiber.Res) SendStream(stream io.Reader, size ...int) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.SendString("Hello, World!")
  // => "Hello, World!"

  return c.SendStream(bytes.NewReader([]byte("Hello, World!")))
  // => "Hello, World!"
})
```

### SendEarlyHints

Sends an informational `103 Early Hints` response with one or more
[`Link` headers](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Link)
before the final response. This allows the browser to start preloading
resources while the server prepares the full response.

:::caution
This feature requires HTTP/2 or newer. Some legacy HTTP/1.1 clients may not support sendEarlyHints.
Early Hints (`103` responses) are supported in HTTP/2 and newer. Older HTTP/1.1 clients may ignore these interim responses or misbehave when receiving them.
See [Enabling HTTP/2](../guide/reverse-proxy#enabling-http2) for instructions on how to use a reverse proxy (e.g. Nginx or Traefik) to enable HTTP/2 support.
:::

For requests that are not HTTP/1.1 (e.g. HTTP/1.0), no interim `103` response is
sent — RFC 9110 forbids sending 1xx responses to HTTP/1.0 clients — but the
`Link` headers are still included in the final response.

:::caution
Interim responses need Fiber's own server. When the app is mounted into
`net/http` via the `adaptor` middleware, there is no client connection for
interim responses: the `103` is silently skipped and the `Link` headers are
still delivered on the final response.
:::

```go title="Signature"
func (c fiber.Ctx) SendEarlyHints(hints []string) error
func (r fiber.Res) SendEarlyHints(hints []string) error
```

```go title="Example"
hints := []string{"<https://cdn.com/app.js>; rel=preload; as=script"}
app.Get("/early", func(c fiber.Ctx) error {
  if err := c.SendEarlyHints(hints); err != nil {
    return err
  }
  return c.SendString("done")
})
```

### SendFile

Transfers the file from the given path. Sets the [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) response HTTP header field based on the **file** extension or format.

```go title="Config" title="Config"
// SendFile defines configuration options when to transfer file with SendFile.
type SendFile struct {
  // FS is the file system to serve the static files from.
  // You can use interfaces compatible with fs.FS like embed.FS, os.DirFS etc.
  //
  // Optional. Default: nil
  FS fs.FS

  // When set to true, the server tries minimizing CPU usage by caching compressed files.
  // This works differently than the github.com/gofiber/compression middleware.
  // You have to set Content-Encoding header to compress the file.
  // Available compression methods are gzip, br, and zstd.
  // The request's Accept-Encoding header is left untouched either way, so the
  // compress middleware can still compress the response when this is false.
  //
  // Optional. Default: false
  Compress bool `json:"compress"`

  // When set to true, enables byte range requests.
  //
  // Optional. Default: false
  ByteRange bool `json:"byte_range"`

  // When set to true, enables direct download.
  //
  // Optional. Default: false
  Download bool `json:"download"`

  // Expiration duration for inactive file handlers.
  // Use a negative time.Duration to disable it.
  //
  // Optional. Default: 10 * time.Second
  CacheDuration time.Duration `json:"cache_duration"`

  // The value for the Cache-Control HTTP-header
  // that is set on the file response. MaxAge is defined in seconds.
  //
  // Optional. Default: 0
  MaxAge int `json:"max_age"`
}
```

```go title="Signature" title="Signature"
func (c fiber.Ctx) SendFile(file string, config ...SendFile) error
func (r fiber.Res) SendFile(file string, config ...SendFile) error
```

```go title="Example"
app.Get("/not-found", func(c fiber.Ctx) error {
  return c.SendFile("./public/404.html")

  // Serve pre-compressed copies, caching them to save CPU. This is the file
  // server's own compression, not the compress middleware, which keeps
  // compressing the response either way.
  return c.SendFile("./static/index.html", fiber.SendFile{
    Compress: true,
  })
})
```

:::info
If the file contains a URL-specific character, you have to escape it before passing the file path into the `SendFile` function.
:::

```go title="Example"
app.Get("/file-with-url-chars", func(c fiber.Ctx) error {
  return c.SendFile(url.PathEscape("hash_sign_#.txt"))
})
```

:::info
You can set the `CacheDuration` config property to `-1` to disable caching.
:::

```go title="Example"
app.Get("/file", func(c fiber.Ctx) error {
  return c.SendFile("style.css", fiber.SendFile{
    CacheDuration: -1,
  })
})
```

:::info
You can use multiple `SendFile` calls with different configurations in a single route. Fiber creates different filesystem handlers per config.
:::

```go title="Example"
app.Get("/file", func(c fiber.Ctx) error {
  switch c.Query("config") {
    case "filesystem":
      return c.SendFile("style.css", fiber.SendFile{
        FS: os.DirFS(".")
      })
    case "filesystem-compress":
      return c.SendFile("style.css", fiber.SendFile{
        FS: os.DirFS("."),
        Compress: true,
      })
    case "compress":
      return c.SendFile("style.css", fiber.SendFile{
        Compress: true,
      })
    default:
      return c.SendFile("style.css")
  }

  return nil
})
```

:::info
For sending multiple files from an embedded file system, [this functionality](../middleware/static.md#serving-files-using-embedfs) can be used.
:::

### SendStatus

Sets the status code and the correct status message in the body if the response body is **empty**.

:::tip
You can find all used status codes and messages [in the Fiber source code](https://github.com/gofiber/fiber/blob/dffab20bcdf4f3597d2c74633a7705a517d2c8c2/utils.go#L183-L244).
:::

```go title="Signature"
func (c fiber.Ctx) SendStatus(status int) error
func (r fiber.Res) SendStatus(status int) error
```

```go title="Example"
app.Get("/not-found", func(c fiber.Ctx) error {
  return c.SendStatus(415)
  // => 415 "Unsupported Media Type"

  c.SendString("Hello, World!")
  return c.SendStatus(415)
  // => 415 "Hello, World!"
})
```

### SendStream

Sets the response body to a stream of data and adds an optional body size.

```go title="Signature"
func (c fiber.Ctx) SendStream(stream io.Reader, size ...int) error
func (r fiber.Res) SendStream(stream io.Reader, size ...int) error
```

:::info
`SendStream` operates asynchronously. The handler returns immediately after setting up the stream,
but the actual reading and sending of data happens **after** the handler completes. This is handled
by the underlying `fasthttp` library.

If the provided stream implements `io.Closer`, it will be automatically closed by `fasthttp` after
the response is fully sent or if an error occurs.
:::

:::caution
When passing `fiber.Ctx` as a `context.Context` to libraries that spawn goroutines (e.g., for streaming operations),
those goroutines may attempt to access the context after the handler returns. Since `fiber.Ctx` is recycled and
released after the handler completes, this can cause issues.

**Recommended approach**: Use `c.Context()` or `c.RequestCtx()` instead of passing `c` directly to such libraries.
See the [Context Guide](../guide/context.md) for more details.
:::

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.SendStream(bytes.NewReader([]byte("Hello, World!")))
  // => "Hello, World!"
})
```

```go title="Example with file streaming"
app.Get("/download", func(c fiber.Ctx) error {
  file, err := os.Open("large-file.zip")
  if err != nil {
    return err
  }
  // File will be automatically closed by fasthttp after streaming completes
  
  stat, err := file.Stat()
  if err != nil {
    file.Close()
    return err
  }
  
  return c.SendStream(file, int(stat.Size()))
})
```

### SendStreamWriter

Sets the response body stream writer.

:::note
The argument `streamWriter` represents a function that populates
the response body using a buffered stream writer.
:::

```go title="Signature"
func (c fiber.Ctx) SendStreamWriter(streamWriter func(*bufio.Writer)) error
func (r fiber.Res) SendStreamWriter(streamWriter func(*bufio.Writer)) error
```

```go title="Example"
app.Get("/", func (c fiber.Ctx) error {
  return c.SendStreamWriter(func(w *bufio.Writer) {
    fmt.Fprintf(w, "Hello, World!\n")
  })
  // => "Hello, World!"
})
```

:::info
To send data before `streamWriter` returns, you can call `w.Flush()`
on the provided writer. Otherwise, the buffered stream flushes after
`streamWriter` returns.
:::

:::note
`w.Flush()` will return an error if the client disconnects before `streamWriter` finishes writing a response.
:::

```go title="Example"
app.Get("/wait", func(c fiber.Ctx) error {
  return c.SendStreamWriter(func(w *bufio.Writer) {
    // Begin Work
    fmt.Fprintf(w, "Please wait for 10 seconds\n")
    if err := w.Flush(); err != nil {
      log.Print("Client disconnected!")
      return
    }

    // Send progress over time
    time.Sleep(time.Second)
    for i := 0; i < 9; i++ {
      fmt.Fprintf(w, "Still waiting...\n")
      if err := w.Flush(); err != nil {
        // If client disconnected, cancel work and finish
        log.Print("Client disconnected!")
        return
      }
      time.Sleep(time.Second)
    }

    // Finish
    fmt.Fprintf(w, "Done!\n")
  })
})
```

### SendString

Sets the response body to a string.

```go title="Signature"
func (c fiber.Ctx) SendString(body string) error
func (r fiber.Res) SendString(body string) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.SendString("Hello, World!")
  // => "Hello, World!"
})
```

### Set

Sets the response’s HTTP header field to the specified `key`, `value`.

```go title="Signature"
func (c fiber.Ctx) Set(key string, val string)
func (r fiber.Res) Set(key string, val string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Set("Content-Type", "text/plain")
  // => "Content-Type: text/plain"

  // ...
})
```

### Status

Sets the HTTP status for the response.

:::info
This method is **chainable**.
:::

```go title="Signature"
func (c fiber.Ctx) Status(status int) fiber.Ctx
func (r fiber.Res) Status(status int) fiber.Ctx
```

```go title="Example"
app.Get("/fiber", func(c fiber.Ctx) error {
  c.Status(fiber.StatusOK)
  return nil
})

app.Get("/hello", func(c fiber.Ctx) error {
  return c.Status(fiber.StatusBadRequest).SendString("Bad Request")
})

app.Get("/world", func(c fiber.Ctx) error {
  return c.Status(fiber.StatusNotFound).SendFile("./public/gopher.png")
})
```

### StatusCode

Returns the status code currently set on the response. It is the read side of [`Status`](#status), and reports `200` until something sets another code.

Called after [`Next`](#next) it is the status the chain settled on, which is what logging, metrics, and caching middleware key off.

```go title="Signature"
func (c fiber.Ctx) StatusCode() int
func (r fiber.Res) StatusCode() int
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  err := c.Next()

  if c.StatusCode() >= fiber.StatusInternalServerError {
    alert(c.Route().Path, c.StatusCode())
  }

  return err
})
```

### Type

Sets the [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) HTTP header to the MIME type listed [in the Nginx MIME types configuration](https://github.com/nginx/nginx/blob/master/conf/mime.types) specified by the file **extension**.

:::info
This method is **chainable**.
:::

```go title="Signature"
func (c fiber.Ctx) Type(ext string, charset ...string) fiber.Ctx
func (r fiber.Res) Type(ext string, charset ...string) fiber.Ctx
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Type(".html") // => "text/html"
  c.Type("html")  // => "text/html"
  c.Type("png")   // => "image/png"

  c.Type("json", "utf-8")  // => "application/json; charset=utf-8"

  // ...
})
```

### Vary

Adds the given header field to the [Vary](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Vary) response header. This will append the header if not already listed; otherwise, it leaves it listed in the current location.

:::info
Multiple fields are **allowed**. Per RFC 9110, the wildcard `"*"` is only meaningful as the sole member of the field: adding `"*"` collapses the header to a single `*`, and once `*` is present no further fields are appended.
:::

```go title="Signature"
func (c fiber.Ctx) Vary(fields ...string)
func (r fiber.Res) Vary(fields ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Vary("Origin")     // => Vary: Origin
  c.Vary("User-Agent") // => Vary: Origin, User-Agent

  // No duplicates
  c.Vary("Origin") // => Vary: Origin, User-Agent

  c.Vary("Accept-Encoding", "Accept")
  // => Vary: Origin, User-Agent, Accept-Encoding, Accept

  c.Vary("*") // => Vary: *

  // ...
})
```

### Write

Adopts the `Writer` interface.

```go title="Signature"
func (c fiber.Ctx) Write(p []byte) (n int, err error)
func (r fiber.Res) Write(p []byte) (n int, err error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Write([]byte("Hello, World!")) // => "Hello, World!"

  fmt.Fprintf(c, "%s\n", "Hello, World!") // => "Hello, World!"
})
```

### Writef

Writes a formatted string using a format specifier.

```go title="Signature"
func (c fiber.Ctx) Writef(format string, a ...any) (n int, err error)
func (r fiber.Res) Writef(format string, a ...any) (n int, err error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  world := "World!"
  c.Writef("Hello, %s", world) // => "Hello, World!"

  fmt.Fprintf(c, "%s\n", "Hello, World!") // => "Hello, World!"
})
```

### WriteString

Writes a string to the response body.

```go title="Signature"
func (c fiber.Ctx) WriteString(s string) (n int, err error)
func (r fiber.Res) WriteString(s string) (n int, err error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.WriteString("Hello, World!")
  // => "Hello, World!"
})
```

### Written

Reports whether anything has been written to the response body yet, so middleware can tell a handler that produced a response from one that left it untouched. A streamed body counts as written without draining the stream, which is the case [`Body`](#body-res) deliberately answers `nil` for.

:::note
Status and headers are not body writes: a handler that only called [`Status`](#status) leaves this `false`.
:::

```go title="Signature"
func (c fiber.Ctx) Written() bool
func (r fiber.Res) Written() bool
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  if err := c.Next(); err != nil {
    return err
  }

  if !c.Written() {
    return c.NoContent()
  }

  return nil
})
```

### XML

Converts any **interface** or **string** to XML using the standard `encoding/xml` package.

:::info
XML also sets the content header to `application/xml; charset=utf-8`.
:::

```go title="Signature"
func (c fiber.Ctx) XML(data any) error
func (r fiber.Res) XML(data any) error
```

```go title="Example"
type SomeStruct struct {
  XMLName xml.Name `xml:"Fiber"`
  Name    string   `xml:"Name"`
  Age     uint8    `xml:"Age"`
}

app.Get("/", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.XML(data)
  // <Fiber>
  //     <Name>Grame</Name>
  //     <Age>20</Age>
  // </Fiber>
})
```
