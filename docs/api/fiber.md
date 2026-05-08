---
id: fiber
title: 📦 Fiber
description: Fiber represents the fiber package where you start to create an instance.
sidebar_position: 1
---

import Reference from '@site/src/components/reference';

## Server start

### New

This method creates a new **App** named instance. You can pass optional [config](#config) when creating a new instance.

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

| Property                                                                              | Type                                                            | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Default                                                                |
|---------------------------------------------------------------------------------------|-----------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------|
| <Reference id="appname">AppName</Reference>                                           | `string`                                                        | Sets the application name used in logs and the Server header                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | `""`                                                                   |
| <Reference id="bodylimit">BodyLimit</Reference>                                       | `int`                                                           | Sets the maximum allowed size for a request body. Zero or negative values fall back to the default limit. If the size exceeds the configured limit, it sends `413 - Request Entity Too Large` response. This limit also applies when running Fiber through the adaptor middleware from `net/http`, when decoding compressed request bodies via [`Ctx.Body()`](./ctx.md#body), and when parsing multipart form data via [`Ctx.MultipartForm()`](./ctx.md#multipartform).                                                                                                                                                                                                                                                                                                                                                                                                                                                    | `4 * 1024 * 1024`                                                      |
| <Reference id="casesensitive">CaseSensitive</Reference>                               | `bool`                                                          | When enabled, `/Foo` and `/foo` are different routes. When disabled, `/Foo` and `/foo` are treated the same.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | `false`                                                                |
| <Reference id="cbordecoder">CBORDecoder</Reference>                                   | `utils.CBORUnmarshal`                                           | Allowing for flexibility in using another cbor library for decoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `binder.UnimplementedCborUnmarshal`                                   |
| <Reference id="cborencoder">CBOREncoder</Reference>                                   | `utils.CBORMarshal`                                             | Allowing for flexibility in using another cbor library for encoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `binder.UnimplementedCborMarshal`                                     |
| <Reference id="colorscheme">ColorScheme</Reference>                                   | [`Colors`](https://github.com/gofiber/fiber/blob/main/color.go) | You can define custom color scheme. They'll be used for startup message, route list and some middlewares.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | [`DefaultColors`](https://github.com/gofiber/fiber/blob/main/color.go) |
| <Reference id="compressedfilesuffixes">CompressedFileSuffixes</Reference>             | `map[string]string`                                             | Adds a suffix to the original file name and tries saving the resulting compressed file under the new file name.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | `{"gzip": ".fiber.gz", "br": ".fiber.br", "zstd": ".fiber.zst"}`       |
| <Reference id="concurrency">Concurrency</Reference>                                   | `int`                                                           | Maximum number of concurrent connections.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | `256 * 1024`                                                           |
| <Reference id="disabledefaultcontenttype">DisableDefaultContentType</Reference>       | `bool`                                                          | When true, omits the default Content-Type header from the response.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | `false`                                                                |
| <Reference id="disabledefaultdate">DisableDefaultDate</Reference>                     | `bool`                                                          | When true, omits the Date header from the response.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `false`                                                                |
| <Reference id="disableheadautoregister">DisableHeadAutoRegister</Reference>           | `bool`                          | Prevents Fiber from automatically registering `HEAD` routes for each `GET` route so you can supply custom `HEAD` handlers; manual `HEAD` routes still override the generated ones. | `false`                                                                |
| <Reference id="disableheadernormalizing">DisableHeaderNormalizing</Reference>         | `bool`                                                          | By default all header names are normalized: conteNT-tYPE -&gt; Content-Type                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | `false`                                                                |
| <Reference id="disablekeepalive">DisableKeepalive</Reference>                         | `bool`                                                          | Disables keep-alive connections so the server closes each connection after the first response.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | `false`                                                                |
| <Reference id="disablepreparsemultipartform">DisablePreParseMultipartForm</Reference> | `bool`                                                          | Will not pre parse Multipart Form data if set to true. This option is useful for servers that desire to treat multipart form data as a binary blob, or choose when to parse the data.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | `false`                                                                |
| <Reference id="enableipvalidation">EnableIPValidation</Reference>                     | `bool`                                                          | If set to true, `c.IP()` and `c.IPs()` will validate IP addresses before returning them. Also, `c.IP()` will return only the first valid IP rather than just the raw header value that may be a comma separated string.<br /><br />**WARNING:** There is a small performance cost to doing this validation. Keep disabled if speed is your only concern and your application is behind a trusted proxy that already validates this header.                                                                                                                                                                                                                                                                                                                                                                         | `false`                                                                |
| <Reference id="enablesplittingonparsers">EnableSplittingOnParsers</Reference>         | `bool`                                                          | Splits query, body, and header parameters on commas when enabled.<br /><br />For example, `/api?foo=bar,baz` becomes `foo[]=bar&foo[]=baz`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | `false`                                                                |
| <Reference id="errorhandler">ErrorHandler</Reference>                                 | `ErrorHandler`                                                  | ErrorHandler is executed when an error is returned from fiber.Handler. Mounted fiber error handlers are retained by the top-level app and applied on prefix associated requests.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `DefaultErrorHandler`                                                  |
| <Reference id="getonly">GETOnly</Reference>                                           | `bool`                                                          | Rejects all non-GET requests if set to true. This option is useful as anti-DoS protection for servers accepting only GET requests. The request size is limited by ReadBufferSize if GETOnly is set.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | `false`                                                                |
| <Reference id="idletimeout">IdleTimeout</Reference>                                   | `time.Duration`                                                 | The maximum amount of time to wait for the next request when keep-alive is enabled. If IdleTimeout is zero, the value of ReadTimeout is used.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | `0`                                                                    |
| <Reference id="immutable">Immutable</Reference>                                       | `bool`                                                          | When enabled, all values returned by context methods are immutable. By default, they are valid until you return from the handler; see issue [\#185](https://github.com/gofiber/fiber/issues/185).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `false`                                                                |
| <Reference id="jsondecoder">JSONDecoder</Reference>                                   | `utils.JSONUnmarshal`                                           | Allowing for flexibility in using another json library for decoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `json.Unmarshal`                                                       |
| <Reference id="jsonencoder">JSONEncoder</Reference>                                   | `utils.JSONMarshal`                                             | Allowing for flexibility in using another json library for encoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | `json.Marshal`                                                         |
| <Reference id="maxranges">MaxRanges</Reference>                                       | `int`                                                           | Sets the maximum number of ranges parsed from a `Range` header. Zero or negative values fall back to the default limit. If the limit is exceeded, the request is rejected with `416 - Requested Range Not Satisfiable` and `Content-Range: bytes */<size>`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | `16`                                                                   |
| <Reference id="msgpackdecoder">MsgPackDecoder</Reference>                             | `utils.MsgPackUnmarshal`                                        | Allowing for flexibility in using another msgpack library for decoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | `binder.UnimplementedMsgpackUnmarshal`                                 |
| <Reference id="msgpackencoder">MsgPackEncoder</Reference>                             | `utils.MsgPackMarshal`                                          | Allowing for flexibility in using another msgpack library for encoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | `binder.UnimplementedMsgpackMarshal`                                   |
| <Reference id="passlocalstocontext">PassLocalsToContext</Reference>                   | `bool`                                                          | Controls whether `StoreInContext` also propagates values into the request `context.Context` for Fiber-backed contexts. `StoreInContext` always writes to `c.Locals()`. `ValueFromContext` for Fiber-backed contexts always reads from `c.Locals()`. | `false`                                                                |
| <Reference id="passlocalstoviews">PassLocalsToViews</Reference>                       | `bool`                                                          | PassLocalsToViews Enables passing of the locals set on a fiber.Ctx to the template engine. See our **Template Middleware** for supported engines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `false`                                                                |
| <Reference id="proxyheader">ProxyHeader</Reference>                                   | `string`                                                        | Specifies the header name to read the client's real IP address from when behind a reverse proxy. Common values: `fiber.HeaderXForwardedFor`, `"X-Real-IP"`, `"CF-Connecting-IP"` (Cloudflare). <br /><br />**Important:** This setting **requires** `TrustProxy` to be enabled; `TrustProxyConfig` controls which proxy IPs are trusted for reading this header. Without `TrustProxy`, this setting has no effect and `c.IP()` will always return the remote IP from the TCP connection. <br /><br />**Behavior note:** `X-Forwarded-For` often contains a comma-separated chain of IP addresses. With the default `EnableIPValidation = false`, `c.IP()` will return the raw header value (the whole chain) rather than a single parsed client IP. With `EnableIPValidation = true`, `c.IP()` parses the header and returns the **first syntactically valid IP address** it finds; it does **not** walk the chain to find the first non-proxy hop. For a reliable client IP, configure your reverse proxy to overwrite or sanitize this header and/or to provide a single-IP header such as `"X-Real-IP"` or a provider-specific header like `"CF-Connecting-IP"`. <br /><br />**Security Warning:** Headers can be easily spoofed. Always configure `TrustProxyConfig` to validate the proxy IP address, otherwise malicious clients can forge headers to bypass IP-based access controls.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `""`                                                                   |
| <Reference id="readbuffersize">ReadBufferSize</Reference>                             | `int`                                                           | per-connection buffer size for requests' reading. This also limits the maximum header size. Increase this buffer if your clients send multi-KB RequestURIs and/or multi-KB headers \(for example, BIG cookies\).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `4096`                                                                 |
| <Reference id="readtimeout">ReadTimeout</Reference>                                   | `time.Duration`                                                 | The amount of time allowed to read the full request, including the body. The default timeout is unlimited.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | `0`                                                                    |
| <Reference id="reducememoryusage">ReduceMemoryUsage</Reference>                       | `bool`                                                          | Aggressively reduces memory usage at the cost of higher CPU usage if set to true.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `false`                                                                |
| <Reference id="requestmethods">RequestMethods</Reference>                             | `[]string`                                                      | RequestMethods provides customizability for HTTP methods. You can add/remove methods as you wish.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | `DefaultMethods`                                                       |
| <Reference id="serverheader">ServerHeader</Reference>                                 | `string`                                                        | Enables the `Server` HTTP header with the given value.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | `""`                                                                   |
| <Reference id="streamrequestbody">StreamRequestBody</Reference>                       | `bool`                                                          | StreamRequestBody enables request body streaming, and calls the handler sooner when given body is larger than the current limit.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `false`                                                                |
| <Reference id="strictrouting">StrictRouting</Reference>                               | `bool`                                                          | When enabled, the router treats `/foo` and `/foo/` as different. Otherwise, the router treats `/foo` and `/foo/` as the same.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | `false`                                                                |
| <Reference id="structvalidator">StructValidator</Reference>                           | `StructValidator`                                               | If you want to validate header/form/query... automatically when to bind, you can define struct validator. Fiber doesn't have default validator, so it'll skip validator step if you don't use any validator.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | `nil`                                                                  |
| <Reference id="trustproxy">TrustProxy</Reference>                                     | `bool` | Enables trust of reverse proxy headers. When enabled, Fiber will check if the request is coming from a trusted proxy (configured in `TrustProxyConfig`) before reading values from proxy headers. <br /><br />**Required for**: Using `ProxyHeader` to read client IP from headers like `X-Forwarded-For`. <br /><br />**Behavior when enabled:** If the remote IP is trusted (matches `TrustProxyConfig`), then `c.IP()` reads from `ProxyHeader` (when configured; otherwise it uses `RemoteIP()`), `c.Scheme()` first checks standard proxy scheme headers (`X-Forwarded-Proto`, `X-Forwarded-Protocol`, `X-Forwarded-Ssl`, `X-Url-Scheme`) and falls back to the actual connection scheme if none are set, and `c.Hostname()` prefers `X-Forwarded-Host` but falls back to the request Host header when the proxy header is not present. If the remote IP is NOT trusted, these methods ignore proxy headers and use the actual connection values instead. <br /><br />**Security:** This prevents header spoofing by validating the proxy's IP address. Always configure `TrustProxyConfig` when enabling this option and set `ProxyHeader` if you want `c.IP()` to use a specific header. | `false`                                                                |
| <Reference id="trustproxyconfig">TrustProxyConfig</Reference>                         | `TrustProxyConfig`                                              | Configures which proxy IP addresses or ranges to trust. Only effective when `TrustProxy` is enabled. <br /><br />**Fields:** <br />• `Proxies` - List of trusted proxy IPs or CIDR ranges (e.g., `[]string{"10.10.0.58", "192.168.0.0/24"}`) <br />• `Loopback` - Trust loopback addresses (127.0.0.0/8, ::1/128) <br />• `Private` - Trust all private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7) <br />• `LinkLocal` - Trust link-local addresses (169.254.0.0/16, fe80::/10) <br />• `UnixSocket` - Trust Unix domain socket connections <br /><br />**Example:** For an app behind Nginx at 10.10.0.58, use `TrustProxyConfig{Proxies: []string{"10.10.0.58"}}` or `TrustProxyConfig{Private: true}` if using private network IPs.                                                                                                                                                                                                                                                                                                                                                | `{}`                                                                  |
| <Reference id="unescapepath">UnescapePath</Reference>                                 | `bool`                                                          | Converts all encoded characters in the route back before setting the path for the context, so that the routing can also work with URL encoded special characters                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `false`                                                                |
| <Reference id="views">Views</Reference>                                               | `Views`                                                         | Views is the interface that wraps the Render function. See our **Template Middleware** for supported engines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | `nil`                                                                  |
| <Reference id="viewslayout">ViewsLayout</Reference>                                   | `string`                                                        | Views Layout is the global layout for all template render until override on Render function. See our **Template Middleware** for supported engines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | `""`                                                                   |
| <Reference id="writebuffersize">WriteBufferSize</Reference>                           | `int`                                                           | Per-connection buffer size for responses' writing.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | `4096`                                                                 |
| <Reference id="writetimeout">WriteTimeout</Reference>                                 | `time.Duration`                                                 | The maximum duration before timing out writes of the response. The default timeout is unlimited.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `0`                                                                    |
| <Reference id="xmldecoder">XMLDecoder</Reference>                                     | `utils.XMLUnmarshal`                                            | Allowing for flexibility in using another XML library for decoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | `xml.Unmarshal`                                                        |
| <Reference id="xmlencoder">XMLEncoder</Reference>                                     | `utils.XMLMarshal`                                              | Allowing for flexibility in using another XML library for encoding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | `xml.Marshal`                                                          |

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

| Property                                                                | Type                          | Description                                                                                                                                                                                                                                                                                                                  | Default            |
|-------------------------------------------------------------------------|-------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------|
| <Reference id="beforeservefunc">BeforeServeFunc</Reference>             | `func(app *App) error`        | Allows customizing and accessing fiber app before serving the app.                                                                                                                                                                                                                                                           | `nil`              |
| <Reference id="certclientfile">CertClientFile</Reference>               | `string`                      | Path of the client certificate. If you want to use mTLS, you must enter this field.                                                                                                                                                                                                                                          | `""`               |
| <Reference id="certfile">CertFile</Reference>                           | `string`                      | Path of the certificate file. If you want to use TLS, you must enter this field.                                                                                                                                                                                                                                             | `""`               |
| <Reference id="certkeyfile">CertKeyFile</Reference>                     | `string`                      | Path of the certificate's private key. If you want to use TLS, you must enter this field.                                                                                                                                                                                                                                    | `""`               |
| <Reference id="disablestartupmessage">DisableStartupMessage</Reference> | `bool`                        | When set to true, it will not print out the «Fiber» ASCII art and listening address.                                                                                                                                                                                                                                         | `false`            |
| <Reference id="enableprefork">EnablePrefork</Reference>                 | `bool`                        | When set to true, this will spawn multiple Go processes listening on the same port.                                                                                                                                                                                                                                          | `false`            |
| <Reference id="enableprintroutes">EnablePrintRoutes</Reference>         | `bool`                        | If set to true, will print all routes with their method, path, and handler.                                                                                                                                                                                                                                                  | `false`            |
| <Reference id="gracefulcontext">GracefulContext</Reference>             | `context.Context`             | Field to shutdown Fiber by given context gracefully.                                                                                                                                                                                                                                                                         | `nil`              |
| <Reference id="ShutdownTimeout">ShutdownTimeout</Reference>             | `time.Duration`               | Specifies the maximum duration to wait for the server to gracefully shutdown. When the timeout is reached, the graceful shutdown process is interrupted and forcibly terminated, and the `context.DeadlineExceeded` error is passed to the `OnPostShutdown` callback. Set to 0 to disable the timeout and wait indefinitely. | `10 * time.Second` |
| <Reference id="listeneraddrfunc">ListenerAddrFunc</Reference>           | `func(addr net.Addr)`         | Allows accessing and customizing `net.Listener`.                                                                                                                                                                                                                                                                             | `nil`              |
| <Reference id="listenernetwork">ListenerNetwork</Reference>             | `string`                      | Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only), "unix" (Unix Domain Sockets). WARNING: When prefork is set to true, only "tcp4" and "tcp6" can be chosen.                                                                                                                                                  | `tcp4`             |
| <Reference id="preforkrecoverthreshold">PreforkRecoverThreshold</Reference> | `int`                      | Defines the maximum number of child process restarts after crashes before the prefork master exits with an error. Only applies when prefork is enabled.                                                                                                                                                                        | `runtime.GOMAXPROCS(0) / 2` |
| <Reference id="preforklogger">PreforkLogger</Reference>                 | `PreforkLoggerInterface`      | Sets a custom logger for the prefork process manager. Only applies when prefork is enabled.                                                                                                                                                                                                                                  | Fiber logger       |
| <Reference id="unixsocketfilemode">UnixSocketFileMode</Reference>       | `os.FileMode`                 | FileMode to set for Unix Domain Socket (ListenerNetwork must be "unix")                                                                                                                                                                                                                                                      | `0770`             |
| <Reference id="tlsconfigfunc">TLSConfigFunc</Reference>                 | `func(tlsConfig *tls.Config)` | Allows customizing `tls.Config` as you want. Ignored when `TLSConfig` is set.                                                                                                                                                                                                                                                | `nil`              |
| <Reference id="tlsconfig">TLSConfig</Reference>                         | `*tls.Config`                 | Recommended base TLS configuration (cloned). Use for external certificate providers via `GetCertificate`. When set, other TLS fields are ignored.                                                                                                                                                                             | `nil`              |
| <Reference id="autocertmanager">AutoCertManager</Reference>             | `*autocert.Manager`           | Manages TLS certificates automatically using the ACME protocol. Enables integration with Let's Encrypt or other ACME-compatible providers.                                                                                                                                                                                   | `nil`              |
| <Reference id="tlsminversion">TLSMinVersion</Reference>                 | `uint16`                      | Allows customizing the TLS minimum version.                                                                                                                                                                                                                                                                                  | `tls.VersionTLS12` |

### Listen

Listen serves HTTP requests from the given address.

```go title="Signature"
func (app *App) Listen(addr string, config ...ListenConfig) error
```

```go title="Basic Listen usage"
// Listen on port :8080
app.Listen(":8080")

// Listen on port :8080 with Prefork
app.Listen(":8080", fiber.ListenConfig{EnablePrefork: true})

// Custom host
app.Listen("127.0.0.1:8080")
```

#### Prefork

Prefork is a feature that allows you to spawn multiple Go processes listening on the same port. This can be useful for scaling across multiple CPU cores.

```go title="Prefork listener"
app.Listen(":8080", fiber.ListenConfig{EnablePrefork: true})
```

Depending on the operating system, prefork can distribute incoming connections between the spawned processes and allow more requests to be handled simultaneously.

On Linux, prefork typically relies on the `SO_REUSEPORT` socket option for kernel-assisted load distribution across workers. On Windows, Fiber falls back to `SO_REUSEADDR`; this is not a functional equivalent to Linux `SO_REUSEPORT` as it lacks native load balancing and may allow other processes to bind to the same port. Operators should validate this behavior against their security and availability requirements.

##### Security Considerations

Prefork changes the port-ownership model from strict single-owner binding to an intentional multi-listener setup. In shared hosts, a local co-resident attacker with sufficient privileges may be able to race for shared binds or receive a portion of traffic, depending on platform behavior and user boundaries.

- Run prefork only within a trusted boundary (same deployment unit / same trust domain).
- Use a dedicated service account for Fiber workers; avoid broad shared-user deployments.
- Prefer container or VM isolation and avoid shared host namespaces for unrelated workloads.
- If strict single-owner port semantics are required, run Fiber without prefork.

#### TLS

Prefer `TLSConfig` for TLS configuration so you can fully control certificates and settings. When `TLSConfig` is set, Fiber ignores `CertFile`, `CertKeyFile`, `CertClientFile`, `TLSMinVersion`, `AutoCertManager`, and `TLSConfigFunc`.

TLS serves HTTPs requests from the given address using certFile and keyFile paths as TLS certificate and key file.

```go title="TLS with cert and key files"
app.Listen(":443", fiber.ListenConfig{CertFile: "./cert.pem", CertKeyFile: "./cert.key"})
```

#### TLS with client CA certificate

`CertClientFile` only configures the client CA for mTLS when using `CertFile`/`CertKeyFile`. If `TLSConfig` is set, `CertClientFile` is ignored, so configure client CAs in the provided `tls.Config` instead.

```go title="TLS with client CA certificate"
app.Listen(":443", fiber.ListenConfig{
    CertFile:       "./cert.pem",
    CertKeyFile:    "./cert.key",
    CertClientFile: "./ca-chain-cert.pem",
})
```

#### TLS AutoCert support (ACME / Let's Encrypt)

Provides automatic access to certificates management from Let's Encrypt and any other ACME-based providers.

```go title="AutoCert (ACME) configuration"
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

#### Precedence and conflicts

- `TLSConfig` is preferred and ignores `CertFile`/`CertKeyFile`, `CertClientFile`, `AutoCertManager`, `TLSMinVersion`, and `TLSConfigFunc`.
- `AutoCertManager` cannot be combined with `CertFile`/`CertKeyFile`.

#### TLS with external certificate provider

Use `TLSConfig` to supply a base `tls.Config` that can fetch certificates at runtime. `TLSConfig` is cloned and used as-is.

```go title="TLSConfig with dynamic certificate provider"
app.Listen(":443", fiber.ListenConfig{
    TLSConfig: &tls.Config{
        GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
            return myProvider.Certificate(info.ServerName)
        },
    },
})
```

#### Mutual TLS with TLSConfig

Use `TLSConfig` to configure mutual TLS by setting `ClientAuth` and `ClientCAs`. This replaces `CertClientFile` when you manage TLS configuration directly.

```go title="TLSConfig with client CA pool"
certPEM := []byte(certPEMString)
keyPEM := []byte(keyPEMString)
caPEM := []byte(caPEMString)

cert, err := tls.X509KeyPair(certPEM, keyPEM)
if err != nil {
    log.Fatal(err)
}

clientCAs := x509.NewCertPool()
if ok := clientCAs.AppendCertsFromPEM(caPEM); !ok {
    log.Fatal("failed to append client CA")
}

app.Listen(":443", fiber.ListenConfig{
    TLSConfig: &tls.Config{
        Certificates: []tls.Certificate{cert},
        ClientAuth:   tls.RequireAndVerifyClientCert,
        ClientCAs:    clientCAs,
    },
})
```

Load certificates from memory or environment variables and provide them via `TLSConfig`.

```go title="TLSConfig with in-memory certificate"
certPEM := []byte(certPEMString)
keyPEM := []byte(keyPEMString)

cert, err := tls.X509KeyPair(certPEM, keyPEM)
if err != nil {
    log.Fatal(err)
}

app.Listen(":443", fiber.ListenConfig{
    TLSConfig: &tls.Config{
        Certificates: []tls.Certificate{cert},
    },
})
```

```go title="TLSConfig with certificate from environment"
certPEM := []byte(os.Getenv("TLS_CERT_PEM"))
keyPEM := []byte(os.Getenv("TLS_KEY_PEM"))

cert, err := tls.X509KeyPair(certPEM, keyPEM)
if err != nil {
    log.Fatal(err)
}

app.Listen(":443", fiber.ListenConfig{
    TLSConfig: &tls.Config{
        Certificates: []tls.Certificate{cert},
    },
})
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

ShutdownWithContext shuts down the server including by force if the context's deadline is exceeded. Shutdown hooks will still be executed, even if an error occurs during the shutdown process, as they are deferred to ensure cleanup happens regardless of errors.

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

### NewErrorf

NewErrorf creates a new HTTPError instance with an optional formatted message.

```go title="Signature"
func NewErrorf(code int, message ...any) *Error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    return fiber.NewErrorf(782, "Custom error %s", "message")
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
