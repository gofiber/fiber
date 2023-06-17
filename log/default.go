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

// log logs a message at a given level in the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) log(lv Level, format string, fmtArgs, keysAndValues []interface{}) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	buf.WriteString(level) //nolint:errcheck

	// Write format in buffer
	if format != "" {
		buf.WriteString(format) //nolint:errcheck
	}
	var once sync.Once
	var isFirst bool = true
	// Write keys and values in buffer
	if len(keysAndValues) > 0 {
		if (len(keysAndValues) & 1) == 1 {
			keysAndValues = append(keysAndValues, "KEYVALS UNPAIRED")
		}

		for i := 0; i < len(keysAndValues); i += 2 {
			if format == "" && isFirst {
				once.Do(func() {
					_, _ = fmt.Fprintf(buf, "%s=%v", keysAndValues[i], keysAndValues[i+1]) //nolint:errcheck
					isFirst = false
				})
				continue
			}
			_, _ = fmt.Fprintf(buf, " %s=%v", keysAndValues[i], keysAndValues[i+1]) //nolint:errcheck
		}
	}

	if len(fmtArgs) > 0 {
		fmt.Fprintf(buf, format, fmtArgs...) //nolint:errcheck
	} else {
		fmt.Fprint(buf, fmtArgs...) //nolint:errcheck
	}
	_ = l.stdlog.Output(l.depth, buf.String()) //nolint:errcheck // we don't care about the error here
	buf.Reset()
	bytebufferpool.Put(buf)
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

func (l *defaultLogger) Trace(v ...interface{}) {
	l.log(LevelTrace, "", v, nil)
}

func (l *defaultLogger) Debug(v ...interface{}) {
	l.log(LevelDebug, "", v, nil)
}

func (l *defaultLogger) Info(v ...interface{}) {
	l.log(LevelInfo, "", v, nil)
}

func (l *defaultLogger) Warn(v ...interface{}) {
	l.log(LevelWarn, "", v, nil)
}

func (l *defaultLogger) Error(v ...interface{}) {
	l.log(LevelError, "", v, nil)
}

func (l *defaultLogger) Fatal(v ...interface{}) {
	l.log(LevelFatal, "", v, nil)
}

func (l *defaultLogger) Panic(v ...interface{}) {
	l.log(LevelPanic, "", v, nil)
}

func (l *defaultLogger) Tracef(format string, v ...interface{}) {
	l.log(LevelTrace, format, v, nil)
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	l.log(LevelDebug, format, v, nil)
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	l.log(LevelInfo, format, v, nil)
}

func (l *defaultLogger) Warnf(format string, v ...interface{}) {
	l.log(LevelWarn, format, v, nil)
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	l.log(LevelError, format, v, nil)
}

func (l *defaultLogger) Fatalf(format string, v ...interface{}) {
	l.log(LevelFatal, format, v, nil)
}

func (l *defaultLogger) Panicf(format string, v ...interface{}) {
	l.log(LevelPanic, format, v, nil)
}

func (l *defaultLogger) Tracew(msg string, keysAndValues ...interface{}) {
	l.log(LevelTrace, msg, nil, keysAndValues)
}

func (l *defaultLogger) Debugw(msg string, keysAndValues ...interface{}) {
	l.log(LevelDebug, msg, nil, keysAndValues)
}

func (l *defaultLogger) Infow(msg string, keysAndValues ...interface{}) {
	l.log(LevelInfo, msg, nil, keysAndValues)
}

func (l *defaultLogger) Warnw(msg string, keysAndValues ...interface{}) {
	l.log(LevelWarn, msg, nil, keysAndValues)
}

func (l *defaultLogger) Errorw(msg string, keysAndValues ...interface{}) {
	l.log(LevelError, msg, nil, keysAndValues)
}

func (l *defaultLogger) Fatalw(msg string, keysAndValues ...interface{}) {
	l.log(LevelFatal, msg, nil, keysAndValues)
}

func (l *defaultLogger) Panicw(msg string, keysAndValues ...interface{}) {
	l.log(LevelPanic, msg, nil, keysAndValues)
}

func (l *defaultLogger) CtxTracef(_ context.Context, format string, v ...interface{}) {
	l.log(LevelTrace, format, v, nil)
}

func (l *defaultLogger) CtxDebugf(_ context.Context, format string, v ...interface{}) {
	l.log(LevelDebug, format, v, nil)
}

func (l *defaultLogger) CtxInfof(_ context.Context, format string, v ...interface{}) {
	l.log(LevelInfo, format, v, nil)
}

func (l *defaultLogger) CtxWarnf(_ context.Context, format string, v ...interface{}) {
	l.log(LevelWarn, format, v, nil)
}

func (l *defaultLogger) CtxErrorf(_ context.Context, format string, v ...interface{}) {
	l.log(LevelError, format, v, nil)
}

func (l *defaultLogger) CtxFatalf(_ context.Context, format string, v ...interface{}) {
	l.log(LevelFatal, format, v, nil)
}

func (l *defaultLogger) CtxPanicf(_ context.Context, format string, v ...interface{}) {
	l.log(LevelPanic, format, v, nil)
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
