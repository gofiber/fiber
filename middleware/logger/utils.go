package logger

import (
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/logtemplate"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// writeColored writes value to output wrapped in the given (framework-controlled)
// prefix/suffix color codes, sanitizing only value. It is used for tags whose
// payload can echo user-controlled input but is rendered with ANSI colors.
func writeColored(output Buffer, prefix, value, suffix string) (int, error) {
	total, err := output.WriteString(prefix)
	if err != nil {
		return total, err
	}
	n, err := logtemplate.WriteSanitizedString(output, value)
	total += n
	if err != nil {
		return total, err
	}
	n, err = output.WriteString(suffix)
	total += n
	return total, err
}

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
