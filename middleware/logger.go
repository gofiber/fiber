package middleware

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasttemplate"
)

// Config ...
type LoggerCfg struct {
	// Filter defines a function to skip middleware.
	// Optional. Default: nil
	Filter func(*fiber.Ctx) bool
	// Format defines the logging format with defined variables
	// Optional. Default: "${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n"
	// Possible values:
	// time, ip, ips, url, host, method, path, protocol, route
	// referer, ua, latency, status, body, error, bytesSent, bytesReceived
	// header:<key>, query:<key>, form:<key>, cookie:<key>
	Format string
	// TimeFormat https://programming.guide/go/format-parse-string-time-date-example.html
	// Optional. Default: 15:04:05
	TimeFormat string
	// Output is a writter where logs are written
	// Default: os.Stderr
	Output io.Writer
}

// Recover will recover from panics and calls the ErrorHandler
func Logger(config ...LoggerCfg) fiber.Handler {
	// Init config
	var cfg LoggerCfg
	// Set config if provided
	if len(config) > 0 {
		cfg = config[0]
	}
	// Set config default values
	if cfg.Format == "" {
		cfg.Format = "${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n"
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = "15:04:05"
	}
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}
	var mutex sync.RWMutex
	// Middleware settings
	tmpl := fasttemplate.New(cfg.Format, "${", "}")
	timestamp := time.Now().Format(cfg.TimeFormat)
	// Update date/time every second in a seperate go routine
	if strings.Contains(cfg.Format, "${time}") {
		go func() {
			for {
				mutex.Lock()
				timestamp = time.Now().Format(cfg.TimeFormat)
				mutex.Unlock()
				time.Sleep(250 * time.Millisecond)
			}
		}()
	}
	// Middleware function
	return func(ctx *fiber.Ctx) {
		// Filter request to skip middleware
		if cfg.Filter != nil && cfg.Filter(ctx) {
			ctx.Next()
			return
		}
		start := time.Now()
		// handle request
		ctx.Next()
		// build log
		stop := time.Now()
		// Get new buffer
		buf := bytebufferpool.Get()
		_, err := tmpl.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
			switch tag {
			case logTime:
				mutex.RLock()
				defer mutex.RUnlock()
				return buf.WriteString(timestamp)
			case logReferer:
				return buf.WriteString(ctx.Get(fiber.HeaderReferer))
			case logProtocol:
				return buf.WriteString(ctx.Protocol())
			case logIp:
				return buf.WriteString(ctx.IP())
			case logIps:
				return buf.WriteString(ctx.Get(fiber.HeaderXForwardedFor))
			case logHost:
				return buf.WriteString(ctx.Hostname())
			case logMethod:
				return buf.WriteString(ctx.Method())
			case logPath:
				return buf.WriteString(ctx.Path())
			case logUrl:
				return buf.WriteString(ctx.OriginalURL())
			case logUa:
				return buf.WriteString(ctx.Get(fiber.HeaderUserAgent))
			case logLatency:
				return buf.WriteString(stop.Sub(start).String())
			case logStatus:
				return buf.WriteString(strconv.Itoa(ctx.Fasthttp.Response.StatusCode()))
			case logBody:
				return buf.WriteString(ctx.Body())
			case logBytesReceived:
				return buf.WriteString(strconv.Itoa(len(ctx.Fasthttp.Request.Body())))
			case logBytesSent:
				return buf.WriteString(strconv.Itoa(len(ctx.Fasthttp.Response.Body())))
			case logRoute:
				return buf.WriteString(ctx.Route().Path)
			case logError:
				return buf.WriteString(ctx.Error().Error())
			default:
				switch {
				case strings.HasPrefix(tag, logHeader):
					return buf.WriteString(ctx.Get(tag[7:]))
				case strings.HasPrefix(tag, logQuery):
					return buf.WriteString(ctx.Query(tag[6:]))
				case strings.HasPrefix(tag, logForm):
					return buf.WriteString(ctx.FormValue(tag[5:]))
				case strings.HasPrefix(tag, logCookie):
					return buf.WriteString(ctx.Cookies(tag[7:]))
				}
			}
			return 0, nil
		})
		if err != nil {
			_, _ = buf.WriteString(err.Error())
		}
		if _, err := cfg.Output.Write(buf.Bytes()); err != nil {
			fmt.Println(err)
		}
		bytebufferpool.Put(buf)
	}
}

// Filter variables
const (
	logTime          = "time"
	logReferer       = "referer"
	logProtocol      = "protocol"
	logIp            = "ip"
	logIps           = "ips"
	logHost          = "host"
	logMethod        = "method"
	logPath          = "path"
	logUrl           = "url"
	logUa            = "ua"
	logLatency       = "latency"
	logStatus        = "status"
	logBody          = "body"
	logBytesSent     = "bytesSent"
	logBytesReceived = "bytesReceived"
	logRoute         = "route"
	logError         = "error"
	logHeader        = "header:"
	logQuery         = "query:"
	logForm          = "form:"
	logCookie        = "cookie:"
)
