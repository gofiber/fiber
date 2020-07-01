package middleware

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/bytebufferpool"
)

// Middleware types
type (
	// LoggerConfig defines the config for Logger middleware.
	LoggerConfig struct {
		// Next defines a function to skip this middleware.
		Next func(ctx *fiber.Ctx) bool

		// Format defines the logging tags
		//
		// - time
		// - ip
		// - ips
		// - url
		// - host
		// - method
		// - path
		// - protocol
		// - route
		// - referer
		// - ua
		// - latency
		// - status
		// - body
		// - error
		// - bytesSent
		// - bytesReceived
		// - header:<key>
		// - query:<key>
		// - form:<key>
		// - cookie:<key>
		//
		// Optional. Default: ${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n
		Format string

		// TimeFormat https://programming.guide/go/format-parse-string-time-date-example.html
		//
		// Optional. Default: 15:04:05
		TimeFormat string

		// Output is a writter where logs are written
		//
		// Default: os.Stderr
		Output io.Writer
	}
)

// Logger variables
const (
	LoggerTagTime          = "time"
	LoggerTagReferer       = "referer"
	LoggerTagProtocol      = "protocol"
	LoggerTagIP            = "ip"
	LoggerTagIPs           = "ips"
	LoggerTagHost          = "host"
	LoggerTagMethod        = "method"
	LoggerTagPath          = "path"
	LoggerTagURL           = "url"
	LoggerTagUA            = "ua"
	LoggerTagLatency       = "latency"
	LoggerTagStatus        = "status"
	LoggerTagBody          = "body"
	LoggerTagBytesSent     = "bytesSent"
	LoggerTagBytesReceived = "bytesReceived"
	LoggerTagRoute         = "route"
	LoggerTagError         = "error"
	LoggerTagHeader        = "header:"
	LoggerTagQuery         = "query:"
	LoggerTagForm          = "form:"
	LoggerTagCookie        = "cookie:"
)

// LoggerConfigDefault is the default config
var LoggerConfigDefault = LoggerConfig{
	Next:       nil,
	Format:     "${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n",
	TimeFormat: "15:04:05",
	Output:     os.Stderr,
}

// Logger is the default initiator allowing to pass a log format
func Logger(format ...string) fiber.Handler {
	// Create default config
	var config = LoggerConfigDefault
	// Set format if provided
	if len(format) > 0 {
		config.Format = format[0]
	}
	// Return LoggerWithConfig
	return LoggerWithConfig(config)
}

// LoggerWithConfig allows you to pass an CompressConfig
func LoggerWithConfig(config LoggerConfig) fiber.Handler {
	// Middleware settings
	var mutex sync.RWMutex

	var tmpl loggerTemplate
	tmpl.new(config.Format, "${", "}")

	timestamp := time.Now().Format(config.TimeFormat)
	// Update date/time every second in a separate go routine
	if strings.Contains(config.Format, "${time}") {
		go func() {
			for {
				mutex.Lock()
				timestamp = time.Now().Format(config.TimeFormat)
				mutex.Unlock()
				time.Sleep(250 * time.Millisecond)
			}
		}()
	}