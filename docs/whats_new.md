---
id: whats_new
title: 🆕 What's New in v3
sidebar_position: 2
toc_max_heading_level: 4
---

## 🎉 Welcome

We are excited to announce the release of Fiber v3! 🚀

In this guide, we'll walk you through the most important changes in Fiber `v3` and show you how to migrate your existing Fiber `v2` applications to Fiber `v3`.

### 🛠️ Migration tool

Fiber v3 introduces a CLI-powered migration helper. Install the CLI and let
it update your project automatically:

```bash
go install github.com/gofiber/cli/fiber@latest
fiber migrate --to v3.0.0-rc.3
```

See the [migration guide](#-migration-guide) for more details and options.

Here's a quick overview of the changes in Fiber `v3`:

- [🚀 App](#-app)
- [🎣 Hooks](#-hooks)
- [🚀 Listen](#-listen)
- [🗺️ Router](#-router)
- [🧠 Context](#-context)
- [📎 Binding](#-binding)
- [🔬 Extractors Package](#-extractors-package)
- [🔄️ Redirect](#-redirect)
- [🌎 Client package](#-client-package)
- [🧰 Generic functions](#-generic-functions)
- [🛠️ Utils](#utils)
- [🧩 Services](#-services)
- [📃 Log](#-log)
- [📦 Storage Interface](#-storage-interface)
- [🧬 Middlewares](#-middlewares)
  - [Important Change for Accessing Middleware Data](#important-change-for-accessing-middleware-data)
  - [Adaptor](#adaptor)
  - [BasicAuth](#basicauth)
  - [Cache](#cache)
  - [CORS](#cors)
  - [CSRF](#csrf)
  - [Compression](#compression)
  - [EncryptCookie](#encryptcookie)
  - [Filesystem](#filesystem)
  - [Healthcheck](#healthcheck)
  - [KeyAuth](#keyauth)
  - [Logger](#logger)
  - [Monitor](#monitor)
  - [Proxy](#proxy)
  - [Session](#session)
- [🔌 Addons](#-addons)
- [📋 Migration guide](#-migration-guide)

## Drop for old Go versions

Fiber `v3` drops support for Go versions below `1.25`. We recommend upgrading to Go `1.25` or higher to use Fiber `v3`.

## 🚀 App

We have made several changes to the Fiber app, including:

- **Listen**: The `Listen` method has been unified with the configuration, allowing for more streamlined setup.
- **Static**: The `Static` method has been removed and its functionality has been moved to the [static middleware](./middleware/static.md).
- **app.Config properties**: Several properties have been moved to the listen configuration:
  - `DisableStartupMessage`
  - `EnablePrefork` (previously `Prefork`)
  - `EnablePrintRoutes`
  - `ListenerNetwork` (previously `Network`)
- **Trusted Proxy Configuration**: The `EnabledTrustedProxyCheck` has been moved to `app.Config.TrustProxy`, and `TrustedProxies` has been moved to `TrustProxyConfig.Proxies`.
- **XMLDecoder Config Property**: The `XMLDecoder` property has been added to allow usage of 3rd-party XML libraries in XML binder.

### New Methods

- **RegisterCustomBinder**: Allows for the registration of custom binders.
- **RegisterCustomConstraint**: Allows for the registration of custom constraints.
- **NewWithCustomCtx**: Initialize an app with a custom context in one step.
- **State**: Provides a global state for the application, which can be used to store and retrieve data across the application. Check out the [State](./api/state) method for further details.
- **NewErrorf**: Allows variadic parameters when creating formatted errors.
- **GetBytes / GetString**: Helpers that detach values only when `Immutable` is enabled and the data still references request or response buffers. Access via `c.App().GetString` and `c.App().GetBytes`.
- **ReloadViews**: Lets you re-run the configured view engine's `Load()` logic at runtime, including guard rails for missing or nil view engines so development hot-reload hooks can refresh templates safely.

#### Custom Route Constraints

Custom route constraints enable you to define your own validation rules for route parameters.
Use `RegisterCustomConstraint` to add a constraint type that implements the `CustomConstraint` interface.

<details>
<summary>Example</summary>

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

app.RegisterCustomConstraint(&UlidConstraint{})

app.Get("/login/:id<ulid>", func(c fiber.Ctx) error {
    return c.SendString("User " + c.Params("id"))
})
```

</details>

### Removed Methods

- **Mount**: Use `app.Use()` instead.
- **ListenTLS**: Use `app.Listen()` with `tls.Config`.
- **ListenTLSWithCertificate**: Use `app.Listen()` with `tls.Config`.
- **ListenMutualTLS**: Use `app.Listen()` with `tls.Config`.
- **ListenMutualTLSWithCertificate**: Use `app.Listen()` with `tls.Config`.

### Method Changes

- **Test**: The `Test` method has replaced the timeout parameter with a configuration parameter. `0` or lower represents no timeout.
- **Listen**: Now has a configuration parameter.
- **Listener**: Now has a configuration parameter.

### Custom Ctx Interface in Fiber v3

Fiber v3 introduces a customizable `Ctx` interface, allowing developers to extend and modify the context to fit their needs. This feature provides greater flexibility and control over request handling.

#### Idea Behind Custom Ctx Classes

The idea behind custom `Ctx` classes is to give developers the ability to extend the default context with additional methods and properties tailored to the specific requirements of their application. This allows for better request handling and easier implementation of specific logic.

#### NewWithCustomCtx

`NewWithCustomCtx` creates the application and sets the custom context factory at initialization time.

```go title="Signature"
func NewWithCustomCtx(fn func(app *App) CustomCtx, config ...Config) *App
```

<details>
<summary>Example</summary>

```go
package main

import (
    "log"
    "github.com/gofiber/fiber/v3"
)

type CustomCtx struct {
    fiber.DefaultCtx
}

func (c *CustomCtx) CustomMethod() string {
    return "custom value"
}

func main() {
    app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
        return &CustomCtx{
            DefaultCtx: *fiber.NewDefaultCtx(app),
        }
    })

    app.Get("/", func(c fiber.Ctx) error {
        customCtx := c.(*CustomCtx)
        return c.SendString(customCtx.CustomMethod())
    })

    log.Fatal(app.Listen(":3000"))
}
```

This example creates a `CustomCtx` with an extra `CustomMethod` and initializes the app with `NewWithCustomCtx`.

</details>

### Configurable TLS Minimum Version

We have added support for configuring the TLS minimum version. This field allows you to set the TLS minimum version for TLSAutoCert and the server listener.

```go
app.Listen(":444", fiber.ListenConfig{TLSMinVersion: tls.VersionTLS12})
```

#### TLS AutoCert support (ACME / Let's Encrypt)

We have added native support for automatic certificates management from Let's Encrypt and any other ACME-based providers.

```go
// Certificate manager
certManager := &autocert.Manager{
    Prompt: autocert.AcceptTOS,
    // Replace with your domain name
    HostPolicy: autocert.HostWhitelist("example.com"),
    // Folder to store the certificates
    Cache: autocert.DirCache("./certs"),
}

app.Listen(":444", fiber.ListenConfig{
    AutoCertManager:    certManager,
})
```

### MIME Constants

`MIMEApplicationJavaScript` and `MIMEApplicationJavaScriptCharsetUTF8` are deprecated. Use `MIMETextJavaScript` and `MIMETextJavaScriptCharsetUTF8` instead.

## 🎣 Hooks

We have made several changes to the Fiber hooks, including:

- Added new shutdown hooks to provide better control over the shutdown process:
  - `OnPreShutdown` - Executes before the server starts shutting down
  - `OnPostShutdown` - Executes after the server has shut down, receives any shutdown error
  - `OnPreStartupMessage` - Executes before the startup message is printed, allowing customization of the banner and info entries
  - `OnPostStartupMessage` - Executes after the startup message is printed, allowing post-startup logic
- Deprecated `OnShutdown` in favor of the new pre/post shutdown hooks
- Improved shutdown hook execution order and reliability
- Added mutex protection for hook registration and execution

Important: When using shutdown hooks, ensure app.Listen() is called in a separate goroutine:

```go
// Correct usage
go app.Listen(":3000")
// ... register shutdown hooks
app.Shutdown()

// Incorrect usage - hooks won't work
app.Listen(":3000") // This blocks
app.Shutdown()      // Never reached
```

## 🚀 Listen

We have made several changes to the Fiber listen, including:

- Removed `OnShutdownError` and `OnShutdownSuccess` from `ListenerConfig` in favor of using `OnPostShutdown` hook which receives the shutdown error

```go
app := fiber.New()

// Before - using ListenerConfig callbacks
app.Listen(":3000", fiber.ListenerConfig{
    OnShutdownError: func(err error) {
        log.Printf("Shutdown error: %v", err)
    },
    OnShutdownSuccess: func() {
        log.Println("Shutdown successful")
    },
})

// After - using OnPostShutdown hook
app.Hooks().OnPostShutdown(func(err error) error {
    if err != nil {
        log.Printf("Shutdown error: %v", err)
    } else {
        log.Println("Shutdown successful")
    }
    return nil
})
go app.Listen(":3000")
```

This change simplifies the shutdown handling by consolidating the shutdown callbacks into a single hook that receives the error status.

- Added support for Unix domain sockets via `ListenerNetwork` and `UnixSocketFileMode`

```go
// v2 - Requires manual deletion of old file and permissions change
app := fiber.New(fiber.Config{
    Network: "unix",
})

os.Remove("app.sock")
app.Hooks().OnListen(func(fiber.ListenData) error {
    return os.Chmod("app.sock", 0770)
})
app.Listen("app.sock")

// v3 - Fiber does it for you
app := fiber.New()
app.Listen("app.sock", fiber.ListenerConfig{
    ListenerNetwork:    fiber.NetworkUnix,
    UnixSocketFileMode: 0770,
})
```

- Expanded `ListenData` with versioning, handler, process, and PID metadata, plus dedicated startup message hooks for customization. Check out the [Hooks](./api/hooks#startup-message-customization) documentation for further details.

```go title="Customize the startup message"
package main

import (
    "fmt"
    "os"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Hooks().OnPreStartupMessage(func(sm *fiber.PreStartupMessageData) error {
        sm.BannerHeader = "FOOBER " + sm.Version + "\n-------"

        // Optional: you can also remove old entries
        // sm.ResetEntries()

        sm.AddInfo("git-hash", "Git hash", os.Getenv("GIT_HASH"))
        sm.AddInfo("prefork", "Prefork", fmt.Sprintf("%v", sm.Prefork), 15)
        return nil
    })

    app.Hooks().OnPostStartupMessage(func(sm *fiber.PostStartupMessageData) error {
        if !sm.Disabled && !sm.IsChild && !sm.Prevented {
            fmt.Println("startup completed")
        }
        return nil
    })

    app.Listen(":5000")
}
```

## 🗺 Router

We have slightly adapted our router interface

### Handler compatibility

Fiber now ships with a routing adapter (see `adapter.go`) that understands native Fiber handlers alongside `net/http` and `fasthttp` handlers. Route registration helpers accept a required `handler` argument plus optional additional `handlers`, all typed as `any`, and the adapter transparently converts supported handler styles so you can keep using the ecosystem functions you're familiar with.

To align even closer with Express, you can also register handlers that accept the new `fiber.Req` and `fiber.Res` helper interfaces. The adapter understands both two-argument (`func(fiber.Req, fiber.Res)`) and three-argument (`func(fiber.Req, fiber.Res, func() error)`) callbacks, regardless of whether they return an `error`. When you include the optional `next` callback, Fiber wires it to `c.Next()` for you so middleware continues to behave as expected. If your handler returns an `error`, the value returned from the injected `next()` bubbles straight back to the caller. When your handler omits an `error` return, Fiber records the result of `next()` and returns it after your function exits so downstream failures still propagate.

| Case | Handler signature | Notes |
| ---- | ----------------- | ----- |
| 1 | `fiber.Handler` | Native Fiber handler. |
| 2 | `func(fiber.Ctx)` | Fiber handler without an error return. |
| 3 | `func(fiber.Req, fiber.Res) error` | Express-style request handler with error return. |
| 4 | `func(fiber.Req, fiber.Res)` | Express-style request handler without error return. |
| 5 | `func(fiber.Req, fiber.Res, func() error) error` | Express-style middleware with an error-returning `next` callback and handler error return. |
| 6 | `func(fiber.Req, fiber.Res, func() error)` | Express-style middleware with an error-returning `next` callback. |
| 7 | `func(fiber.Req, fiber.Res, func()) error` | Express-style middleware with a no-argument `next` callback and handler error return. |
| 8 | `func(fiber.Req, fiber.Res, func())` | Express-style middleware with a no-argument `next` callback. |
| 9 | `http.HandlerFunc` | Standard-library handler function adapted through `fasthttpadaptor`. |
| 10 | `http.Handler` | Standard-library handler implementation; pointer receivers must be non-nil. |
| 11 | `func(http.ResponseWriter, *http.Request)` | Standard-library function handlers via `fasthttpadaptor`. |
| 12 | `fasthttp.RequestHandler` | Direct fasthttp handler without error return. |
| 13 | `func(*fasthttp.RequestCtx) error` | fasthttp handler that returns an error to Fiber. |

### Route chaining

`RouteChain` is a new helper inspired by [`Express`](https://expressjs.com/en/api.html#app.route) that makes it easy to declare a stack of handlers on the same path, while the existing `Route` helper stays available for prefix encapsulation.

```go
RouteChain(path string) Register
```

<details>
<summary>Example</summary>

```go
app.RouteChain("/api").RouteChain("/user/:id?")
    .Get(func(c fiber.Ctx) error {
        // Get user
        return c.JSON(fiber.Map{"message": "Get user", "id": c.Params("id")})
    })
    .Post(func(c fiber.Ctx) error {
        // Create user
        return c.JSON(fiber.Map{"message": "User created"})
    })
    .Put(func(c fiber.Ctx) error {
        // Update user
        return c.JSON(fiber.Map{"message": "User updated", "id": c.Params("id")})
    })
    .Delete(func(c fiber.Ctx) error {
        // Delete user
        return c.JSON(fiber.Map{"message": "User deleted", "id": c.Params("id")})
    })
```

</details>

You can find more information about `app.RouteChain` and `app.Route` in the API documentation ([RouteChain](./api/app#routechain), [Route](./api/app#route)).

### Automatic HEAD routes for GET

Fiber now auto-registers a `HEAD` route whenever you add a `GET` route. The generated handler chain matches the `GET` chain so status codes and headers stay in sync while the response body remains empty, ensuring `HEAD` clients observe the same metadata as a `GET` consumer.

```go title="GET now enables HEAD automatically"
app := fiber.New()

app.Get("/health", func(c fiber.Ctx) error {
    c.Set("X-Service", "api")
    return c.SendString("OK")
})

// HEAD /health reuses the GET middleware chain and returns headers only.
```

You can still register explicit `HEAD` handlers for any `GET` route, and they continue to win when you add them:

```go title="Override the generated HEAD handler"
app.Head("/health", func(c fiber.Ctx) error {
    return c.SendStatus(fiber.StatusNoContent)
})
```

Prefer to manage `HEAD` routes yourself? Disable the feature through `fiber.Config.DisableHeadAutoRegister`:

```go title="Disable automatic HEAD registration"
handler := func(c fiber.Ctx) error {
    c.Set("X-Service", "api")
    return c.SendString("OK")
}

app := fiber.New(fiber.Config{DisableHeadAutoRegister: true})
app.Get("/health", handler) // HEAD /health now returns 405 unless you add it manually.
```

Auto-generated `HEAD` routes appear in tooling such as `app.Stack()` and cover the same routing scenarios as their `GET` counterparts, including groups, mounted apps, dynamic parameters, and static file handlers.

### Middleware registration

We have aligned our method for middlewares closer to [`Express`](https://expressjs.com/en/api.html#app.use) and now also support the [`Use`](./api/app#use) of multiple prefixes.

Prefix matching is now stricter: partial matches must end at a slash boundary (or be an exact match). This keeps `/api` middleware from running on `/apiv1` while still allowing `/api/:version` style patterns that leverage route parameters, optional segments, or wildcards.

Registering a subapp is now also possible via the [`Use`](./api/app#use) method instead of the old `app.Mount` method.

<details>
<summary>Example</summary>

```go
// register multiple prefixes
app.Use(["/v1", "/v2"], func(c fiber.Ctx) error {
    // Middleware for /v1 and /v2
    return c.Next()
})

// define subapp
api := fiber.New()
api.Get("/user", func(c fiber.Ctx) error {
    return c.SendString("User")
})
// register subapp
app.Use("/api", api)
```

</details>

To enable the routing changes above we had to slightly adjust the signature of the `Add` method.

```diff
-    Add(method, path string, handlers ...Handler) Router
+    Add(methods []string, path string, handler any, handlers ...any) Router
```

### Test Config

The `app.Test()` method now allows users to customize their test configurations:

<details>
<summary>Example</summary>

```go
// Create a test app with a handler to test
app := fiber.New()
app.Get("/", func(c fiber.Ctx) {
    return c.SendString("hello world")
})

// Define the HTTP request and custom TestConfig to test the handler
req := httptest.NewRequest(MethodGet, "/", nil)
testConfig := fiber.TestConfig{
    Timeout:       0,
    FailOnTimeout: false,
}

// Test the handler using the request and testConfig
resp, err := app.Test(req, testConfig)
```

</details>

To provide configurable testing capabilities, we had to change
the signature of the `Test` method.

```diff
-    Test(req *http.Request, timeout ...time.Duration) (*http.Response, error)
+    Test(req *http.Request, config ...fiber.TestConfig) (*http.Response, error)
```

The `TestConfig` struct provides the following configuration options:

- `Timeout`: The duration to wait before timing out the test. Use 0 for no timeout.
- `FailOnTimeout`: Controls the behavior when a timeout occurs:
  - When true, the test will return an `os.ErrDeadlineExceeded` if the test exceeds the `Timeout` duration.
  - When false, the test will return the partial response received before timing out.

If a custom `TestConfig` isn't provided, then the following will be used:

```go
testConfig := fiber.TestConfig{
    Timeout:       time.Second,
    FailOnTimeout: true,
}
```

**Note:** Using this default is **NOT** the same as providing an empty `TestConfig` as an argument to `app.Test()`.

An empty `TestConfig` is the equivalent of:

```go
testConfig := fiber.TestConfig{
    Timeout:       0,
    FailOnTimeout: false,
}
```

## 🧠 Context

### New Features

- Cookie now allows Partitioned cookies for [CHIPS](https://developers.google.com/privacy-sandbox/3pcd/chips) support. CHIPS (Cookies Having Independent Partitioned State) is a feature that improves privacy by allowing cookies to be partitioned by top-level site, mitigating cross-site tracking.
- Cookie automatic security enforcement: When setting a cookie with `SameSite=None`, Fiber automatically sets `Secure=true` as required by RFC 6265bis and modern browsers (Chrome, Firefox, Safari). This ensures compliance with the "None" SameSite policy. See [Mozilla docs](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#none) and [Chrome docs](https://developers.google.com/search/blog/2020/01/get-ready-for-new-samesitenone-secure) for details.
- `Ctx` now implements the [context.Context](https://pkg.go.dev/context#Context) interface, replacing the former `UserContext` helpers.

### New Methods

- **AutoFormat**: Similar to Express.js, automatically formats the response based on the request's `Accept` header.
- **Deadline**: For implementing `context.Context`.
- **Done**: For implementing `context.Context`.
- **Err**: For implementing `context.Context`.
- **Host**: Similar to Express.js, returns the host name of the request.
- **Port**: Similar to Express.js, returns the port number of the request.
- **IsProxyTrusted**: Checks the trustworthiness of the remote IP.
- **Reset**: Resets context fields for server handlers.
- **Schema**: Similar to Express.js, returns the schema (HTTP or HTTPS) of the request.
- **SendEarlyHints**: Sends `HTTP 103 Early Hints` status code with `Link` headers so browsers can preload resources while the final response is being prepared.
- **SendStream**: Similar to Express.js, sends a stream as the response.
- **SendStreamWriter**: Sends a stream using a writer function.
- **SendString**: Similar to Express.js, sends a string as the response.
- **String**: Similar to Express.js, converts a value to a string.
- **Value**: For implementing `context.Context`. Returns request-scoped value from Locals.
- **Context()**: Returns a `context.Context` that can be used outside the handler.
- **SetContext**: Sets the base `context.Context` returned by `Context()` for propagating deadlines or values.
- **ViewBind**: Binds data to a view, replacing the old `Bind` method.
- **CBOR**: Introducing [CBOR](https://cbor.io/) binary encoding format for both request & response body. CBOR is a binary data serialization format which is both compact and efficient, making it ideal for use in web applications.
- **MsgPack**: Introducing [MsgPack](https://msgpack.org/) binary encoding format for both request & response body. MsgPack is a binary serialization format that is more efficient than JSON, making it ideal for high-performance applications.
- **Drop**: Terminates the client connection silently without sending any HTTP headers or response body. This can be used for scenarios where you want to block certain requests without notifying the client, such as mitigating DDoS attacks or protecting sensitive endpoints from unauthorized access.
- **End**: Similar to Express.js, immediately flushes the current response and closes the underlying connection.
- **AcceptsLanguagesExtended**: Matches language ranges using RFC 4647 Extended Filtering with wildcard subtags.
- **Matched**: Detects when the current request path matched a registered route.
- **IsMiddleware**: Indicates if the current handler was registered as middleware.
- **HasBody**: Quickly checks whether the request includes a body.
- **OverrideParam**: Overwrites the value of an existing route parameter, or does nothing if the parameter does not exist
- **IsWebSocket**: Reports if the request attempts a WebSocket upgrade.
- **IsPreflight**: Identifies CORS preflight requests before handlers run.

### Removed Methods

- **AllParams**: Use `c.Bind().URI()` instead.
- **ParamsInt**: Use `Params` with generic types.
- **QueryBool**: Use `Query` with generic types.
- **QueryFloat**: Use `Query` with generic types.
- **QueryInt**: Use `Query` with generic types.
- **BodyParser**: Use `c.Bind().Body()` instead.
- **CookieParser**: Use `c.Bind().Cookie()` instead.
- **ParamsParser**: Use `c.Bind().URI()` instead.
- **RedirectToRoute**: Use `c.Redirect().Route()` instead.
- **RedirectBack**: Use `c.Redirect().Back()` instead.
- **ReqHeaderParser**: Use `c.Bind().Header()` instead.
- **UserContext**: Removed. `Ctx` itself now satisfies `context.Context`; pass `c` directly where a `context.Context` is required.
- **SetUserContext**: Removed. Use `SetContext` and `Context()` or `context.WithValue` on `c` to store additional request-scoped values.

### Changed Methods

- **Bind**: Now used for binding instead of view binding. Use `c.ViewBind()` for view binding.
- **Format**: Parameter changed from `body interface{}` to `handlers ...ResFmt`.
- **Redirect**: Use `c.Redirect().To()` instead.
- **SendFile**: Now supports different configurations using a config parameter.
- **Attachment and Download**: Non-ASCII filenames now use `filename*` as
  specified by [RFC 6266](https://www.rfc-editor.org/rfc/rfc6266) and
  [RFC 8187](https://www.rfc-editor.org/rfc/rfc8187).
- **Context()**: Renamed to `RequestCtx()` to access the underlying `fasthttp.RequestCtx`.

### SendEarlyHints

`SendEarlyHints` sends an informational [`103 Early Hints`](https://developer.chrome.com/docs/web-platform/early-hints) response with `Link` headers based on the provided `hints` argument. This allows a browser to start preloading assets while the server is still preparing the final response.

```go
hints := []string{"<https://cdn.com/app.js>; rel=preload; as=script"}
app.Get("/early", func(c fiber.Ctx) error {
    if err := c.SendEarlyHints(hints); err != nil {
        return err
    }
    return c.SendString("done")
})
```

Older HTTP/1.1 clients may ignore these interim responses or handle them inconsistently.

### SendStreamWriter

In v3, we introduced support for buffered streaming with the addition of the `SendStreamWriter` method:

```go
func (c Ctx) SendStreamWriter(streamWriter func(w *bufio.Writer))
```

With this new method, you can implement:

- Server-Side Events (SSE)
- Large file downloads
- Live data streaming

```go
app.Get("/sse", func(c fiber.Ctx) {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")

    return c.SendStreamWriter(func(w *bufio.Writer) {
        for {
            fmt.Fprintf(w, "event: my-event\n")
            fmt.Fprintf(w, "data: Hello SSE\n\n")

            if err := w.Flush(); err != nil {
                log.Print("Client disconnected!")
                return
            }
        }
    })
})
```

You can find more details about this feature in [/docs/api/ctx.md](./api/ctx.md).

### Drop

In v3, we introduced support to silently terminate requests through `Drop`.

```go
func (c Ctx) Drop()
```

With this method, you can:

- Block certain requests without notifying the client to mitigate DDoS attacks
- Protect sensitive endpoints from unauthorized access without leaking errors.

:::caution
While this feature adds the ability to drop connections, it is still **highly recommended** to use additional
measures (such as **firewalls**, **proxies**, etc.) to further protect your server endpoints by blocking
malicious connections before the server establishes a connection.
:::

```go
app.Get("/", func(c fiber.Ctx) error {
    if c.IP() == "192.168.1.1" {
        return c.Drop()
    }

    return c.SendString("Hello World!")
})
```

You can find more details about this feature in [/docs/api/ctx.md](./api/ctx.md).

### End

In v3, we introduced a new method to match the Express.js API's `res.end()` method.

```go
func (c Ctx) End()
```

With this method, you can:

- Stop middleware from controlling the connection after a handler further up the method chain
  by immediately flushing the current response and closing the connection.
- Use `return c.End()` as an alternative to `return nil`

```go
app.Use(func (c fiber.Ctx) error {
    err := c.Next()
    if err != nil {
        log.Println("Got error: %v", err)
        return c.SendString(err.Error()) // Will be unsuccessful since the response ended below
    }
    return nil
})

app.Get("/hello", func (c fiber.Ctx) error {
    query := c.Query("name", "")
    if query == "" {
        c.SendString("You don't have a name?")
        c.End() // Closes the underlying connection
        return errors.New("No name provided")
    }
    return c.SendString("Hello, " + query + "!")
})
```

---

## 📎 Binding

Fiber v3 introduces a new binding mechanism that simplifies the process of binding request data to structs. The new binding system supports binding from various sources such as URL parameters, query parameters, headers, and request bodies. This unified approach makes it easier to handle different types of request data in a consistent manner.

### New Features

- Unified binding from URL parameters, query parameters, headers, and request bodies.
- Support for custom binders and constraints.
- Improved error handling and validation.
- Support multipart file binding for `*multipart.FileHeader`, `*[]*multipart.FileHeader`, and `[]*multipart.FileHeader` field types.
- Support for unified binding (`Bind().All()`) with defined precedence order: (URI -> Body -> Query -> Headers -> Cookies). [Learn more](./api/bind.md#all).
- Support MsgPack binding for request body.

<details>
<summary>Example</summary>

```go
type User struct {
    ID    int    `params:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

app.Post("/user/:id", func(c fiber.Ctx) error {
    var user User
    if err := c.Bind().Body(&user); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(user)
})
```

In this example, the `Bind` method is used to bind the request body to the `User` struct. The `Body` method of the `Bind` class performs the actual binding.

</details>

## 🔬 Extractors Package

Fiber v3 introduces a new shared `extractors` package that consolidates value extraction utilities previously duplicated across middleware packages. This package provides a unified API for extracting values from headers, cookies, query parameters, form data, and URL parameters with built-in chain/fallback logic and security considerations.

### Key Features

- **Unified API**: Single package for extracting values from headers, cookies, query parameters, form data, and URL parameters
- **Chain Logic**: Built-in fallback mechanism to try multiple extraction sources in order
- **Source Awareness**: Source inspection capabilities for security-sensitive operations
- **Type Safety**: Strongly typed extraction with proper error handling
- **Performance**: Optimized extraction functions with minimal overhead

### Available Extractors

- `FromAuthHeader(authScheme string)`: Extract from Authorization header with scheme support
- `FromCookie(key string)`: Extract from HTTP cookies
- `FromParam(param string)`: Extract from URL path parameters
- `FromForm(param string)`: Extract from form data
- `FromHeader(header string)`: Extract from custom HTTP headers
- `FromQuery(param string)`: Extract from URL query parameters
- `FromCustom(key string, extractor func(c fiber.Ctx) (string, error))`: Define custom extraction logic with metadata
- `Chain(extractors ...Extractor)`: Chain multiple extractors with fallback logic

### Usage Example

```go
import "github.com/gofiber/fiber/v3/extractors"

// Extract API key from multiple sources with fallback
apiKeyExtractor := extractors.Chain(
    extractors.FromHeader("X-API-Key"),
    extractors.FromQuery("api_key"),
    extractors.FromCookie("api_key"),
)

app.Use(func(c fiber.Ctx) error {
    apiKey, err := apiKeyExtractor.Extract(c)
    if err != nil {
        return c.Status(401).SendString("API key required")
    }
    // Use apiKey for authentication
    return c.Next()
})
```

### Migration from Middleware-Specific Extractors

Middleware packages in Fiber v3 now use the shared extractors package instead of maintaining their own extraction logic. This provides:

- **Code Deduplication**: Eliminates ~500+ lines of duplicated extraction code
- **Consistency**: Standardized extraction behavior across all middleware
- **Maintainability**: Single source of truth for extraction logic
- **Security**: Unified security considerations and warnings

## 🔄 Redirect

Fiber v3 enhances the redirect functionality by introducing new methods and improving existing ones. The new redirect methods provide more flexibility and control over the redirection process.

### New Methods

- `Redirect().To()`: Redirects to a specific URL.
- `Redirect().Route()`: Redirects to a named route.
- `Redirect().Back()`: Redirects to the previous URL.

<details>
<summary>Example</summary>

```go
app.Get("/old", func(c fiber.Ctx) error {
    return c.Redirect().To("/new")
})

app.Get("/new", func(c fiber.Ctx) error {
    return c.SendString("Welcome to the new route!")
})
```

</details>

### Changed behavior

:::info

The default redirect status code has been updated from `302 Found` to `303 See Other` to ensure more consistent behavior across different browsers.

:::

## 🌎 Client package

The Gofiber client has been completely rebuilt. It includes numerous new features such as Cookiejar, request/response hooks, and more.
You can take a look to [client docs](./client/rest.md) to see what's new with the client.

### Configuration improvements

The v3 client centralizes common configuration on the client instance and lets you override it per request with `client.Config`.
You can define base URLs, defaults (headers, cookies, path parameters, timeouts), and toggle path normalization once, while still
using axios-style helpers for each call.

```go
cc := client.New().
    SetBaseURL("https://api.service.local").
    AddHeader("Authorization", "Bearer <token>").
    SetTimeout(5 * time.Second).
    SetPathParam("tenant", "acme")

resp, err := cc.Get("/users/:tenant/:id", client.Config{
    PathParam:              map[string]string{"id": "42"},
    Param:                  map[string]string{"include": "profile"},
    DisablePathNormalizing: true,
})
if err != nil {
    panic(err)
}
defer resp.Close()
fmt.Println(resp.StatusCode(), resp.String())
```

### Fasthttp transport integration

- `client.NewWithHostClient` and `client.NewWithLBClient` allow you to plug existing `fasthttp` clients directly into Fiber while keeping retries, redirects, and hook logic consistent.
- Dialer, TLS, and proxy helpers now update every host client inside a load balancer, so complex pools inherit the same configuration.
- The Fiber client exposes `Do`, `DoTimeout`, `DoDeadline`, and `CloseIdleConnections`, matching the surface area of the wrapped fasthttp transports.

## 🧰 Generic functions

Fiber v3 introduces new generic functions that provide additional utility and flexibility for developers. These functions are designed to simplify common tasks and improve code readability.

### New Generic Functions

- **Convert**: Converts a value with a specified converter function and default value.
- **Locals**: Retrieves or sets local values within a request context.
- **Params**: Retrieves route parameters and can handle various types of route parameters.
- **Query**: Retrieves the value of a query parameter from the request URI and can handle various types of query parameters.
- **GetReqHeader**: Returns the HTTP request header specified by the field and can handle various types of header values.

### Example

<details>
<summary>Convert</summary>

```go
package main

import (
    "strconv"
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/convert", func(c fiber.Ctx) error {
        value, err := fiber.Convert[int](c.Query("value"), strconv.Atoi, 0)
        if err != nil {
            return c.Status(fiber.StatusBadRequest).SendString(err.Error())
        }
        return c.JSON(value)
    })

    app.Listen(":3000")
}
```

```sh
curl "http://localhost:3000/convert?value=123"
# Output: 123

curl "http://localhost:3000/convert?value=abc"
# Output: "failed to convert: strconv.Atoi: parsing \"abc\": invalid syntax"
```

</details>

<details>
<summary>Locals</summary>

```go
package main

import (
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Use("/user/:id", func(c fiber.Ctx) error {
        // ask database for user
        // ...
        // set local values from database
        fiber.Locals[string](c, "user", "john")
        fiber.Locals[int](c, "age", 25)
        // ...

        return c.Next()
    })

    app.Get("/user/*", func(c fiber.Ctx) error {
        // get local values
        name := fiber.Locals[string](c, "user")
        age := fiber.Locals[int](c, "age")
        // ...
        return c.JSON(fiber.Map{"name": name, "age": age})
    })

    app.Listen(":3000")
}
```

```sh
curl "http://localhost:3000/user/5"
# Output: {"name":"john","age":25}
```

</details>

<details>
<summary>Params</summary>

```go
package main

import (
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/params/:id", func(c fiber.Ctx) error {
        id := fiber.Params[int](c, "id", 0)
        return c.JSON(id)
    })

    app.Listen(":3000")
}
```

```sh
curl "http://localhost:3000/params/123"
# Output: 123

curl "http://localhost:3000/params/abc"
# Output: 0
```

</details>

<details>
<summary>Query</summary>

```go
package main

import (
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/query", func(c fiber.Ctx) error {
        age := fiber.Query[int](c, "age", 0)
        return c.JSON(age)
    })

    app.Listen(":3000")
}

```

```sh
curl "http://localhost:3000/query?age=25"
# Output: 25

curl "http://localhost:3000/query?age=abc"
# Output: 0
```

</details>

<details>
<summary>GetReqHeader</summary>

```go
package main

import (
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/header", func(c fiber.Ctx) error {
        userAgent := fiber.GetReqHeader[string](c, "User-Agent", "Unknown")
        return c.JSON(userAgent)
    })

    app.Listen(":3000")
}
```

```sh
curl -H "User-Agent: CustomAgent" "http://localhost:3000/header"
# Output: "CustomAgent"

curl "http://localhost:3000/header"
# Output: "Unknown"
```

</details>

## 🛠️ Utils {#utils}

Fiber v3 removes the built-in `utils` directory and now imports utility helpers from the separate [`github.com/gofiber/utils/v2`](https://github.com/gofiber/utils) module. See the [migration guide](#utils-migration) for detailed replacement steps and examples.

The `github.com/gofiber/utils` module also introduces new helpers like `ParseInt`, `ParseUint`, `Walk`, `ReadFile`, and `Timestamp`.

## 🧩 Services

Fiber v3 introduces a new feature called Services. This feature allows developers to quickly start services that the application depends on, removing the need to manually provision things like database servers, caches, or message brokers, to name a few.

### Example

<details>
<summary>Adding a service</summary>

```go
package main

import (
    "strconv"
    "github.com/gofiber/fiber/v3"
)

type myService struct {
    img string
    // ...
}

// Start initializes and starts the service. It implements the [fiber.Service] interface.
func (s *myService) Start(ctx context.Context) error {
    // start the service
    return nil
}

// String returns a string representation of the service.
// It is used to print a human-readable name of the service in the startup message.
// It implements the [fiber.Service] interface.
func (s *myService) String() string {
    return s.img
}

// State returns the current state of the service.
// It implements the [fiber.Service] interface.
func (s *myService) State(ctx context.Context) (string, error) {
    return "running", nil
}

// Terminate stops and removes the service. It implements the [fiber.Service] interface.
func (s *myService) Terminate(ctx context.Context) error {
    // stop the service
    return nil
}

func main() {
    cfg := &fiber.Config{}

    cfg.Services = append(cfg.Services, &myService{img: "postgres:latest"})
    cfg.Services = append(cfg.Services, &myService{img: "redis:latest"})

    app := fiber.New(*cfg)

    // ...
}
```

</details>

<details>
<summary>Output</summary>

```sh
$ go run . -v

    _______ __
   / ____(_) /_  ___  _____
  / /_  / / __ \/ _ \/ ___/
 / __/ / / /_/ /  __/ /
/_/   /_/_.___/\___/_/          v3.0.0
--------------------------------------------------
INFO Server started on:         http://127.0.0.1:3000 (bound on host 0.0.0.0 and port 3000)
INFO Services:     2
INFO   🧩 [ RUNNING ] postgres:latest
INFO   🧩 [ RUNNING ] redis:latest
INFO Total handlers count:      2
INFO Prefork:                   Disabled
INFO PID:                       12279
INFO Total process count:       1
```

</details>

## 📃 Log

`fiber.AllLogger[T]` interface now has a new generic type parameter `T` and a method called `Logger`. This method can be used to get the underlying logger instance from the Fiber logger middleware. This is useful when you want to configure the logger middleware with a custom logger and still want to access the underlying logger instance with the appropriate type.

You can find more details about this feature in [/docs/api/log.md](./api/log.md#logger).

`logger.Config` now supports a new field called `ForceColors`. This field allows you to force the logger to always use colors, even if the output is not a terminal. This is useful when you want to ensure that the logs are always colored, regardless of the output destination.

```go
package main

import "github.com/gofiber/fiber/v3/middleware/logger"

app.Use(logger.New(logger.Config{
    ForceColors: true,
}))
```

## 📦 Storage Interface

The storage interface has been updated to include new subset of methods with `WithContext` suffix. These methods allow you to pass a context to the storage operations, enabling better control over timeouts and cancellation if needed. This is particularly useful when storage implementations used outside of the Fiber core, such as in background jobs or long-running tasks.

**New Methods Signatures:**

```go
// GetWithContext gets the value for the given key with a context.
// `nil, nil` is returned when the key does not exist
GetWithContext(ctx context.Context, key string) ([]byte, error)

// SetWithContext stores the given value for the given key
// with an expiration value, 0 means no expiration.
// Empty key or value will be ignored without an error.
SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error

// DeleteWithContext deletes the value for the given key with a context.
// It returns no error if the storage does not contain the key,
DeleteWithContext(ctx context.Context, key string) error

// ResetWithContext resets the storage and deletes all keys with a context.
ResetWithContext(ctx context.Context) error
```

## 🧬 Middlewares

### Important Change for Accessing Middleware Data

In Fiber v3, many middlewares that previously set values in `c.Locals()` using string keys (e.g., `c.Locals("requestid")`) have been updated. To align with Go's context best practices and prevent key collisions, these middlewares now store their specific data in the request's context using unexported keys of custom types.

This means that directly accessing these values via `c.Locals("some_string_key")` will no longer work for such middleware-provided data.

**How to Access Middleware Data in v3:**

Each affected middleware now provides dedicated exported functions to retrieve its specific data from the context. You should use these functions instead of relying on string-based lookups in `c.Locals()`.

Examples include:

- `requestid.FromContext(c)`
- `csrf.TokenFromContext(c)`
- `csrf.HandlerFromContext(c)`
- `session.FromContext(c)`
- `basicauth.UsernameFromContext(c)`
- `keyauth.TokenFromContext(c)`

When used with the Logger middleware, the recommended approach is to use the `CustomTags` feature of the logger, which allows you to call these specific `FromContext` functions. See the [Logger](#logger) section for more details.

### Adaptor

The adaptor middleware has been significantly optimized for performance and efficiency. Key improvements include reduced response times, lower memory usage, and fewer memory allocations. These changes make the middleware more reliable and capable of handling higher loads effectively. Enhancements include the introduction of a `sync.Pool` for managing `fasthttp.RequestCtx` instances and better HTTP request and response handling between net/http and fasthttp contexts.

Incoming body sizes now respect the Fiber app's configured `BodyLimit` (falling back to the default when unset) when running Fiber from `net/http` through the adaptor, returning `413 Request Entity Too Large` for oversized payloads.

| Payload Size | Metric         | V2           | V3          | Percent Change |
| ------------ | -------------- | ------------ | ----------- | -------------- |
| 100KB        | Execution Time | 1056 ns/op   | 588.6 ns/op | -44.25%        |
|              | Memory Usage   | 2644 B/op    | 254 B/op    | -90.39%        |
|              | Allocations    | 16 allocs/op | 5 allocs/op | -68.75%        |
| 500KB        | Execution Time | 1061 ns/op   | 562.9 ns/op | -46.94%        |
|              | Memory Usage   | 2644 B/op    | 248 B/op    | -90.62%        |
|              | Allocations    | 16 allocs/op | 5 allocs/op | -68.75%        |
| 1MB          | Execution Time | 1080 ns/op   | 629.7 ns/op | -41.68%        |
|              | Memory Usage   | 2646 B/op    | 267 B/op    | -89.91%        |
|              | Allocations    | 16 allocs/op | 5 allocs/op | -68.75%        |
| 5MB          | Execution Time | 1093 ns/op   | 540.3 ns/op | -50.58%        |
|              | Memory Usage   | 2654 B/op    | 254 B/op    | -90.43%        |
|              | Allocations    | 16 allocs/op | 5 allocs/op | -68.75%        |
| 10MB         | Execution Time | 1044 ns/op   | 533.1 ns/op | -48.94%        |
|              | Memory Usage   | 2665 B/op    | 258 B/op    | -90.32%        |
|              | Allocations    | 16 allocs/op | 5 allocs/op | -68.75%        |
| 25MB         | Execution Time | 1069 ns/op   | 540.7 ns/op | -49.42%        |
|              | Memory Usage   | 2706 B/op    | 289 B/op    | -89.32%        |
|              | Allocations    | 16 allocs/op | 5 allocs/op | -68.75%        |
| 50MB         | Execution Time | 1137 ns/op   | 554.6 ns/op | -51.21%        |
|              | Memory Usage   | 2734 B/op    | 298 B/op    | -89.10%        |
|              | Allocations    | 16 allocs/op | 5 allocs/op | -68.75%        |

### BasicAuth

The BasicAuth middleware now validates the `Authorization` header more rigorously and sets security-focused response headers. Passwords must be provided in **hashed** form (e.g. SHA-256 or bcrypt) rather than plaintext. The default challenge includes the `charset="UTF-8"` parameter and disables caching. Responses also set a `Vary: Authorization` header to prevent caching based on credentials. Passwords are no longer stored in the request context. A `Charset` option controls the value used in the challenge header.
A new `HeaderLimit` option restricts the maximum length of the `Authorization` header (default: `8192` bytes).
The `Authorizer` function now receives the current `fiber.Ctx` as a third argument, allowing credential checks to incorporate request context.

### Cache

We are excited to introduce a new option in our caching middleware: Cache Invalidator. This feature provides greater control over cache management, allowing you to define custom conditions for invalidating cache entries.

The middleware now emits `Cache-Control` headers by default via the new `DisableCacheControl` flag, increases the default `Expiration` from `1 minute` to `5 minutes`, and applies a new `MaxBytes` limit of `1 MB` (previously unlimited).

Additionally, the caching middleware has been optimized to avoid caching non-cacheable status codes, as defined by the [HTTP standards](https://datatracker.ietf.org/doc/html/rfc7231#section-6.1). This improvement enhances cache accuracy and reduces unnecessary cache storage usage.
Cached responses now include an RFC-compliant Age header, providing a standardized indication of how long a response has been stored in cache since it was originally generated. This enhancement improves HTTP compliance and facilitates better client-side caching strategies.

Cache keys are now redacted in logs and error messages by default, and a `DisableValueRedaction` boolean (default `false`) lets you opt out when you need the raw value for troubleshooting.

:::note
The deprecated `Store` and `Key` options have been removed in v3. Use `Storage` and `KeyGenerator` instead.
:::

### ResponseTime

A new response time middleware measures how long each request takes to process and adds the duration to the response headers.
By default it writes the elapsed time to `X-Response-Time`, and you can change the header name. A `Next` hook lets you skip
endpoints such as health checks.

### CORS

We've made some changes to the CORS middleware to improve its functionality and flexibility. Here's what's new:

#### New Struct Fields

- `Config.AllowPrivateNetwork`: This new field is a boolean that allows you to control whether private networks are allowed. This is related to the [Private Network Access (PNA)](https://wicg.github.io/private-network-access/) specification from the [Web Incubator Community Group (WICG)](https://wicg.io/). When set to `true`, the CORS middleware will allow CORS preflight requests from private networks and respond with the `Access-Control-Allow-Private-Network: true` header. This could be useful in development environments or specific use cases, but should be done with caution due to potential security risks.

#### Updated Struct Fields

We've updated several fields from a single string (containing comma-separated values) to slices, allowing for more explicit declaration of multiple values. Here are the updated fields:

- `Config.AllowOrigins`: Now accepts a slice of strings, each representing an allowed origin.
- `Config.AllowMethods`: Now accepts a slice of strings, each representing an allowed method.
- `Config.AllowHeaders`: Now accepts a slice of strings, each representing an allowed header.
- `Config.ExposeHeaders`: Now accepts a slice of strings, each representing an exposed header.

Additionally, panic messages and logs redact misconfigured origins by default, and a `DisableValueRedaction` flag (default `false`) lets you reveal them when necessary.

### Compression

- Added support for `zstd` compression alongside `gzip`, `deflate`, and `brotli`.
- Strong `ETag` values are now recomputed for compressed payloads so validators remain accurate.
- Compression is bypassed for responses that already specify `Content-Encoding`, for range requests or `206` statuses, and when either side sends `Cache-Control: no-transform`.
- `HEAD` requests still negotiate compression so `Content-Encoding`, `Content-Length`, `ETag`, and `Vary` match a corresponding `GET`, but the body is omitted.
- `Vary: Accept-Encoding` is merged into responses even when compression is skipped, preventing caches from mixing encoded and unencoded variants.

### CSRF

The `Expiration` field in the CSRF middleware configuration has been renamed to `IdleTimeout` to better describe its functionality. Additionally, the default value has been reduced from 1 hour to 30 minutes.

CSRF now redacts tokens and storage keys by default and exposes a `DisableValueRedaction` toggle (default `false`) if you must surface those values in diagnostics.

The CSRF middleware now validates the [`Sec-Fetch-Site`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Sec-Fetch-Site) header for unsafe HTTP methods. When present, requests with invalid `Sec-Fetch-Site` values (not one of "same-origin", "none", "same-site", or "cross-site") are rejected with `ErrFetchSiteInvalid`. Valid or absent headers proceed to standard origin and token validation checks, providing an early gate to catch malformed requests while maintaining compatibility with legitimate cross-site traffic.

### Idempotency

Idempotency middleware now redacts keys by default and offers a `DisableValueRedaction` configuration flag (default `false`) to expose them when debugging.

### EncryptCookie

- Added support for specifying key length when using `encryptcookie.GenerateKey(length)`. Keys must be base64-encoded and may be 16, 24, or 32 bytes when decoded, supporting AES-128, AES-192, and AES-256 (default).
- Custom encryptor and decryptor callbacks now receive the cookie name. The default AES-GCM helpers bind it as additional authenticated data (AAD) so ciphertext cannot be replayed under a different cookie.
- **Breaking change:** Custom encryptor/decryptor hooks now accept the cookie name as their first argument. Update overrides like:

  ```go
  // Before
  Encryptor func(value, key string) (string, error)
  Decryptor func(value, key string) (string, error)

  // After
  Encryptor func(name, value, key string) (string, error)
  Decryptor func(name, value, key string) (string, error)
  ```

### EnvVar

The `ExcludeVars` field has been removed from the EnvVar middleware configuration. When upgrading, remove any references to this field and explicitly list the variables you wish to expose using `ExportVars`.

### Filesystem

We've decided to remove filesystem middleware to clear up the confusion between static and filesystem middleware.
Now, static middleware can do everything that filesystem middleware and static do. You can check out [static middleware](./middleware/static.md) or [migration guide](#-migration-guide) to see what has been changed.

### Healthcheck

The healthcheck middleware has been simplified into a single generic probe handler. No endpoints are registered automatically. Register the middleware on each route you need—using helpers like `healthcheck.LivenessEndpoint`, `healthcheck.ReadinessEndpoint`, or `healthcheck.StartupEndpoint`—and optionally supply a `Probe` function to determine the service's health. This approach lets you expose any number of health check routes.

Refer to the [healthcheck middleware migration guide](./middleware/healthcheck.md) or the [general migration guide](#-migration-guide) to review the changes.

### KeyAuth

The keyauth middleware was updated to introduce a configurable `Realm` field for the `WWW-Authenticate` header.
The old string-based `KeyLookup` configuration has been replaced with an `Extractor` field. Use helper functions like `keyauth.FromHeader`, `keyauth.FromAuthHeader`, or `keyauth.FromCookie` to define where the key should be retrieved from. Multiple sources can be combined with `keyauth.Chain`. See the migration guide below.
New `Challenge`, `Error`, `ErrorDescription`, `ErrorURI`, and `Scope` fields allow customizing the `WWW-Authenticate` header, returning Bearer error details, and specifying required scopes. `ErrorURI` values are validated as absolute, a default `ApiKey` challenge is emitted when using non-Authorization extractors, Bearer `error` values are validated, credentials must conform to RFC 7235 `token68` syntax, and `scope` values are checked against RFC 6750's `scope-token` format. The header is also emitted only after the status code is finalized.

### Logger

New helper function called `LoggerToWriter` has been added to the logger middleware. This function allows you to use 3rd party loggers such as `logrus` or `zap` with the Fiber logger middleware without any extra afford. For example, you can use `zap` with Fiber logger middleware like this:

Custom logger integrations should update any `LoggerFunc` implementations to the new signature that receives a pointer to the middleware config: `func(c fiber.Ctx, data *logger.Data, cfg *logger.Config) error`.

<details>
<summary>Example</summary>

```go
package main

import (
    "github.com/gofiber/contrib/fiberzap/v2"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/log"
    "github.com/gofiber/fiber/v3/middleware/logger"
)

func main() {
    // Create a new Fiber instance
    app := fiber.New()

    // Create a new zap logger which is compatible with Fiber AllLogger interface
    zap := fiberzap.NewLogger(fiberzap.LoggerConfig{
        ExtraKeys: []string{"request_id"},
    })

    // Use the logger middleware with zerolog logger
    app.Use(logger.New(logger.Config{
        Output: logger.LoggerToWriter(zap, log.LevelDebug),
    }))

    // Define a route
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    // Start server on http://localhost:3000
    app.Listen(":3000")
}
```

</details>

:::note
The deprecated `TagHeader` constant was removed. Use `TagReqHeader` when you need to log request headers.
:::

#### Logging Middleware Values (e.g., Request ID)

In Fiber v3, middleware (like `requestid`) now stores values in the request context using unexported keys of custom types. This aligns with Go's context best practices to prevent key collisions between packages.

As a result, directly accessing these values using string keys with `c.Locals("your_key")` or in the logger format string with `${locals:your_key}` (e.g., `${locals:requestid}`) will no longer work for values set by such middleware.

**Recommended Solution: `CustomTags`**

The cleanest and most maintainable way to include these middleware-specific values in your logs is by using the `CustomTags` option in the logger middleware configuration. This allows you to define a custom function to retrieve the value correctly from the context.

<details>
<summary>Example: Logging Request ID with CustomTags</summary>

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
    "github.com/gofiber/fiber/v3/middleware/requestid"
)

func main() {
    app := fiber.New()

    // Ensure requestid middleware is used before the logger
    app.Use(requestid.New())

    app.Use(logger.New(logger.Config{
        CustomTags: map[string]logger.LogFunc{
            "requestid": func(output logger.Buffer, c fiber.Ctx, data *logger.Data, extraParam string) (int, error) {
                // Retrieve the request ID using the middleware's specific function
                return output.WriteString(requestid.FromContext(c))
            },
        },
        // Use the custom tag in your format string
        Format: "[${time}] ${ip} - ${requestid} - ${status} ${method} ${path}\n",
    }))

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    app.Listen(":3000")
}
```

</details>

**Alternative: Manually Copying to `Locals`**

If you have existing logging patterns that rely on `c.Locals` or prefer to manage these values in `Locals` for other reasons, you can manually copy the value from the context to `c.Locals` in a preceding middleware:

<details>
<summary>Example: Manually setting requestid in Locals</summary>

```go
app.Use(requestid.New()) // Request ID middleware
app.Use(func(c fiber.Ctx) error {
    // Manually copy the request ID to Locals
    c.Locals("requestid", requestid.FromContext(c))
    return c.Next()
})
app.Use(logger.New(logger.Config{
    // Now ${locals:requestid} can be used, but CustomTags is generally preferred
    Format: "[${time}] ${ip} - ${locals:requestid} - ${status} ${method} ${path}\n",
}))
```

</details>

Both approaches ensure your logger can access these values while respecting Go's context practices.

The `Skip` is a function to determine if logging is skipped or written to `Stream`.

<details>
<summary>Example Usage</summary>

```go
app.Use(logger.New(logger.Config{
    Skip: func(c fiber.Ctx) bool {
        // Skip logging HTTP 200 requests
        return c.Response().StatusCode() == fiber.StatusOK
    },
}))
```

```go
app.Use(logger.New(logger.Config{
    Skip: func(c fiber.Ctx) bool {
        // Only log errors, similar to an error.log
        return c.Response().StatusCode() < 400
    },
}))
```

</details>

#### Predefined Formats

Logger provides predefined formats that you can use by name or directly by specifying the format string.
<details>

<summary>Example Usage</summary>

```go
app.Use(logger.New(logger.Config{
    Format: logger.FormatCombined,
}))
```

See more in [Logger](./middleware/logger.md#predefined-formats)
</details>

### Limiter

The limiter middleware uses a new Fixed Window Rate Limiter implementation.

Custom limiter algorithms should now implement the updated `limiter.Handler` interface, whose `New` method receives a pointer to the active config: `New(cfg *limiter.Config) fiber.Handler`.

Limiter now redacts request keys in error paths by default. A new `DisableValueRedaction` boolean (default `false`) lets you reveal the raw limiter key if diagnostics require it.

:::note
Deprecated fields `Duration`, `Store`, and `Key` have been removed in v3. Use `Expiration`, `Storage`, and `KeyGenerator` instead.
:::

### Monitor

Monitor middleware is migrated to the [Contrib package](https://github.com/gofiber/contrib/tree/main/monitor) with [PR #1172](https://github.com/gofiber/contrib/pull/1172).

### Proxy

The proxy middleware has been updated to improve consistency with Go naming conventions. The `TlsConfig` field in the configuration struct has been renamed to `TLSConfig`. Additionally, the `WithTlsConfig` method has been removed; you should now configure TLS directly via the `TLSConfig` property within the `Config` struct.

The new `KeepConnectionHeader` option (default `false`) drops the `Connection` header unless explicitly enabled to retain it.

`proxy.Balancer` now accepts an optional variadic configuration: call `proxy.Balancer()` to use defaults or continue passing a `proxy.Config` value as before.

### Session

The Session middleware has undergone key changes in v3 to improve functionality and flexibility. While v2 methods remain available for backward compatibility, we now recommend using the new middleware handler for session management.

#### Key Updates

### Session

The session middleware has undergone significant improvements in v3, focusing on type safety, flexibility, and better developer experience.

#### Key Changes

- **Extractor Pattern**: The string-based `KeyLookup` configuration has been replaced with a more flexible and type-safe `Extractor` function pattern.

- **New Middleware Handler**: The `New` function now returns a middleware handler instead of a `*Store`. To access the session store, use the `Store` method on the middleware, or opt for `NewStore` or `NewWithStore` for custom store integration.

- **Manual Session Release**: Session instances are no longer automatically released after being saved. To ensure proper lifecycle management, you must manually call `sess.Release()`.

- **Idle Timeout**: The `Expiration` field has been replaced with `IdleTimeout`, which handles session inactivity. If the session is idle for the specified duration, it will expire. The idle timeout is updated when the session is saved. If you are using the middleware handler, the idle timeout will be updated automatically.

- **Absolute Timeout**: The `AbsoluteTimeout` field has been added. If you need to set an absolute session timeout, you can use this field to define the duration. The session will expire after the specified duration, regardless of activity.

- **Default KeyGenerator**: Changed from `utils.UUIDv4` to `utils.SecureToken`, producing base64-encoded tokens instead of UUID format.

For more details on these changes and migration instructions, check the [Session Middleware Migration Guide](./middleware/session.md#migration-guide).

### Timeout

The timeout middleware is now configurable. A new `Config` struct allows customizing the timeout duration, defining a handler that runs when a timeout occurs, and specifying errors to treat as timeouts. The `New` function now accepts a `Config` value instead of a duration.

**Behavioral changes:**

- **Immediate response on timeout**: The middleware now returns immediately when the timeout expires, even if the handler is still running. Previously, it waited for the handler to complete before returning the timeout error.
- **Context propagation**: The timeout context is properly propagated to the handler. Handlers can detect timeouts by listening on `c.Context().Done()`.
- **Panic handling**: Panics in the handler are caught and converted to `500 Internal Server Error` responses.

**Migration:** Replace calls like `timeout.New(handler, 2*time.Second)` with `timeout.New(handler, timeout.Config{Timeout: 2 * time.Second})`.

## 🔌 Addons

In v3, Fiber introduced Addons. Addons are additional useful packages that can be used in Fiber.

### Retry

The Retry addon is a new addon that implements a retry mechanism for unsuccessful network operations. It uses an exponential backoff algorithm with jitter.
It calls the function multiple times and tries to make it successful. If all calls are failed, then, it returns an error.
It adds a jitter at each retry step because adding a jitter is a way to break synchronization across the client and avoid collision.

<details>
<summary>Example</summary>

```go
package main

import (
    "fmt"

    "github.com/gofiber/fiber/v3/addon/retry"
    "github.com/gofiber/fiber/v3/client"
)

func main() {
    expBackoff := retry.NewExponentialBackoff(retry.Config{})

    // Local variables that will be used inside of Retry
    var resp *client.Response
    var err error

    // Retry a network request and return an error to signify to try again
    err = expBackoff.Retry(func() error {
        client := client.New()
        resp, err = client.Get("https://gofiber.io")
        if err != nil {
            return fmt.Errorf("GET gofiber.io failed: %w", err)
        }
        if resp.StatusCode() != 200 {
            return fmt.Errorf("GET gofiber.io did not return OK 200")
        }
        return nil
    })

    // If all retries failed, panic
    if err != nil {
        panic(err)
    }
    fmt.Printf("GET gofiber.io succeeded with status code %d\n", resp.StatusCode())
}
```

</details>

## 📋 Migration guide

To streamline upgrades between Fiber versions, the Fiber CLI ships with a
`migrate` command:

```bash
go install github.com/gofiber/cli/fiber@latest
fiber migrate --to v3.0.0-rc.3
```

### Options

- `-t, --to string` migrate to a specific version, e.g. `v3.0.0`
- `-f, --force` force migration even if already on that version
- `-s, --skip_go_mod` skip running `go mod tidy`, `go mod download`, and `go mod vendor`

### Changes Overview

- [🚀 App](#-app-1)
- [🎣 Hooks](#-hooks-1)
- [🚀 Listen](#-listen-1)
- [🗺 Router](#-router-1)
- [🧠 Context](#-context-1)
- [📎 Binding (was Parser)](#-parser)
- [🔄 Redirect](#-redirect-1)
- [🧾 Log](#-log-1)
- [🌎 Client package](#-client-package-1)
- [🛠️ Utils](#utils-migration)
- [🧬 Middlewares](#-middlewares-1)
  - [Important Change for Accessing Middleware Data](#important-change-for-accessing-middleware-data)
  - [BasicAuth](#basicauth-1)
  - [Cache](#cache-1)
  - [CORS](#cors-1)
  - [CSRF](#csrf-1)
  - [Filesystem](#filesystem-1)
  - [EnvVar](#envvar-1)
  - [Healthcheck](#healthcheck-1)
  - [Monitor](#monitor-1)
  - [Proxy](#proxy-1)
  - [Session](#session-1)

### 🚀 App

#### Static

Since we've removed `app.Static()`, you need to move methods to static middleware like the example below:

```go
// Before
app.Static("/", "./public")
app.Static("/prefix", "./public")
app.Static("/prefix", "./public", Static{
    Index: "index.htm",
})
app.Static("*", "./public/index.html")
```

```go
// After
app.Get("/*", static.New("./public"))
app.Get("/prefix*", static.New("./public"))
app.Get("/prefix*", static.New("./public", static.Config{
    IndexNames: []string{"index.htm", "index.html"},
}))
app.Get("*", static.New("./public/index.html"))
```

:::caution
You have to put `*` to the end of the route if you don't define static route with `app.Use`.
:::

#### Trusted Proxies

We've renamed `EnableTrustedProxyCheck` to `TrustProxy` and moved `TrustedProxies` to `TrustProxyConfig`.

```go
// Before
app := fiber.New(fiber.Config{
    // EnableTrustedProxyCheck enables the trusted proxy check.
    EnableTrustedProxyCheck: true,
    // TrustedProxies is a list of trusted proxy IP ranges/addresses.
    TrustedProxies: []string{"0.8.0.0", "127.0.0.0/8", "::1/128"},
})
```

```go
// After
app := fiber.New(fiber.Config{
    // TrustProxy enables the trusted proxy check
    TrustProxy: true,
    // TrustProxyConfig allows for configuring trusted proxies.
    TrustProxyConfig: fiber.TrustProxyConfig{
        // Proxies is a list of trusted proxy IP ranges/addresses.
        Proxies: []string{"0.8.0.0"},
        // Trust all loop-back IP addresses (127.0.0.0/8, ::1/128)
        Loopback: true,
    }
})
```

### 🎣 Hooks

`OnShutdown` has been replaced by two hooks: `OnPreShutdown` and `OnPostShutdown`.
Use them to run cleanup code before and after the server shuts down. When handling
shutdown errors, register an `OnPostShutdown` hook and call `app.Listen()` in a goroutine.

```go
// Before
app.OnShutdown(func() {
    // Code to run before shutdown
})
```

```go
// After
app.OnPreShutdown(func() {
    // Code to run before shutdown
})
```

### 🚀 Listen

The `Listen` helpers (`ListenTLS`, `ListenMutualTLS`, etc.) were removed. Use
`app.Listen()` with `fiber.ListenConfig` and a `tls.Config` when TLS is required.
Options such as `ListenerNetwork` and `UnixSocketFileMode` are now configured via
this struct.

```go
// Before
app.ListenTLS(":3000", "cert.pem", "key.pem")
```

```go
// After
app.Listen(":3000", fiber.ListenConfig{
    CertFile: "./cert.pem",
    CertKeyFile: "./cert.key",
})
```

### 🗺 Router

#### Direct `net/http` handlers

Route registration helpers now accept native `net/http` handlers. Pass an
`http.Handler`, `http.HandlerFunc`, or compatible function directly to methods
such as `app.Get`, `Group`, or `RouteChain` and Fiber will adapt it at
registration time. Manual wrapping through the adaptor middleware is no longer
required for these common cases.

:::note Compatibility considerations
Adapted handlers stick to `net/http` semantics. They do not interact with `fiber.Ctx`
and are slower than native Fiber handlers because of the extra conversion layer. Use
them to ease migrations, but prefer Fiber handlers in performance-critical paths.
:::

```go
httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if _, err := w.Write([]byte("served by net/http")); err != nil {
        panic(err)
    }
})

app.Get("/", httpHandler)
```

#### Middleware Registration

The signatures for [`Add`](#middleware-registration) and [`Route`](#route-chaining) have been changed.

To migrate [`Add`](#middleware-registration) you must change the `methods` in a slice.

```go
// Before
app.Add(fiber.MethodPost, "/api", myHandler)
```

```go
// After
app.Add([]string{fiber.MethodPost}, "/api", myHandler)
```

#### Mounting

In this release, the `Mount` method has been removed. Instead, you can use the `Use` method to achieve similar functionality.

```go
// Before
app.Mount("/api", apiApp)
```

```go
// After
app.Use("/api", apiApp)
```

#### Route Chaining

Refer to the [route chaining](#route-chaining) section for details on the new `RouteChain` helper. The `Route` function now matches its v2 behavior for prefix encapsulation.

```go
// Before
app.Route("/api", func(apiGrp Router) {
    apiGrp.Route("/user/:id?", func(userGrp Router) {
        userGrp.Get("/", func(c fiber.Ctx) error {
            // Get user
            return c.JSON(fiber.Map{"message": "Get user", "id": c.Params("id")})
        })
        userGrp.Post("/", func(c fiber.Ctx) error {
            // Create user
            return c.JSON(fiber.Map{"message": "User created"})
        })
    })
})
```

```go
// After
app.RouteChain("/api").RouteChain("/user/:id?")
    .Get(func(c fiber.Ctx) error {
        // Get user
        return c.JSON(fiber.Map{"message": "Get user", "id": c.Params("id")})
    })
    .Post(func(c fiber.Ctx) error {
        // Create user
        return c.JSON(fiber.Map{"message": "User created"})
    });
```

### 🗺 RebuildTree

We introduced a new method that enables rebuilding the route tree stack at runtime. This allows you to add routes dynamically while your application is running and update the route tree to make the new routes available for use.

For more details, refer to the [app documentation](./api/app.md#rebuildtree):

#### Example Usage

```go
app.Get("/define", func(c Ctx) error {  // Define a new route dynamically
    app.Get("/dynamically-defined", func(c Ctx) error {  // Adding a dynamically defined route
        return c.SendStatus(http.StatusOK)
    })

    app.RebuildTree()  // Rebuild the route tree to register the new route

    return c.SendStatus(http.StatusOK)
})
```

In this example, a new route is defined, and `RebuildTree()` is called to ensure the new route is registered and available.

Note: Use this method with caution. It is **not** thread-safe and can be very performance-intensive. Therefore, it should be used sparingly and primarily in development mode. It should not be invoke concurrently.

#### RemoveRoute

- **RemoveRoute**: Removes route by path

- **RemoveRouteByName**: Removes route by name

- **RemoveRouteFunc**: Removes route by a function having `*Route` parameter

For more details, refer to the [app documentation](./api/app.md#removeroute):

### 🧠 Context

Fiber v3 introduces several new features and changes to the Ctx interface, enhancing its functionality and flexibility.

- **ParamsInt**: Use `Params` with generic types.
- **QueryBool**: Use `Query` with generic types.
- **QueryFloat**: Use `Query` with generic types.
- **QueryInt**: Use `Query` with generic types.
- **Bind**: Now used for binding instead of view binding. Use `c.ViewBind()` for view binding.

In Fiber v3, the `Ctx` parameter in handlers is now an interface, which means the `*` symbol is no longer used. Here is an example demonstrating this change:

<details>
<summary>Example</summary>

**Before**:

```go
package main

import (
    "github.com/gofiber/fiber/v2"
)

func main() {
    app := fiber.New()

    // Route Handler with *fiber.Ctx
    app.Get("/", func(c *fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    app.Listen(":3000")
}
```

**After**:

```go
package main

import (
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Route Handler without *fiber.Ctx
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    app.Listen(":3000")
}
```

**Explanation**:

In this example, the `Ctx` parameter in the handler is used as an interface (`fiber.Ctx`) instead of a pointer (`*fiber.Ctx`). This change allows for more flexibility and customization in Fiber v3.

</details>

#### 📎 Parser

The `Parser` section in Fiber v3 has undergone significant changes to improve functionality and flexibility.

##### Migration Instructions

1. **BodyParser**: Use `c.Bind().Body()` instead of `c.BodyParser()`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    app.Post("/user", func(c *fiber.Ctx) error {
        var user User
        if err := c.BodyParser(&user); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(user)
    })
    ```

    ```go
    // After
    app.Post("/user", func(c fiber.Ctx) error {
        var user User
        if err := c.Bind().Body(&user); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(user)
    })
    ```

    </details>

2. **ParamsParser**: Use `c.Bind().URI()` instead of `c.ParamsParser()`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    app.Get("/user/:id", func(c *fiber.Ctx) error {
        var params Params
        if err := c.ParamsParser(&params); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(params)
    })
    ```

    ```go
    // After
    app.Get("/user/:id", func(c fiber.Ctx) error {
        var params Params
        if err := c.Bind().URI(&params); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(params)
    })
    ```

    </details>

3. **QueryParser**: Use `c.Bind().Query()` instead of `c.QueryParser()`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    app.Get("/search", func(c *fiber.Ctx) error {
        var query Query
        if err := c.QueryParser(&query); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(query)
    })
    ```

    ```go
    // After
    app.Get("/search", func(c fiber.Ctx) error {
        var query Query
        if err := c.Bind().Query(&query); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(query)
    })
    ```

    </details>

4. **CookieParser**: Use `c.Bind().Cookie()` instead of `c.CookieParser()`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    app.Get("/cookie", func(c *fiber.Ctx) error {
        var cookie Cookie
        if err := c.CookieParser(&cookie); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(cookie)
    })
    ```

    ```go
    // After
    app.Get("/cookie", func(c fiber.Ctx) error {
        var cookie Cookie
        if err := c.Bind().Cookie(&cookie); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(cookie)
    })
    ```

    </details>

#### 🔄 Redirect

Fiber v3 enhances the redirect functionality by introducing new methods and improving existing ones. The new redirect methods provide more flexibility and control over the redirection process.

##### Migration Instructions

1. **RedirectToRoute**: Use `c.Redirect().Route()` instead of `c.RedirectToRoute()`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    app.Get("/old", func(c *fiber.Ctx) error {
        return c.RedirectToRoute("newRoute")
    })
    ```

    ```go
    // After
    app.Get("/old", func(c fiber.Ctx) error {
        return c.Redirect().Route("newRoute")
    })
    ```

    </details>

2. **RedirectBack**: Use `c.Redirect().Back()` instead of `c.RedirectBack()`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    app.Get("/back", func(c *fiber.Ctx) error {
        return c.RedirectBack()
    })
    ```

    ```go
    // After
    app.Get("/back", func(c fiber.Ctx) error {
        return c.Redirect().Back()
    })
    ```

    </details>

3. **Redirect**: Use `c.Redirect().To()` instead of `c.Redirect()`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    app.Get("/old", func(c *fiber.Ctx) error {
        return c.Redirect("/new")
    })
    ```

    ```go
    // After
    app.Get("/old", func(c fiber.Ctx) error {
        return c.Redirect().To("/new")
    })
    ```

    </details>

#### 🧾 Log

The `ConfigurableLogger` and `AllLogger` interfaces now use generics. You can specify the underlying logger type when implementing these interfaces. While `any` can be used for maximum flexibility in some contexts, when retrieving the concrete logger via `log.DefaultLogger`, you must specify the exact underlying logger type, for example `log.DefaultLogger[*MyLogger]().Logger()`.

### 🌎 Client package

Fiber v3 introduces a completely rebuilt client package with numerous new features such as Cookiejar, request/response hooks, and more. Here is a guide to help you migrate from Fiber v2 to Fiber v3.

#### New Features

- **Cookiejar**: Manage cookies automatically.
- **Request/Response Hooks**: Customize request and response handling.
- **Improved Error Handling**: Better error management and reporting.

#### Migration Instructions

**Import Path**:

Update the import path to the new client package.

<details>
<summary>Before</summary>

```go
import "github.com/gofiber/fiber/v2/client"
```

</details>

<details>
<summary>After</summary>

```go
import "github.com/gofiber/fiber/v3/client"
```

</details>

**Common migrations**:

1. **Shared defaults instead of per-call mutation**: Move headers and timeouts into the reusable client and override with `client.Config` when needed.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    status, body, errs := fiber.Get("https://api.example.com/users").
        Set("Authorization", "Bearer "+token).
        Timeout(5 * time.Second).
        String()
    if len(errs) > 0 {
        return fmt.Errorf("request failed: %v", errs)
    }
    fmt.Println(status, body)
    ```

    ```go
    // After
    cli := client.New().
        AddHeader("Authorization", "Bearer "+token).
        SetTimeout(5 * time.Second)

    resp, err := cli.Get("https://api.example.com/users")
    if err != nil {
        return err
    }
    defer resp.Close()
    fmt.Println(resp.StatusCode(), resp.String())
    ```

    </details>

2. **Body handling**: Replace `Agent.JSON(...).Struct(&dst)` with request bodies through `client.Config` (or `Request.SetJSON`) and decode the response via `Response.JSON`.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    var created user
    status, _, errs := fiber.Post("https://api.example.com/users").
        JSON(payload).
        Struct(&created)
    if len(errs) > 0 {
        return fmt.Errorf("request failed: %v", errs)
    }
    fmt.Println(status, created)
    ```

    ```go
    // After
    cli := client.New()

    resp, err := cli.Post("https://api.example.com/users", client.Config{
        Body: payload,
    })
    if err != nil {
        return err
    }
    defer resp.Close()

    var created user
    if err := resp.JSON(&created); err != nil {
        return fmt.Errorf("decode failed: %w", err)
    }
    fmt.Println(resp.StatusCode(), created)
    ```

    </details>

3. **Path and query parameters**: Use the new path/query helpers instead of manually formatting URLs.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    code, body, errs := fiber.Get(fmt.Sprintf("https://api.example.com/users/%s", id)).
        QueryString("active=true").
        String()
    if len(errs) > 0 {
        return fmt.Errorf("request failed: %v", errs)
    }
    fmt.Println(code, body)
    ```

    ```go
    // After
    cli := client.New().SetBaseURL("https://api.example.com")
    resp, err := cli.Get("/users/:id", client.Config{
        PathParam: map[string]string{"id": id},
        Param:     map[string]string{"active": "true"},
    })
    if err != nil {
        return err
    }
    defer resp.Close()
    fmt.Println(resp.StatusCode(), resp.String())
    ```

    </details>

4. **Agent helpers**: `Agent.Bytes`, `AcquireAgent`, and `Agent.Parse` have been removed. Reuse a `client.Client` instance (or pool requests/responses directly) and access response data through the new typed helpers.

    <details>
    <summary>Example</summary>

    ```go
    // Before
    agent := fiber.AcquireAgent()
    status, body, errs := agent.Get("https://api.example.com/users").Bytes()
    fiber.ReleaseAgent(agent)
    if len(errs) > 0 {
        return fmt.Errorf("request failed: %v", errs)
    }

    var users []user
    if err := fiber.Parse(body, &users); err != nil {
        return fmt.Errorf("parse failed: %w", err)
    }
    fmt.Println(status, len(users))
    ```

    ```go
    // After
    cli := client.New()
    resp, err := cli.Get("https://api.example.com/users")
    if err != nil {
        return err
    }
    defer resp.Close()

    var users []user
    if err := resp.JSON(&users); err != nil {
        return fmt.Errorf("decode failed: %w", err)
    }
    fmt.Println(resp.StatusCode(), len(users))
    ```

    :::tip
    If you need pooling, use `client.AcquireRequest`, `client.AcquireResponse`, and their corresponding release functions around a long-lived `client.Client` instead of the removed agent pool.
    :::

    </details>

5. **Fiber-level shortcuts**: The `fiber.Get`, `fiber.Post`, and similar top-level helpers are no longer exposed from the main module. Use the client package equivalents (`client.Get`, `client.Post`, etc.) which call the shared default client (or pass your own client instance for custom defaults).

    <details>
    <summary>Example</summary>

    ```go
    // Before
    status, body, errs := fiber.Get("https://api.example.com/health").String()
    if len(errs) > 0 {
        return fmt.Errorf("request failed: %v", errs)
    }
    fmt.Println(status, body)
    ```

    ```go
    // After
    resp, err := client.Get("https://api.example.com/health")
    if err != nil {
        return err
    }
    defer resp.Close()

    fmt.Println(resp.StatusCode(), resp.String())
    ```

    :::note
    The `client.Get`/`client.Post` helpers use `client.C()` (the default shared client). For custom defaults, construct a client with `client.New()` and invoke its methods instead.
    :::

    </details>

#### Complete API Migration Reference

<details>
<summary>Click to expand full v2 → v3 API mapping tables</summary>

##### Core Concepts

| Description | v2 | v3 |
|-------------|----|----|
| Import | `github.com/gofiber/fiber/v2` | `github.com/gofiber/fiber/v3/client` |
| Client Concept | `*fiber.Agent` | `*client.Client` + `*client.Request` |
| Response Concept | `(code int, body []byte, errs []error)` | `(*client.Response, error)` |

##### Client/Agent Creation

| Description | v2 | v3 |
|-------------|----|----|
| Create Agent/Client | `fiber.AcquireAgent()` | `client.New()` |
| Get from pool | `fiber.AcquireAgent()` | `client.AcquireRequest()` |
| Release | `fiber.ReleaseAgent(a)` | `client.ReleaseRequest(req)` |
| With fasthttp.Client | - | `client.NewWithClient(c)` |
| With HostClient | - | `client.NewWithHostClient(hc)` |
| With LBClient | - | `client.NewWithLBClient(lb)` |
| Get Request object | `a.Request()` | `c.R()` |
| Default client | - | `client.C()` |
| Replace default | - | `client.Replace(c)` |

##### HTTP Methods

| Description | v2 | v3 (Client) | v3 (Request) |
|-------------|----|----|--------------|
| GET | `fiber.Get(url)` | `c.Get(url, cfg...)` | `req.Get(url)` |
| POST | `fiber.Post(url)` | `c.Post(url, cfg...)` | `req.Post(url)` |
| PUT | `fiber.Put(url)` | `c.Put(url, cfg...)` | `req.Put(url)` |
| PATCH | `fiber.Patch(url)` | `c.Patch(url, cfg...)` | `req.Patch(url)` |
| DELETE | `fiber.Delete(url)` | `c.Delete(url, cfg...)` | `req.Delete(url)` |
| HEAD | `fiber.Head(url)` | `c.Head(url, cfg...)` | `req.Head(url)` |
| OPTIONS | - | `c.Options(url, cfg...)` | `req.Options(url)` |
| Custom | - | `c.Custom(url, method, cfg...)` | `req.Custom(url, method)` |

##### URL & Method

| Description | v2 | v3 |
|-------------|----|----|
| Set URL | `req.SetRequestURI(url)` | `req.SetURL(url)` |
| Get URL | `req.URI().String()` | `req.URL()` |
| Set Method | `req.Header.SetMethod(method)` | `req.SetMethod(method)` |
| Set Base URL | - | `c.SetBaseURL(url)` |

##### Request Execution & Response

| Description | v2 | v3 |
|-------------|----|----|
| Parse Request | `a.Parse()` | Not needed |
| Execute (bytes) | `a.Bytes()` → `(code, body, errs)` | `req.Send()` → `(*Response, error)` |
| Execute (string) | `a.String()` | `resp.String()` |
| Execute (struct) | `a.Struct(&v)` | `resp.JSON(&v)` / `resp.XML(&v)` |
| Status Code | Return value `code` | `resp.StatusCode()` |
| Status Text | - | `resp.Status()` |
| Body (bytes) | Return value `body` | `resp.Body()` |
| Response Header | `resp.Header.Peek(key)` | `resp.Header(key)` |
| All Headers | `resp.Header.VisitAll(fn)` | `resp.Headers()` |
| Cookies | - | `resp.Cookies()` |
| Save to file | - | `resp.Save(path)` |
| Close | - | `resp.Close()` |

##### Headers

| Description | v2 | v3 (Client) | v3 (Request) |
|-------------|----|----|--------------|
| Set Header | `a.Set(k, v)` | `c.SetHeader(k, v)` | `req.SetHeader(k, v)` |
| Add Header | `a.Add(k, v)` | `c.AddHeader(k, v)` | `req.AddHeader(k, v)` |
| Multiple Headers | - | `c.SetHeaders(map)` | `req.SetHeaders(map)` |
| Bytes variants | `a.SetBytesK/V/KV()` | - | - |

##### User-Agent, Referer, Content-Type, Host

| Description | v2 | v3 (Client) | v3 (Request) |
|-------------|----|----|--------------|
| User-Agent | `a.UserAgent(ua)` | `c.SetUserAgent(ua)` | `req.SetUserAgent(ua)` |
| Referer | `a.Referer(ref)` | `c.SetReferer(ref)` | `req.SetReferer(ref)` |
| Content-Type | `a.ContentType(ct)` | - | `req.SetHeader("Content-Type", ct)` |
| Host | `a.Host(host)` | - | `req.SetHeader("Host", host)` |
| Connection Close | `a.ConnectionClose()` | - | `req.SetHeader("Connection", "close")` |

##### Cookies

| Description | v2 | v3 (Client) | v3 (Request) |
|-------------|----|----|--------------|
| Set Cookie | `a.Cookie(k, v)` | `c.SetCookie(k, v)` | `req.SetCookie(k, v)` |
| Multiple | `a.Cookies(k1, v1, ...)` | `c.SetCookies(map)` | `req.SetCookies(map)` |
| With Struct | - | `c.SetCookiesWithStruct(v)` | `req.SetCookiesWithStruct(v)` |
| Cookie Jar | - | `c.SetCookieJar(jar)` | - |

##### Query Parameters

| Description | v2 | v3 (Client) | v3 (Request) |
|-------------|----|----|--------------|
| Query String | `a.QueryString(qs)` | - | - |
| Add Param | - | `c.AddParam(k, v)` | `req.AddParam(k, v)` |
| Set Param | - | `c.SetParam(k, v)` | `req.SetParam(k, v)` |
| With Struct | - | `c.SetParamsWithStruct(v)` | `req.SetParamsWithStruct(v)` |

##### Path Parameters (NEW)

| Description | v2 | v3 (Client) | v3 (Request) |
|-------------|----|----|--------------|
| Set Path Param | - | `c.SetPathParam(k, v)` | `req.SetPathParam(k, v)` |
| Multiple | - | `c.SetPathParams(map)` | `req.SetPathParams(map)` |
| With Struct | - | `c.SetPathParamsWithStruct(v)` | `req.SetPathParamsWithStruct(v)` |

##### Request Body

| Description | v2 | v3 |
|-------------|----|----|
| Body (bytes) | `a.Body(body)` | `req.SetRawBody(body)` |
| Body (string) | `a.BodyString(body)` | `req.SetRawBody([]byte(body))` |
| Body Stream | `a.BodyStream(r, size)` | - |
| JSON | `a.JSON(v)` | `req.SetJSON(v)` |
| XML | `a.XML(v)` | `req.SetXML(v)` |
| CBOR (NEW) | - | `req.SetCBOR(v)` |

##### Form Data

| Description | v2 | v3 |
|-------------|----|----|
| Create Args | `fiber.AcquireArgs()` | Direct on Request |
| Send Form | `a.Form(args)` | `req.SetFormData(k, v)` |
| Add Form Data | `args.Set(k, v)` | `req.AddFormData(k, v)` |
| With Map | - | `req.SetFormDataWithMap(map)` |
| With Struct | - | `req.SetFormDataWithStruct(v)` |

##### File Upload

| Description | v2 | v3 |
|-------------|----|----|
| Multipart Form | `a.MultipartForm(args)` | Automatic |
| Boundary | `a.Boundary(b)` | `req.SetBoundary(b)` |
| Send File | `a.SendFile(f, field...)` | `req.AddFile(path)` |
| Multiple Files | `a.SendFiles(...)` | `req.AddFiles(files...)` |
| With Reader | - | `req.AddFileWithReader(name, r)` |
| FileData | `a.FileData(files...)` | `req.AddFiles(files...)` |

##### Timeout & TLS

| Description | v2 | v3 (Client) | v3 (Request) |
|-------------|----|----|--------------|
| Timeout | `a.Timeout(d)` | `c.SetTimeout(d)` | `req.SetTimeout(d)` |
| Max Redirects | `a.MaxRedirectsCount(n)` | Via Config | `req.SetMaxRedirects(n)` |
| TLS Config | `a.TLSConfig(cfg)` | `c.SetTLSConfig(cfg)` | - |
| Skip Verify | `a.InsecureSkipVerify()` | Via `tls.Config` | - |
| Certificates | - | `c.SetCertificates(...)` | - |
| Root Cert | - | `c.SetRootCertificate(path)` | - |

##### JSON/XML Encoder

| Description | v2 | v3 |
|-------------|----|----|
| JSON Encoder | `a.JSONEncoder(fn)` | `c.SetJSONMarshal(fn)` |
| JSON Decoder | `a.JSONDecoder(fn)` | `c.SetJSONUnmarshal(fn)` |
| XML Encoder | - | `c.SetXMLMarshal(fn)` |
| XML Decoder | - | `c.SetXMLUnmarshal(fn)` |
| CBOR (NEW) | - | `c.SetCBORMarshal/Unmarshal(fn)` |

##### Authentication

| Description | v2 | v3 |
|-------------|----|----|
| Basic Auth | `a.BasicAuth(user, pass)` | Via Header (Base64) |

##### Debug & Retry

| Description | v2 | v3 |
|-------------|----|----|
| Debug | `a.Debug(w...)` | `c.Debug()` |
| Disable Debug | - | `c.DisableDebug()` |
| Logger | - | `c.SetLogger(logger)` |
| Retry | `a.RetryIf(fn)` | `c.SetRetryConfig(cfg)` |

##### Reuse & Reset

| Description | v2 | v3 |
|-------------|----|----|
| Reuse Agent | `a.Reuse()` | Use pool |
| Reset Client | - | `c.Reset()` |
| Dest Buffer | `a.Dest(dest)` | - |

##### NEW in v3

| Feature | v3 API |
|---------|--------|
| Request Hooks | `c.AddRequestHook(fn)` |
| Response Hooks | `c.AddResponseHook(fn)` |
| Proxy | `c.SetProxyURL(url)` |
| Context | `req.SetContext(ctx)` |
| Dial Function | `c.SetDial(fn)` |
| Raw Request | `req.RawRequest` |
| Raw Response | `resp.RawResponse` |

##### Key Differences

1. **Architecture**: v2 `Agent` → v3 separate `Client`, `Request`, `Response`
2. **Error Handling**: v2 `[]error` → v3 single `error`
3. **Response**: v2 tuple `(code, body, errs)` → v3 `*Response` object
4. **No Parse()**: v3 auto-initializes requests
5. **Hooks**: v3 adds request/response middleware
6. **Path Params**: v3 native `:param` support
7. **Cookie Jar**: v3 built-in session management
8. **CBOR**: v3 adds CBOR encoding
9. **Context**: v3 native cancellation support
10. **Iterators**: v3 uses `iter.Seq2` for collections
11. **Bytes variants removed**: v2 `*Bytes*` methods gone

</details>

### 🛠️ Utils {#utils-migration}

Fiber v3 removes the in-repo `utils` package in favor of the external [`github.com/gofiber/utils/v2`](https://github.com/gofiber/utils) module.

1. Replace imports:

```go
- import "github.com/gofiber/fiber/v2/utils"
+ import "github.com/gofiber/utils/v2"
```

1. Review function changes:

| v2 function | v3 replacement |
| --- | --- |
| `AssertEqual` | removed; use testing libraries like [`github.com/stretchr/testify/assert`](https://pkg.go.dev/github.com/stretchr/testify/assert) |
| `ToLowerBytes` | `utils.ToLowerBytes` |
| `ToUpperBytes` | `utils.ToUpperBytes` |
| `TrimRightBytes` | `utils.TrimRight` |
| `TrimLeftBytes` | `utils.TrimLeft` |
| `TrimBytes` | `utils.Trim` |
| `EqualFoldBytes` | `utils.EqualFold` |
| `UUID` | `utils.UUID` |
| `UUIDv4` | `utils.UUIDv4` |
| `FunctionName` | `utils.FunctionName` |
| `GetArgument` | `utils.GetArgument` |
| `IncrementIPRange` | `utils.IncrementIPRange` |
| `ConvertToBytes` | `utils.ConvertToBytes` |
| `CopyString` | `utils.CopyString` |
| `CopyBytes` | `utils.CopyBytes` |
| `ByteSize` | `utils.ByteSize` |
| `ToString` | `utils.ToString` |
| `UnsafeString` | `utils.UnsafeString` |
| `UnsafeBytes` | `utils.UnsafeBytes` |
| `GetString` | removed; use `utils.ToString` or the standard library |
| `GetBytes` | removed; use `utils.CopyBytes` or `[]byte(s)` |
| `ImmutableString` | removed; strings are already immutable |
| `GetMIME` | `utils.GetMIME` |
| `ParseVendorSpecificContentType` | `utils.ParseVendorSpecificContentType` |
| `StatusMessage` | `utils.StatusMessage` |
| `IsIPv4` | `utils.IsIPv4` |
| `IsIPv6` | `utils.IsIPv6` |
| `ToLower` | `utils.ToLower` |
| `ToUpper` | `utils.ToUpper` |
| `TrimLeft` | `strings.TrimLeft` |
| `Trim` | `strings.Trim` |
| `TrimRight` | `strings.TrimRight` |
| `EqualFold` | `strings.EqualFold` |
| `StartTimeStampUpdater` | `utils.StartTimeStampUpdater` (new `utils.Timestamp` provides the current value) |

1. Update your code. For example:

```go
// v2
import oldutils "github.com/gofiber/fiber/v2/utils"

func demo() {
    b := oldutils.TrimBytes([]byte(" fiber "))
    id := oldutils.UUIDv4()
    s := oldutils.GetString([]byte("foo"))
}

// v3
import (
    "github.com/gofiber/utils/v2"
    "strings"
)

func demo() {
    s := utils.TrimSpace(" fiber ")
    id := utils.UUIDv4()
    str := utils.ToString([]byte("foo"))
    t := strings.TrimRight("bar  ", " ")
}
```

The `github.com/gofiber/utils/v2` module also introduces new helpers like `ParseInt`, `ParseUint`, `Walk`, `ReadFile`, and `Timestamp`.

### 🧬 Middlewares

#### Important Change for Accessing Middleware Data

**Change:** In Fiber v2, some middlewares set data in `c.Locals()` using string keys (e.g., `c.Locals("requestid")`). In Fiber v3, to align with Go's context best practices and prevent key collisions, these middlewares now store their specific data in the request's context using unexported keys of custom types.

**Impact:** Directly accessing these middleware-provided values via `c.Locals("some_string_key")` will no longer work.

**Migration Action:**
The `ContextKey` configuration option has been removed from all middlewares. Values are no longer stored under user-defined keys. You must update your code to use the dedicated exported functions provided by each affected middleware to retrieve its data from the context.

**Examples of new helper functions to use:**

- `requestid.FromContext(c)`
- `csrf.TokenFromContext(c)`
- `csrf.HandlerFromContext(c)`
- `session.FromContext(c)`
- `basicauth.UsernameFromContext(c)`
- `keyauth.TokenFromContext(c)`

**For logging these values:**
The recommended approach is to use the `CustomTags` feature of the Logger middleware, which allows you to call these specific `FromContext` functions. Refer to the [Logger section in "What's New"](#logger) for detailed examples.

:::note
If you were manually setting and retrieving your own application-specific values in `c.Locals()` using string keys, that functionality remains unchanged. This change specifically pertains to how Fiber's built-in (and some contrib) middlewares expose their data.
:::

#### BasicAuth

The `Authorizer` callback now receives the current request context. Update custom
functions from:

```go
Authorizer: func(user, pass string) bool {
    // v2 style
    return user == "admin" && pass == "secret"
}
```

to:

```go
Authorizer: func(user, pass string, _ fiber.Ctx) bool {
    // v3 style with access to the Fiber context
    return user == "admin" && pass == "secret"
}
```

Passwords configured for BasicAuth must now be pre-hashed. If no prefix is supplied the middleware expects a SHA-256 digest encoded in hex. Common prefixes like `{SHA256}` and `{SHA512}` and bcrypt strings are also supported. Plaintext passwords are no longer accepted. Unauthorized responses also include a `Vary: Authorization` header for correct caching behavior.

You can also set the optional `HeaderLimit` and `Charset`
options to further control authentication behavior.

#### KeyAuth

The keyauth middleware was updated to introduce a configurable `Realm` field for the `WWW-Authenticate` header.
The old string-based `KeyLookup` configuration has been replaced with an `Extractor` field, and the `AuthScheme` field has been removed. The auth scheme is now inferred from the extractor used (e.g., `keyauth.FromAuthHeader`). Use helper functions like `keyauth.FromHeader`, `keyauth.FromAuthHeader`, or `keyauth.FromCookie` to define where the key should be retrieved from. Multiple sources can be combined with `keyauth.Chain`.
New `Challenge`, `Error`, `ErrorDescription`, `ErrorURI`, and `Scope` options let you customize challenge responses, include Bearer error parameters, and specify required scopes. `ErrorURI` values are validated as absolute, credentials containing whitespace are rejected, and when multiple authorization extractors are chained, all schemes are advertised in the `WWW-Authenticate` header. The middleware defers emitting `WWW-Authenticate` until a 401 status is final, and `FromAuthHeader` now trims surrounding whitespace.

```go
// Before
app.Use(keyauth.New(keyauth.Config{
    KeyLookup: "header:Authorization",
    AuthScheme: "Bearer",
    Validator: validateAPIKey,
}))

// After
app.Use(keyauth.New(keyauth.Config{
    Extractor: keyauth.FromAuthHeader(fiber.HeaderAuthorization, "Bearer"),
    Validator: validateAPIKey,
}))
```

Combine multiple sources with `keyauth.Chain()` when needed.

#### Cache

The deprecated `Store` and `Key` fields were removed. Use `Storage` and
`KeyGenerator` instead to configure caching backends and cache keys.

Defaults also changed: the middleware now emits `Cache-Control` headers, the default `Expiration` increased to `5 minutes` (from `1 minute`), and a new `MaxBytes` limit of `1 MB` (previously unlimited) now caps cached payloads.

To restore v2 behavior:

- Set `DisableCacheControl` to `true` to suppress automatic `Cache-Control` headers.
- Configure `Expiration` to `1*time.Minute`.
- Set `MaxBytes` to `0` (or a higher value) when caching large responses.

#### CORS

The CORS middleware has been updated to use slices instead of strings for the `AllowOrigins`, `AllowMethods`, `AllowHeaders`, and `ExposeHeaders` fields. Here's how you can update your code:

```go
// Before
app.Use(cors.New(cors.Config{
    AllowOrigins: "https://example.com,https://example2.com",
    AllowMethods: strings.Join([]string{fiber.MethodGet, fiber.MethodPost}, ","),
    AllowHeaders: "Content-Type",
    ExposeHeaders: "Content-Length",
}))

// After
app.Use(cors.New(cors.Config{
    AllowOrigins: []string{"https://example.com", "https://example2.com"},
    AllowMethods: []string{fiber.MethodGet, fiber.MethodPost},
    AllowHeaders: []string{"Content-Type"},
    ExposeHeaders: []string{"Content-Length"},
}))
```

#### CSRF

- **Field Renaming**: The `Expiration` field in the CSRF middleware configuration has been renamed to `IdleTimeout` to better describe its functionality. Additionally, the default value has been reduced from 1 hour to 30 minutes. Update your code as follows:

```go
// Before
app.Use(csrf.New(csrf.Config{
    Expiration: 10 * time.Minute,
}))

// After
app.Use(csrf.New(csrf.Config{
    IdleTimeout: 10 * time.Minute,
}))
```

- **Session Key Removal**: The `SessionKey` field has been removed from the CSRF middleware configuration. The session key is now an unexported constant within the middleware to avoid potential key collisions in the session store.

- **KeyLookup Field Removal**: The `KeyLookup` field has been removed from the CSRF middleware configuration. This field was deprecated and is no longer needed as the middleware now uses a more secure approach for token management.
- **DisableValueRedaction Toggle**: CSRF redacts tokens and storage keys by default; set `DisableValueRedaction` to `true` when diagnostics require the raw values.

- **Default KeyGenerator**: Changed from `utils.UUIDv4` to `utils.SecureToken`, producing base64-encoded tokens instead of UUID format.

```go
// Before
app.Use(csrf.New(csrf.Config{
    KeyLookup: "header:X-Csrf-Token",
    // other config...
}))

// After - use Extractor instead
app.Use(csrf.New(csrf.Config{
    Extractor: csrf.FromHeader("X-Csrf-Token"),
    // other config...
}))
```

- **FromCookie Extractor Removal**: The `csrf.FromCookie` extractor has been intentionally removed for security reasons. Using cookie-based extraction defeats the purpose of CSRF protection by making the extracted token always match the cookie value.

```go
// Before - This was a security vulnerability
app.Use(csrf.New(csrf.Config{
    Extractor: csrf.FromCookie("csrf_token"), // ❌ Insecure!
}))

// After - Use secure extractors instead
app.Use(csrf.New(csrf.Config{
    Extractor: csrf.FromHeader("X-Csrf-Token"), // ✅ Secure
    // or
    Extractor: csrf.FromForm("_csrf"),          // ✅ Secure
    // or
    Extractor: csrf.FromQuery("csrf_token"),    // ✅ Acceptable
}))
```

**Security Note**: The removal of `FromCookie` prevents a common misconfiguration that would completely bypass CSRF protection. The middleware uses the Double Submit Cookie pattern, which requires the token to be submitted through a different channel than the cookie to provide meaningful protection.

#### Idempotency

- **DisableValueRedaction Toggle**: The idempotency middleware now hides keys in logs and error paths by default, with a `DisableValueRedaction` boolean (default `false`) to reveal them when needed.

#### Timeout

The timeout middleware now accepts a configuration struct instead of a duration.
Update your code as follows:

```go
// Before
app.Use(timeout.New(handler, 2*time.Second))

// After
app.Use(timeout.New(handler, timeout.Config{Timeout: 2 * time.Second}))
```

**Important behavioral changes:**

- The middleware now returns **immediately** when the timeout expires, even if the handler is still running. This ensures clients receive timely responses.
- Handlers can detect timeouts by listening on `c.Context().Done()`.
- Panics in the handler are caught and converted to `500 Internal Server Error`.

#### Filesystem

You need to move filesystem middleware to static middleware due to it has been removed from the core.

```go
// Before
app.Use(filesystem.New(filesystem.Config{
    Root: http.Dir("./assets"),
}))

app.Use(filesystem.New(filesystem.Config{
    Root:         http.Dir("./assets"),
    Browse:       true,
    Index:        "index.html",
    MaxAge:       3600,
}))
```

```go
// After
app.Use(static.New("", static.Config{
    FS: os.DirFS("./assets"),
}))

app.Use(static.New("", static.Config{
    FS:           os.DirFS("./assets"),
    Browse:       true,
    IndexNames:   []string{"index.html"},
    MaxAge:       3600,
}))
```

#### EnvVar

The `ExcludeVars` option has been removed. Remove any references to it and use
`ExportVars` to explicitly list environment variables that should be exposed.

#### Healthcheck

Previously, the Healthcheck middleware was configured with a combined setup for liveness and readiness probes:

```go
//before
app.Use(healthcheck.New(healthcheck.Config{
    LivenessProbe: func(c fiber.Ctx) bool {
        return true
    },
    LivenessEndpoint: "/live",
    ReadinessProbe: func(c fiber.Ctx) bool {
        return serviceA.Ready() && serviceB.Ready() && ...
    },
    ReadinessEndpoint: "/ready",
}))
```

With the new version, each health check endpoint is configured separately, allowing for more flexibility:

```go
// after

// Default liveness endpoint configuration
app.Get(healthcheck.LivenessEndpoint, healthcheck.New(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))

// Default readiness endpoint configuration
app.Get(healthcheck.ReadinessEndpoint, healthcheck.New())

// New default startup endpoint configuration
// Default endpoint is /startupz
app.Get(healthcheck.StartupEndpoint, healthcheck.New(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return serviceA.Ready() && serviceB.Ready() && ...
    },
}))

// Custom liveness endpoint configuration
app.Get("/live", healthcheck.New())
```

#### Monitor

Since v3 the Monitor middleware has been moved to the [Contrib package](https://github.com/gofiber/contrib/tree/main/monitor)

```go
// Before
import "github.com/gofiber/fiber/v2/middleware/monitor"

app.Use("/metrics", monitor.New())
```

You only need to change the import path to the contrib package.

```go
// After
import "github.com/gofiber/contrib/monitor"

app.Use("/metrics", monitor.New())
```

#### Proxy

In previous versions, TLS settings for the proxy middleware were set using the `WithTlsConfig` method. This method has been removed in favor of a more idiomatic configuration via the `TLSConfig` field in the `Config` struct.

#### Before (v2 usage)

```go
proxy.WithTlsConfig(&tls.Config{
    InsecureSkipVerify: true,
})

// Forward to url
app.Get("/gif", proxy.Forward("https://i.imgur.com/IWaBepg.gif"))
```

#### After (v3 usage)

```go
proxy.WithClient(&fasthttp.Client{
    TLSConfig: &tls.Config{InsecureSkipVerify: true},
})

// Forward to url
app.Get("/gif", proxy.Forward("https://i.imgur.com/IWaBepg.gif"))
```

`proxy.Balancer` also adopts the common middleware signature pattern and now accepts an optional variadic config: call `proxy.Balancer()` to use the defaults or continue passing a single `proxy.Config` value as in v2.

#### Session

`session.New()` now returns a middleware handler. When using the store pattern,
create a store with `session.NewStore()` or call `Store()` on the middleware.
Sessions obtained from a store must be released manually via `sess.Release()`.
Additionally, replace the deprecated `KeyLookup` option with extractor
functions such as `session.FromCookie()` or `session.FromHeader()`. Multiple
extractors can be combined with `session.Chain()`.

```go
// Before
app.Use(session.New(session.Config{
    KeyLookup: "cookie:session_id",
    Store:     session.NewStore(),
}))
```

```go
// After
app.Use(session.New(session.Config{
    Extractor: session.FromCookie("session_id"),
    Store:     session.NewStore(),
}))
```

See the [Session Middleware Migration Guide](./middleware/session.md#migration-guide)
for complete details.
