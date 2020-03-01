package middleware

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	".."
	"github.com/valyala/fasttemplate"
)

// LoggerConfig ...
type LoggerConfig struct {
	// Skip defines a function to skip middleware.
	// Optional. Default: nil
	Skip func(*fiber.Ctx) bool
	// Format defines the logging format with defined variables
	// Optional. Default: "${time} - ${ip} - ${method} ${path}\t${ua}\n"
	// Possible values: time, ip, url, host, method, path, protocol
	// referer, ua, header:<key>, query:<key>, formform:<key>, cookie:<key>
	Format string
	// TimeFormat https://programming.guide/go/format-parse-string-time-date-example.html
	// Optional. Default: 15:04:05
	TimeFormat string
	// Output is a writter where logs are written
	// Default: os.Stderr
	Output io.Writer
}

// LoggerConfigDefault is the defaul Logger middleware config.
var LoggerConfigDefault = LoggerConfig{
	Skip:       nil,
	Format:     "${time} - ${ip} - ${method} ${path}\t${ua}\n",
	TimeFormat: "15:04:05",
	Output:     os.Stderr,
}

// Logger ...
func Logger(config ...LoggerConfig) func(*fiber.Ctx) {
	// Init config
	var cfg LoggerConfig
	// Set config if provided
	if len(config) > 0 {
		cfg = config[0]
	}
	// Set config default values
	if cfg.Format == "" {
		cfg.Format = LoggerConfigDefault.Format
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = LoggerConfigDefault.TimeFormat
	}
	if cfg.Output == nil {
		cfg.Output = LoggerConfigDefault.Output
	}
	// Middleware settings
	tmpl := fasttemplate.New(cfg.Format, "${", "}")
	pool := &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}
	timestamp := time.Now().Format(cfg.TimeFormat)
	// Update date/time every second in a seperate go routine
	if strings.Contains(cfg.Format, "${time}") {
		go func() {
			for {
				timestamp = time.Now().Format(cfg.TimeFormat)
				time.Sleep(1 * time.Second)
			}
		}()
	}
	// Middleware function
	return func(c *fiber.Ctx) {
		// Skip middleware if Skip returns true
		if cfg.Skip != nil && cfg.Skip(c) {
			c.Next()
			return
		}
		buf := pool.Get().(*bytes.Buffer)
		buf.Reset()
		defer pool.Put(buf)
		tmpl.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
			switch tag {
			case "time":
				return buf.WriteString(timestamp)
			case "referer":
				return buf.WriteString(c.Get(fiber.HeaderReferer))
			case "protocol":
				return buf.WriteString(c.Protocol())
			case "ip":
				return buf.WriteString(c.IP())
			case "host":
				return buf.WriteString(c.Hostname())
			case "method":
				return buf.WriteString(c.Method())
			case "path":
				return buf.WriteString(c.Path())
			case "url":
				return buf.WriteString(c.OriginalURL())
			case "ua":
				return buf.WriteString(c.Get(fiber.HeaderUserAgent))
			default:
				switch {
				case strings.HasPrefix(tag, "header:"):
					return buf.WriteString(c.Get(tag[7:]))
				case strings.HasPrefix(tag, "query:"):
					return buf.WriteString(c.Query(tag[6:]))
				case strings.HasPrefix(tag, "form:"):
					return buf.WriteString(c.FormValue(tag[5:]))
				case strings.HasPrefix(tag, "cookie:"):
					return buf.WriteString(c.Cookies(tag[7:]))
				}
			}
			return 0, nil
		})
		cfg.Output.Write(buf.Bytes())
		c.Next()
	}
}
