---
id: log
title: 📃 Log
description: Fiber's built-in log package
sidebar_position: 6
---

Logs help you observe program behavior, diagnose issues, and trigger alerts. Structured logs improve searchability and speed up troubleshooting.

Fiber logs to standard error by default and exposes global helpers such as `log.Info`, `log.Errorf`, and `log.Warnw`.

## Log Levels

```go
const (
    LevelTrace Level = iota
    LevelDebug
    LevelInfo
    LevelWarn
    LevelError
    LevelFatal
    LevelPanic
)
```

## Custom Log

Fiber provides the generic `AllLogger[T]` interface for adapting various log libraries. A logger passed to `log.SetLogger` must implement every method of these interfaces.

```go
type Logger interface {
    Trace(v ...any)
    Debug(v ...any)
    Info(v ...any)
    Warn(v ...any)
    Error(v ...any)
    Fatal(v ...any)
    Panic(v ...any)
}

type FormatLogger interface {
    Tracef(format string, v ...any)
    Debugf(format string, v ...any)
    Infof(format string, v ...any)
    Warnf(format string, v ...any)
    Errorf(format string, v ...any)
    Fatalf(format string, v ...any)
    Panicf(format string, v ...any)
}

type WithLogger interface {
    Tracew(msg string, keysAndValues ...any)
    Debugw(msg string, keysAndValues ...any)
    Infow(msg string, keysAndValues ...any)
    Warnw(msg string, keysAndValues ...any)
    Errorw(msg string, keysAndValues ...any)
    Fatalw(msg string, keysAndValues ...any)
    Panicw(msg string, keysAndValues ...any)
}

type CommonLogger interface {
    Logger
    FormatLogger
    WithLogger
}

type ConfigurableLogger[T any] interface {
    // SetLevel sets logging level.
    SetLevel(level Level)

    // SetOutput sets the logger output.
    SetOutput(w io.Writer)

    // Logger returns the logger instance.
    Logger() T
}

type AllLogger[T any] interface {
    CommonLogger
    ConfigurableLogger[T]

    // WithContext returns a new logger with the given context.
    WithContext(ctx any) CommonLogger
}
```

## Print Log

**Note:** The Fatal level methods terminate the program after printing the log message, and they do so even when `SetLevel` filters the message out. The Panic level methods print the message and then panic. Please use them with caution.

### Basic Logging

Call level-specific methods directly. The default logger writes the standard `log` header (date, time with microseconds, and `file:line`), then the level and the message, for example `2026/10/12 14:30:44.616405 main.go:46: [Info] Hello, World!`.

```go
log.Trace("Entering the handler.")
log.Info("Hello, World!")
log.Debug("Are you OK?")
log.Info("42 is the answer to life, the universe, and everything")
log.Warn("We are under attack!")
log.Error("Houston, we have a problem.")
log.Fatal("So Long, and Thanks for All the Fish.")
log.Panic("The system is down.")
```

### Formatted Logging

Append `f` to format the message.

```go
log.Tracef("Entering %s", "the handler")
log.Debugf("Hello %s", "boy")
log.Infof("%d is the answer to life, the universe, and everything", 42)
log.Warnf("We are under attack, %s!", "boss")
log.Errorf("%s, we have a problem.", "John Smith")
log.Fatalf("So Long, and Thanks for All the %s.", "fish")
log.Panicf("The system is %s.", "down")
```

### Key-Value Logging

Key-value helpers log structured fields after the message, as `key=value` pairs; a key without a value is logged as `key=KEYVALS UNPAIRED`.

```go
log.Tracew("", "step", "init")
log.Debugw("", "greeting", "Hello", "target", "boy")
log.Infow("", "number", 42)
log.Warnw("", "job", "boss")
log.Errorw("", "name", "John Smith")
log.Fatalw("", "fruit", "fish")
log.Panicw("", "system", "down")
```

## Global Log

Fiber also exposes a global logger for quick messages.

```go
import "github.com/gofiber/fiber/v3/log"

log.Info("info")
log.Warn("warn")
```

