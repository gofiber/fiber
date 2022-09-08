package logger

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Logger variables
const (
	TagPid               = "pid"
	TagTime              = "time"
	TagReferer           = "referer"
	TagProtocol          = "protocol"
	TagScheme            = "scheme"
	TagPort              = "port"
	TagIP                = "ip"
	TagIPs               = "ips"
	TagHost              = "host"
	TagMethod            = "method"
	TagPath              = "path"
	TagURL               = "url"
	TagUA                = "ua"
	TagLatency           = "latency"
	TagStatus            = "status"
	TagResBody           = "resBody"
	TagReqHeaders        = "reqHeaders"
	TagQueryStringParams = "queryParams"
	TagBody              = "body"
	TagBytesSent         = "bytesSent"
	TagBytesReceived     = "bytesReceived"
	TagRoute             = "route"
	TagError             = "error"
	TagReqHeader         = "reqHeader:"
	TagRespHeader        = "respHeader:"
	TagLocals            = "locals:"
	TagQuery             = "query:"
	TagForm              = "form:"
	TagCookie            = "cookie:"
	TagBlack             = "black"
	TagRed               = "red"
	TagGreen             = "green"
	TagYellow            = "yellow"
	TagBlue              = "blue"
	TagMagenta           = "magenta"
	TagCyan              = "cyan"
	TagWhite             = "white"
	TagReset             = "reset"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Get timezone location
	tz, err := time.LoadLocation(cfg.TimeZone)
	if err != nil || tz == nil {
		cfg.timeZoneLocation = time.Local
	} else {
		cfg.timeZoneLocation = tz
	}

	// Check if format contains latency
	cfg.enableLatency = strings.Contains(cfg.Format, "${latency}")

	// Create correct timeformat
	var timestamp atomic.Value
	timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))

	// Update date/time every 750 milliseconds in a separate go routine
	if strings.Contains(cfg.Format, "${time}") {
		go func() {
			for {
				time.Sleep(cfg.TimeInterval)
				timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))
			}
		}()
	}

	// Set PID once
	pid := strconv.Itoa(os.Getpid())

	// Set variables
	var (
		mu         sync.Mutex
		once       sync.Once
		errHandler fiber.ErrorHandler
	)

	// Err padding
	errPadding := 15
	errPaddingStr := strconv.Itoa(errPadding)

	// Before handling func
	cfg.BeforeHandlerFunc(cfg)

	// Logger data
	data := &LoggerData{
		Pid:           pid,
		ErrPaddingStr: errPaddingStr,
		Timestamp:     timestamp,
	}

	// Return new handler
	return func(c fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set error handler once
		once.Do(func() {
			// get longested possible path
			stack := c.App().Stack()
			for m := range stack {
				for r := range stack[m] {
					if len(stack[m][r].Path) > errPadding {
						errPadding = len(stack[m][r].Path)
						errPaddingStr = strconv.Itoa(errPadding)
					}
				}
			}
			// override error handler
			errHandler = c.App().ErrorHandler
		})

		var start, stop time.Time

		// Set latency start time
		if cfg.enableLatency {
			start = time.Now()
		}

		// Handle request, store err for logging
		chainErr := c.Next()

		// Manually call error handler
		if chainErr != nil {
			if err := errHandler(c, chainErr); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		// Set latency stop time
		if cfg.enableLatency {
			stop = time.Now()
		}

		// Logger instance & update some logger data fields
		mu.Lock()
		data.ChainErr = chainErr
		data.Start = start
		data.Stop = stop

		if err = cfg.LoggerFunc(c, data, cfg); err != nil {
			return err
		}
		mu.Unlock()

		return nil
	}
}
