package logger

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
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
		mu         sync.Mutex
		errHandler fiber.ErrorHandler

		dataPool = sync.Pool{New: func() interface{} { return new(Data) }}
	)

	// If colors are enabled, check terminal compatibility
	if cfg.enableColors {
		cfg.Output = colorable.NewColorableStdout()
		if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
			cfg.Output = colorable.NewNonColorable(os.Stdout)
		}
	}

	errPadding := 15
	errPaddingStr := strconv.Itoa(errPadding)

	// instead of analyzing the template inside(handler) each time, this is done once before
	// and we create several slices of the same length with the functions to be executed and fixed parts.
	templateChain, logFunChain, err := buildLogFuncChain(&cfg, createTagMap(&cfg))
	if err != nil {
		panic(err)
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Alias colors
		colors := c.App().Config().ColorScheme

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

		// Get new buffer
		buf := bytebufferpool.Get()

		// Default output when no custom Format or io.Writer is given
		if cfg.Format == ConfigDefault.Format {
			// Format error if exist
			formatErr := ""
			if cfg.enableColors {
				if chainErr != nil {
					formatErr = colors.Red + " | " + chainErr.Error() + colors.Reset
				}
				_, _ = buf.WriteString( //nolint:errcheck // This will never fail
					fmt.Sprintf("%s |%s %3d %s| %7v | %15s |%s %-7s %s| %-"+errPaddingStr+"s %s\n",
						timestamp.Load().(string),
						statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset,
						data.Stop.Sub(data.Start).Round(time.Millisecond),
						c.IP(),
						methodColor(c.Method(), colors), c.Method(), colors.Reset,
						c.Path(),
						formatErr,
					),
				)
			} else {
				if chainErr != nil {
					formatErr = " | " + chainErr.Error()
				}
				_, _ = buf.WriteString( //nolint:errcheck // This will never fail
					fmt.Sprintf("%s | %3d | %7v | %15s | %-7s | %-"+errPaddingStr+"s %s\n",
						timestamp.Load().(string),
						c.Response().StatusCode(),
						data.Stop.Sub(data.Start).Round(time.Millisecond),
						c.IP(),
						c.Method(),
						c.Path(),
						formatErr,
					),
				)
			}

			// Write buffer to output
			_, _ = cfg.Output.Write(buf.Bytes()) //nolint:errcheck // This will never fail

			if cfg.Done != nil {
				cfg.Done(c, buf.Bytes())
			}

			// Put buffer back to pool
			bytebufferpool.Put(buf)

			// End chain
			return nil
		}

		var err error
		// Loop over template parts execute dynamic parts and add fixed parts to the buffer
		for i, logFunc := range logFunChain {
			if logFunc == nil {
				_, _ = buf.Write(templateChain[i]) //nolint:errcheck // This will never fail
			} else if templateChain[i] == nil {
				_, err = logFunc(buf, c, data, "")
			} else {
				_, err = logFunc(buf, c, data, utils.UnsafeString(templateChain[i]))
			}
			if err != nil {
				break
			}
		}

		// Also write errors to the buffer
		if err != nil {
			_, _ = buf.WriteString(err.Error()) //nolint:errcheck // This will never fail
		}
		mu.Lock()
		// Write buffer to output
		if _, err := cfg.Output.Write(buf.Bytes()); err != nil {
			// Write error to output
			if _, err := cfg.Output.Write([]byte(err.Error())); err != nil {
				// There is something wrong with the given io.Writer
				_, _ = fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
			}
		}
		mu.Unlock()

		if cfg.Done != nil {
			cfg.Done(c, buf.Bytes())
		}

		// Put buffer back to pool
		bytebufferpool.Put(buf)

		return nil
	}
}

func appendInt(output Buffer, v int) (int, error) {
	old := output.Len()
	output.Set(fasthttp.AppendUint(output.Bytes(), v))
	return output.Len() - old, nil
}
