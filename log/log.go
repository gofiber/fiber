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
	Trace(v ...any)
	Debug(v ...any)
	Info(v ...any)
	Warn(v ...any)
	Error(v ...any)
	Fatal(v ...any)
	Panic(v ...any)
}

// FormatLogger is a logger interface that output logs with a format.
type FormatLogger interface {
	Tracef(format string, v ...any)
	Debugf(format string, v ...any)
	Infof(format string, v ...any)
	Warnf(format string, v ...any)
	Errorf(format string, v ...any)
	Fatalf(format string, v ...any)
	Panicf(format string, v ...any)
}

// WithLogger is a logger interface that output logs with a message and key-value pairs.
type WithLogger interface {
	Tracew(msg string, keysAndValues ...any)
	Debugw(msg string, keysAndValues ...any)
	Infow(msg string, keysAndValues ...any)
	Warnw(msg string, keysAndValues ...any)
	Errorw(msg string, keysAndValues ...any)
	Fatalw(msg string, keysAndValues ...any)
	Panicw(msg string, keysAndValues ...any)
}

type CommonLogger interface {
	Logger
	FormatLogger
	WithLogger
}

// ControlLogger provides methods to config a logger.
type ControlLogger interface {
	SetLevel(Level)
	SetOutput(io.Writer)
}

// AllLogger is the combination of Logger, FormatLogger, CtxLogger and ControlLogger.
// Custom extensions can be made through AllLogger
type AllLogger interface {
	CommonLogger
	ControlLogger
	WithContext(ctx context.Context) CommonLogger
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
