package logger

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

var mu sync.Mutex

// default logger for fiber
func defaultLoggerInstance(c fiber.Ctx, data *Data, cfg Config) error {
	// Alias colors
	colors := c.App().Config().ColorScheme

	// Get new buffer
	buf := bytebufferpool.Get()

	// Default output when no custom Format or io.Writer is given
	if cfg.Format == defaultFormat {
		// Format error if exist
		formatErr := ""
		if cfg.enableColors {
			if data.ChainErr != nil {
				formatErr = colors.Red + " | " + data.ChainErr.Error() + colors.Reset
			}
			_, _ = buf.WriteString( //nolint:errcheck // This will never fail
				fmt.Sprintf("%s |%s %3d %s| %13v | %15s |%s %-7s %s| %-"+data.ErrPaddingStr+"s %s\n",
					data.Timestamp.Load().(string),
					statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset,
					data.Stop.Sub(data.Start),
					c.IP(),
					methodColor(c.Method(), colors), c.Method(), colors.Reset,
					c.Path(),
					formatErr,
				),
			)
		} else {
			if data.ChainErr != nil {
				formatErr = " | " + data.ChainErr.Error()
			}

			// Helper function to append fixed-width string with padding
			fixedWidth := func(s string, width int, rightAlign bool) {
				if rightAlign {
					for i := len(s); i < width; i++ {
						buf.WriteByte(' ') //nolint:errcheck // It is fine to ignore the error
					}
					buf.WriteString(s) //nolint:errcheck // It is fine to ignore the error
				} else {
					buf.WriteString(s) //nolint:errcheck // It is fine to ignore the error
					for i := len(s); i < width; i++ {
						buf.WriteByte(' ') //nolint:errcheck // It is fine to ignore the error
					}
				}
			}

			// Timestamp
			buf.WriteString(data.Timestamp.Load().(string)) //nolint:errcheck // It is fine to ignore the error
			buf.WriteString(" | ")                          //nolint:errcheck // It is fine to ignore the error

			// Status Code with 3 fixed width, right aligned
			fixedWidth(strconv.Itoa(c.Response().StatusCode()), 3, true)
			buf.WriteString(" | ") //nolint:errcheck // It is fine to ignore the error

			// Duration with 13 fixed width, right aligned
			duration := time.Duration(data.Stop.Sub(data.Start)).String()
			fixedWidth(duration, 13, true)
			buf.WriteString(" | ") //nolint:errcheck // It is fine to ignore the error

			// Client IP with 15 fixed width, right aligned
			fixedWidth(c.IP(), 15, true)
			buf.WriteString(" | ") //nolint:errcheck // It is fine to ignore the error

			// HTTP Method with 7 fixed width, left aligned
			fixedWidth(c.Method(), 7, false)
			buf.WriteString(" | ") //nolint:errcheck // It is fine to ignore the error

			// Path with dynamic padding for error message, left aligned
			errPadding, _ := strconv.Atoi(data.ErrPaddingStr) // Assuming this is a valid int
			fixedWidth(c.Path(), errPadding, false)

			// Error message
			buf.WriteString(" ")       //nolint:errcheck // It is fine to ignore the error
			buf.WriteString(formatErr) //nolint:errcheck // It is fine to ignore the error
			buf.WriteString("\n")      //nolint:errcheck // It is fine to ignore the error

			// _, _ = buf.WriteString( //nolint:errcheck // This will never fail
			// 	fmt.Sprintf("%s | %3d | %13v | %15s | %-7s | %-"+data.ErrPaddingStr+"s %s\n",
			// 		data.Timestamp.Load().(string),
			// 		c.Response().StatusCode(),
			// 		data.Stop.Sub(data.Start),
			// 		c.IP(),
			// 		c.Method(),
			// 		c.Path(),
			// 		formatErr,
			// 	),
			// )
		}

		// Write buffer to output
		writeLog(cfg.Output, buf.Bytes())

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
	for i, logFunc := range data.LogFuncChain {
		if logFunc == nil {
			_, _ = buf.Write(data.TemplateChain[i]) //nolint:errcheck // This will never fail
		} else if data.TemplateChain[i] == nil {
			_, err = logFunc(buf, c, data, "")
		} else {
			_, err = logFunc(buf, c, data, utils.UnsafeString(data.TemplateChain[i]))
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
	writeLog(cfg.Output, buf.Bytes())
	mu.Unlock()

	if cfg.Done != nil {
		cfg.Done(c, buf.Bytes())
	}

	// Put buffer back to pool
	bytebufferpool.Put(buf)

	return nil
}

// run something before returning the handler
func beforeHandlerFunc(cfg Config) {
	// If colors are enabled, check terminal compatibility
	if cfg.enableColors {
		cfg.Output = colorable.NewColorableStdout()
		if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
			cfg.Output = colorable.NewNonColorable(os.Stdout)
		}
	}
}

func appendInt(output Buffer, v int) (int, error) {
	old := output.Len()
	output.Set(fasthttp.AppendUint(output.Bytes(), v))
	return output.Len() - old, nil
}

// writeLog writes a msg to w, printing a warning to stderr if the log fails.
func writeLog(w io.Writer, msg []byte) {
	if _, err := w.Write(msg); err != nil {
		// Write error to output
		if _, err := w.Write([]byte(err.Error())); err != nil {
			// There is something wrong with the given io.Writer
			_, _ = fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
		}
	}
}
