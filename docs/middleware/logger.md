---
id: logger
---

# Logger

Logger middleware for [Fiber](https://github.com/gofiber/fiber) that logs HTTP requests and responses.

## Signatures

```go
func New(config ...Config) fiber.Handler
func RegisterTag(tag string, fn LogFunc) error
func MustRegisterTag(tag string, fn LogFunc)
```

## Examples

Import the package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
    "github.com/gofiber/fiber/v3/middleware/requestid"
)
```

:::tip
Registration order matters: only routes added after the logger are logged, so register it early.
:::

Once your Fiber app is initialized, use the middleware like this:

```go
// Initialize default config
app.Use(logger.New())

// Or extend your config for customization
// Log remote IP and port
app.Use(logger.New(logger.Config{
    Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
}))

// Logging Request ID
app.Use(requestid.New()) // Ensure requestid middleware is used before the logger
app.Use(logger.New(logger.Config{
    // requestid.New() registers ${requestid} automatically.
    Format: "${pid} ${requestid} ${status} - ${method} ${path}\n",
}))

// Changing TimeZone & TimeFormat
app.Use(logger.New(logger.Config{
    Format:     "${pid} ${status} - ${method} ${path}\n",
    TimeFormat: "02-Jan-2006",
    TimeZone:   "America/New_York",
}))

// Custom File Writer
accessLog, err := os.OpenFile("./access.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
if err != nil {
    log.Fatalf("error opening access.log file: %v", err)
}
defer accessLog.Close()
app.Use(logger.New(logger.Config{
    Stream: accessLog,
}))

// Add Custom Tags
app.Use(logger.New(logger.Config{
    CustomTags: map[string]logger.LogFunc{
        "custom_tag": func(output logger.Buffer, c fiber.Ctx, data *logger.Data, extraParam string) (int, error) {
            return output.WriteString("it is a custom tag")
        },
    },
}))

// Callback after log is written
app.Use(logger.New(logger.Config{
    TimeFormat: time.RFC3339Nano,
    TimeZone:   "Asia/Shanghai",
    Done: func(c fiber.Ctx, logString []byte) {
        if c.Response().StatusCode() != fiber.StatusOK {
            reporter.SendToSlack(logString)
        }
    },
}))

// Disable colors when outputting to default format
app.Use(logger.New(logger.Config{
    DisableColors: true,
}))

// Force the use of colors
app.Use(logger.New(logger.Config{
    ForceColors: true,
}))

// Use predefined formats
app.Use(logger.New(logger.Config{
    Format: logger.CommonFormat,
}))

app.Use(logger.New(logger.Config{
    Format: logger.CombinedFormat,
}))

app.Use(logger.New(logger.Config{
    Format: logger.JSONFormat,
}))

app.Use(logger.New(logger.Config{
    Format: logger.ECSFormat,
}))
```

### Logging Handler Errors

The `${error}` tag reports a non-nil error returned by a downstream handler or middleware. When no error is returned, it renders `-`. A response status alone is not an error: `return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "..."})` sets `${status}` to `500`, but `${error}` remains `-`.

To include an error in the log, return it and let Fiber's `ErrorHandler` create the response. The logger invokes the configured error handler before rendering the log, so a custom handler can return JSON while `${error}` keeps the original error message:

```go
app := fiber.New(fiber.Config{
    ErrorHandler: func(c fiber.Ctx, err error) error {
        code := fiber.StatusInternalServerError
        if fiberErr, ok := err.(*fiber.Error); ok {
            code = fiberErr.Code
        }
        return c.Status(code).JSON(fiber.Map{"error": "request failed"})
    },
})

app.Use(logger.New(logger.Config{
    Format: "${status} ${method} ${path} ${error}\n",
}))

