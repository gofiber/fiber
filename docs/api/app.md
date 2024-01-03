---
id: app
title: 🚀 App
description: The app instance conventionally denotes the Fiber application.
sidebar_position: 2
---

import RoutingHandler from './../partials/routing/handler.md';

## Static

Use the **Static** method to serve static files such as **images**, **CSS,** and **JavaScript**.

:::info
By default, **Static** will serve `index.html` files in response to a request on a directory.
:::

```go title="Signature"
func (app *App) Static(prefix, root string, config ...Static) Router
```

Use the following code to serve files in a directory named `./public`

```go
app.Static("/", "./public")

// => http://localhost:3000/hello.html
// => http://localhost:3000/js/jquery.js
// => http://localhost:3000/css/style.css
```

```go title="Examples"
// Serve files from multiple directories
app.Static("/", "./public")

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

If you want to have a little bit more control regarding the settings for serving static files. You could use the `fiber.Static` struct to enable specific settings.

```go title="fiber.Static{}"
// Static defines configuration options when defining static assets.
type Static struct {
    // When set to true, the server tries minimizing CPU usage by caching compressed files.
    // This works differently than the github.com/gofiber/compression middleware.
    // Optional. Default value false
    Compress bool `json:"compress"`

    // When set to true, enables byte range requests.
    // Optional. Default value false
    ByteRange bool `json:"byte_range"`

    // When set to true, enables directory browsing.
    // Optional. Default value false.
    Browse bool `json:"browse"`

    // When set to true, enables direct download.
    // Optional. Default value false.
    Download bool `json:"download"`

    // The name of the index file for serving a directory.
    // Optional. Default value "index.html".
    Index string `json:"index"`

    // Expiration duration for inactive file handlers.
    // Use a negative time.Duration to disable it.
    //
    // Optional. Default value 10 * time.Second.
    CacheDuration time.Duration `json:"cache_duration"`

    // The value for the Cache-Control HTTP-header
    // that is set on the file response. MaxAge is defined in seconds.
    //
    // Optional. Default value 0.
    MaxAge int `json:"max_age"`

    // ModifyResponse defines a function that allows you to alter the response.
    //
    // Optional. Default: nil
    ModifyResponse Handler

    // Next defines a function to skip this middleware when returned true.
    //
    // Optional. Default: nil
    Next func(c *Ctx) bool
}
```

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

## Route Handlers

<RoutingHandler />

## Mount

You can Mount Fiber instance by creating a `*Mount`

```go title="Signature"
func (a *App) Mount(prefix string, app *App) Router
```

```go title="Examples"
func main() {
    app := fiber.New()
    micro := fiber.New()
    app.Mount("/john", micro) // GET /john/doe -> 200 OK

    micro.Get("/doe", func(c *fiber.Ctx) error {
        return c.SendStatus(fiber.StatusOK)
    })

    log.Fatal(app.Listen(":3000"))
}
```

## MountPath

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

	two.Mount("/three", three)
	one.Mount("/two", two)
	app.Mount("/one", one)
  
	one.MountPath()   // "/one"
	two.MountPath()   // "/one/two"
	three.MountPath() // "/one/two/three"
	app.MountPath()   // ""
}
```

:::caution
Mounting order is important for MountPath. If you want to get mount paths properly, you should start mounting from the deepest app.
:::

## Group

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

## Route

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

## Server

