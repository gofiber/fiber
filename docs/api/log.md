---
id: log
title: 📃 Log
description: Fiber's built-in log package
sidebar_position: 6
---

Logs help you observe program behavior, diagnose issues, and trigger alerts. Structured logs improve searchability and speed up troubleshooting.

Fiber logs to standard output by default and exposes global helpers such as `log.Info`, `log.Errorf`, and `log.Warnw`.

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

Fiber provides the generic `AllLogger[T]` interface for adapting various log libraries.

```go
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

**Note:** The Fatal level method will terminate the program after printing the log message. Please use it with caution.

### Basic Logging

Call level-specific methods directly; entries use the `messageKey` (default `msg`).

```go
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
log.Debugf("Hello %s", "boy")
log.Infof("%d is the answer to life, the universe, and everything", 42)
log.Warnf("We are under attack, %s!", "boss")
log.Errorf("%s, we have a problem.", "John Smith")
log.Fatalf("So Long, and Thanks for All the %s.", "fish")
```

### Key-Value Logging

Key-value helpers log structured fields; mismatched pairs emit `KEYVALS UNPAIRED`.

```go
log.Debugw("", "greeting", "Hello", "target", "boy")
log.Infow("", "number", 42)
log.Warnw("", "job", "boss")
log.Errorw("", "name", "John Smith")
log.Fatalw("", "fruit", "fish")
```

## Global Log

Fiber also exposes a global logger for quick messages.

```go
import "github.com/gofiber/fiber/v3/log"

log.Info("info")
log.Warn("warn")
```

The example uses `log.DefaultLogger`, which writes to stdout. The [contrib](https://github.com/gofiber/contrib) repo offers adapters like `fiberzap` and `fiberzerolog`, or you can register your own with `log.SetLogger`.

Here's an example using a custom logger:

```go
import (
    "log"
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

`log.SetOutput` sets where logs are written. By default, they go to the console.

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

Context binding can render request-specific data for easier tracing. The default context format is `log.DefaultFormat` (the empty string), so `log.WithContext(ctx)` adds no fields until you configure a format with `log.SetContextTemplate` (or its `MustSetContextTemplate` panic-on-error variant).

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

Middleware that stores request values registers its log context tags automatically. For example, importing `requestid` makes `${requestid}` and `${request-id}` available; the actual value is filled in once `requestid.New()` runs in your handler stack. Until then the tag renders as an empty string, so a format that references `${requestid}` still compiles without `requestid.New()` in the chain.

Use `log.WithContext(c)` inside handlers when you want tags to read values stored by Fiber middleware. Passing `c.Context()` only exposes values propagated into the standard request context.

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
func SetContextTemplate(config ContextConfig) error
func MustSetContextTemplate(config ContextConfig)

// Register a tag globally; the new renderer becomes available to subsequent
// SetContextTemplate calls. The reserved TagContextValue ("value:") name
// cannot be registered.
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
type ContextTagFunc = logtemplate.Func[any, ContextData]
type Buffer        = logtemplate.Buffer

// Format constants.
const (
    DefaultFormat   = ""
    RequestIDFormat = "[${requestid}] "
    KeyValueFormat  = "request-id=${request-id} username=${username} api-key=${api-key} csrf-token=${csrf-token} session-id=${session-id} "
    TagContextValue = "value:"
)
```

### Context Formats

| Format Constant | Format String | Description |
| :-- | :-- | :-- |
| `DefaultFormat` | `""` | Disables contextual fields. |
| `RequestIDFormat` | `"[${requestid}] "` | Prepends the request ID when the requestid middleware is used. |
| `KeyValueFormat` | `"request-id=${request-id} username=${username} api-key=${api-key} csrf-token=${csrf-token} session-id=${session-id} "` | Prepends common middleware context values as key/value fields. Sensitive values are redacted by the registering middleware. |

### Context Tags

| Tag | Source |
| :-- | :-- |
| `${requestid}` / `${request-id}` | `requestid` middleware |
| `${username}` | `basicauth` middleware — written **in clear text** for audit-log use cases. Avoid this tag if your usernames are PII. |
| `${api-key}` | `keyauth` middleware, redacted to a 4-byte prefix |
| `${csrf-token}` | `csrf` middleware, redacted to a 4-byte prefix |
| `${session-id}` | `session` middleware, redacted to a 4-byte prefix |
| `${value:key}` | Any bound value with `Value(key)` or `UserValue(key)` lookup methods |

:::caution
`${value:KEY}` looks up arbitrary context values. CR, LF, NUL, and other ASCII control bytes (except tab) are replaced with spaces before they reach the log line, so attacker-controlled values cannot forge log lines via header smuggling. The lookup still writes the value verbatim apart from that scrub — strip or hash sensitive fields before storing them on the context if you do not want them in operator logs.
:::

### Custom Context Tags

Register custom tags with `log.RegisterContextTag`, then reference them from a format passed to `SetContextTemplate`. The built-in `${value:key}` tag is reserved for context-value lookups and cannot be overridden.

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
Register tags **before** referencing them in `SetContextTemplate`. Compiling a format that references an unregistered name returns `*logtemplate.UnknownTagError` (or panics from `MustSetContextTemplate`).
:::

## Logger

Use `Logger` to access the underlying logger and call its native methods:

```go
logger := fiberlog.DefaultLogger[*log.Logger]() // Get the default logger instance

stdlogger := logger.Logger() // stdlogger is *log.Logger
stdlogger.SetFlags(0) // Hide timestamp by setting flags to 0
```
