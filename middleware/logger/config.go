package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

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
	// If you don't define anything for this field, it'll use default logger of Fiber.
	//
	// Optional. Default: defaultLogger
	LoggerFunc func(c fiber.Ctx, data LoggerData, cfg Config) error

	enableColors     bool
	enableLatency    bool
	timeZoneLocation *time.Location
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:              nil,
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

	if cfg.BeforeHandlerFunc == nil {
		cfg.BeforeHandlerFunc = ConfigDefault.BeforeHandlerFunc
	}

	if cfg.LoggerFunc == nil {
		cfg.LoggerFunc = ConfigDefault.LoggerFunc
	}

	return cfg
}
