package logger

import (
	"fmt"
	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Done is a function that is called after the log string for a request is written to Output,
	// and pass the log string as parameter.
	//
	// Optional. Default: a function that does nothing.
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

	enableColors     bool
	enableLatency    bool
	timeZoneLocation *time.Location
	tagFunctions     map[string]LogFunc
}

type LogFunc func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error)

type TagMap struct {
	Tag         string
	TagFunction LogFunc
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	Done:         func(c *fiber.Ctx, logString []byte) {},
	Format:       "[${time}] ${status} - ${latency} ${method} ${path}\n",
	TimeFormat:   "15:04:05",
	TimeZone:     "Local",
	TimeInterval: 500 * time.Millisecond,
	Output:       os.Stdout,
	enableColors: true,
}

// Function to check if the logger format is compatible for coloring
func validCustomFormat(format string) bool {
	validTemplates := []string{"${status}", "${method}"}
	if format == "" {
		return true
	}
	for _, template := range validTemplates {
		if strings.Contains(format, template) {
			return true
		}
	}
	return false
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Enable colors if no custom format or output is given
	if validCustomFormat(cfg.Format) && cfg.Output == nil {
		cfg.enableColors = true
	}

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Done == nil {
		cfg.Done = ConfigDefault.Done
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
	if int(cfg.TimeInterval) <= 0 {
		cfg.TimeInterval = ConfigDefault.TimeInterval
	}
	if cfg.Output == nil {
		cfg.Output = ConfigDefault.Output
	}
	// Set custom tags
	cfg.tagFunctions = map[string]LogFunc{
		TagReferer: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Get(fiber.HeaderReferer))
		},
		TagProtocol: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Protocol())
		},
		TagPort: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Port())
		},
		TagIP: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.IP())
		},
		TagIPs: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Get(fiber.HeaderXForwardedFor))
		},
		TagHost: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Hostname())
		},
		TagPath: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Path())
		},
		TagURL: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.OriginalURL())
		},
		TagUA: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Get(fiber.HeaderUserAgent))
		},
		TagBody: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.Write(c.Body())
		},
		TagBytesReceived: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return appendInt(buf, len(c.Request().Body()))
		},
		TagBytesSent: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return appendInt(buf, len(c.Response().Body()))
		},
		TagRoute: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Route().Path)
		},
		TagResBody: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.Write(c.Response().Body())
		},
		TagReqHeaders: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			reqHeaders := make([]string, 0)
			for k, v := range c.GetReqHeaders() {
				reqHeaders = append(reqHeaders, k+"="+v)
			}
			return buf.Write([]byte(strings.Join(reqHeaders, "&")))
		},
		TagQueryStringParams: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Request().URI().QueryArgs().String())
		},

		TagBlack: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Black)
		},
		TagRed: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Red)
		},
		TagGreen: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Green)
		},
		TagYellow: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Yellow)
		},
		TagBlue: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Blue)
		},
		TagMagenta: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Magenta)
		},
		TagCyan: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Cyan)
		},
		TagWhite: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.White)
		},
		TagReset: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Reset)
		},
		TagError: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			if s := c.Context().UserValue("loggerChainError").(string); s != "" {
				return buf.WriteString(s)
			}
			return buf.WriteString("-")
		},
		TagReqHeader: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Get(tag[10:]))
		},
		TagHeader: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Get(tag[7:]))
		},
		TagRespHeader: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.GetRespHeader(tag[11:]))
		},
		TagQuery: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Query(tag[6:]))
		},
		TagForm: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.FormValue(tag[5:]))
		},
		TagCookie: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(c.Cookies(tag[7:]))
		},
		TagLocals: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			switch v := c.Locals(tag[7:]).(type) {
			case []byte:
				return buf.Write(v)
			case string:
				return buf.WriteString(v)
			case nil:
				return 0, nil
			default:
				return buf.WriteString(fmt.Sprintf("%v", v))
			}
		},
		TagStatus: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			colors := c.App().Config().ColorScheme
			if cfg.enableColors {
				return buf.WriteString(fmt.Sprintf("%s %3d %s", statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset))
			}
			return appendInt(buf, c.Response().StatusCode())
		},
		TagMethod: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			colors := c.App().Config().ColorScheme
			if cfg.enableColors {
				return buf.WriteString(fmt.Sprintf("%s %-7s %s", methodColor(c.Method(), colors), c.Method(), colors.Reset))
			}
			return buf.WriteString(c.Method())
		},
		TagPid: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(pid)
		},
		TagLatency: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			ctx := c.Context()
			latency := ctx.UserValue(loggerStop).(time.Time).Sub(ctx.UserValue(loggerStart).(time.Time))
			return buf.WriteString(fmt.Sprintf("%7v", latency))
		},
		TagTime: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, w io.Writer, tag string) (int, error) {
			return buf.WriteString(timestamp.Load().(string))
		},
	}
	for k, v := range cfg.CustomTags {
		cfg.tagFunctions[k] = v
	}
	return cfg
}
