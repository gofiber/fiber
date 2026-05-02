package logger

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/logtemplate"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/valyala/bytebufferpool"
)

// default logger for fiber
func defaultLoggerInstance(c fiber.Ctx, data *Data, cfg *Config) error {
	if cfg == nil {
		cfg = &Config{
			Stream:       os.Stdout,
			Format:       DefaultFormat,
			enableColors: true,
		}
	}
	// Check if Skip is defined and call it.
	// Now, if Skip(c) == true, we SKIP logging:
	if cfg.Skip != nil && cfg.Skip(c) {
		return nil // Skip logging if Skip returns true
	}

	// Alias colors
	colors := c.App().Config().ColorScheme

	// Get new buffer
	buf := bytebufferpool.Get()

	// Default output when no custom Format or io.Writer is given
	if cfg.Format == DefaultFormat {
		// Format error if exist
		formatErr := ""
		if cfg.enableColors {
			if data.ChainErr != nil {
				formatErr = colors.Red + " | " + data.ChainErr.Error() + colors.Reset
			}
			fmt.Fprintf(buf,
				"%s |%s %3d %s| %13v | %15s |%s %-7s %s| %-"+data.ErrPaddingStr+"s %s\n",
				data.Timestamp.Load().(string), //nolint:forcetypeassert,errcheck // Timestamp is always a string
				statusColor(c.Response().StatusCode(), &colors), c.Response().StatusCode(), colors.Reset,
				data.Stop.Sub(data.Start),
				c.IP(),
				methodColor(c.Method(), &colors), c.Method(), colors.Reset,
				c.Path(),
				formatErr,
			)
		} else {
			if data.ChainErr != nil {
				formatErr = " | " + data.ChainErr.Error()
			}

			// Helper function to append fixed-width string with padding
			fixedWidth := func(s string, width int, rightAlign bool) {
				if rightAlign {
					for i := len(s); i < width; i++ {
						buf.WriteByte(' ')
					}
					buf.WriteString(s)
				} else {
					buf.WriteString(s)
					for i := len(s); i < width; i++ {
						buf.WriteByte(' ')
					}
				}
			}

			// Timestamp
			buf.WriteString(data.Timestamp.Load().(string)) //nolint:forcetypeassert,errcheck // Timestamp is always a string
			buf.WriteString(" | ")

			// Status Code with 3 fixed width, right aligned
			fixedWidth(strconv.Itoa(c.Response().StatusCode()), 3, true)
			buf.WriteString(" | ")

			// Duration with 13 fixed width, right aligned
			fixedWidth(data.Stop.Sub(data.Start).String(), 13, true)
			buf.WriteString(" | ")

			// Client IP with 15 fixed width, right aligned
			fixedWidth(c.IP(), 15, true)
			buf.WriteString(" | ")

			// HTTP Method with 7 fixed width, left aligned
			fixedWidth(c.Method(), 7, false)
			buf.WriteString(" | ")

			// Path with dynamic padding for error message, left aligned
			errPadding, _ := strconv.Atoi(data.ErrPaddingStr) //nolint:errcheck // It is fine to ignore the error
			fixedWidth(c.Path(), errPadding, false)

			// Error message
			buf.WriteString(" ")
			buf.WriteString(formatErr)
			buf.WriteString("\n")
		}

		// Write buffer to output
		writeLog(cfg.Stream, buf.Bytes())

		if cfg.Done != nil {
			cfg.Done(c, buf.Bytes())
		}

		// Put buffer back to pool
		bytebufferpool.Put(buf)

		// End chain
		return nil
	}

	err := logtemplate.ExecuteChains(buf, c, data, data.TemplateChain, data.LogFuncChain)
	// Also write errors to the buffer
	if err != nil {
		buf.WriteString(err.Error())
	}

	writeLog(cfg.Stream, buf.Bytes())

	if cfg.Done != nil {
		cfg.Done(c, buf.Bytes())
	}

	// Put buffer back to pool
	bytebufferpool.Put(buf)

	return nil
}

// run something before returning the handler
func beforeHandlerFunc(cfg *Config) {
	if cfg == nil {
		return
	}

	// If colors are enabled, check terminal compatibility
	if cfg.enableColors && cfg.Stream == os.Stdout {
		cfg.Stream = colorable.NewColorableStdout()
		if !cfg.ForceColors && (os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()))) {
			cfg.Stream = colorable.NewNonColorable(os.Stdout)
		}
	}
}

// appendInt writes the decimal form of v into output without going through
// fmt boxing. The 20-byte stack scratch fits any int64; strconv.AppendInt
// only grows the slice when the formatted value exceeds that capacity, which
// cannot happen for a fixed-width int.
func appendInt(output Buffer, v int) (int, error) {
	var scratch [20]byte
	return output.Write(strconv.AppendInt(scratch[:0], int64(v), 10))
}

// writeLog writes a msg to w, printing a warning to stderr if the log fails.
func writeLog(w io.Writer, msg []byte) {
	if _, err := w.Write(msg); err != nil {
		// Write error to output
		if _, writeErr := w.Write([]byte(err.Error())); writeErr != nil {
			// There is something wrong with the given io.Writer
			_, _ = fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", writeErr)
		}
	}
}
