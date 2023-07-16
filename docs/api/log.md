---
id: log
title: 📃 Log
description: Fiber's built-in log package
sidebar_position: 6
---

We can use logs to observe program behavior, diagnose problems, or configure corresponding alarms.
And defining a well structured log can improve search efficiency and facilitate handling of problems.

Fiber provides a default way to print logs in the standard output. 
It also provides several global functions, such as `log.Info`, `log.Errorf`, `log.Warnw`, etc. 

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
Note: The method of calling the Fatal level will interrupt the program running after printing the log, please use it with caution. 
Directly print logs of different levels, which will be entered into messageKey, the default is msg.

```go
log.Info("Hello, World!")
log.Debug("Are you OK?")
log.Info("42 is the answer to life, the universe, and everything")
log.Warn("We are under attack!")
log.Error("Houston, we have a problem.")
log.Fatal("So Long, and Thanks for All the Fislog.")
log.Panic("The system is down.")
```
Format and print logs of different levels, all methods end with f

```go
log.Debugf("Hello %s", "boy")
log.Infof("%d is the answer to life, the universe, and everything", 233)
log.Warnf("We are under attack %s!", "boss")
log.Errorf("%s, we have a problem.", "Master Shifu")
log.Fatalf("So Long, and Thanks for All the %s.", "banana")
```

Print a message with the key and value, or `KEYVALS UNPAIRED` if the key and value are not a pair.

```go
log.Debugw("", "Hello", "boy")
log.Infow("", "number", 233)
log.Warnw("", "job", "boss")
log.Errorw("", "name", "Master Shifu")
log.Fatalw("", "fruit", "banana")
```

## Global log
If you are in a project and just want to use a simple log function that can be printed at any time in the global, we provide a global log.

```go
import "github.com/gofiber/fiber/v2/log"

log.Info("info")
log.Warn("warn")
```

The above is using the default `log.DefaultLogger` standard output. 
You can also find an already implemented adaptation under contrib, or use your own implemented Logger and use `log.SetLogger` to set the global log logger.

```go
import (
    "log"
    fiberlog "github.com/gofiber/fiber/v2/log"
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
`log.SetLevel` sets the level of logs below which logs will not be output.
The default logger is LevelTrace.

Note that this method is not **concurrent-safe**.

```go
import "github.com/gofiber/fiber/v2/log"

log.SetLevel(log.LevelInfo)
```
## Set output

`log.SetOutput` sets the output destination of the logger. The default logger types the log in the console.

```go
var logger AllLogger = &defaultLogger{
    stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
    depth:  4,
}
```

Set the output destination to the file.

```go
// Output to ./test.log file
f, err := os.OpenFile("test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
if err != nil {
    return
}
log.SetOutput(f)
```
Set the output destination to the console and file.

```go
// Output to ./test.log file
file, _ := os.OpenFile("test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
iw := io.MultiWriter(os.Stdout, file)
log.SetOutput(iw)
```
## Bind context
Set the context, using the following method will return a `CommonLogger` instance bound to the specified context
```go
commonLogger := log.WithContext(ctx)
commonLogger.Info("info")
```

