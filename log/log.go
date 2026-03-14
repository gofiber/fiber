package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// ContextExtractor extracts a key-value pair from the given context for
// inclusion in log output when using WithContext.
// It returns the log field name, its value, and whether extraction succeeded.
// The ctx parameter can be fiber.Ctx, *fasthttp.RequestCtx, or context.Context.
type ContextExtractor func(ctx any) (string, any, bool)

// contextExtractorsMu guards contextExtractors for concurrent registration
// and snapshot reads.
var contextExtractorsMu sync.RWMutex

// contextExtractors holds all registered context field extractors.
// Use loadContextExtractors to obtain a safe snapshot for iteration.
var contextExtractors []ContextExtractor

// loadContextExtractors returns an immutable snapshot of the registered
// extractors. The returned slice must not be modified.
func loadContextExtractors() []ContextExtractor {
	contextExtractorsMu.RLock()
	snapshot := contextExtractors
	contextExtractorsMu.RUnlock()
	return snapshot
}

// RegisterContextExtractor registers a function that extracts a key-value pair
// from context for inclusion in log output when using WithContext.
//
// This function is safe to call concurrently with logging and with other
// registrations. All calls to RegisterContextExtractor should happen during
// program initialization (e.g. in an init function or middleware constructor)
// so that extractors are in place before requests are processed.
func RegisterContextExtractor(extractor ContextExtractor) {
	if extractor == nil {
		panic("log: RegisterContextExtractor called with nil extractor")
	}
	contextExtractorsMu.Lock()
	// Copy-on-write: always allocate a new backing array so snapshots taken
	// by concurrent readers remain stable.
	n := len(contextExtractors)
	next := make([]ContextExtractor, n+1)
	copy(next, contextExtractors)
	next[n] = extractor
	contextExtractors = next
	contextExtractorsMu.Unlock()
}

// baseLogger defines the minimal logger functionality required by the package.
// It allows storing any logger implementation regardless of its generic type.
type baseLogger interface {
	CommonLogger
	SetLevel(Level)
	SetOutput(io.Writer)
	WithContext(ctx any) CommonLogger
}

var logger baseLogger = &defaultLogger{
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

// CommonLogger is the set of logging operations available across Fiber's
// logging implementations.
type CommonLogger interface {
	Logger
	FormatLogger
	WithLogger
}

// ConfigurableLogger provides methods to config a logger.
type ConfigurableLogger[T any] interface {
	// SetLevel sets logging level.
	//
	// Available levels: Trace, Debug, Info, Warn, Error, Fatal, Panic.
	SetLevel(level Level)

	// SetOutput sets the logger output.
	SetOutput(w io.Writer)

	// Logger returns the logger instance. It can be used to adjust the logger configurations in case of need.
	Logger() T
}

// AllLogger is the combination of Logger, FormatLogger, CtxLogger and ConfigurableLogger.
// Custom extensions can be made through AllLogger
type AllLogger[T any] interface {
	CommonLogger
	ConfigurableLogger[T]

	// WithContext returns a new logger with the given context.
	// The ctx parameter can be fiber.Ctx, *fasthttp.RequestCtx, or context.Context.
	WithContext(ctx any) CommonLogger
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
