package logger

import (
	"io"

	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

func methodColor(method string, colors fiber.Colors) string {
	switch method {
	case fiber.MethodGet:
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

func statusColor(code int, colors fiber.Colors) string {
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

type customLoggerWriter struct {
	level          fiberlog.Level
	loggerInstance fiberlog.AllLogger
}

func (cl *customLoggerWriter) Write(p []byte) (n int, err error) {
	switch cl.level {
	case fiberlog.LevelInfo:
		cl.loggerInstance.Info(utils.UnsafeString(p))
	case fiberlog.LevelTrace:
		cl.loggerInstance.Trace(utils.UnsafeString(p))
	case fiberlog.LevelWarn:
		cl.loggerInstance.Warn(utils.UnsafeString(p))
	case fiberlog.LevelDebug:
		cl.loggerInstance.Debug(utils.UnsafeString(p))
	case fiberlog.LevelError:
		cl.loggerInstance.Error(utils.UnsafeString(p))
	}

	return len(p), nil
}

// LoggerToWriter is a helper function that returns an io.Writer that writes to a custom logger.
// You can integrate 3rd party loggers such as zerolog, logrus, etc. to logger middleware using this function.
//
// Valid levels: fiberlog.LevelInfo, fiberlog.LevelTrace, fiberlog.LevelWarn, fiberlog.LevelDebug, fiberlog.LevelError
func LoggerToWriter(customLogger fiberlog.AllLogger, level fiberlog.Level) io.Writer {
	// Check if customLogger is nil
	if customLogger == nil {
		fiberlog.Panic("LoggerToWriter: customLogger must not be nil")
	}

	// Check if level is valid
	if level == fiberlog.LevelFatal || level == fiberlog.LevelPanic {
		fiberlog.Panic("LoggerToWriter: invalid level")
	}

	return &customLoggerWriter{
		level:          level,
		loggerInstance: customLogger,
	}
}
