# Logger Middleware
Logger middleware for [Fiber](https://github.com/gofiber/fiber) that logs HTTP request/response details.

## Table of Contents
- [Logger Middleware](#logger-middleware)
	- [Table of Contents](#table-of-contents)
	- [Signatures](#signatures)
	- [Examples](#examples)
		- [Default Config](#default-config)
		- [Logging remote IP and Port](#logging-remote-ip-and-port)
		- [Logging Request ID](#logging-request-id)
		- [Changing TimeZone & TimeFormat](#changing-timezone--timeformat)
		- [Custom File Writer](#custom-file-writer)
		- [Logging with Zerolog](#logging-with-zerolog)
        - [Add Custom Tags](#add-custom-tags)
	- [Config](#config)
	- [Default Config](#default-config-1)
	- [Constants](#constants)

## Signatures
```go
func New(config ...Config) fiber.Handler
```

## Examples
First ensure the appropriate packages are imported
```go
import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)
```

### Default Config
```go
// Default middleware config
app.Use(logger.New())
```

### Logging remote IP and Port

```go
app.Use(logger.New(logger.Config{
	Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
}))
```

### Logging Request ID
```go
app.Use(requestid.New())

app.Use(logger.New(logger.Config{
	// For more options, see the Config section
	Format: "${pid} ${locals:requestid} ${status} - ${method} ${path}​\n",
}))
```

### Changing TimeZone & TimeFormat

```go
app.Use(logger.New(logger.Config{
	Format:     "${pid} ${status} - ${method} ${path}\n",
	TimeFormat: "02-Jan-2006",
	TimeZone:   "America/New_York",
}))
```

### Custom File Writer
```go
file, err := os.OpenFile("./123.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
if err != nil {
	log.Fatalf("error opening file: %v", err)
}
defer file.Close()

app.Use(logger.New(logger.Config{
	Output: file,
}))
```
### Add Custom Tags
```go
app.Use(logger.New(logger.Config{
	CustomTags: map[string]logger.LogFunc{
		"custom_tag": func(output logger.Buffer, c *fiber.Ctx, data *logger.Data, extraParam string) (int, error) {
			return output.WriteString("it is a custom tag")
		},
	},
}))
```

### Callback after log is written

```go
app.Use(logger.New(logger.Config{
	TimeFormat: time.RFC3339Nano,
	TimeZone:   "Asia/Shanghai",
	Done: func(c *fiber.Ctx, logString []byte) {
		if c.Response().StatusCode() != fiber.StatusOK {
			reporter.SendToSlack(logString) 
		}
	},
}))
```

### Logging with Zerolog
```go
package main

import (
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	app := fiber.New()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	app.Use(logger.New(logger.Config{LoggerFunc: func(c fiber.Ctx, data *logger.LoggerData, cfg logger.Config) error {
		log.Info().
			Str("path", c.Path()).
			Str("method", c.Method()).
			Int("status", c.Response().
				StatusCode()).
			Msg("new request")

		return nil
	}}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("test")
	})

	app.Listen(":3000")
}
```

## Config
```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

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

	// You can define specific things before the returning the handler: colors, template, etc.
	//
	// Optional. Default: beforeHandlerFunc
	BeforeHandlerFunc func(Config)

	// You can use custom loggers with Fiber by using this field.
	// This field is really useful if you're using Zerolog, Zap, Logrus, apex/log etc.
	// If you don't define anything for this field, it'll use classical logger of Fiber.
	//
	// Optional. Default: defaultLogger
	LoggerFunc func(c fiber.Ctx, data *LoggerData, cfg Config) error
}

type LogFunc func(buf logger.Buffer, c *fiber.Ctx, data *logger.Data, extraParam string) (int, error)
```

## Default Config
```go
// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:              nil,
	Done:         	   nil,
	Format:            defaultFormat,
	TimeFormat:        "15:04:05",
	TimeZone:          "Local",
	TimeInterval:      500 * time.Millisecond,
	Output:            os.Stdout,
	BeforeHandlerFunc: beforeHandlerFunc,
	LoggerFunc:        defaultLogger,
	enableColors:      true,
}

// default logging format for Fiber's default logger
var defaultFormat = "[${time}] ${status} - ${latency} ${method} ${path}\n"
```

## Constants
```go
// Logger variables
const (
	TagPid					= "pid"
	TagTime					= "time"
	TagReferer				= "referer"
	TagProtocol				= "protocol"
	TagPort                                 = "port"
	TagIP					= "ip"
	TagIPs					= "ips"
	TagHost					= "host"
	TagMethod				= "method"
	TagPath					= "path"
	TagURL					= "url"
	TagUA					= "ua"
	TagLatency				= "latency"
	TagStatus				= "status"	// response status
	TagResBody				= "resBody"	// response body
	TagReqHeaders                           = "reqHeaders"
        TagQueryStringParams			= "queryParams"	// request query parameters
        TagBody					= "body"	// request body
	TagBytesSent				= "bytesSent"
	TagBytesReceived			= "bytesReceived"
	TagRoute				= "route"
	TagError                		= "error"
	// DEPRECATED: Use TagReqHeader instead
	TagHeader               		= "header:"     // request header
	TagReqHeader            		= "reqHeader:"  // request header
	TagRespHeader           		= "respHeader:" // response header
	TagQuery				= "query:"      // request query
	TagForm					= "form:"       // request form
	TagCookie				= "cookie:"     // request cookie
	TagLocals				= "locals:"

	// colors
	TagBlack        			= "black"
	TagRed           			= "red"
	TagGreen        			= "green"
	TagYellow        			= "yellow"
	TagBlue          			= "blue"
	TagMagenta       			= "magenta"
	TagCyan          			= "cyan"
	TagWhite         			= "white"
	TagReset         			= "reset"
)
```
