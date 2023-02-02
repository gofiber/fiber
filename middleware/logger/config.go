package logger

import (
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

	enableColors     bool
	enableLatency    bool
	timeZoneLocation *time.Location
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

type LogFunc func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error)

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	Done:         nil,
	Format:       "[${time}] ${status} - ${latency} ${method} ${path}\n",
	TimeFormat:   "15:04:05",
	TimeZone:     "Local",
	TimeInterval: 500 * time.Millisecond,
	Output:       os.Stdout,
	enableColors: true,
}

// Function to check if the logger format is compatible for coloring
func checkColorEnable(format string) bool {
	validTemplates := []string{"${status}", "${method}"}
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

	// Enable colors if no custom format or output is given
	if cfg.Output == ConfigDefault.Output && checkColorEnable(cfg.Format) {
		cfg.enableColors = true
	}

	return cfg
}
