---
id: fiber
title: 📦 Fiber
description: Fiber represents the fiber package where you start to create an instance.
sidebar_position: 1
---

## Server start

### New

This method creates a new **App** named instance. You can pass optional [config](#config)when creating a new instance.

```go title="Signature"
func New(config ...Config) *App
```

```go title="Example"
// Default config
app := fiber.New()

// ...
```

### Config

You can pass an optional Config when creating a new Fiber instance.

```go title="Example"
// Custom config
app := fiber.New(fiber.Config{
    CaseSensitive: true,
    StrictRouting: true,
    ServerHeader:  "Fiber",
    AppName: "Test App v1.0.1",
})

// ...
```

#### Config fields

| Property                     | Type                                                              | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Default                                                                  |
|------------------------------|-------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------|
| AppName                      | `string`                                                          | This allows to setup app name for the app                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | `""`                                                                     |
| BodyLimit                    | `int`                                                             | Sets the maximum allowed size for a request body, if the size exceeds the configured limit, it sends `413 - Request Entity Too Large` response.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | `4 * 1024 * 1024`                                                        |
| CaseSensitive                | `bool`                                                            | When enabled, `/Foo` and `/foo` are different routes. When disabled, `/Foo`and `/foo` are treated the same.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | `false`                                                                  |
| ColorScheme                  | [`Colors`](https://github.com/gofiber/fiber/blob/master/color.go) | You can define custom color scheme. They'll be used for startup message, route list and some middlewares.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | [`DefaultColors`](https://github.com/gofiber/fiber/blob/master/color.go) |
| CompressedFileSuffix         | `string`                                                          | Adds a suffix to the original file name and tries saving the resulting compressed file under the new file name.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | `".fiber.gz"`                                                            |
| Concurrency                  | `int`                                                             | Maximum number of concurrent connections.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | `256 * 1024`                                                             |
| DisableDefaultContentType    | `bool`                                                            | When set to true, causes the default Content-Type header to be excluded from the Response.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | `false`                                                                  |
| DisableDefaultDate           | `bool`                                                            | When set to true causes the default date header to be excluded from the response.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | `false`                                                                  |
| DisableHeaderNormalizing     | `bool`                                                            | By default all header names are normalized: conteNT-tYPE -&gt; Content-Type                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | `false`                                                                  |
| DisableKeepalive             | `bool`                                                            | Disable keep-alive connections, the server will close incoming connections after sending the first response to the client                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | `false`                                                                  |
| DisablePreParseMultipartForm | `bool`                                                            | Will not pre parse Multipart Form data if set to true. This option is useful for servers that desire to treat multipart form data as a binary blob, or choose when to parse the data.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | `false`                                                                  |
| EnableIPValidation           | `bool`                                                            | If set to true, `c.IP()` and `c.IPs()` will validate IP addresses before returning them. Also, `c.IP()` will return only the first valid IP rather than just the raw header value that may be a comma separated string.<br /><br />**WARNING:** There is a small performance cost to doing this validation. Keep disabled if speed is your only concern and your application is behind a trusted proxy that already validates this header.                                                                                                                                                                                                                                                                                                                                                                                     | `false`                                                                  |
| EnableSplittingOnParsers     | `bool`                                                            | EnableSplittingOnParsers splits the query/body/header parameters by comma when it's true. <br /> <br /> For example, you can use it to parse multiple values from a query parameter like this: `/api?foo=bar,baz == foo[]=bar&foo[]=baz`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | `false`                                                                  |
| EnableTrustedProxyCheck      | `bool`                                                            | When set to true, fiber will check whether proxy is trusted, using TrustedProxies list. <br /><br />By default  `c.Protocol()` will get value from X-Forwarded-Proto, X-Forwarded-Protocol, X-Forwarded-Ssl or X-Url-Scheme header, `c.IP()` will get value from `ProxyHeader` header, `c.Hostname()` will get value from X-Forwarded-Host header. <br /> If `EnableTrustedProxyCheck` is true, and `RemoteIP` is in the list of `TrustedProxies` `c.Protocol()`, `c.IP()`, and `c.Hostname()` will have the same behaviour when `EnableTrustedProxyCheck` disabled, if `RemoteIP` isn't in the list, `c.Protocol()` will return https in case when tls connection is handled by the app, or http otherwise, `c.IP()` will return RemoteIP() from fasthttp context, `c.Hostname()` will return `fasthttp.Request.URI().Host()` | `false`                                                                  |
| ErrorHandler                 | `ErrorHandler`                                                    | ErrorHandler is executed when an error is returned from fiber.Handler. Mounted fiber error handlers are retained by the top-level app and applied on prefix associated requests.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `DefaultErrorHandler`                                                    |
| GETOnly                      | `bool`                                                            | Rejects all non-GET requests if set to true. This option is useful as anti-DoS protection for servers accepting only GET requests. The request size is limited by ReadBufferSize if GETOnly is set.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | `false`                                                                  |
| IdleTimeout                  | `time.Duration`                                                   | The maximum amount of time to wait for the next request when keep-alive is enabled. If IdleTimeout is zero, the value of ReadTimeout is used.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `nil`                                                                    |
| Immutable                    | `bool`                                                            | When enabled, all values returned by context methods are immutable. By default, they are valid until you return from the handler; see issue [\#185](https://github.com/gofiber/fiber/issues/185).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | `false`                                                                  |
| JSONDecoder                  | `utils.JSONUnmarshal`                                             | Allowing for flexibility in using another json library for decoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | `json.Unmarshal`                                                         |
| JSONEncoder                  | `utils.JSONMarshal`                                               | Allowing for flexibility in using another json library for encoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | `json.Marshal`                                                           |
| PassLocalsToViews            | `bool`                                                            | PassLocalsToViews Enables passing of the locals set on a fiber.Ctx to the template engine. See our **Template Middleware** for supported engines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | `false`                                                                  |
| ProxyHeader                  | `string`                                                          | This will enable `c.IP()` to return the value of the given header key. By default `c.IP()`will return the Remote IP from the TCP connection, this property can be useful if you are behind a load balancer e.g. _X-Forwarded-\*_.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | `""`                                                                     |
| ReadBufferSize               | `int`                                                             | per-connection buffer size for requests' reading. This also limits the maximum header size. Increase this buffer if your clients send multi-KB RequestURIs and/or multi-KB headers \(for example, BIG cookies\).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `4096`                                                                   |
| ReadTimeout                  | `time.Duration`                                                   | The amount of time allowed to read the full request, including the body. The default timeout is unlimited.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | `nil`                                                                    |
| ReduceMemoryUsage            | `bool`                                                            | Aggressively reduces memory usage at the cost of higher CPU usage if set to true.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | `false`                                                                  |
| RequestMethods               | `[]string`                                                        | RequestMethods provides customizibility for HTTP methods. You can add/remove methods as you wish.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | `DefaultMethods`                                                         |
| ServerHeader                 | `string`                                                          | Enables the `Server` HTTP header with the given value.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | `""`                                                                     |
| StreamRequestBody            | `bool`                                                            | StreamRequestBody enables request body streaming, and calls the handler sooner when given body is larger than the current limit.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `false`                                                                  |
| StrictRouting                | `bool`                                                            | When enabled, the router treats `/foo` and `/foo/` as different. Otherwise, the router treats `/foo` and `/foo/` as the same.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `false`                                                                  |
| StructValidator              | `StructValidator`                                                 | If you want to validate header/form/query... automatically when to bind, you can define struct validator. Fiber doesn't have default validator, so it'll skip validator step if you don't use any validator.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `nil`                                                                    |
| TrustedProxies               | `[]string`                                                        | Contains the list of trusted proxy IP's. Look at `EnableTrustedProxyCheck` doc. <br /> <br /> It can take IP or IP range addresses.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | `nil`                                                                    |
| UnescapePath                 | `bool`                                                            | Converts all encoded characters in the route back before setting the path for the context, so that the routing can also work with URL encoded special characters                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `false`                                                                  |
| Views                        | `Views`                                                           | Views is the interface that wraps the Render function. See our **Template Middleware** for supported engines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `nil`                                                                    |
| ViewsLayout                  | `string`                                                          | Views Layout is the global layout for all template render until override on Render function. See our **Template Middleware** for supported engines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | `""`                                                                     |
| WriteBufferSize              | `int`                                                             | Per-connection buffer size for responses' writing.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | `4096`                                                                   |
| WriteTimeout                 | `time.Duration`                                                   | The maximum duration before timing out writes of the response. The default timeout is unlimited.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `nil`                                                                    |
| XMLEncoder                   | `utils.XMLMarshal`                                                | Allowing for flexibility in using another XML library for encoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | `xml.Marshal`                                                            |


## Server listening

### Config

You can pass an optional ListenConfig when calling the [`Listen`](#listen) or [`Listener`](#listener) method.

```go title="Example"
// Custom config
app.Listen(":8080", fiber.ListenConfig{
    EnablePrefork: true,
    DisableStartupMessage: true,
})
```

#### Config fields

| Property              | Type                          | Description                                                                                                                                   | Default |
|-----------------------|-------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------|---------|
| BeforeServeFunc       | `func(app *App) error`        | Allows customizing and accessing fiber app before serving the app.                                                                            | `nil`   |
| CertClientFile        | `string`                      | Path of the client certificate. If you want to use mTLS, you must enter this field.                                                           | `""`    |
| CertFile              | `string`                      | Path of the certificate file. If you want to use TLS, you must enter this field.                                                              | `""`    |
| CertKeyFile           | `string`                      | Path of the certificate's private key. If you want to use TLS, you must enter this field.                                                     | `""`    |
| DisableStartupMessage | `bool`                        | When set to true, it will not print out the «Fiber» ASCII art and listening address.                                                          | `false` |
| EnablePrefork         | `bool`                        | When set to true, this will spawn multiple Go processes listening on the same port.                                                           | `false` |
| EnablePrintRoutes     | `bool`                        | If set to true, will print all routes with their method, path, and handler.                                                                   | `false` |
| GracefulContext       | `context.Context`             | Field to shutdown Fiber by given context gracefully.                                                                                          | `nil`   |
| ListenerAddrFunc      | `func(addr net.Addr)`         | Allows accessing and customizing `net.Listener`.                                                                                              | `nil`   |
| ListenerNetwork       | `string`                      | Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only). WARNING: When prefork is set to true, only "tcp4" and "tcp6" can be chosen. | `tcp4`  |
| OnShutdownError       | `func(err error)`             | Allows to customize error behavior when gracefully shutting down the server by given signal.  Prints error with `log.Fatalf()`                | `nil`   |
| OnShutdownSuccess     | `func()`                      | Allows to customize success behavior when gracefully shutting down the server by given signal.                                                | `nil`   |
| TLSConfigFunc         | `func(tlsConfig *tls.Config)` | Allows customizing `tls.Config` as you want.                                                                                                  | `nil`   |


### Listen

Listen serves HTTP requests from the given address.

```go title="Signature"
func (app *App) Listen(addr string, config ...ListenConfig) error
```

```go title="Examples"
// Listen on port :8080 
app.Listen(":8080")

// Listen on port :8080 with Prefork 
app.Listen(":8080", fiber.ListenConfig{EnablePrefork: true})

// Custom host
app.Listen("127.0.0.1:8080")
```

#### Prefork

Prefork is a feature that allows you to spawn multiple Go processes listening on the same port. This can be useful for scaling across multiple CPU cores.

```go title="Examples"
app.Listen(":8080", fiber.ListenConfig{EnablePrefork: true})
```

This distributes the incoming connections between the spawned processes and allows more requests to be handled simultaneously.

#### TLS

TLS serves HTTPs requests from the given address using certFile and keyFile paths to as TLS certificate and key file.

```go title="Examples"
app.Listen(":443", fiber.ListenConfig{CertFile: "./cert.pem", CertKeyFile: "./cert.key"})
```

#### TLS with certificate

```go title="Examples"
app.Listen(":443", fiber.ListenConfig{CertClientFile: "./ca-chain-cert.pem"})
```

#### TLS with certFile, keyFile and clientCertFile

```go title="Examples"
app.Listen(":443", fiber.ListenConfig{CertFile: "./cert.pem", CertKeyFile: "./cert.key", CertClientFile: "./ca-chain-cert.pem"})
```

### Listener

You can pass your own [`net.Listener`](https://pkg.go.dev/net/#Listener) using the `Listener` method. This method can be used to enable **TLS/HTTPS** with a custom tls.Config.

```go title="Signature"
func (app *App) Listener(ln net.Listener, config ...ListenConfig) error
```

```go title="Examples"
ln, _ := net.Listen("tcp", ":3000")

cer, _:= tls.LoadX509KeyPair("server.crt", "server.key")

ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cer}})

app.Listener(ln)
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


## Helper functions

### NewError

NewError creates a new HTTPError instance with an optional message.

```go title="Signature"
func NewError(code int, message ...string) *Error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    return fiber.NewError(782, "Custom error message")
})
```

### IsChild

IsChild determines if the current process is a result of Prefork.

```go title="Signature"
func IsChild() bool
```

```go title="Example"
// Config app
app := fiber.New()

app.Get("/", func(c fiber.Ctx) error {
    if !fiber.IsChild() {
        fmt.Println("I'm the parent process")
    } else {
        fmt.Println("I'm a child process")
    }
    return c.SendString("Hello, World!")
})

// ...

// With prefork enabled, the parent process will spawn child processes
app.Listen(":8080", fiber.ListenConfig{EnablePrefork: true})
```
