---
id: whats_new
title: 🆕 Whats New in v3
sidebar_position: 2
toc_max_heading_level: 4
---

[//]: # (https://github.com/gofiber/fiber/releases/tag/v3.0.0-beta.2)

## 🎉 Welcome

We are excited to announce the release of Fiber v3! 🚀

In this guide, we'll walk you through the most important changes in Fiber `v3` and show you how to migrate your existing Fiber `v2` applications to Fiber `v3`.

Here's a quick overview of the changes in Fiber `v3`:

- [🚀 App](#-app)
- [🎣 Hooks](#-hooks)
- [🚀 Listen](#-listen)
- [🗺️ Router](#-router)
- [🧠 Context](#-context)
- [📎 Binding](#-binding)
- [🔄️ Redirect](#-redirect)
- [🌎 Client package](#-client-package)
- [🧰 Generic functions](#-generic-functions)
- [📃 Log](#-log)
- [🧬 Middlewares](#-middlewares)
  - [CORS](#cors)
  - [CSRF](#csrf)
  - [Session](#session)
  - [Logger](#logger)
  - [Filesystem](#filesystem)
  - [Monitor](#monitor)
  - [Healthcheck](#healthcheck)
- [🔌 Addons](#-addons)
- [📋 Migration guide](#-migration-guide)

## Drop for old Go versions

Fiber `v3` drops support for Go versions below `1.23`. We recommend upgrading to Go `1.23` or higher to use Fiber `v3`.

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
- **NewCtxFunc**: Introduces a new context function.

### Removed Methods

- **Mount**: Use `app.Use()` instead.
- **ListenTLS**: Use `app.Listen()` with `tls.Config`.
- **ListenTLSWithCertificate**: Use `app.Listen()` with `tls.Config`.
- **ListenMutualTLS**: Use `app.Listen()` with `tls.Config`.
- **ListenMutualTLSWithCertificate**: Use `app.Listen()` with `tls.Config`.

### Method Changes

- **Test**: The `Test` method has replaced the timeout parameter with a configuration parameter. `-1` represents no timeout, and `0` represents no timeout.
- **Listen**: Now has a configuration parameter.
- **Listener**: Now has a configuration parameter.

### Custom Ctx Interface in Fiber v3

Fiber v3 introduces a customizable `Ctx` interface, allowing developers to extend and modify the context to fit their needs. This feature provides greater flexibility and control over request handling.

#### Idea Behind Custom Ctx Classes

The idea behind custom `Ctx` classes is to give developers the ability to extend the default context with additional methods and properties tailored to the specific requirements of their application. This allows for better request handling and easier implementation of specific logic.

#### NewCtxFunc

The `NewCtxFunc` method allows you to customize the `Ctx` struct as needed.

```go title="Signature"
func (app *App) NewCtxFunc(function func(app *App) CustomCtx)
```

<details>
<summary>Example</summary>

Here’s an example of how to customize the `Ctx` interface:

```go
package main

import (
    "log"
    "github.com/gofiber/fiber/v3"
)

type CustomCtx struct {
    fiber.Ctx
}

// Custom method
func (c *CustomCtx) CustomMethod() string {
    return "custom value"
}

func main() {
    app := fiber.New()

    app.NewCtxFunc(func(app *fiber.App) fiber.Ctx {
        return &CustomCtx{
            Ctx: *fiber.NewCtx(app),
        }
    })

    app.Get("/", func(c fiber.Ctx) error {
        customCtx := c.(*CustomCtx)
        return c.SendString(customCtx.CustomMethod())
    })

    log.Fatal(app.Listen(":3000"))
}
```

In this example, a custom context `CustomCtx` is created with an additional method `CustomMethod`. The `NewCtxFunc` method is used to replace the default context with the custom one.

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

## 🎣 Hooks

We have made several changes to the Fiber hooks, including:

- Added new shutdown hooks to provide better control over the shutdown process:
  - `OnPreShutdown` - Executes before the server starts shutting down
  - `OnPostShutdown` - Executes after the server has shut down, receives any shutdown error
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

## 🗺 Router

We have slightly adapted our router interface

### HTTP method registration

In `v2` one handler was already mandatory when the route has been registered, but this was checked at runtime and was not correctly reflected in the signature, this has now been changed in `v3` to make it more explicit.

```diff
-    Get(path string, handlers ...Handler) Router
+    Get(path string, handler Handler, middleware ...Handler) Router
-    Head(path string, handlers ...Handler) Router
+    Head(path string, handler Handler, middleware ...Handler) Router
-    Post(path string, handlers ...Handler) Router
+    Post(path string, handler Handler, middleware ...Handler) Router
-    Put(path string, handlers ...Handler) Router
+    Put(path string, handler Handler, middleware ...Handler) Router
-    Delete(path string, handlers ...Handler) Router
+    Delete(path string, handler Handler, middleware ...Handler) Router
-    Connect(path string, handlers ...Handler) Router
+    Connect(path string, handler Handler, middleware ...Handler) Router
-    Options(path string, handlers ...Handler) Router
+    Options(path string, handler Handler, middleware ...Handler) Router
-    Trace(path string, handlers ...Handler) Router
+    Trace(path string, handler Handler, middleware ...Handler) Router
-    Patch(path string, handlers ...Handler) Router
+    Patch(path string, handler Handler, middleware ...Handler) Router
-    All(path string, handlers ...Handler) Router
+    All(path string, handler Handler, middleware ...Handler) Router
```

### Route chaining

The route method is now like [`Express`](https://expressjs.com/de/api.html#app.route) which gives you the option of a different notation and allows you to concatenate the route declaration.

```diff
-    Route(prefix string, fn func(router Router), name ...string) Router
+    Route(path string) Register    
```

<details>
<summary>Example</summary>

```go
app.Route("/api").Route("/user/:id?")
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

[Here](./api/app#route) you can find more information.

### Middleware registration

We have aligned our method for middlewares closer to [`Express`](https://expressjs.com/de/api.html#app.use) and now also support the [`Use`](./api/app#use) of multiple prefixes.

Registering a subapp is now also possible via the [`Use`](./api/app#use) method instead of the old `app.Mount` method.

<details>
<summary>Example</summary>

```go
// register mulitple prefixes
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
+    Add(methods []string, path string, handler Handler, middleware ...Handler) Router
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

### New Methods

- **AutoFormat**: Similar to Express.js, automatically formats the response based on the request's `Accept` header.
- **Host**: Similar to Express.js, returns the host name of the request.
- **Port**: Similar to Express.js, returns the port number of the request.
- **IsProxyTrusted**: Checks the trustworthiness of the remote IP.
- **Reset**: Resets context fields for server handlers.
- **Schema**: Similar to Express.js, returns the schema (HTTP or HTTPS) of the request.
- **SendStream**: Similar to Express.js, sends a stream as the response.
- **SendStreamWriter**: Sends a stream using a writer function.
- **SendString**: Similar to Express.js, sends a string as the response.
- **String**: Similar to Express.js, converts a value to a string.
- **ViewBind**: Binds data to a view, replacing the old `Bind` method.
- **CBOR**: Introducing [CBOR](https://cbor.io/) binary encoding format for both request & response body. CBOR is a binary data serialization format which is both compact and efficient, making it ideal for use in web applications.
- **Drop**: Terminates the client connection silently without sending any HTTP headers or response body. This can be used for scenarios where you want to block certain requests without notifying the client, such as mitigating DDoS attacks or protecting sensitive endpoints from unauthorized access.
- **End**: Similar to Express.js, immediately flushes the current response and closes the underlying connection.

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

### Changed Methods

- **Bind**: Now used for binding instead of view binding. Use `c.ViewBind()` for view binding.
- **Format**: Parameter changed from `body interface{}` to `handlers ...ResFmt`.
- **Redirect**: Use `c.Redirect().To()` instead.
- **SendFile**: Now supports different configurations using a config parameter.
- **Context**: Renamed to `RequestCtx` to correspond with the FastHTTP Request Context.
- **UserContext**: Renamed to `Context`, which returns a `context.Context` object.
- **SetUserContext**: Renamed to `SetContext`.

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

## 🌎 Client package

The Gofiber client has been completely rebuilt. It includes numerous new features such as Cookiejar, request/response hooks, and more.
You can take a look to [client docs](./client/rest.md) to see what's new with the client.

## 📎 Binding

Fiber v3 introduces a new binding mechanism that simplifies the process of binding request data to structs. The new binding system supports binding from various sources such as URL parameters, query parameters, headers, and request bodies. This unified approach makes it easier to handle different types of request data in a consistent manner.

### New Features

- Unified binding from URL parameters, query parameters, headers, and request bodies.
- Support for custom binders and constraints.
- Improved error handling and validation.
- Support multipart file binding for `*multipart.FileHeader`, `*[]*multipart.FileHeader`, and `[]*multipart.FileHeader` field types.

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
        value, err := fiber.Convert[string](c.Query("value"), strconv.Atoi, 0)
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

## 📃 Log

`fiber.AllLogger` interface now has a new method called `Logger`. This method can be used to get the underlying logger instance from the Fiber logger middleware. This is useful when you want to configure the logger middleware with a custom logger and still want to access the underlying logger instance.

You can find more details about this feature in [/docs/api/log.md](./api/log.md#logger).

## 🧬 Middlewares

### Adaptor

The adaptor middleware has been significantly optimized for performance and efficiency. Key improvements include reduced response times, lower memory usage, and fewer memory allocations. These changes make the middleware more reliable and capable of handling higher loads effectively. Enhancements include the introduction of a `sync.Pool` for managing `fasthttp.RequestCtx` instances and better HTTP request and response handling between net/http and fasthttp contexts.

| Payload Size | Metric           |     V2    |    V3    |    Percent Change |
|--------------|------------------|-----------|----------|-------------------|
| 100KB        | Execution Time   | 1056 ns/op| 588.6 ns/op | -44.25%        |
|              | Memory Usage     | 2644 B/op | 254 B/op    | -90.39%        |
|              | Allocations      | 16 allocs/op | 5 allocs/op | -68.75%     |
| 500KB        | Execution Time   | 1061 ns/op| 562.9 ns/op | -46.94%        |
|              | Memory Usage     | 2644 B/op | 248 B/op    | -90.62%        |
|              | Allocations      | 16 allocs/op | 5 allocs/op | -68.75%     |
| 1MB          | Execution Time   | 1080 ns/op| 629.7 ns/op | -41.68%        |
|              | Memory Usage     | 2646 B/op | 267 B/op    | -89.91%        |
|              | Allocations      | 16 allocs/op | 5 allocs/op | -68.75%     |
| 5MB          | Execution Time   | 1093 ns/op| 540.3 ns/op | -50.58%        |
|              | Memory Usage     | 2654 B/op | 254 B/op    | -90.43%        |
|              | Allocations      | 16 allocs/op | 5 allocs/op | -68.75%     |
| 10MB         | Execution Time   | 1044 ns/op| 533.1 ns/op | -48.94%        |
|              | Memory Usage     | 2665 B/op | 258 B/op    | -90.32%        |
|              | Allocations      | 16 allocs/op | 5 allocs/op | -68.75%     |
| 25MB         | Execution Time   | 1069 ns/op| 540.7 ns/op | -49.42%        |
|              | Memory Usage     | 2706 B/op | 289 B/op    | -89.32%        |
|              | Allocations      | 16 allocs/op | 5 allocs/op | -68.75%     |
| 50MB         | Execution Time   | 1137 ns/op| 554.6 ns/op | -51.21%        |
|              | Memory Usage     | 2734 B/op | 298 B/op    | -89.10%        |
|              | Allocations      | 16 allocs/op | 5 allocs/op | -68.75%     |

### Cache

We are excited to introduce a new option in our caching middleware: Cache Invalidator. This feature provides greater control over cache management, allowing you to define a custom conditions for invalidating cache entries.  
Additionally, the caching middleware has been optimized to avoid caching non-cacheable status codes, as defined by the [HTTP standards](https://datatracker.ietf.org/doc/html/rfc7231#section-6.1). This improvement enhances cache accuracy and reduces unnecessary cache storage usage.

### CORS

We've made some changes to the CORS middleware to improve its functionality and flexibility. Here's what's new:

#### New Struct Fields

- `Config.AllowPrivateNetwork`: This new field is a boolean that allows you to control whether private networks are allowed. This is related to the [Private Network Access (PNA)](https://wicg.github.io/private-network-access/) specification from the Web Incubator Community Group (WICG). When set to `true`, the CORS middleware will allow CORS preflight requests from private networks and respond with the `Access-Control-Allow-Private-Network: true` header. This could be useful in development environments or specific use cases, but should be done with caution due to potential security risks.

#### Updated Struct Fields

We've updated several fields from a single string (containing comma-separated values) to slices, allowing for more explicit declaration of multiple values. Here are the updated fields:

- `Config.AllowOrigins`: Now accepts a slice of strings, each representing an allowed origin.
- `Config.AllowMethods`: Now accepts a slice of strings, each representing an allowed method.
- `Config.AllowHeaders`: Now accepts a slice of strings, each representing an allowed header.
- `Config.ExposeHeaders`: Now accepts a slice of strings, each representing an exposed header.

### Compression

We've added support for `zstd` compression on top of `gzip`, `deflate`, and `brotli`.

### EncryptCookie

Added support for specifying Key length when using `encryptcookie.GenerateKey(length)`. This allows the user to generate keys compatible with `AES-128`, `AES-192`, and `AES-256` (Default).

### Session

The Session middleware has undergone key changes in v3 to improve functionality and flexibility. While v2 methods remain available for backward compatibility, we now recommend using the new middleware handler for session management.

#### Key Updates

- **New Middleware Handler**: The `New` function now returns a middleware handler instead of a `*Store`. To access the session store, use the `Store` method on the middleware, or opt for `NewStore` or `NewWithStore` for custom store integration.

- **Manual Session Release**: Session instances are no longer automatically released after being saved. To ensure proper lifecycle management, you must manually call `sess.Release()`.

- **Idle Timeout**: The `Expiration` field has been replaced with `IdleTimeout`, which handles session inactivity. If the session is idle for the specified duration, it will expire. The idle timeout is updated when the session is saved. If you are using the middleware handler, the idle timeout will be updated automatically.

- **Absolute Timeout**: The `AbsoluteTimeout` field has been added. If you need to set an absolute session timeout, you can use this field to define the duration. The session will expire after the specified duration, regardless of activity.

For more details on these changes and migration instructions, check the [Session Middleware Migration Guide](./middleware/session.md#migration-guide).

### Logger

New helper function called `LoggerToWriter` has been added to the logger middleware. This function allows you to use 3rd party loggers such as `logrus` or `zap` with the Fiber logger middleware without any extra afford. For example, you can use `zap` with Fiber logger middleware like this:

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

### Filesystem

We've decided to remove filesystem middleware to clear up the confusion between static and filesystem middleware.
Now, static middleware can do everything that filesystem middleware and static do. You can check out [static middleware](./middleware/static.md) or [migration guide](#-migration-guide) to see what has been changed.

### Monitor

Monitor middleware is migrated to the [Contrib package](https://github.com/gofiber/contrib/tree/main/monitor) with [PR #1172](https://github.com/gofiber/contrib/pull/1172).

### Healthcheck

The Healthcheck middleware has been enhanced to support more than two routes, with default endpoints for liveliness, readiness, and startup checks. Here's a detailed breakdown of the changes and how to use the new features.

1. **Support for More Than Two Routes**:
   - The updated middleware now supports multiple routes beyond the default liveliness and readiness endpoints. This allows for more granular health checks, such as startup probes.

2. **Default Endpoints**:
   - Three default endpoints are now available:
     - **Liveness**: `/livez`
     - **Readiness**: `/readyz`
     - **Startup**: `/startupz`
   - These endpoints can be customized or replaced with user-defined routes.

3. **Simplified Configuration**:
   - The configuration for each health check endpoint has been simplified. Each endpoint can be configured separately, allowing for more flexibility and readability.

Refer to the [healthcheck middleware migration guide](./middleware/healthcheck.md) or the [general migration guide](#-migration-guide) to review the changes.

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

- [🚀 App](#-app-1)
- [🗺 Router](#-router-1)
- [🧠 Context](#-context-1)
- [📎 Parser](#-parser)
- [🔄 Redirect](#-redirect-1)
- [🌎 Client package](#-client-package-1)
- [🧬 Middlewares](#-middlewares-1)

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

### 🗺 Router

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

To migrate [`Route`](#route-chaining) you need to read [this](#route-chaining).

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
app.Route("/api").Route("/user/:id?")
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

## RemoveRoute

- **RemoveRoute**: Removes route by path

- **RemoveRouteByName**: Removes route by name

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

:::caution
DRAFT section
:::

### 🧬 Middlewares

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

#### Healthcheck

Previously, the Healthcheck middleware was configured with a combined setup for liveliness and readiness probes:

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
app.Get(healthcheck.DefaultLivenessEndpoint, healthcheck.NewHealthChecker(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return true
    },
}))

// Default readiness endpoint configuration
app.Get(healthcheck.DefaultReadinessEndpoint, healthcheck.NewHealthChecker())

// New default startup endpoint configuration
// Default endpoint is /startupz
app.Get(healthcheck.DefaultStartupEndpoint, healthcheck.NewHealthChecker(healthcheck.Config{
    Probe: func(c fiber.Ctx) bool {
        return serviceA.Ready() && serviceB.Ready() && ...
    },
}))

// Custom liveness endpoint configuration
app.Get("/live", healthcheck.NewHealthChecker())
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
