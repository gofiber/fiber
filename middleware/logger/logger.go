package logger

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/logtemplate"
)

func loadTimestamp(value *atomic.Value) string {
	if timestamp, ok := value.Load().(string); ok {
		return timestamp
	}

	return ""
}

// defaultErrPadding is the initial column width used by the default access-log
// formatter to align the request path against the optional error suffix. The
// width grows on first request to fit the longest registered route, but a
// non-zero default keeps short-lived test apps (with no routes) readable.
const defaultErrPadding = 15

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Get timezone location
	tz, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		cfg.timeZoneLocation = time.Local
	} else {
		cfg.timeZoneLocation = tz
	}

	// Check if format contains latency
	cfg.isLatencyEnabled = strings.Contains(cfg.Format, "${"+TagLatency+"}")

	timeEnabled := strings.Contains(cfg.Format, "${"+TagTime+"}")
	var timestamp *atomic.Value
	if timeEnabled {
		if cfg.TimeDone != nil {
			// Caller-managed lifecycle: dedicated updater so it can be stopped
			// via TimeDone (useful for tests and short-lived apps).
			timestamp = &atomic.Value{}
			timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))
			startTimestampUpdater(timestamp, &cfg)
		} else {
			// Process-lifetime shared updater, deduplicated by format/zone/interval.
			timestamp = sharedTimestamp(cfg.TimeFormat, cfg.timeZoneLocation, cfg.TimeInterval)
		}
	}
	// Set PID once
	pid := strconv.Itoa(os.Getpid())

	// Set variables
	var (
		once     sync.Once
		dataPool = sync.Pool{New: func() any { return new(Data) }}
	)

	// Err padding starts at the documented default and grows once on first
	// request to fit the longest registered route path.
	errPadding := defaultErrPadding
	errPaddingStr := strconv.Itoa(errPadding)

	// Before handling func
	cfg.BeforeHandlerFunc(&cfg)

	logsTag := func(tag string) bool { return strings.Contains(cfg.Format, "${"+tag+"}") }
	logsMethod, logsPath, logsURL := logsTag(TagMethod), logsTag(TagPath), logsTag(TagURL)
	logsProtocol, logsHost := logsTag(TagProtocol), logsTag(TagHost)

	// Logger data
	// instead of analyzing the template inside(handler) each time, this is done once before
	// and we create several slices of the same length with the functions to be executed and fixed parts.
	template, err := logtemplate.Build[fiber.Ctx, Data](cfg.Format, createTagMap(&cfg))
	if err != nil {
		if translated := translateBuildError(err); translated != nil {
			panic(translated)
		}
		panic(err)
	}
	templateChain, logFuncChain := template.Chains()

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set error padding once
		once.Do(func() {
			// get longest possible path
			stack := c.App().Stack()
			for m := range stack {
				for r := range stack[m] {
					if len(stack[m][r].Path) > errPadding {
						errPadding = len(stack[m][r].Path)
						errPaddingStr = strconv.Itoa(errPadding)
					}
				}
			}
		})

		// Logger data
		data := dataPool.Get().(*Data) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
		// no need for a reset, as long as we always override everything
		data.Pid = pid
		data.ErrPaddingStr = errPaddingStr
		if timeEnabled {
			data.Timestamp = loadTimestamp(timestamp)
		} else {
			data.Timestamp = ""
		}
		// These compiled chains are shared across requests, so a custom
		// LoggerFunc must only read them — never append to or reorder them.
		// Walk Data.TemplateChain and Data.LogFuncChain in step; the helper the
		// default logger uses for this lives in internal/ and is not importable
		// from outside the module.
		data.TemplateChain = templateChain
		data.LogFuncChain = logFuncChain
		// put data back in the pool
		defer dataPool.Put(data)

		// Copied while nothing else uses the request: the handler of one that times out may still rewrite it.
		if logsMethod {
			data.req.method = c.Method()
		}
		if logsPath {
			data.req.path = append(data.req.path[:0], c.Path()...)
		}
		if logsURL {
			data.req.url = append(data.req.url[:0], c.OriginalURL()...)
		}
		if logsProtocol {
			data.req.protocol = append(data.req.protocol[:0], c.Protocol()...)
		}
		if logsHost {
			data.req.host = append(data.req.host[:0], c.Hostname()...)
		}

		// Set latency start time
		if cfg.isLatencyEnabled {
			data.Start = time.Now()
		}

		// Handle request, store err for logging
		chainErr := c.Next()

		data.ChainErr = chainErr
		data.timeoutResp = timeoutResponse(c)
		// The error handler of the app serving this request: one logger may be registered in several apps.
		if chainErr != nil {
			if err := c.App().ErrorHandler(c, chainErr); err != nil {
				// The error handler failed, so fall back to a bare 500. SendStatus only
				// sets the status and body on the response and does no I/O; if it fails,
				// there is no further fallback left.
				_ = c.SendStatus(fiber.StatusInternalServerError) //nolint:errcheck // no further fallback if this fails
			}
		}

		// Set latency stop time
		if cfg.isLatencyEnabled {
			data.Stop = time.Now()
		}

		// Logger instance & update some logger data fields
		err := cfg.LoggerFunc(c, data, &cfg)
		if err == nil && data.timeoutResp != nil {
			// The error handler answered nothing, and the error keeps the middleware before us off the handler's response.
			return chainErr
		}
		return err
	}
}

// startTimestampUpdater refreshes the cached formatted timestamp until TimeDone
// is closed.
func startTimestampUpdater(timestamp *atomic.Value, cfg *Config) {
	_ = startTimestampUpdaterWithStop(timestamp, cfg)
}

func startTimestampUpdaterWithStop(timestamp *atomic.Value, cfg *Config) <-chan struct{} {
	stopped := make(chan struct{})
	if !strings.Contains(cfg.Format, "${"+TagTime+"}") {
		close(stopped)
		return stopped
	}

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(cfg.TimeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))
			case <-cfg.TimeDone:
				return
			}
		}
	}()
	return stopped
}
