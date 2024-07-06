---
id: whats_new
title: 🆕 Whats New in v3
sidebar_position: 2
toc_max_heading_level: 3
---

:::caution

Its a draft, not finished yet.

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
  - [Session](#session)
  - [Filesystem](#filesystem)
  - [Monitor](#monitor)
- [📋 Migration guide](#-migration-guide)

## Drop for old Go versions

Fiber `v3` drops support for Go versions below `1.21`. We recommend upgrading to Go `1.21` or higher to use Fiber `v3`.

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

- Test -> timeout changed to 1 second
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

:::caution
DRAFT section
:::

### Filesystem

We've decided to remove filesystem middleware to clear up the confusion between static and filesystem middleware.
Now, static middleware can do everything that filesystem middleware and static do. You can check out [static middleware](./middleware/static.md) or [migration guide](#-migration-guide) to see what has been changed.

### Monitor

:::caution
DRAFT section
:::

Monitor middleware is now in Contrib package.

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
