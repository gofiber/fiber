package logger

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/logtemplate"
	"github.com/gofiber/utils/v2"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/valyala/bytebufferpool"
)

// default logger for fiber
func defaultLoggerInstance(c fiber.Ctx, data *Data, cfg *Config) error {
	if cfg == nil {
		cfg = &Config{
			Stream:           os.Stdout,
			Format:           DefaultFormat,
			areColorsEnabled: true,
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
		// The request-derived values below (IP, path, and the chain error,
		// which routinely embeds decoded request data) are scrubbed of control
		// bytes for the same reason the template tags are: raw CR/LF lets a
		// client forge additional access-log lines. See #4341. The method is
		// not scrubbed — fasthttp rejects a request line whose method token
		// holds one — which keeps this path consistent with ${method}.
		formatErr := ""
		if cfg.areColorsEnabled {
			if data.ChainErr != nil {
				formatErr = colors.Red + " | " + sanitizeLogValue(data.ChainErr.Error()) + colors.Reset
			}

			// The same line the uncolored branch below writes, with the
			// status and method wrapped in their colors. It was one Fprintf
			// with the format
			//
			//	"%s |%s %3d %s| %13v | %15s |%s %-7s %s| %-<pad>s %s\n"
			//
			// which reflects over eleven arguments and boxes each: eleven
			// allocations and ~2.5x the time of the uncolored branch, on the
			// path every terminal user sees. Written in place instead. Two
			// details of fmt's output are kept deliberately: it pads by
			// runes, so the latency and path columns here count runes where
			// the uncolored branch has always counted bytes, and the
			// concatenated format string's error padding is parsed the same
			// way the uncolored branch parses it.
			status := c.Res().StatusCode()
			method := c.Method()
			errPadding, _ := strconv.Atoi(data.ErrPaddingStr) //nolint:errcheck // It is fine to ignore the error

			buf.WriteString(data.Timestamp)
			buf.WriteString(" |")
			buf.WriteString(statusColor(status, &colors))
			buf.WriteByte(' ')
			appendIntPadded(buf, status, 3)
			buf.WriteByte(' ')
			buf.WriteString(colors.Reset)
			buf.WriteString("| ")
			appendDurationColumns(buf, data.Stop.Sub(data.Start), 13)
			buf.WriteString(" | ")
			writeColumnRight(buf, sanitizeLogValue(c.IP()), 15)
			buf.WriteString(" |")
			buf.WriteString(methodColor(method, &colors))
			buf.WriteByte(' ')
			writeColumnLeft(buf, method, 7)
			buf.WriteByte(' ')
			buf.WriteString(colors.Reset)
			buf.WriteString("| ")
			writeColumnLeft(buf, sanitizeLogValue(c.Path()), errPadding)
			buf.WriteByte(' ')
			buf.WriteString(formatErr)
			buf.WriteByte('\n')
		} else {
			if data.ChainErr != nil {
				formatErr = " | " + sanitizeLogValue(data.ChainErr.Error())
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
			buf.WriteString(data.Timestamp)
			buf.WriteString(" | ")

			// Status Code with 3 fixed width, right aligned; appended digit-wise
			// to avoid the per-request Itoa string.
			appendIntPadded(buf, c.Res().StatusCode(), 3)
			buf.WriteString(" | ")

			// Duration with 13 fixed width, right aligned; rendered straight
			// into the buffer instead of via Duration.String.
			appendDurationPadded(buf, data.Stop.Sub(data.Start), 13)
			buf.WriteString(" | ")

			// Client IP with 15 fixed width, right aligned
			fixedWidth(sanitizeLogValue(c.IP()), 15, true)
			buf.WriteString(" | ")

			// HTTP Method with 7 fixed width, left aligned
			fixedWidth(c.Method(), 7, false)
			buf.WriteString(" | ")

			// Path with dynamic padding for error message, left aligned
			errPadding, _ := strconv.Atoi(data.ErrPaddingStr) //nolint:errcheck // It is fine to ignore the error
			fixedWidth(sanitizeLogValue(c.Path()), errPadding, false)

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
	if cfg.areColorsEnabled && cfg.Stream == os.Stdout {
		cfg.Stream = colorable.NewColorableStdout()
		if !cfg.ForceColors && (os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()))) {
			cfg.Stream = colorable.NewNonColorable(os.Stdout)
		}
	}
}

// maxIntLen is the widest decimal an int64 can render ("-9223372036854775808"),
// so a scratch array of that size lets utils.AppendInt format any of them
// without growing the slice.
const maxIntLen = 20

// writeScratch writes s to output one byte at a time.
//
// Handing the slice to output.Write instead would push the caller's scratch
// array onto the heap: Buffer is an interface, so the compiler has to assume
// Write keeps the pointer. That allocation costs more than the extra calls —
// a status code measured 47ns with one 24-byte allocation through Write
// against 21ns and none this way.
func writeScratch(output Buffer, s []byte) (int, error) {
	for i, c := range s {
		if err := output.WriteByte(c); err != nil {
			return i, err
		}
	}
	return len(s), nil
}

// writePadding writes the spaces that right-align a used-column value in a
// field of the given width, and returns how many it wrote.
func writePadding(output Buffer, width, used int) (int, error) {
	written := 0
	for i := used; i < width; i++ {
		if err := output.WriteByte(' '); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

// appendInt writes the decimal form of v into output without going through
// fmt boxing.
func appendInt(output Buffer, v int) (int, error) {
	var scratch [maxIntLen]byte
	return writeScratch(output, utils.AppendInt(scratch[:0], int64(v)))
}

// appendIntTag is appendInt right-aligned to width with spaces, the shape
// fmt's "%*d" verbs produced for the colored status column.
func appendIntTag(output Buffer, v, width int) (int, error) {
	var scratch [maxIntLen]byte
	s := utils.AppendInt(scratch[:0], int64(v))

	written, err := writePadding(output, width, len(s))
	if err != nil {
		return written, err
	}
	n, err := writeScratch(output, s)
	return written + n, err
}

// appendIntPadded appends the decimal form of v to buf, right-aligned to
// width with spaces, without allocating an intermediate string.
func appendIntPadded(buf *bytebufferpool.ByteBuffer, v, width int) {
	var scratch [maxIntLen]byte
	s := utils.AppendInt(scratch[:0], int64(v))
	for i := len(s); i < width; i++ {
		buf.WriteByte(' ')
	}
	buf.Write(s)
}

// maxDurationLen is the widest output time.Duration.String can produce
// ("-2562047h47m16.854775808s"), so a scratch array of that size lets
// utils.AppendDuration render any latency without touching the heap.
const maxDurationLen = 25

// appendDurationPadded appends d in time.Duration.String form to buf,
// right-aligned to width with spaces. utils.AppendDuration renders the same
// bytes straight into a stack scratch instead of building the intermediate
// string Duration.String returns, which measured ~17% faster on this column.
// Padding counts bytes, as the string form this replaced did.
func appendDurationPadded(buf *bytebufferpool.ByteBuffer, d time.Duration, width int) {
	var scratch [maxDurationLen]byte
	s := utils.AppendDuration(scratch[:0], d)
	for i := len(s); i < width; i++ {
		buf.WriteByte(' ')
	}
	buf.Write(s)
}

// appendDurationColumns is appendDurationPadded with the padding counted in
// columns rather than bytes, the width "%13v" produced for the colored default
// format: a sub-millisecond latency renders "µs", whose 'µ' is two bytes but
// one column.
func appendDurationColumns(buf *bytebufferpool.ByteBuffer, d time.Duration, width int) {
	var scratch [maxDurationLen]byte
	s := utils.AppendDuration(scratch[:0], d)
	for i := utf8.RuneCount(s); i < width; i++ {
		buf.WriteByte(' ')
	}
	buf.Write(s)
}

// writeColumnRight appends s to buf right-aligned in width columns, the shape
// fmt's "%15s" produced for the colored default format. Columns are runes, as
// fmt counted them, so a multi-byte rune takes the one column it displays as.
func writeColumnRight(buf *bytebufferpool.ByteBuffer, s string, width int) {
	for pad := width - utf8.RuneCountInString(s); pad > 0; pad-- {
		buf.WriteByte(' ')
	}
	buf.WriteString(s)
}

// writeColumnLeft is writeColumnRight with the padding after s, the shape
// fmt's "%-7s" produced.
func writeColumnLeft(buf *bytebufferpool.ByteBuffer, s string, width int) {
	buf.WriteString(s)
	for pad := width - utf8.RuneCountInString(s); pad > 0; pad-- {
		buf.WriteByte(' ')
	}
}

// appendDurationTag is appendDurationPadded for the Buffer interface the
// ${latency} tag writes through. Its padding counts runes rather than bytes so
// the column keeps the width fmt's "%13v" produced: a sub-millisecond latency
// renders "µs", whose 'µ' is two bytes but one column.
func appendDurationTag(output Buffer, d time.Duration, width int) (int, error) {
	var scratch [maxDurationLen]byte
	s := utils.AppendDuration(scratch[:0], d)

	written, err := writePadding(output, width, utf8.RuneCount(s))
	if err != nil {
		return written, err
	}
	n, err := writeScratch(output, s)
	return written + n, err
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
