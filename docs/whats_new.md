---
id: whats_new
title: 🆕 Whats New in v3
sidebar_position: 2
toc_max_heading_level: 3
---

:::caution

It's a draft, not finished yet.

:::

[//]: # (https://github.com/gofiber/fiber/releases/tag/v3.0.0-beta.2)

## 🎉 Welcome

We are excited to announce the release of Fiber v3! 🚀

In this guide, we'll walk you through the most important changes in Fiber `v3` and show you how to migrate your existing Fiber `v2` applications to Fiber `v3`.

Here's a quick overview of the changes in Fiber `v3`:

- [🚀 App](#-app)
- [🗺️ Router](#-router)
- [🧠 Context](#-context)
- [📎 Binding](#-binding)
- [🔄️ Redirect](#-redirect)
- [🌎 Client package](#-client-package)
- [🧰 Generic functions](#-generic-functions)
- [🧬 Middlewares](#-middlewares)
  - [CORS](#cors)
  - [CSRF](#csrf)
  - [Session](#session)
  - [Filesystem](#filesystem)
  - [Monitor](#monitor)
  - [Healthcheck](#healthcheck)
- [📋 Migration guide](#-migration-guide)

## Drop for old Go versions

Fiber `v3` drops support for Go versions below `1.22`. We recommend upgrading to Go `1.22` or higher to use Fiber `v3`.

## 🚀 App

:::caution
DRAFT section
:::

We have made several changes to the Fiber app, including:

- Listen -> unified with config
- Static -> has been removed and moved to [static middleware](./middleware/static.md)
- app.Config properties moved to listen config
  - DisableStartupMessage
  - EnablePrefork -> previously Prefork
  - EnablePrintRoutes
  - ListenerNetwork -> previously Network
- app.Config.EnabledTrustedProxyCheck -> has been moved to app.Config.TrustProxy
  - TrustedProxies -> has been moved to TrustProxyConfig.Proxies

### new methods

- RegisterCustomBinder
- RegisterCustomConstraint
- NewCtxFunc

### removed methods

- Mount -> Use app.Use() instead
- ListenTLS -> Use app.Listen() with tls.Config
- ListenTLSWithCertificate -> Use app.Listen() with tls.Config
- ListenMutualTLS -> Use app.Listen() with tls.Config
- ListenMutualTLSWithCertificate -> Use app.Listen() with tls.Config

### Methods changes

- Test -> Replaced timeout with a config parameter
  - -1 represents no timeout -> 0 represents no timeout
- Listen -> has a config parameter
- Listener -> has a config parameter

### CTX interface + customizable

---

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
app.Use(["/v1", "/v2"], func(c *fiber.Ctx) error {
  // Middleware for /v1 and /v2
  return c.Next() 
})

// define subapp
api := fiber.New()
api.Get("/user", func(c *fiber.Ctx) error {
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

---

## 🧠 Context

:::caution
DRAFT section
:::

### New Features

- Cookie now allows Partitioned cookies for [CHIPS](https://developers.google.com/privacy-sandbox/3pcd/chips) support. CHIPS (Cookies Having Independent Partitioned State) is a feature that improves privacy by allowing cookies to be partitioned by top-level site, mitigating cross-site tracking.

### new methods

- AutoFormat -> ExpressJs like
- Host -> ExpressJs like
- Port -> ExpressJs like
- IsProxyTrusted
- Reset
- Schema -> ExpressJs like
- SendStream -> ExpressJs like
- SendString -> ExpressJs like
- String -> ExpressJs like
- ViewBind -> instead of Bind

### removed methods

- AllParams -> c.Bind().URL() ?
- ParamsInt -> Params Generic
- QueryBool -> Query Generic
- QueryFloat -> Query Generic
- QueryInt -> Query Generic
- BodyParser -> c.Bind().Body()
- CookieParser -> c.Bind().Cookie()
- ParamsParser -> c.Bind().URL()
- RedirectToRoute -> c.Redirect().Route()
- RedirectBack -> c.Redirect().Back()
- ReqHeaderParser -> c.Bind().Header()

### changed methods

- Bind -> for Binding instead of View, us c.ViewBind()
- Format -> Param: body interface{} -> handlers ...ResFmt
- Redirect -> c.Redirect().To()
- SendFile now supports different configurations using the config parameter.
- Context has been renamed to RequestCtx which corresponds to the FastHTTP Request Context.
- UserContext has been renamed to Context which returns a context.Context object.
- SetUserContext has been renamed to SetContext.

---

## 🌎 Client package

The Gofiber client has been completely rebuilt. It includes numerous new features such as Cookiejar, request/response hooks, and more.
You can take a look to [client docs](./client/rest.md) to see what's new with the client.

## 📎 Binding

:::caution
DRAFT section
:::

## 🔄 Redirect

:::caution
DRAFT section
:::

## 🧰 Generic functions

:::caution
DRAFT section
:::

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

We have added a new method that allows the route tree stack to be rebuilt in runtime, with it, you can add a route while your application is running and rebuild the route tree stack to make it registered and available for calls.

You can find more reference on it in the [app](./api/app.md#rebuildtree):

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

In this example, a new route is defined and then `RebuildTree()` is called to make sure the new route is registered and available.

**Note:** Use this method with caution. It is **not** thread-safe and calling it can be very performance-intensive, so it should be used sparingly and only in
development mode. Avoid using it concurrently.

### 🧠 Context

### 📎 Parser

### 🔄 Redirect

### 🌎 Client package

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
  LivenessProbe: func(c *fiber.Ctx) bool {
    return true
  },
  LivenessEndpoint: "/live",
  ReadinessProbe: func(c *fiber.Ctx) bool {
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
  Probe: func(c *fiber.Ctx) bool {
    return true
  },
}))

// Default readiness endpoint configuration
app.Get(healthcheck.DefaultReadinessEndpoint, healthcheck.NewHealthChecker())

// New default startup endpoint configuration
// Default endpoint is /startupz
app.Get(healthcheck.DefaultStartupEndpoint, healthcheck.NewHealthChecker(healthcheck.Config{
  Probe: func(c *fiber.Ctx) bool {
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
