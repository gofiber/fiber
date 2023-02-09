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
	cfg.enableLatency = strings.Contains(cfg.Format, "${"+TagLatency+"}")

	var timestamp atomic.Value
	// Create correct timeformat
	timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))

	// Update date/time every 500 milliseconds in a separate go routine
	if strings.Contains(cfg.Format, "${"+TagTime+"}") {
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
		once       sync.Once
		errHandler fiber.ErrorHandler

		dataPool = sync.Pool{New: func() interface{} { return new(Data) }}
	)

	// Err padding
	errPadding := 15
	errPaddingStr := strconv.Itoa(errPadding)

	// Before handling func
	cfg.BeforeHandlerFunc(cfg)

	// Logger data
	// instead of analyzing the template inside(handler) each time, this is done once before
	// and we create several slices of the same length with the functions to be executed and fixed parts.
	templateChain, logFunChain, err := buildLogFuncChain(&cfg, createTagMap(&cfg))
	if err != nil {
		panic(err)
	}

	// Return new handler
	return func(c fiber.Ctx) error {
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

		// Logger data
		data := dataPool.Get().(*Data) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
		// no need for a reset, as long as we always override everything
		data.Pid = pid
		data.ErrPaddingStr = errPaddingStr
		data.Timestamp = timestamp
		data.TemplateChain = templateChain
		data.LogFuncChain = logFunChain
		// put data back in the pool
		defer dataPool.Put(data)

		// Set latency start time
		if cfg.enableLatency {
			data.Start = time.Now()
		}

		// Handle request, store err for logging
		chainErr := c.Next()

		data.ChainErr = chainErr
		// Manually call error handler
		if chainErr != nil {
			if err := errHandler(c, chainErr); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError) //nolint:errcheck // TODO: Explain why we ignore the error here
			}
		}

		// Set latency stop time
		if cfg.enableLatency {
			data.Stop = time.Now()
		}

		// Logger instance & update some logger data fields
		if err = cfg.LoggerFunc(c, data, cfg); err != nil {
			return err
		}

		return nil
	}
}
