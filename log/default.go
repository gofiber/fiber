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

var _ AllLogger = (*defaultLogger)(nil)

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
	buf.WriteString(fmt.Sprint(fmtArgs...))

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
		_, _ = fmt.Fprintf(buf, format, fmtArgs...) //nolint: errcheck // It is fine to ignore the error
	} else {
		_, _ = fmt.Fprint(buf, fmtArgs...) //nolint: errcheck // It is fine to ignore the error
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

func (l *defaultLogger) Trace(v ...any) {
	l.privateLog(LevelTrace, v)
}

func (l *defaultLogger) Debug(v ...any) {
	l.privateLog(LevelDebug, v)
}

func (l *defaultLogger) Info(v ...any) {
	l.privateLog(LevelInfo, v)
}

func (l *defaultLogger) Warn(v ...any) {
	l.privateLog(LevelWarn, v)
}

func (l *defaultLogger) Error(v ...any) {
	l.privateLog(LevelError, v)
}

func (l *defaultLogger) Fatal(v ...any) {
	l.privateLog(LevelFatal, v)
}

func (l *defaultLogger) Panic(v ...any) {
	l.privateLog(LevelPanic, v)
}

func (l *defaultLogger) Tracef(format string, v ...any) {
	l.privateLogf(LevelTrace, format, v)
}

func (l *defaultLogger) Debugf(format string, v ...any) {
	l.privateLogf(LevelDebug, format, v)
}

func (l *defaultLogger) Infof(format string, v ...any) {
	l.privateLogf(LevelInfo, format, v)
}

func (l *defaultLogger) Warnf(format string, v ...any) {
	l.privateLogf(LevelWarn, format, v)
}

func (l *defaultLogger) Errorf(format string, v ...any) {
	l.privateLogf(LevelError, format, v)
}

func (l *defaultLogger) Fatalf(format string, v ...any) {
	l.privateLogf(LevelFatal, format, v)
}

func (l *defaultLogger) Panicf(format string, v ...any) {
	l.privateLogf(LevelPanic, format, v)
}

func (l *defaultLogger) Tracew(msg string, keysAndValues ...any) {
	l.privateLogw(LevelTrace, msg, keysAndValues)
}

func (l *defaultLogger) Debugw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelDebug, msg, keysAndValues)
}

func (l *defaultLogger) Infow(msg string, keysAndValues ...any) {
	l.privateLogw(LevelInfo, msg, keysAndValues)
}

func (l *defaultLogger) Warnw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelWarn, msg, keysAndValues)
}

func (l *defaultLogger) Errorw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelError, msg, keysAndValues)
}

func (l *defaultLogger) Fatalw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelFatal, msg, keysAndValues)
}

func (l *defaultLogger) Panicw(msg string, keysAndValues ...any) {
	l.privateLogw(LevelPanic, msg, keysAndValues)
}

func (l *defaultLogger) WithContext(_ context.Context) CommonLogger {
	return &defaultLogger{
		stdlog: l.stdlog,
		level:  l.level,
		depth:  l.depth - 1,
	}
}

func (l *defaultLogger) SetLevel(level Level) {
	l.level = level
}

func (l *defaultLogger) SetOutput(writer io.Writer) {
	l.stdlog.SetOutput(writer)
}

// Logger returns the logger instance. It can be used to adjust the logger configurations in case of need.
func (l *defaultLogger) Logger() any {
	return l.stdlog
}

// DefaultLogger returns the default logger.
func DefaultLogger() AllLogger {
	return logger
}
