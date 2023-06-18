package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

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
func (l *defaultLogger) privateLog(lv Level, fmtArgs []interface{}) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	_, _ = buf.WriteString(level)                  //nolint:errcheck // It is fine to ignore the error
	_, _ = buf.WriteString(fmt.Sprint(fmtArgs...)) //nolint:errcheck // It is fine to ignore the error

	_ = l.stdlog.Output(l.depth, buf.String()) //nolint:errcheck // It is fine to ignore the error
	buf.Reset()
	bytebufferpool.Put(buf)
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

// privateLog logs a message at a given level log the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) privateLogf(lv Level, format string, fmtArgs []interface{}) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	_, _ = buf.WriteString(level) //nolint:errcheck // It is fine to ignore the error

	if len(fmtArgs) > 0 {
		_, _ = fmt.Fprintf(buf, format, fmtArgs...)
	} else {
		_, _ = fmt.Fprint(buf, fmtArgs...)
	}
	_ = l.stdlog.Output(l.depth, buf.String()) //nolint:errcheck // It is fine to ignore the error
	buf.Reset()
	bytebufferpool.Put(buf)
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

// privateLogw logs a message at a given level log the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) privateLogw(lv Level, format string, keysAndValues []interface{}) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	_, _ = buf.WriteString(level) //nolint:errcheck // It is fine to ignore the error

	// Write format privateLog buffer
	if format != "" {
		_, _ = buf.WriteString(format) //nolint:errcheck // It is fine to ignore the error
	}
	var once sync.Once
	isFirst := true
	// Write keys and values privateLog buffer
	if len(keysAndValues) > 0 {
		if (len(keysAndValues) & 1) == 1 {
			keysAndValues = append(keysAndValues, "KEYVALS UNPAIRED")
		}

		for i := 0; i < len(keysAndValues); i += 2 {
			if format == "" && isFirst {
				once.Do(func() {
					_, _ = fmt.Fprintf(buf, "%s=%v", keysAndValues[i], keysAndValues[i+1])
					isFirst = false
				})
				continue
			}
			_, _ = fmt.Fprintf(buf, " %s=%v", keysAndValues[i], keysAndValues[i+1])
		}
	}

	_ = l.stdlog.Output(l.depth, buf.String()) //nolint:errcheck // It is fine to ignore the error
	buf.Reset()
	bytebufferpool.Put(buf)
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

func (l *defaultLogger) Trace(v ...interface{}) {
	l.privateLog(LevelTrace, v)
}

func (l *defaultLogger) Debug(v ...interface{}) {
	l.privateLog(LevelDebug, v)
}

func (l *defaultLogger) Info(v ...interface{}) {
	l.privateLog(LevelInfo, v)
}

func (l *defaultLogger) Warn(v ...interface{}) {
	l.privateLog(LevelWarn, v)
}

func (l *defaultLogger) Error(v ...interface{}) {
	l.privateLog(LevelError, v)
}

func (l *defaultLogger) Fatal(v ...interface{}) {
	l.privateLog(LevelFatal, v)
}

func (l *defaultLogger) Panic(v ...interface{}) {
	l.privateLog(LevelPanic, v)
}

func (l *defaultLogger) Tracef(format string, v ...interface{}) {
	l.privateLogf(LevelTrace, format, v)
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	l.privateLogf(LevelDebug, format, v)
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	l.privateLogf(LevelInfo, format, v)
}

func (l *defaultLogger) Warnf(format string, v ...interface{}) {
	l.privateLogf(LevelWarn, format, v)
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	l.privateLogf(LevelError, format, v)
}

func (l *defaultLogger) Fatalf(format string, v ...interface{}) {
	l.privateLogf(LevelFatal, format, v)
}

func (l *defaultLogger) Panicf(format string, v ...interface{}) {
	l.privateLogf(LevelPanic, format, v)
}

func (l *defaultLogger) Tracew(msg string, keysAndValues ...interface{}) {
	l.privateLogw(LevelTrace, msg, keysAndValues)
}

func (l *defaultLogger) Debugw(msg string, keysAndValues ...interface{}) {
	l.privateLogw(LevelDebug, msg, keysAndValues)
}

func (l *defaultLogger) Infow(msg string, keysAndValues ...interface{}) {
	l.privateLogw(LevelInfo, msg, keysAndValues)
}

func (l *defaultLogger) Warnw(msg string, keysAndValues ...interface{}) {
	l.privateLogw(LevelWarn, msg, keysAndValues)
}

func (l *defaultLogger) Errorw(msg string, keysAndValues ...interface{}) {
	l.privateLogw(LevelError, msg, keysAndValues)
}

func (l *defaultLogger) Fatalw(msg string, keysAndValues ...interface{}) {
	l.privateLogw(LevelFatal, msg, keysAndValues)
}

func (l *defaultLogger) Panicw(msg string, keysAndValues ...interface{}) {
	l.privateLogw(LevelPanic, msg, keysAndValues)
}

func (l *defaultLogger) WithContext(_ context.Context) CommonLogger {
	return l
}

func (l *defaultLogger) SetLevel(level Level) {
	l.level = level
}

func (l *defaultLogger) SetOutput(writer io.Writer) {
	l.stdlog.SetOutput(writer)
}

// DefaultLogger returns the default logger.
func DefaultLogger() AllLogger {
	return logger
}
