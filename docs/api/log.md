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
    WithLogger
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

```go
var logger fiberlog.AllLogger[*log.Logger] = &defaultLogger{
    stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
    depth:  4,
}
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

Context binding adds request-specific data for easier tracing. The method accepts any value implementing `context.Context`, including `fiber.Ctx`, `*fasthttp.RequestCtx`, and standard `context.Context`.

### Automatic Context Fields

Middleware that stores values in the request context can register extractors so that `log.WithContext` automatically includes those values in every log entry. The `requestid` and `basicauth` middlewares register extractors when their `New()` constructor is called.

```go
app.Use(requestid.New())

app.Get("/", func(c fiber.Ctx) error {
    // Automatically includes request-id=<id> in the log output
    log.WithContext(c).Info("processing request")
    return c.SendString("OK")
})
```

### Custom Context Extractors

Use `log.RegisterContextExtractor` to register your own extractors. Each extractor receives the bound context and returns a field name, value, and success flag:

```go
log.RegisterContextExtractor(func(ctx context.Context) (string, any, bool) {
    if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
        return "trace-id", traceID, true
    }
    return "", nil, false
})
```

:::note
`RegisterContextExtractor` is not concurrent-safe and must be called during program initialization (e.g. in an `init` function or middleware constructor).
:::

## Logger

Use `Logger` to access the underlying logger and call its native methods:

```go
logger := fiberlog.DefaultLogger[*log.Logger]() // Get the default logger instance

stdlogger := logger.Logger() // stdlogger is *log.Logger
stdlogger.SetFlags(0) // Hide timestamp by setting flags to 0
```
