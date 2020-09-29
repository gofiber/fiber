# Logger
Logger middleware for [Fiber](https://github.com/gofiber/fiber) that logs HTTP request/response details.

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)
- [Constants](#constants)

### Signatures
```go
func New(config ...Config) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Default middleware config
app.Use(logger.New())

// Or extend your config for customization
app.Use(logger.New(logger.Config{
	Format:     "${pid} ${status} - ${method} ${path}\n",
	TimeFormat: "02-Jan-2006",
	TimeZone:   "America/New_York",
	Output:     os.Stdout,
}))
```

### Config
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
	// Output is a writter where logs are written
	//
	// Default: os.Stderr
	Output io.Writer
}
```

### Default Config
```go
var ConfigDefault = Config{
	Next:       nil,
	Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
	TimeFormat: "15:04:05",
	TimeZone:   "Local",
	Output:     os.Stderr,
}
```

### Constants
```go
// Logger variables
const (
	TagPid           = "pid"
	TagTime          = "time"
	TagReferer       = "referer"
	TagProtocol      = "protocol"
	TagIP            = "ip"
	TagIPs           = "ips"
	TagHost          = "host"
	TagMethod        = "method"
	TagPath          = "path"
	TagURL           = "url"
	TagUA            = "ua"
	TagLatency       = "latency"
	TagStatus        = "status"
	TagBody          = "body"
	TagBytesSent     = "bytesSent"
	TagBytesReceived = "bytesReceived"
	TagRoute         = "route"
	TagError         = "error"
	TagHeader        = "header:"
	TagQuery         = "query:"
	TagForm          = "form:"
	TagCookie        = "cookie:"
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