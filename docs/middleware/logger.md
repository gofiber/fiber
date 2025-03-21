---
id: logger
---

# Logger

Logger middleware for [Fiber](https://github.com/gofiber/fiber) that logs HTTP request/response details.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
)
```

:::tip
The order of registration plays a role. Only all routes that are registered after this one will be logged.
The middleware should therefore be one of the first to be registered.
:::

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
app.Use(logger.New())

// Or extend your config for customization
// Logging remote IP and Port
app.Use(logger.New(logger.Config{
    Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
}))

// Logging Request ID
app.Use(requestid.New())
app.Use(logger.New(logger.Config{
    // For more options, see the Config section
    Format: "${pid} ${locals:requestid} ${status} - ${method} ${path}\n",
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


// Use predefined formats
app.Use(logger.New(logger.Config{
    CustomFormat: "common",
}))

app.Use(logger.New(logger.Config{
    CustomFormat: "combined", 
}))

app.Use(logger.New(logger.Config{
    CustomFormat: "json", 
}))

app.Use(logger.New(logger.Config{
    CustomFormat: "ecs",
}))


// Use predefined consts 
app.Use(logger.New(logger.Config{
    Format: logger.FormatCommon,
}))

app.Use(logger.New(logger.Config{
    Format: logger.FormatCombined,
}))

app.Use(logger.New(logger.Config{
    Format: logger.FormatJSON, 
}))

app.Use(logger.New(logger.Config{
    Format: logger.FormatECS, 
}))
```

### Use Logger Middleware with Other Loggers

In order to use Fiber logger middleware with other loggers such as zerolog, zap, logrus; you can use `LoggerToWriter` helper which converts Fiber logger to a writer, which is compatible with the middleware.

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

    // Use the logger middleware with zerolog logger
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
Writing to os.File is goroutine-safe, but if you are using a custom Stream that is not goroutine-safe, make sure to implement locking to properly serialize writes.
:::

## Config

### Config

| Property      | Type                                              | Description                                                                                                                                   | Default                                                               |
| :------------ | :------------------------------------------------ | :-------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------- |
| Next          | `func(fiber.Ctx) bool`                            | Next defines a function to skip this middleware when returned true.                                                                           | `nil`                                                                 |
| Skip          | `func(fiber.Ctx) bool`                            | Skip is a function to determine if logging is skipped or written to Stream.                                                                   | `nil`                                                                 |
| Done          | `func(fiber.Ctx, []byte)`                         | Done is a function that is called after the log string for a request is written to Stream, and pass the log string as parameter.              | `nil`                                                                 |
| CustomTags    | `map[string]LogFunc`                              | tagFunctions defines the custom tag action.                                                                                                   | `map[string]LogFunc`                                                  |
| Format        | `string`                                          | Format defines the logging tags.                                         | `[${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}\n` |
| CustomFormat  |  `string`  | Predefined format for log.                                                                                                                    | `default`, `common`, `combined`, `json`, `ecs`                                                                 |
| TimeFormat    | `string`                                          | TimeFormat defines the time format for log timestamps.                                                                                        | `15:04:05`                                                            |
| TimeZone      | `string`                                          | TimeZone can be specified, such as "UTC" and "America/New_York" and "Asia/Chongqing", etc                                                     | `"Local"`                                                             |
| TimeInterval  | `time.Duration`                                   | TimeInterval is the delay before the timestamp is updated.                                                                                    | `500 * time.Millisecond`                                              |
| Stream        | `io.Writer`                                       | Stream is a writer where logs are written.                                                                                                    | `os.Stdout`                                                           |
| LoggerFunc    | `func(c fiber.Ctx, data *Data, cfg Config) error` | Custom logger function for integration with logging libraries (Zerolog, Zap, Logrus, etc). Defaults to Fiber's default logger if not defined. | `see default_logger.go defaultLoggerInstance`                         |
| DisableColors | `bool`                                            | DisableColors defines if the logs output should be colorized.                                                                                 | `false`                                                               |

## Default Config

```go
var ConfigDefault = Config{
    Next:              nil,
    Skip:              nil,
    Done:              nil,
    Format:            FormatDefault,
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

| **Format Name** | **Format Constant** | **Format String** | **Description** |
|-------------------|---------------------|--------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| default | `FormatDefault` | `"[${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}\n"` | Fiber's default logger format. |
| common | `FormatCommonLog` | `"${ip} - - [${time}] "${method} ${url} ${protocol}" ${status} ${bytesSent}\n"` | Common Log Format (CLF) used in web server logs. |
| combined | `FormatCombined` | `"${ip} - - [${time}] "${method} ${url} ${protocol}" ${status} ${bytesSent} "${referer}" "${ua}"\n"` | CLF format plus the `referer` and `user agent` fields. |
| json | `FormatJSON` | `"{time: ${time}, ip: ${ip}, method: ${method}, url: ${url}, status: ${status}, bytesSent: ${bytesSent}}\n"` | JSON format for structured logging. |
| ecs | `FormatECS` | `"{\"@timestamp\":\"${time}\",\"ecs\":{\"version\":\"1.6.0\"},\"client\":{\"ip\":\"${ip}\"},\"http\":{\"request\":{\"method\":\"${method}\",\"url\":\"${url}\",\"protocol\":\"${protocol}\"},\"response\":{\"status_code\":${status},\"body\":{\"bytes\":${bytesSent}}}},\"log\":{\"level\":\"INFO\",\"logger\":\"fiber\"},\"message\":\"${method} ${url} responded with ${status}\"}\n"` | Elastic Common Schema (ECS) format for structured logging. |

## Constants

```go
// Logger variables
const (
    TagPid               = "pid"
    TagTime              = "time"
    TagReferer           = "referer"
    TagProtocol          = "protocol"
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
    // DEPRECATED: Use TagReqHeader instead
    TagHeader            = "header:"        // request header
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
