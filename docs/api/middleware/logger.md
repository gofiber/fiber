---
id: logger
title: Logger
---

Logger middleware for [Fiber](https://github.com/gofiber/fiber) that logs HTTP request/response details.

## Signatures
```go
func New(config ...Config) fiber.Handler
```
## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
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
	Format: "${pid} ${locals:requestid} ${status} - ${method} ${path}​\n",
}))

// Changing TimeZone & TimeFormat
app.Use(logger.New(logger.Config{
	Format:     "${pid} ${status} - ${method} ${path}\n",
	TimeFormat: "02-Jan-2006",
	TimeZone:   "America/New_York",
}))

// Custom File Writer
file, err := os.OpenFile("./123.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
if err != nil {
	log.Fatalf("error opening file: %v", err)
}
defer file.Close()
app.Use(logger.New(logger.Config{
	Output: file,
}))

// Add Custom Tags
app.Use(logger.New(logger.Config{
	CustomTags: map[string]logger.LogFunc{
		"custom_tag": func(output logger.Buffer, c *fiber.Ctx, data *logger.Data, extraParam string) (int, error) {
			return output.WriteString("it is a custom tag")
		},
	},
}))

// Callback after log is written
app.Use(logger.New(logger.Config{
	TimeFormat: time.RFC3339Nano,
	TimeZone:   "Asia/Shanghai",
	Done: func(c *fiber.Ctx, logString []byte) {
		if c.Response().StatusCode() != fiber.StatusOK {
			reporter.SendToSlack(logString) 
		}
	},
}))

// Disable colors when outputting to default format
app.Use(logger.New(logger.Config{
    DisableColors: true,
}))
```

## Config
```go
// Config defines the config for middleware.
type Config struct {
    // Next defines a function to skip this middleware when returned true.
    //
    // Optional. Default: nil
    Next func(c *fiber.Ctx) bool
    
    // Done is a function that is called after the log string for a request is written to Output,
    // and pass the log string as parameter.
    //
    // Optional. Default: nil
    Done func(c *fiber.Ctx, logString []byte)
    
    // tagFunctions defines the custom tag action
    //
    // Optional. Default: map[string]LogFunc
    CustomTags map[string]LogFunc
    
    // Format defines the logging tags
    //
    // Optional. Default: [${time}] ${status} - ${latency} ${method} ${path}\n
    Format string
    
    // TimeFormat https://programming.guide/go/format-parse-string-time-date-example.html
    //
    // Optional. Default: 15:04:05
    TimeFormat string
    
    // TimeZone can be specified, such as "UTC" and "America/New_York" and "Asia/Chongqing", etc
    //
    // Optional. Default: "Local"
    TimeZone string
    
    // TimeInterval is the delay before the timestamp is updated
    //
    // Optional. Default: 500 * time.Millisecond
    TimeInterval time.Duration
    
    // Output is a writer where logs are written
    //
    // Default: os.Stdout
    Output io.Writer
    
    // DisableColors defines if the logs output should be colorized
    //
    // Default: false
    DisableColors bool
    
    enableColors     bool
    enableLatency    bool
    timeZoneLocation *time.Location
}
type LogFunc func(buf logger.Buffer, c *fiber.Ctx, data *logger.Data, extraParam string) (int, error)
```
## Default Config
```go
var ConfigDefault = Config{
	Next:         nil,
	Done:         nil,
	Format:       "[${time}] ${status} - ${latency} ${method} ${path}\n",
	TimeFormat:   "15:04:05",
	TimeZone:     "Local",
	TimeInterval: 500 * time.Millisecond,
	Output:       os.Stdout,
    DisableColors: true,
}
```

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
