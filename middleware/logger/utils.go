package logger

import (
	"fmt"
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/logtemplate"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
)

func methodColor(method string, colors *fiber.Colors) string {
	if colors == nil {
		return ""
	}
	switch method {
	case fiber.MethodGet, fiber.MethodQuery:
		return colors.Cyan
	case fiber.MethodPost:
		return colors.Green
	case fiber.MethodPut:
		return colors.Yellow
	case fiber.MethodDelete:
		return colors.Red
	case fiber.MethodPatch:
		return colors.White
	case fiber.MethodHead:
		return colors.Magenta
	case fiber.MethodOptions:
		return colors.Blue
	default:
		return colors.Reset
	}
}

func statusColor(code int, colors *fiber.Colors) string {
	if colors == nil {
		return ""
	}
	switch {
	case code >= fiber.StatusOK && code < fiber.StatusMultipleChoices:
		return colors.Green
	case code >= fiber.StatusMultipleChoices && code < fiber.StatusBadRequest:
		return colors.Blue
	case code >= fiber.StatusBadRequest && code < fiber.StatusInternalServerError:
		return colors.Yellow
	default:
		return colors.Red
	}
}

type customLoggerWriter[T any] struct {
	loggerInstance fiberlog.AllLogger[T]
	level          fiberlog.Level
}

// Write implements io.Writer and forwards the payload to the configured logger.
func (cl *customLoggerWriter[T]) Write(p []byte) (int, error) {
	switch cl.level {
	case fiberlog.LevelTrace:
		cl.loggerInstance.Trace(utils.UnsafeString(p))
	case fiberlog.LevelDebug:
		cl.loggerInstance.Debug(utils.UnsafeString(p))
	case fiberlog.LevelInfo:
		cl.loggerInstance.Info(utils.UnsafeString(p))
	case fiberlog.LevelWarn:
		cl.loggerInstance.Warn(utils.UnsafeString(p))
	case fiberlog.LevelError:
		cl.loggerInstance.Error(utils.UnsafeString(p))
	default:
		return 0, nil
	}

	return len(p), nil
}

// LoggerToWriter is a helper function that returns an io.Writer that writes to a custom logger.
// You can integrate 3rd party loggers such as zerolog, logrus, etc. to logger middleware using this function.
//
// Valid levels: fiberlog.LevelInfo, fiberlog.LevelTrace, fiberlog.LevelWarn, fiberlog.LevelDebug, fiberlog.LevelError
func LoggerToWriter[T any](logger fiberlog.AllLogger[T], level fiberlog.Level) io.Writer {
	// Check if customLogger is nil
	if logger == nil {
		fiberlog.Panic("LoggerToWriter: customLogger must not be nil")
	}

	// Check if level is valid
	if level == fiberlog.LevelFatal || level == fiberlog.LevelPanic {
		fiberlog.Panic("LoggerToWriter: invalid level")
	}

	return &customLoggerWriter[T]{
		level:          level,
		loggerInstance: logger,
	}
}

// writeSanitized writes p to output with ASCII control bytes replaced by
// spaces (tabs are preserved), so user-controlled values such as a request
// body or a decoded query parameter cannot inject CR/LF and forge log lines.
func writeSanitized(output Buffer, p []byte) (int, error) {
	return logtemplate.WriteSanitized(output, p)
}

// writeSanitizedString is writeSanitized for strings.
func writeSanitizedString(output Buffer, s string) (int, error) {
	return logtemplate.WriteSanitizedString(output, s)
}

// writeSanitizedColored writes value between the two color escapes, scrubbing
// only value. The color sequences are library-controlled and must reach the
// output verbatim.
func writeSanitizedColored(output Buffer, color, value, reset string) (int, error) {
	n, err := output.WriteString(color)
	if err != nil {
		return n, err
	}
	m, err := writeSanitizedString(output, value)
	n += m
	if err != nil {
		return n, err
	}
	m, err = output.WriteString(reset)
	return n + m, err
}

// SanitizeValue returns s with ASCII control bytes replaced by spaces,
// preserving horizontal tab. It is the same scrubbing the built-in tags apply
// to request-derived values, exported so that code taking one of the paths
// that bypasses those tags can apply it too:
//
//   - a tag registered with RegisterTag, MustRegisterTag or RegisterContextTag
//   - an entry in Config.CustomTags
//   - a Config.LoggerFunc, which replaces the whole rendering pipeline
//
// Any of those can write a value that reached the handler percent-decoded — a
// query parameter, a form field, a header — and an unscrubbed CR/LF there
// forges an extra access-log line.
//
// Clean input, the overwhelmingly common case, is returned unchanged with no
// allocation. Only ASCII controls are replaced; bytes at or above 0x80 pass
// through, so a value that may carry C1 controls (U+0080–U+009F, including
// NEL U+0085) needs its own handling.
func SanitizeValue(s string) string {
	return sanitizeLogValue(s)
}

// sanitizeLogValue returns s with ASCII control bytes replaced by spaces
// (tabs preserved). It is the string-returning counterpart of
// writeSanitizedString, for the default-format writer, which composes its line
// with fmt.Fprintf and fixed-width padding rather than by writing tags into
// the buffer. Clean input — the overwhelmingly common case — is returned
// unchanged with no allocation.
func sanitizeLogValue(s string) string {
	idx := logtemplate.IndexControlByte(s)
	if idx == -1 {
		return s
	}
	return string(logtemplate.ScrubControls(s, idx))
}

// writeSanitizedValue renders v with %v and writes the scrubbed result.
//
// The rendering goes through a pooled scratch buffer because the bytes have to
// be scrubbed before they reach output, and scrubbing in place afterwards is
// not an option: Buffer is an interface, and an implementation whose Bytes()
// returns a copy would silently skip the scrub. The pool keeps that
// indirection allocation-free; it costs roughly 30ns over writing straight to
// output, paid only by ${locals:} values that are neither string nor []byte.
func writeSanitizedValue(output Buffer, v any) (int, error) {
	b := bytebufferpool.Get()
	defer bytebufferpool.Put(b)

	// Called without assignment deliberately: the only error Fprintf can report
	// is one from the writer, and bytebufferpool.ByteBuffer.Write always returns
	// a nil error. The error that matters — a failed write to output — is the
	// one returned below.
	fmt.Fprintf(b, "%v", v)
	return writeSanitized(output, b.B)
}
