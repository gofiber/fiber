---
id: log
title: 📃 Log
description: Fiber's built-in log package
sidebar_position: 6
---

Logs serve as an essential tool for observing program behavior, diagnosing issues, and setting up corresponding alerts. Well-structured logs can significantly enhance search efficiency and streamline the troubleshooting process.

Fiber offers a default mechanism for logging to standard output. Additionally, it provides several global functions, including `log.Info`, `log.Errorf`, `log.Warnw`, among others, to facilitate comprehensive logging capabilities.

## Log levels

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

## Custom log

Fiber provides the `AllLogger` interface for adapting the various log libraries.

```go
type CommonLogger interface {
    Logger
    FormatLogger
    WithLogger
}

type AllLogger interface {
    CommonLogger
    ControlLogger
    WithLogger
}
```

## Print log
Note: The Fatal level method will terminate the program after printing the log message. Please use it with caution.

### Basic Logging
Logs of different levels can be directly printed. These will be entered into `messageKey`, with the default key being `msg`.

```go
log.Info("Hello, World!")
log.Debug("Are you OK?")
log.Info("42 is the answer to life, the universe, and everything")
log.Warn("We are under attack!")
log.Error("Houston, we have a problem.")
log.Fatal("So Long, and Thanks for All the Fislog.")
log.Panic("The system is down.")
```

### Formatted Logging
Logs of different levels can be formatted before printing. All such methods end with an `f`.

```go
log.Debugf("Hello %s", "boy")
log.Infof("%d is the answer to life, the universe, and everything", 233)
log.Warnf("We are under attack %s!", "boss")
log.Errorf("%s, we have a problem.", "Master Shifu")
log.Fatalf("So Long, and Thanks for All the %s.", "banana")
```

### Key-Value Logging
Print a message with key-value pairs. If the key and value are not paired correctly, the log will output `KEYVALS UNPAIRED`.

```go
log.Debugw("", "Hello", "boy")
log.Infow("", "number", 233)
log.Warnw("", "job", "boss")
log.Errorw("", "name", "Master Shifu")
log.Fatalw("", "fruit", "banana")
```

## Global log
For projects that require a simple, global logging function to print messages at any time, Fiber provides a global log.

```go
import "github.com/gofiber/fiber/v3/log"

log.Info("info")
log.Warn("warn")
```

These global log functions allow you to log messages conveniently throughout your project.

The above example uses the default `log.DefaultLogger` for standard output. You can also find various pre-implemented adapters under the [contrib](https://github.com/gofiber/contrib) package such as `fiberzap` and `fiberzerolog`, or you can implement your own logger and set it as the global logger using `log.SetLogger`.This flexibility allows you to tailor the logging behavior to suit your project's needs.

Here's an example using a custom logger:

```go
import (
    "log"
    fiberlog "github.com/gofiber/fiber/v3/log"
)

var _ log.AllLogger = (*customLogger)(nil)

type customLogger struct {
    stdlog *log.Logger
}

// ...
// inject your custom logger
fiberlog.SetLogger(customLogger)
```

## Set Level
`log.SetLevel` sets the minimum level of logs that will be output. The default log level is `LevelTrace`.

Note that this method is not **concurrent-safe**.

```go
import "github.com/gofiber/fiber/v3/log"

log.SetLevel(log.LevelInfo)
```

Setting the log level allows you to control the verbosity of the logs, filtering out messages below the specified level.

## Set output

`log.SetOutput` sets the output destination of the logger. By default, the logger outputs logs to the console.

### Writing logs to stderr

```go
var logger AllLogger = &defaultLogger{
    stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
    depth:  4,
}
```

This allows you to customize where the logs are written, such as to a file, an external logging service, or any other desired destination.

### Writing logs to a file

Set the output destination to the file, in this case `test.log`:

```go
// Output to ./test.log file
f, err := os.OpenFile("test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
if err != nil {
    return
}
log.SetOutput(f)
```

### Writing logs to both console and file

The following example will write the logs to both `test.log` and `stdout`:

```go
// Output to ./test.log file
file, _ := os.OpenFile("test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
iw := io.MultiWriter(os.Stdout, file)
log.SetOutput(iw)
```

## Bind context

To bind a logger to a specific context, use the following method. This will return a `CommonLogger` instance that is bound to the specified context.

```go
commonLogger := log.WithContext(ctx)
commonLogger.Info("info")
```

Binding the logger to a context allows you to include context-specific information in your logs, improving traceability and debugging.
