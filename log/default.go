package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"reflect"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
)

var _ AllLogger[*log.Logger] = (*defaultLogger)(nil)

type defaultLogger struct {
	stdlog *log.Logger
	ctx    retainedContext
	level  Level
	depth  int
}

// retainedContext documents that WithContext intentionally returns a logger
// bound to a caller-provided context-like value. The value may be fiber.Ctx,
// *fasthttp.RequestCtx, context.Context, or another value understood by
// ContextTagFunc implementations.
type retainedContext struct {
	value any
	ok    bool
}

// newRetainedContext wraps value, treating both untyped nil and typed-nil
// pointers/interfaces/maps/slices/channels/funcs as "no context". A typed nil
// (e.g. (*fasthttp.RequestCtx)(nil) stored in an `any`) compares non-nil under
// `value != nil` because the interface header carries a non-nil type
// descriptor; passing it through to a tag renderer would cause the renderer
// to dereference a nil receiver. Reflection here is one-shot per WithContext
// call — far off the per-log-line hot path.
func newRetainedContext(value any) retainedContext {
	if value == nil {
		return retainedContext{}
	}
	switch rv := reflect.ValueOf(value); rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			return retainedContext{}
		}
	default:
		// Non-nilable kinds (struct, int, string, ...) cannot be typed-nil;
		// fall through to wrap the value as-is.
	}
	return retainedContext{value: value, ok: true}
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
	l.writeContext(buf)
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

// privateLogf logs a formatted message at a given level log the default logger.
// when the level is fatal, it will exit the program.
func (l *defaultLogger) privateLogf(lv Level, format string, fmtArgs []any) {
	if l.level > lv {
		return
	}
	level := lv.toString()
	buf := bytebufferpool.Get()
	buf.WriteString(level)
	l.writeContext(buf)

	if len(fmtArgs) > 0 {
		_, _ = fmt.Fprintf(buf, format, fmtArgs...)
	} else {
		_, _ = fmt.Fprint(buf, format)
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
	l.writeContext(buf)

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
			switch key := keysAndValues[i].(type) {
			case string:
				buf.WriteString(key)
			default:
				_, _ = fmt.Fprint(buf, key)
			}
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

// writeContext renders the configured context template directly into buf.
// Rendering uses a scratch buffer so that a partially-rendered template (one
// that errors mid-tag) does not leak its prefix into the real log line. When
// rendering fails, a short marker is appended instead of the prefix so the
// failure is visible in the log stream rather than silently producing
// context-less log lines forever.
func (l *defaultLogger) writeContext(buf Buffer) {
	if !l.ctx.ok {
		return
	}

	tmpl := contextTemplate.Load()
	if tmpl == nil {
		return
	}

	scratch := bytebufferpool.Get()
	defer bytebufferpool.Put(scratch)

	if err := tmpl.Execute(scratch, l.ctx.value, &ContextData{}); err != nil {
		// The error string can carry CR/LF/ANSI bytes derived from request data
		// (e.g. when a tag wraps an upstream parsing error around a header). Run
		// it through the same sanitiser the ${value:KEY} renderer uses so a
		// misconfigured tag cannot become a log-injection vector.
		_, _ = buf.WriteString("[ctx-render-error: ") //nolint:errcheck // best-effort marker
		_, _ = writeSanitizedString(buf, err.Error()) //nolint:errcheck // best-effort marker
		_, _ = buf.WriteString("] ")                  //nolint:errcheck // best-effort marker
		return
	}

	_, _ = buf.Write(scratch.Bytes()) //nolint:errcheck // best-effort write; outer log.Output reports IO errors
}

// WithContext returns a logger that shares the underlying output and renders configured contextual fields.
func (l *defaultLogger) WithContext(ctx any) CommonLogger {
	return &defaultLogger{
		stdlog: l.stdlog,
		ctx:    newRetainedContext(ctx),
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