app.Get("/reports", func(_ fiber.Ctx) error {
    return fiber.NewError(fiber.StatusInternalServerError, "report generation failed")
})
```

Register the logger before routes and downstream middleware whose returned errors it should observe. Use `${status}` when code writes a complete response directly; status codes by themselves do not populate `${error}`.

### Auto-Registered Tags

Some Fiber middleware registers logger middleware tags automatically. Register the producing middleware before `logger.New()` and then use the tag in `Format`.

```go
app.Use(requestid.New())
app.Use(logger.New(logger.Config{
    Format: "${requestid} ${status} ${method} ${path}\n",
}))
```

The logger middleware resolves tags in this order:

1. Built-in logger tags, such as `${method}`, `${path}`, and `${status}`.
2. Globally registered tags from Fiber middleware or `logger.RegisterTag`.
3. `Config.CustomTags`, which override tags with the same name for that logger instance.

The following tags are registered by Fiber middleware when the middleware is initialized:

| Tag | Registered by | Value |
| :-- | :------------ | :---- |
| `${requestid}` / `${request-id}` | `requestid.New()` | Request ID stored by the requestid middleware. |
| `${username}` | `basicauth.New()` | Authenticated username stored by the basicauth middleware. |
| `${api-key}` | `keyauth.New()` | Redacted API key stored by the keyauth middleware. |
| `${csrf-token}` | `csrf.New()` | Redacted marker when the csrf middleware stores a token. |
| `${session-id}` | `session.New()` or `session.NewWithStore()` | Redacted session ID stored by the session middleware. |

:::note
Auto-registered tags are access-log tags for `middleware/logger`. The same names are also registered for application logs in the `log` package via `logger.RegisterContextTag` — see [api/log#context-tags](../api/log.md#context-tags) for details on `log.WithContext` enrichment.
:::

### Register Tags from Custom Middleware

Third-party middleware can expose logger tags with `logger.RegisterTag` or `logger.MustRegisterTag`. Use `sync.Once` so the tag is registered once even when the middleware is initialized multiple times.

A tag you register replaces the built-in renderer, so it does not inherit the [control-character scrubbing](#control-character-sanitization) the built-in tags apply. Wrap request-derived values in `logger.SanitizeValue`.

```go
package tenantmw

import (
    "sync"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
)

type tenantContextKey struct{}

var tenantKey tenantContextKey

var registerLoggerTagsOnce sync.Once

func New() fiber.Handler {
    registerLoggerTagsOnce.Do(func() {
        logger.MustRegisterTag("tenant", func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
            tenant, _ := fiber.ValueFromContext[string](c, tenantKey)
            return output.WriteString(tenant)
        })
    })

    return func(c fiber.Ctx) error {
        fiber.StoreInContext(c, tenantKey, "acme")
        return c.Next()
    }
}
```

Use the registered tag in the logger format after installing the middleware:

```go
app.Use(tenantmw.New())
app.Use(logger.New(logger.Config{
    Format: "${tenant} ${status} ${method} ${path}\n",
}))
```

Use `Config.CustomTags` when one logger instance needs a local override without changing the global tag registration. These also bypass the built-in [control-character scrubbing](#control-character-sanitization) — wrap request-derived values in `logger.SanitizeValue`:

```go
app.Use(logger.New(logger.Config{
    Format: "${tenant} ${status} ${method} ${path}\n",
    CustomTags: map[string]logger.LogFunc{
        "tenant": func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
            return output.WriteString("override")
        },
    },
}))
```

### Use Logger Middleware with Other Loggers

To combine the logger middleware with loggers like Zerolog, Zap, or Logrus, use the `LoggerToWriter` helper to adapt them to an `io.Writer`.

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

    // Use the logger middleware with the zap logger
    app.Use(logger.New(logger.Config{
        Stream: logger.LoggerToWriter(zap, log.LevelDebug),
    }))

    // Define a route
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    // Start server on http://localhost:3000
    app.Listen(":3000")
}
```

:::tip
Writing to `os.File` is goroutine-safe, but custom streams may require locking to serialize writes.
:::

## Config

