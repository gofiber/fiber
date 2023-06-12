package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

var logger AllLogger = &defaultLogger{
	stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
	depth:  4,
}

// Logger is a logger interface that provides logging function with levels.
type Logger interface {
	Trace(v ...interface{})
	Debug(v ...interface{})
	Info(v ...interface{})
	Warn(v ...interface{})
	Error(v ...interface{})
	Fatal(v ...interface{})
	Panic(v ...interface{})
}

// FormatLogger is a logger interface that output logs with a format.
type FormatLogger interface {
	Tracef(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
	Panicf(format string, v ...interface{})
}

// CtxLogger is a logger interface that accepts a context argument and output logs with a format.
type CtxLogger interface {
	CtxTracef(ctx context.Context, format string, v ...interface{})
	CtxDebugf(ctx context.Context, format string, v ...interface{})
	CtxInfof(ctx context.Context, format string, v ...interface{})
	CtxWarnf(ctx context.Context, format string, v ...interface{})
	CtxErrorf(ctx context.Context, format string, v ...interface{})
	CtxFatalf(ctx context.Context, format string, v ...interface{})
	CtxPanicf(ctx context.Context, format string, v ...interface{})
}

// ControlLogger provides methods to config a logger.
type ControlLogger interface {
	SetLevel(Level)
	SetOutput(io.Writer)
}

// AllLogger is the combination of Logger, FormatLogger, CtxLogger and ControlLogger.
// Custom extensions can be made through AllLogger
type AllLogger interface {
	Logger
	FormatLogger
	CtxLogger
	ControlLogger
	Log(level Level, keyvals ...interface{})
}

// Level defines the priority of a log message.
// When a logger is configured with a level, any log message with a lower
// log level (smaller by integer comparison) will not be output.
type Level int

// The levels of logs.
const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelPanic
)

var strs = []string{
	"[Trace] ",
	"[Debug] ",
	"[Info] ",
	"[Warn] ",
	"[Error] ",
	"[Fatal] ",
	"[Panic] ",
}

func (lv Level) toString() string {
	if lv >= LevelTrace && lv <= LevelPanic {
		return strs[lv]
	}
	return fmt.Sprintf("[?%d] ", lv)
}
