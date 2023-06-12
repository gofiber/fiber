package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

type defaultLogger struct {
	stdlog *log.Logger
	level  Level
	depth  int
}

func (l *defaultLogger) Log(level Level, keyvals ...interface{}) {
	l.logf(level, nil, keyvals...)
}

// logf logs a message at a given level in the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) logf(lv Level, format *string, v ...interface{}) {
	if l.level > lv {
		return
	}
	msg := lv.toString()
	if format != nil {
		msg += fmt.Sprintf(*format, v...)
	} else {
		msg += fmt.Sprint(v...)
	}
	_ = l.stdlog.Output(l.depth, msg) //nolint:errcheck // we don't care about the error here
	if lv == LevelFatal {
		os.Exit(1) //nolint:revive // we want to exit the program when Fatal is called
	}
}

func (l *defaultLogger) Trace(v ...interface{}) {
	l.logf(LevelTrace, nil, v...)
}

func (l *defaultLogger) Debug(v ...interface{}) {
	l.logf(LevelDebug, nil, v...)
}

func (l *defaultLogger) Info(v ...interface{}) {
	l.logf(LevelInfo, nil, v...)
}

func (l *defaultLogger) Warn(v ...interface{}) {
	l.logf(LevelWarn, nil, v...)
}

func (l *defaultLogger) Error(v ...interface{}) {
	l.logf(LevelError, nil, v...)
}

func (l *defaultLogger) Fatal(v ...interface{}) {
	l.logf(LevelFatal, nil, v...)
}

func (l *defaultLogger) Panic(v ...interface{}) {
	l.logf(LevelPanic, nil, v...)
}

func (l *defaultLogger) Tracef(format string, v ...interface{}) {
	l.logf(LevelTrace, &format, v...)
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	l.logf(LevelDebug, &format, v...)
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	l.logf(LevelInfo, &format, v...)
}

func (l *defaultLogger) Warnf(format string, v ...interface{}) {
	l.logf(LevelWarn, &format, v...)
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	l.logf(LevelError, &format, v...)
}

func (l *defaultLogger) Fatalf(format string, v ...interface{}) {
	l.logf(LevelFatal, &format, v...)
}

func (l *defaultLogger) Panicf(format string, v ...interface{}) {
	l.logf(LevelPanic, &format, v...)
}

func (l *defaultLogger) CtxTracef(_ context.Context, format string, v ...interface{}) {
	l.logf(LevelTrace, &format, v...)
}

func (l *defaultLogger) CtxDebugf(_ context.Context, format string, v ...interface{}) {
	l.logf(LevelDebug, &format, v...)
}

func (l *defaultLogger) CtxInfof(_ context.Context, format string, v ...interface{}) {
	l.logf(LevelInfo, &format, v...)
}

func (l *defaultLogger) CtxWarnf(_ context.Context, format string, v ...interface{}) {
	l.logf(LevelWarn, &format, v...)
}

func (l *defaultLogger) CtxErrorf(_ context.Context, format string, v ...interface{}) {
	l.logf(LevelError, &format, v...)
}

func (l *defaultLogger) CtxFatalf(_ context.Context, format string, v ...interface{}) {
	l.logf(LevelFatal, &format, v...)
}

func (l *defaultLogger) CtxPanicf(_ context.Context, format string, v ...interface{}) {
	l.logf(LevelPanic, &format, v...)
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