The example uses `log.DefaultLogger`, which writes to stderr. The [contrib](https://github.com/gofiber/contrib) repo offers adapters like `fiberzap` and `fiberzerolog`, or you can register your own with `log.SetLogger`. `SetLogger` is not concurrent safe, so call it once at startup, before anything else logs.

Here's an example using a custom logger:

```go
import (
    "log"
    "os"

    fiberlog "github.com/gofiber/fiber/v3/log"
)

var _ fiberlog.AllLogger[*log.Logger] = (*customLogger)(nil)

type customLogger struct {
    stdlog *log.Logger
}

// Implement required methods for the AllLogger interface...

// Inject your custom logger
fiberlog.SetLogger[*log.Logger](&customLogger{
    stdlog: log.New(os.Stdout, "CUSTOM ", log.LstdFlags),
})

// Retrieve the underlying *log.Logger for direct use
std := fiberlog.DefaultLogger[*log.Logger]().Logger()
std.Println("custom logging")
```

## Set Level

`log.SetLevel` sets the minimum level that will be output. The default is `LevelTrace`.

**Note:** This method is not concurrent safe.

```go
import "github.com/gofiber/fiber/v3/log"

log.SetLevel(log.LevelInfo)
```

Setting the log level allows you to control the verbosity of the logs, filtering out messages below the specified level.

## Set Output

`log.SetOutput` sets where logs are written. By default, they go to standard error.

### Writing Logs to Stderr

`log.SetOutput(os.Stderr)` redirects the default logger to standard error.

```go
import (
    "os"

    fiberlog "github.com/gofiber/fiber/v3/log"
)

fiberlog.SetOutput(os.Stderr)
```

This lets you route logs to a file, service, or any destination.

### Writing Logs to a File

To write to a file such as `test.log`:

```go
// Output to ./test.log file
f, err := os.OpenFile("test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
if err != nil {
    log.Fatal("Failed to open log file:", err)
}
log.SetOutput(f)
```

### Writing Logs to Both Console and File

Write to both `test.log` and `stdout`:

```go
// Output to ./test.log file
file, err := os.OpenFile("test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
if err != nil {
    log.Fatal("Failed to open log file:", err)
}
iw := io.MultiWriter(os.Stdout, file)
log.SetOutput(iw)
```

## Bind Context

Bind a logger to a context with `log.WithContext`, which returns a `CommonLogger` tied to that context.

```go
commonLogger := log.WithContext(ctx)
commonLogger.Info("info")
```

Context binding can render request-specific data for easier tracing. The default context format is `log.DefaultFormat` (the empty string), so `log.WithContext(ctx)` adds no fields until you configure a format with `log.SetContextTemplate` (or its `MustSetContextTemplate` panic-on-error variant). Passing `nil` to `log.WithContext` binds no context, so the lines it logs carry no contextual fields at all.

`SetContextTemplate` configures Fiber's built-in default logger. Custom loggers registered with `SetLogger` keep full control over their own `WithContext` behavior and should implement equivalent enrichment themselves when needed.

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/log"
    "github.com/gofiber/fiber/v3/middleware/requestid"
)

app.Use(requestid.New())

log.MustSetContextTemplate(log.ContextConfig{Format: log.RequestIDFormat})

app.Get("/", func(c fiber.Ctx) error {
    log.WithContext(c).Info("start")
    return c.SendString("Hello, World!")
})
```

The `requestid`, `basicauth`, `keyauth`, `csrf` and `session` middleware register their log context tags automatically, the first time their `New` function is called. For example, `requestid.New()` fills in `${requestid}` and `${request-id}`. Other middleware that stores request values, such as `paginate`, registers no tag. Fiber pre-registers the built-in tag names (`requestid`, `request-id`, `username`, `api-key`, `csrf-token`, and `session-id`) as renderers that write nothing, so a format that references `${requestid}` still compiles, and renders an empty string, when `requestid.New()` is not in the chain. Registering a tag also updates the active format, so it does not matter whether `SetContextTemplate` or the middleware's `New` runs first.

Use `log.WithContext(c)` inside handlers when you want tags to read values stored by Fiber middleware. Passing `c.Context()` only exposes values propagated into the standard request context, which `StoreInContext` does only when `Config.PassLocalsToContext` is enabled.

:::tip
Cross-reference: the access-log integration uses the same tag names — see [middleware/logger](../middleware/logger.md#config) for the request-time format.
:::

### Glossary

- **Format** — the string with `${...}` placeholders, e.g. `"[${requestid}] "`.
- **Template** — the compiled, reusable form of a format produced by `SetContextTemplate`.
- **Tag** — a single `${name}` (bare) or `${name:param}` (parametric) placeholder inside a format.

### Signatures

```go
// Configure the active context template (or pass log.ContextConfig{} to disable).
// It returns an error when the format cannot be compiled, and ErrContextTagReserved
// when CustomTags tries to override TagContextValue.
func SetContextTemplate(config ContextConfig) error
func MustSetContextTemplate(config ContextConfig)

// Register a tag globally. The new renderer applies to the active format and to
// subsequent SetContextTemplate calls. It returns ErrContextTagInvalid for an
// empty name or a nil function, and ErrContextTagReserved for the reserved
// TagContextValue ("value:") name. The Must variant panics with the same errors.
func RegisterContextTag(tag string, fn ContextTagFunc) error
func MustRegisterContextTag(tag string, fn ContextTagFunc)

// Bind a context to the default logger. ctx accepts fiber.Ctx,
// *fasthttp.RequestCtx, context.Context, or any value exposing
// Value(key any)/UserValue(key any).
func WithContext(ctx any) CommonLogger

// Public types referenced by the API above.
type ContextConfig struct {
    CustomTags map[string]ContextTagFunc
    Format     string
}
type ContextData struct{}
type Buffer        = logtemplate.Buffer

// ContextTagFunc has the shape
// func(output Buffer, ctx any, data *ContextData, extraParam string) (int, error)
type ContextTagFunc = logtemplate.Func[any, ContextData]

