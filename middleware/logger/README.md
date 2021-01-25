# Logger Middleware
Logger middleware for [Fiber](https://github.com/gofiber/fiber) that logs HTTP request/response details.

## Table of Contents
- [Logger Middleware](#logger-middleware)
	- [Table of Contents](#table-of-contents)
	- [Signatures](#signatures)
	- [Examples](#examples)
		- [Default Config](#default-config)
		- [Logging Request ID](#logging-request-id)
		- [Changing TimeZone & TimeFormat](#changing-timezone--timeformat)
		- [Custom File Writer](#custom-file-writer)
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
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)
```

### Default Config
```go
// Default middleware config
app.Use(logger.New())
```

### Logging Request ID
```go
app.Use(requestid.New())

​app​.​Use​(​logger​.​New​(logger.​Config​{
	// For more options, see the Config section
  Format​: "${pid} ${locals:requestid} ${status} - ${method} ${path}​\n​"​,
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

## Config
```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

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

	// Output is a writter where logs are written
	//
	// Default: os.Stderr
	Output io.Writer
}
```

## Default Config
```go
var ConfigDefault = Config{
	Next:         nil,
	Format:       "[${time}] ${status} - ${latency} ${method} ${path}\n",
	TimeFormat:   "15:04:05",
	TimeZone:     "Local",
	TimeInterval: 500 * time.Millisecond,
	Output:       os.Stderr,
}
```

## Constants
```go
// Logger variables
const (
	TagPid					= "pid"
	TagTime					= "time"
	TagReferer				= "referer"
	TagProtocol				= "protocol"
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
	TagQueryStringParams			= "queryParams"	// request query parameters
	TagBody					= "body"	// request body
	TagBytesSent				= "bytesSent"
	TagBytesReceived			= "bytesReceived"
	TagRoute				= "route"
	TagError				= "error"
	TagHeader				= "header:"     // request header
	TagQuery				= "query:"      // request query
	TagForm					= "form:"       // request form
	TagCookie				= "cookie:"     // request cookie
	TagLocals				= "locals:"

	// colors
	TagBlack         = "black"
	TagRed           = "red"
	TagGreen         = "green"
	TagYellow        = "yellow"
	TagBlue          = "blue"
	TagMagenta       = "magenta"
	TagCyan          = "cyan"
	TagWhite         = "white"
	TagReset         = "reset"
)
```
