package logger

import (
	"io"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Stream is a writer where logs are written
	//
	// Default: os.Stdout
	Stream io.Writer

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Skip is a function to determine if logging is skipped or written to Stream.
	//
	// Optional. Default: nil
	Skip func(c fiber.Ctx) bool

	// Done is a function that is called after the log string for a request is written to Output,
	// and pass the log string as parameter.
	//
	// Optional. Default: nil
	Done func(c fiber.Ctx, logString []byte)

	// tagFunctions defines the custom tag action
	//
	// Optional. Default: map[string]LogFunc
	CustomTags map[string]LogFunc

	// You can define specific things before the returning the handler: colors, template, etc.
	//
	// Optional. Default: beforeHandlerFunc
	BeforeHandlerFunc func(Config)

	// You can use custom loggers with Fiber by using this field.
	// This field is really useful if you're using Zerolog, Zap, Logrus, apex/log etc.
	// If you don't define anything for this field, it'll use default logger of Fiber.
	//
	// Optional. Default: defaultLogger
	LoggerFunc func(c fiber.Ctx, data *Data, cfg Config) error

	timeZoneLocation *time.Location

	// Format defines the logging format for the middleware.
	//
	// You can customize the log output by defining a format string with placeholders
	// such as: ${time}, ${ip}, ${status}, ${method}, ${path}, ${latency}, ${error}, etc.
	// The full list of available placeholders can be found in 'tags.go' or at
	// 'https://docs.gofiber.io/api/middleware/logger/#constants'.
	//
	// Fiber provides predefined logging formats that can be used directly:
	//
	//   - DefaultFormat    → Uses the default log format: "[${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}"
	//   - CommonFormat     → Uses the Apache Common Log Format (CLF): "${ip} - - [${time}] \"${method} ${url} ${protocol}\" ${status} ${bytesSent}\n"
	//   - CombinedFormat   → Uses the Apache Combined Log Format: "${ip} - - [${time}] \"${method} ${url} ${protocol}\" ${status} ${bytesSent} \"${referer}\" \"${ua}\"\n"
	//   - JSONFormat      → Uses the JSON log format: "{\"time\":\"${time}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"url\":\"${url}\",\"status\":${status},\"bytesSent\":${bytesSent}}\n"
	//   - ECSFormat        → Uses the Elastic Common Schema (ECS) log format: {\"@timestamp\":\"${time}\",\"ecs\":{\"version\":\"1.6.0\"},\"client\":{\"ip\":\"${ip}\"},\"http\":{\"request\":{\"method\":\"${method}\",\"url\":\"${url}\",\"protocol\":\"${protocol}\"},\"response\":{\"status_code\":${status},\"body\":{\"bytes\":${bytesSent}}}},\"log\":{\"level\":\"INFO\",\"logger\":\"fiber\"},\"message\":\"${method} ${url} responded with ${status}\"}"
	// If both `Format` and `CustomFormat` are provided, the `CustomFormat` will be used, and the `Format` field will be ignored.
	// If no format is specified, the default format is used:
	// "[${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}"
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

	// DisableColors defines if the logs output should be colorized
	//
	// Default: false
	DisableColors bool

	enableColors  bool
	enableLatency bool
}

const (
	startTag       = "${"
	endTag         = "}"
	paramSeparator = ":"
)

type Buffer interface {
	Len() int
	ReadFrom(r io.Reader) (int64, error)
	WriteTo(w io.Writer) (int64, error)
	Bytes() []byte
	Write(p []byte) (int, error)
	WriteByte(c byte) error
	WriteString(s string) (int, error)
	Set(p []byte)
	SetString(s string)
	String() string
}

type LogFunc func(output Buffer, c fiber.Ctx, data *Data, extraParam string) (int, error)

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:              nil,
	Skip:              nil,
	Done:              nil,
	Format:            DefaultFormat,
	TimeFormat:        "15:04:05",
	TimeZone:          "Local",
	TimeInterval:      500 * time.Millisecond,
	Stream:            os.Stdout,
	BeforeHandlerFunc: beforeHandlerFunc,
	LoggerFunc:        defaultLoggerInstance,
	enableColors:      true,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Skip == nil {
		cfg.Skip = ConfigDefault.Skip
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
	if cfg.Stream == nil {
		cfg.Stream = ConfigDefault.Stream
	}

	if cfg.BeforeHandlerFunc == nil {
		cfg.BeforeHandlerFunc = ConfigDefault.BeforeHandlerFunc
	}

	if cfg.LoggerFunc == nil {
		cfg.LoggerFunc = ConfigDefault.LoggerFunc
	}

	// Enable colors if no custom format or output is given
	if !cfg.DisableColors && cfg.Stream == ConfigDefault.Stream {
		cfg.enableColors = true
	}

	return cfg
}
