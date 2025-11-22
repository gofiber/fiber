package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
)

var _ AllLogger[*log.Logger] = (*defaultLogger)(nil)

type defaultLogger struct {
	stdlog *log.Logger
	level  Level
	depth  int
}

// privateLog logs a message at a given level log the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) privateLog(lv Level, fmtArgs []any) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	buf.WriteString(level)
	fmt.Fprint(buf, fmtArgs...)

	_ = l.stdlog.Output(l.depth, buf.String()) //nolint:errcheck // It is fine to ignore the error
	if lv == LevelPanic {
		panic(buf.String())
	}

	buf.Reset()
	bytebufferpool.Put(buf)
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

// privateLog logs a message at a given level log the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) privateLogf(lv Level, format string, fmtArgs []any) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	buf.WriteString(level)

	if len(fmtArgs) > 0 {
		_, _ = fmt.Fprintf(buf, format, fmtArgs...)
	} else {
		_, _ = fmt.Fprint(buf, fmtArgs...)
	}

	_ = l.stdlog.Output(l.depth, buf.String()) //nolint:errcheck // It is fine to ignore the error
	if lv == LevelPanic {
		panic(buf.String())
	}
	buf.Reset()
	bytebufferpool.Put(buf)
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

// privateLogw logs a message at a given level log the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) privateLogw(lv Level, format string, keysAndValues []any) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	buf.WriteString(level)

	// Write format privateLog buffer
	if format != "" {
		buf.WriteString(format)
	}
	// Write keys and values privateLog buffer
	if len(keysAndValues) > 0 {
		if (len(keysAndValues) & 1) == 1 {
			keysAndValues = append(keysAndValues, "KEYVALS UNPAIRED")
		}

		for i := 0; i < len(keysAndValues); i += 2 {
			if i > 0 || format != "" {
				buf.WriteByte(' ')
			}
			buf.WriteString(keysAndValues[i].(string)) //nolint:forcetypeassert,errcheck // Keys must be strings
			buf.WriteByte('=')
			buf.WriteString(utils.ToString(keysAndValues[i+1]))
		}
	}

	_ = l.stdlog.Output(l.depth, buf.String()) //nolint:errcheck // It is fine to ignore the error
	if lv == LevelPanic {
		panic(buf.String())
	}
	buf.Reset()
	bytebufferpool.Put(buf)
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

// Trace logs the given values at trace level.
func (l *defaultLogger) Trace(v ...any) {
	l.privateLog(LevelTrace, v)
}

// Debug logs the given values at debug level.
func (l *defaultLogger) Debug(v ...any) {
	l.privateLog(LevelDebug, v)
}

// Info logs the given values at info level.
func (l *defaultLogger) Info(v ...any) {
	l.privateLog(LevelInfo, v)
}

// Warn logs the given values at warn level.
func (l *defaultLogger) Warn(v ...any) {
	l.privateLog(LevelWarn, v)
}

// Error logs the given values at error level.
func (l *defaultLogger) Error(v ...any) {
	l.privateLog(LevelError, v)
}

// Fatal logs the given values at fatal level and terminates the process.
func (l *defaultLogger) Fatal(v ...any) {
	l.privateLog(LevelFatal, v)
}

// Panic logs the given values at panic level and panics.
func (l *defaultLogger) Panic(v ...any) {
	l.privateLog(LevelPanic, v)
}

// Tracef formats according to a format specifier and logs at trace level.
func (l *defaultLogger) Tracef(format string, v ...any) {
	l.privateLogf(LevelTrace, format, v)
}

// Debugf formats according to a format specifier and logs at debug level.
func (l *defaultLogger) Debugf(format string, v ...any) {
	l.privateLogf(LevelDebug, format, v)
}

// Infof formats according to a format specifier and logs at info level.
func (l *defaultLogger) Infof(format string, v ...any) {
	l.privateLogf(LevelInfo, format, v)
}

// Warnf formats according to a format specifier and logs at warn level.
func (l *defaultLogger) Warnf(format string, v ...any) {
	l.privateLogf(LevelWarn, format, v)
}

// Errorf formats according to a format specifier and logs at error level.
func (l *defaultLogger) Errorf(format string, v ...any) {
	l.privateLogf(LevelError, format, v)
}

// Fatalf formats according to a format specifier, logs at fatal level, and terminates the process.
func (l *defaultLogger) Fatalf(format string, v ...any) {
	l.privateLogf(LevelFatal, format, v)
}

// Panicf formats according to a format specifier, logs at panic level, and panics.
func (l *defaultLogger) Panicf(format string, v ...any) {
	l.privateLogf(LevelPanic, format, v)
}

// Tracew logs at trace level with a message and key/value pairs.
func (l *defaultLogger) Tracew(msg string, keysAndValues ...any) {
	l.privateLogw(LevelTrace, msg, keysAndValues)
}

// Debugw logs at debug level with a message and key/value pairs.
func (l *defaultLogger) Debugw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelDebug, msg, keysAndValues)
}

// Infow logs at info level with a message and key/value pairs.
func (l *defaultLogger) Infow(msg string, keysAndValues ...any) {
	l.privateLogw(LevelInfo, msg, keysAndValues)
}

// Warnw logs at warn level with a message and key/value pairs.
func (l *defaultLogger) Warnw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelWarn, msg, keysAndValues)
}

// Errorw logs at error level with a message and key/value pairs.
func (l *defaultLogger) Errorw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelError, msg, keysAndValues)
}

// Fatalw logs at fatal level with a message and key/value pairs, then terminates the process.
func (l *defaultLogger) Fatalw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelFatal, msg, keysAndValues)
}

// Panicw logs at panic level with a message and key/value pairs, then panics.
func (l *defaultLogger) Panicw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelPanic, msg, keysAndValues)
}

// WithContext returns a logger that shares the underlying output but adjusts the call depth.
func (l *defaultLogger) WithContext(_ context.Context) CommonLogger {
	return &defaultLogger{
		stdlog: l.stdlog,
		level:  l.level,
		depth:  l.depth - 1,
	}
}

// SetLevel updates the minimum level that will be emitted by the logger.
func (l *defaultLogger) SetLevel(level Level) {
	l.level = level
}

// SetOutput replaces the underlying writer used by the logger.
func (l *defaultLogger) SetOutput(writer io.Writer) {
	l.stdlog.SetOutput(writer)
}

// Logger returns the logger instance. It can be used to adjust the logger configurations in case of need.
func (l *defaultLogger) Logger() *log.Logger {
	return l.stdlog
}

// DefaultLogger returns the default logger.
func DefaultLogger[T any]() AllLogger[T] {
	if l, ok := logger.(AllLogger[T]); ok {
		return l
	}

	return nil
}