| Property      | Type                                              | Description                                                                                                                                   | Default                                                               |
| :------------ | :------------------------------------------------ | :-------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------- |
| Next          | `func(fiber.Ctx) bool`                            | Next defines a function to skip this middleware when it returns true.                                                                           | `nil`                                                                 |
| Skip          | `func(fiber.Ctx) bool`                            | Skip is a function to determine if logging is skipped or written to Stream.                                                                   | `nil`                                                                 |
| Done          | `func(fiber.Ctx, []byte)`                         | Done is a function that is called after the log string for a request is written to Stream, and pass the log string as parameter.              | `nil`                                                                 |
| CustomTags    | `map[string]LogFunc`                              | Defines custom tag actions for this logger instance. These tags override built-in and globally registered tags with the same name.             | `nil`                                                                 |
| `Format`   | `string`  | Defines the logging tags. See more in [Predefined Formats](#predefined-formats), or create your own using [Tags](#constants). | `[${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}\n` (same as `DefaultFormat`) |
| TimeFormat    | `string`                                          | TimeFormat defines the time format for log timestamps.                                                                                        | `15:04:05`                                                            |
| TimeZone      | `string`                                          | TimeZone can be specified, such as "UTC" and "America/New_York" and "Asia/Chongqing", etc                                                     | `"Local"`                                                             |
| TimeInterval  | `time.Duration`                                   | TimeInterval is the delay before the timestamp is updated.                                                                                    | `500 * time.Millisecond`                                              |
| Stream        | `io.Writer`                                       | Stream is a writer where logs are written.                                                                                                    | `os.Stdout`                                                           |
| TimeDone      | `<-chan struct{}`                                 | TimeDone stops the background timestamp updater when it is closed.                                                                            | `nil`                                                                 |
| BeforeHandlerFunc | `func(*Config)`                               | BeforeHandlerFunc runs once before the handler is built, letting you customize colors, template, etc.                                         | `beforeHandlerFunc`                                                   |
| LoggerFunc    | `func(c fiber.Ctx, data *Data, cfg *Config) error` | Custom logger function for integration with logging libraries (Zerolog, Zap, Logrus, etc). Defaults to Fiber's default logger if not defined. | `see default_logger.go defaultLoggerInstance`                         |
| DisableColors | `bool`                                            | DisableColors defines if the logs output should be colorized.                                                                                 | `false`                                                               |
| ForceColors   | `bool`                                            | ForceColors defines if the logs output should be colorized even when the output is not a terminal.                                             | `false`                                                               |

## Default Config

```go
var ConfigDefault = Config{
    Next:              nil,
    Skip:              nil,
    Done:              nil,
    Format:            DefaultFormat,
    TimeFormat:        "15:04:05",
    TimeZone:          "Local",
    TimeInterval:      500 * time.Millisecond,
    Stream:            os.Stdout,
    BeforeHandlerFunc: beforeHandlerFunc,
    LoggerFunc:        defaultLoggerInstance,
    enableColors:      true,
}
```

## Predefined Formats

Logger provides predefined formats that you can use by name or directly by specifying the format string.

| **Format Constant** | **Format String** | **Description** |
|---------------------|--------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| `DefaultFormat` | `"[${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}\n"` | Fiber's default logger format. |
| `CommonFormat` | `"${ip} - - [${time}] "${method} ${url} ${protocol}" ${status} ${bytesSent}\n"` | Common Log Format (CLF) used in web server logs. |
| `CombinedFormat` | `"${ip} - - [${time}] "${method} ${url} ${protocol}" ${status} ${bytesSent} "${referer}" "${ua}"\n"` | CLF format plus the `referer` and `user agent` fields. |
| `JSONFormat` | `"{\"time\":\"${time}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"url\":\"${url}\",\"status\":${status},\"bytesSent\":${bytesSent}}\n"` | JSON format for structured logging. |
| `ECSFormat` | `"{\"@timestamp\":\"${time}\",\"ecs\":{\"version\":\"1.6.0\"},\"client\":{\"ip\":\"${ip}\"},\"http\":{\"request\":{\"method\":\"${method}\",\"url\":\"${url}\",\"protocol\":\"${protocol}\"},\"response\":{\"status_code\":${status},\"body\":{\"bytes\":${bytesSent}}}},\"log\":{\"level\":\"INFO\",\"logger\":\"fiber\"},\"message\":\"${method} ${url} responded with ${status}\"}\n"` | Elastic Common Schema (ECS) format for structured logging. |

:::tip
`${bytesSent}` returns the value the `Content-Length` response header declares when the logger runs. For an ordinary buffered response fasthttp fills the header in on write, after the logger, so `0` is logged; a streaming response of unknown length (chunked encoding) logs `-1`. Likewise `${bytesReceived}` reports the request's declared `Content-Length`: `-2` for a request without a body and `-1` for a chunked one. Fiber does not calculate the actual body sizes for performance reasons.
:::

:::tip
`${resBody}` logs nothing for a streamed response (`SendStream`, `SendStreamWriter`, and `SendFile` for a large file). Reading such a body would pull the whole stream into memory and turn the response into a buffered one — an SSE route would deliver nothing until its writer finished — so logging leaves it alone.
:::

## The `${ips}` tag

`${ips}` logs the chain the framework parsed, `Ctx.IPs()`, joined with `,`.
Reading `X-Forwarded-For` here separately meant reading it a second way, and
under [`DisableHeaderNormalizing`](../api/fiber.md#config) a lower-case
`x-forwarded-for:` logged an empty chain while `Ctx.IPs()` went on returning it.

It is not the list any trust decision is made from. `Ctx.IPs()` parses
`X-Forwarded-For` unconditionally, while `Ctx.IP()` consults
[`TrustProxy`](../api/fiber.md#config) and reads
[`ProxyHeader`](../api/fiber.md#config), which need not be `X-Forwarded-For` at
all — so `${ips}` and the address Fiber acted on can name different hosts.

Because the entries are split and trimmed rather than echoed as sent, repeated
`X-Forwarded-For` header lines and a single comma-joined one log identically,
which is what [RFC 9110 §5.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-5.2)
says they are.

Treat a logged chain as attacker-controlled, trusted peer or not. A proxy you
trust appends the address it saw to whatever the client already put in
`X-Forwarded-For`, and `Ctx.IPs()` returns every element without the
right-to-left walk `Ctx.IP()` uses, so the entries to the left of the ones your
own infrastructure added are still the client's to choose. Use `${ip}`, which is
the peer address, when you need one you can rely on.

## Control-Character Sanitization

Values that come from the request are scrubbed before they reach the log stream: every ASCII control byte (C0 and DEL) is replaced with a space, and horizontal tab is preserved. Without this, a percent-decoded query parameter, form field, or request body containing `\r\n` could forge additional access-log lines and corrupt an audit trail.

Scrubbing covers the default format as well as these tags:

`${path}` `${url}` `${ua}` `${referer}` `${ip}` `${ips}` `${host}` `${scheme}` `${route}` `${body}` `${resBody}` `${reqHeaders}` `${queryParams}` `${error}` `${reqHeader:}` `${respHeader:}` `${query:}` `${form:}` `${cookie:}` `${locals:}`

Tags whose values the framework controls — `${status}`, `${method}`, `${protocol}`, `${port}`, `${latency}`, `${pid}`, `${time}`, `${bytesSent}`, `${bytesReceived}` and the color tags — are written unchanged. `${method}` and `${protocol}` come from the request line, which fasthttp rejects outright if it holds a control byte.

Only ASCII controls are replaced. Bytes at or above `0x80` pass through untouched, so C1 controls (U+0080–U+009F, including NEL U+0085, which some log pipelines treat as a line break) survive scrubbing. Handle those yourself if your values can carry them.

:::caution
Three paths bypass the built-in scrubbing, because each one replaces the renderer rather than wrapping it:

- `Config.CustomTags`
- `RegisterTag` / `MustRegisterTag`
- `Config.LoggerFunc`, which replaces the rendering pipeline wholesale

Anything request-derived that you write from one of these needs scrubbing. Use `logger.SanitizeValue`, which applies exactly what the built-in tags apply:

```go
logger.MustRegisterTag("tenant", func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
    return output.WriteString(logger.SanitizeValue(c.Get("X-Tenant-ID")))
})
```

`RegisterContextTag` is not on that list: it wraps your extractor rather than being one, so what the extractor returns is scrubbed on the way out — in both the access-log renderer and the `log` package one. Fiber's own context tags — `${username}`, `${api-key}`, `${csrf-token}`, `${requestid}`, `${session-id}` — are registered through it, and the middleware behind each one validates or redacts at the source as well.
:::

## Constants

```go
// Logger variables
const (
    TagPid               = "pid"
    TagTime              = "time"
    TagReferer           = "referer"
    TagProtocol          = "protocol"
    TagScheme            = "scheme"
    TagPort              = "port"
    TagIP                = "ip"
    TagIPs               = "ips"
    TagHost              = "host"
    TagMethod            = "method"
    TagPath              = "path"
    TagURL               = "url"
    TagUA                = "ua"
    TagLatency           = "latency"
    TagStatus            = "status"         // response status
    TagResBody           = "resBody"        // response body
    TagReqHeaders        = "reqHeaders"
    TagQueryStringParams = "queryParams"    // request query parameters
    TagBody              = "body"           // request body
    TagBytesSent         = "bytesSent"
    TagBytesReceived     = "bytesReceived"
    TagRoute             = "route"
    TagError             = "error"
    TagReqHeader         = "reqHeader:"     // request header
    TagRespHeader        = "respHeader:"    // response header
    TagQuery             = "query:"         // request query
    TagForm              = "form:"          // request form
    TagCookie            = "cookie:"        // request cookie
    TagLocals            = "locals:"
    // colors
    TagBlack             = "black"
    TagRed               = "red"
    TagGreen             = "green"
    TagYellow            = "yellow"
    TagBlue              = "blue"
    TagMagenta           = "magenta"
    TagCyan              = "cyan"
    TagWhite             = "white"
    TagReset             = "reset"
)
```