Server returns the underlying [fasthttp server](https://godoc.org/github.com/valyala/fasthttp#Server)

```go title="Signature"
func (app *App) Server() *fasthttp.Server
```

```go title="Examples"
func main() {
    app := fiber.New()

    app.Server().MaxConnsPerIP = 1

    // ...
}
```

## Server Shutdown

Shutdown gracefully shuts down the server without interrupting any active connections. Shutdown works by first closing all open listeners and then waits indefinitely for all connections to return to idle before shutting down.

ShutdownWithTimeout will forcefully close any active connections after the timeout expires.

ShutdownWithContext shuts down the server including by force if the context's deadline is exceeded.

```go
func (app *App) Shutdown() error
func (app *App) ShutdownWithTimeout(timeout time.Duration) error
func (app *App) ShutdownWithContext(ctx context.Context) error
```

## HandlersCount

This method returns the amount of registered handlers.

```go title="Signature"
func (app *App) HandlersCount() uint32
```

## Stack

This method returns the original router stack

```go title="Signature"
func (app *App) Stack() [][]*Route
```

```go title="Examples"
var handler = func(c *fiber.Ctx) error { return nil }

func main() {
    app := fiber.New()

    app.Get("/john/:age", handler)
    app.Post("/register", handler)

    data, _ := json.MarshalIndent(app.Stack(), "", "  ")
    fmt.Println(string(data))

    app.Listen(":3000")
}
```

```javascript title="Result"
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

## Name

This method assigns the name of latest created route.

```go title="Signature"
func (app *App) Name(name string) Router
```

```go title="Examples"
var handler = func(c *fiber.Ctx) error { return nil }

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

```javascript title="Result"
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

## GetRoute

This method gets the route by name.

```go title="Signature"
func (app *App) GetRoute(name string) Route
```

```go title="Examples"
var handler = func(c *fiber.Ctx) error { return nil }

func main() {
    app := fiber.New()

    app.Get("/", handler).Name("index")
    
    data, _ := json.MarshalIndent(app.GetRoute("index"), "", "  ")
	fmt.Print(string(data))


	app.Listen(":3000")

}
```

```javascript title="Result"
{
  "method": "GET",
  "name": "index",
  "path": "/",
  "params": null
}
```

## GetRoutes

This method gets all routes.

```go title="Signature"
func (app *App) GetRoutes(filterUseOption ...bool) []Route
```

When filterUseOption equal to true, it will filter the routes registered by the middleware.
```go title="Examples"
func main() {
	app := fiber.New()
	app.Post("/", func (c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	}).Name("index")
	data, _ := json.MarshalIndent(app.GetRoutes(true), "", "  ")
	fmt.Print(string(data))
}
```

```javascript title="Result"
[
    {
        "method": "POST",
        "name": "index",
        "path": "/",
        "params": null
    }
]
```

## Config

Config returns the app config as value \( read-only \).

```go title="Signature"
func (app *App) Config() Config
```

## Handler

Handler returns the server handler that can be used to serve custom \*fasthttp.RequestCtx requests.

```go title="Signature"
func (app *App) Handler() fasthttp.RequestHandler
```

## Listen

Listen serves HTTP requests from the given address.

```go title="Signature"
func (app *App) Listen(addr string) error
```

```go title="Examples"
// Listen on port :8080 
app.Listen(":8080")

// Custom host
app.Listen("127.0.0.1:8080")
```

## ListenTLS

ListenTLS serves HTTPs requests from the given address using certFile and keyFile paths to as TLS certificate and key file.

```go title="Signature"
func (app *App) ListenTLS(addr, certFile, keyFile string) error
```

```go title="Examples"
app.ListenTLS(":443", "./cert.pem", "./cert.key");
```

Using `ListenTLS` defaults to the following config \( use `Listener` to provide your own config \)

```go title="Default \*tls.Config"
&tls.Config{
    MinVersion:               tls.VersionTLS12,
    Certificates: []tls.Certificate{
        cert,
    },
}
```

## ListenTLSWithCertificate

```go title="Signature"
func (app *App) ListenTLS(addr string, cert tls.Certificate) error
```

```go title="Examples"
app.ListenTLSWithCertificate(":443", cert);
```

Using `ListenTLSWithCertificate` defaults to the following config \( use `Listener` to provide your own config \)

```go title="Default \*tls.Config"
&tls.Config{
    MinVersion:               tls.VersionTLS12,
    Certificates: []tls.Certificate{
        cert,
    },
}
```

## ListenMutualTLS

ListenMutualTLS serves HTTPs requests from the given address using certFile, keyFile and clientCertFile are the paths to TLS certificate and key file

```go title="Signature"
func (app *App) ListenMutualTLS(addr, certFile, keyFile, clientCertFile string) error
```

```go title="Examples"
app.ListenMutualTLS(":443", "./cert.pem", "./cert.key", "./ca-chain-cert.pem");
```

Using `ListenMutualTLS` defaults to the following config \( use `Listener` to provide your own config \)

```go title="Default \*tls.Config"
&tls.Config{
	MinVersion: tls.VersionTLS12,
	ClientAuth: tls.RequireAndVerifyClientCert,
	ClientCAs:  clientCertPool,
	Certificates: []tls.Certificate{
		cert,
	},
}
```

## ListenMutualTLSWithCertificate

ListenMutualTLSWithCertificate serves HTTPs requests from the given address using certFile, keyFile and clientCertFile are the paths to TLS certificate and key file

```go title="Signature"
func (app *App) ListenMutualTLSWithCertificate(addr string, cert tls.Certificate, clientCertPool *x509.CertPool) error
```

```go title="Examples"
app.ListenMutualTLSWithCertificate(":443", cert, clientCertPool);
```

Using `ListenMutualTLSWithCertificate` defaults to the following config \( use `Listener` to provide your own config \)

```go title="Default \*tls.Config"
&tls.Config{
	MinVersion: tls.VersionTLS12,
	ClientAuth: tls.RequireAndVerifyClientCert,
	ClientCAs:  clientCertPool,
	Certificates: []tls.Certificate{
		cert,
	},
}
```

## Listener

You can pass your own [`net.Listener`](https://pkg.go.dev/net/#Listener) using the `Listener` method. This method can be used to enable **TLS/HTTPS** with a custom tls.Config.

```go title="Signature"
func (app *App) Listener(ln net.Listener) error
```

```go title="Examples"
ln, _ := net.Listen("tcp", ":3000")

cer, _:= tls.LoadX509KeyPair("server.crt", "server.key")

ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cer}})

app.Listener(ln)
```

## Test

Testing your application is done with the **Test** method. Use this method for creating `_test.go` files or when you need to debug your routing logic. The default timeout is `1s` if you want to disable a timeout altogether, pass `-1` as a second argument.

```go title="Signature"
func (app *App) Test(req *http.Request, msTimeout ...int) (*http.Response, error)
```

```go title="Examples"
// Create route with GET method for test:
app.Get("/", func(c *fiber.Ctx) error {
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

Hooks is a method to return [hooks](../guide/hooks.md) property.

```go title="Signature"
func (app *App) Hooks() *Hooks
```