// Format constants.
const (
    DefaultFormat   = ""
    RequestIDFormat = "[${requestid}] "
    KeyValueFormat  = "request-id=${request-id} username=${username} api-key=${api-key} csrf-token=${csrf-token} session-id=${session-id} "
)

// Tag names. The first six are the tags that Fiber's middleware register.
const (
    TagRequestID       = "requestid"
    TagRequestIDDashed = "request-id"
    TagUsername        = "username"
    TagAPIKey          = "api-key"
    TagCSRFToken       = "csrf-token"
    TagSessionID       = "session-id"
    TagContextValue    = "value:"
)

var (
    // Returned by RegisterContextTag for an empty tag name or a nil function.
    ErrContextTagInvalid = errors.New("log: context tag name and function are required")
    // Returned by RegisterContextTag and SetContextTemplate for the reserved TagContextValue tag.
    ErrContextTagReserved = errors.New("log: context tag is reserved")
)
```

### Context Formats

| Format Constant | Format String | Description |
| :-- | :-- | :-- |
| `DefaultFormat` | `""` | Disables contextual fields. |
| `RequestIDFormat` | `"[${requestid}] "` | Prepends the request ID when the requestid middleware is used. |
| `KeyValueFormat` | `"request-id=${request-id} username=${username} api-key=${api-key} csrf-token=${csrf-token} session-id=${session-id} "` | Prepends common middleware context values as key/value fields. Sensitive values are redacted by the registering middleware. |

### Context Tags

| Tag | Constant | Source |
| :-- | :-- | :-- |
| `${requestid}` | `TagRequestID` | `requestid` middleware |
| `${request-id}` | `TagRequestIDDashed` | `requestid` middleware |
| `${username}` | `TagUsername` | `basicauth` middleware — written **in clear text** for audit-log use cases. Avoid this tag if your usernames are PII. |
| `${api-key}` | `TagAPIKey` | `keyauth` middleware, redacted |
| `${csrf-token}` | `TagCSRFToken` | `csrf` middleware, redacted |
| `${session-id}` | `TagSessionID` | `session` middleware, redacted |
| `${value:key}` | `TagContextValue` | Any bound value with `Value(key)` or `UserValue(key)` lookup methods. `key` is passed as a `string`, so values stored under a custom key type need a [custom tag](#custom-context-tags). |

Redacted values keep their first 4 bytes followed by `****`; a non-empty value shorter than 8 bytes is replaced by `****` entirely, and an empty value stays empty.

:::caution
`${value:KEY}` looks up arbitrary context values. CR, LF, NUL, and other ASCII control bytes (except tab) are replaced with spaces before they reach the log line, so attacker-controlled values cannot forge log lines via header smuggling. The lookup still writes the value verbatim apart from that scrub — strip or hash sensitive fields before storing them on the context if you do not want them in operator logs.
:::

### Custom Context Tags

Register custom tags with `log.RegisterContextTag`, then reference them from a format passed to `SetContextTemplate`. The built-in `${value:key}` tag is reserved for context-value lookups and cannot be overridden.

A tag function receives the buffer to write to, the value passed to `WithContext`, a `*log.ContextData` (reserved, currently empty), and the text after the colon for a parametric tag such as `${value:key}`. It returns the number of bytes written, as `output.WriteString` does.

Registering a name again replaces its renderer, and registration is global to the process. Tags passed in `ContextConfig.CustomTags` to `SetContextTemplate` are registered the same way and stay registered for later calls. `RegisterContextTag` returns `ErrContextTagInvalid` for an empty name or a `nil` function, and both `RegisterContextTag` and `SetContextTemplate` return `ErrContextTagReserved` for `value:`.

```go
type tenantContextKey struct{}

var tenantKey tenantContextKey

log.MustRegisterContextTag("tenant", func(output log.Buffer, ctx any, _ *log.ContextData, _ string) (int, error) {
    tenant, _ := fiber.ValueFromContext[string](ctx, tenantKey)
    return output.WriteString(tenant)
})

log.MustSetContextTemplate(log.ContextConfig{Format: "[${tenant}] "})

app.Use(func(c fiber.Ctx) error {
    fiber.StoreInContext(c, tenantKey, "acme")
    return c.Next()
})
```

:::note
Register tags **before** referencing them in `SetContextTemplate`. Compiling a format that references an unregistered name returns an error whose message reads `logtemplate: unknown tag: "tenant"` (or panics from `MustSetContextTemplate`). Writing `${value}` without a key fails the same way, with a hint to use `${value:PARAM}`.
:::

If a tag function returns an error while a line is logged, the line carries a `[ctx-render-error: <error>]` marker after the level, in place of the context fields, with control bytes in the error replaced by spaces, and the message is still logged.

## Logger

Use `Logger` to access the underlying logger and call its native methods:

```go
logger := fiberlog.DefaultLogger[*log.Logger]() // Get the default logger instance

stdlogger := logger.Logger() // stdlogger is *log.Logger
stdlogger.SetFlags(0) // Hide timestamp by setting flags to 0
```

`DefaultLogger` returns `nil` when the active logger does not implement `AllLogger[T]` for the requested `T`, so check the result before calling `Logger()` if you replaced the logger with `SetLogger`.
