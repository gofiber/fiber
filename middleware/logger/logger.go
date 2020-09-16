package logger

import (
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"github.com/gofiber/fiber/v2/internal/fasttemplate"
)

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

	haveLatency      bool
	timeZoneLocation *time.Location
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:       nil,
	Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
	TimeFormat: "15:04:05",
	TimeZone:   "Local",
	Output:     os.Stderr,
}

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

// Color values
const (
	cBlack   = "\u001b[90m"
	cRed     = "\u001b[91m"
	cGreen   = "\u001b[92m"
	cYellow  = "\u001b[93m"
	cBlue    = "\u001b[94m"
	cMagenta = "\u001b[95m"
	cCyan    = "\u001b[96m"
	cWhite   = "\u001b[97m"
	cReset   = "\u001b[0m"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.Next == nil {
			cfg.Next = ConfigDefault.Next
		}
		if cfg.Format == "" {
			cfg.Format = ConfigDefault.Format
		}
		if cfg.TimeZone == "" {
			cfg.TimeZone = ConfigDefault.TimeZone
		}
		if cfg.TimeFormat == "" {
			cfg.TimeFormat = ConfigDefault.TimeFormat
		}
		if cfg.Output == nil {
			cfg.Output = ConfigDefault.Output
		}
	}

	// Get timezone location
	tz, err := time.LoadLocation(cfg.TimeZone)
	if err != nil || tz == nil {
		cfg.timeZoneLocation = time.Local
	} else {
		cfg.timeZoneLocation = tz
	}

	// Check if format contains latency
	cfg.haveLatency = strings.Contains(cfg.Format, "${latency}")

	// Create template parser
	tmpl := fasttemplate.New(cfg.Format, "${", "}")

	// Create correct timeformat
	var timestamp atomic.Value
	timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))

	// Update date/time every 750 milliseconds in a separate go routine
	if strings.Contains(cfg.Format, "${time}") {
		go func() {
			for {
				time.Sleep(750 * time.Millisecond)
				timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))
			}
		}()
	}

	// Set PID once
	pid := strconv.Itoa(os.Getpid())

	// Set start and stop
	var start, stop time.Time

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Set latency start time
		if cfg.haveLatency {
			start = time.Now()
		}
		// Handle request, store err for logging
		err = c.Next()

		// Set latency stop time
		if cfg.haveLatency {
			stop = time.Now()
		}

		// Get new buffer
		buf := bytebufferpool.Get()

		// Loop over template tags to replace it with the correct value
		_, err = tmpl.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
			switch tag {
			case TagTime:
				return buf.WriteString(timestamp.Load().(string))
			case TagReferer:
				return buf.WriteString(c.Get(fiber.HeaderReferer))
			case TagProtocol:
				return buf.WriteString(c.Protocol())
			case TagPid:
				return buf.WriteString(pid)
			case TagIP:
				return buf.WriteString(c.IP())
			case TagIPs:
				return buf.WriteString(c.Get(fiber.HeaderXForwardedFor))
			case TagHost:
				return buf.WriteString(c.Hostname())
			case TagPath:
				return buf.WriteString(c.Path())
			case TagURL:
				return buf.WriteString(c.OriginalURL())
			case TagUA:
				return buf.WriteString(c.Get(fiber.HeaderUserAgent))
			case TagLatency:
				return buf.WriteString(stop.Sub(start).String())
			case TagBody:
				return buf.Write(c.Body())
			case TagBytesReceived:
				return buf.WriteString(strconv.Itoa(len(c.Request().Body())))
			case TagBytesSent:
				return buf.WriteString(strconv.Itoa(len(c.Response().Body())))
			case TagRoute:
				return buf.WriteString(c.Route().Path)
			case TagStatus:
				return buf.WriteString(strconv.Itoa(c.Response().StatusCode()))
			case TagMethod:
				return buf.WriteString(c.Method())
			case TagBlack:
				return buf.WriteString(cBlack)
			case TagRed:
				return buf.WriteString(cRed)
			case TagGreen:
				return buf.WriteString(cGreen)
			case TagYellow:
				return buf.WriteString(cYellow)
			case TagBlue:
				return buf.WriteString(cBlue)
			case TagMagenta:
				return buf.WriteString(cMagenta)
			case TagCyan:
				return buf.WriteString(cCyan)
			case TagWhite:
				return buf.WriteString(cWhite)
			case TagReset:
				return buf.WriteString(cReset)
			case TagError:
				if err != nil {
					return buf.WriteString(err.Error())
				}
				return buf.WriteString("-")
			default:
				// Check if we have a value tag i.e.: "header:x-key"
				switch {
				case strings.HasPrefix(tag, TagHeader):
					return buf.WriteString(c.Get(tag[7:]))
				case strings.HasPrefix(tag, TagQuery):
					return buf.WriteString(c.Query(tag[6:]))
				case strings.HasPrefix(tag, TagForm):
					return buf.WriteString(c.FormValue(tag[5:]))
				case strings.HasPrefix(tag, TagCookie):
					return buf.WriteString(c.Cookies(tag[7:]))
				}
			}
			return 0, nil
		})
		// Also write errors to the buffer
		if err != nil {
			_, _ = buf.WriteString(err.Error())
		}
		// Write buffer to output
		if _, err := cfg.Output.Write(buf.Bytes()); err != nil {
			// Write error to output
			if _, err := cfg.Output.Write([]byte(err.Error())); err != nil {
				// There is something wrong with the given io.Writer
				// TODO: What should we do here?
			}
		}
		// Put buffer back to pool
		bytebufferpool.Put(buf)

		return nil
	}
}
