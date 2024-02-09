package log

import (
	"context"
	"io"
)

// Fatal calls the default logger's Fatal method and then os.Exit(1).
func Fatal(v ...any) {
	logger.Fatal(v...)
}

// Error calls the default logger's Error method.
func Error(v ...any) {
	logger.Error(v...)
}

// Warn calls the default logger's Warn method.
func Warn(v ...any) {
	logger.Warn(v...)
}

// Info calls the default logger's Info method.
func Info(v ...any) {
	logger.Info(v...)
}

// Debug calls the default logger's Debug method.
func Debug(v ...any) {
	logger.Debug(v...)
}

// Trace calls the default logger's Trace method.
func Trace(v ...any) {
	logger.Trace(v...)
}

// Panic calls the default logger's Panic method.
func Panic(v ...any) {
	logger.Panic(v...)
}

// Fatalf calls the default logger's Fatalf method and then os.Exit(1).
func Fatalf(format string, v ...any) {
	logger.Fatalf(format, v...)
}

// Errorf calls the default logger's Errorf method.
func Errorf(format string, v ...any) {
	logger.Errorf(format, v...)
}

// Warnf calls the default logger's Warnf method.
func Warnf(format string, v ...any) {
	logger.Warnf(format, v...)
}

// Infof calls the default logger's Infof method.
func Infof(format string, v ...any) {
	logger.Infof(format, v...)
}

// Debugf calls the default logger's Debugf method.
func Debugf(format string, v ...any) {
	logger.Debugf(format, v...)
}

// Tracef calls the default logger's Tracef method.
func Tracef(format string, v ...any) {
	logger.Tracef(format, v...)
}

// Panicf calls the default logger's Tracef method.
func Panicf(format string, v ...any) {
	logger.Panicf(format, v...)
}

// Tracew logs a message with some additional context. The variadic key-value
// pairs are treated as they are privateLog With.
func Tracew(msg string, keysAndValues ...any) {
	logger.Tracew(msg, keysAndValues...)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are privateLog With.
func Debugw(msg string, keysAndValues ...any) {
	logger.Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are privateLog With.
func Infow(msg string, keysAndValues ...any) {
	logger.Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are privateLog With.
func Warnw(msg string, keysAndValues ...any) {
	logger.Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are privateLog With.
func Errorw(msg string, keysAndValues ...any) {
	logger.Errorw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context. The variadic key-value
// pairs are treated as they are privateLog With.
func Fatalw(msg string, keysAndValues ...any) {
	logger.Fatalw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context. The variadic key-value
// pairs are treated as they are privateLog With.
func Panicw(msg string, keysAndValues ...any) {
	logger.Panicw(msg, keysAndValues...)
}

func WithContext(ctx context.Context) CommonLogger {
	return logger.WithContext(ctx)
}

// SetLogger sets the default logger and the system logger.
// Note that this method is not concurrent-safe and must not be called
// after the use of DefaultLogger and global functions privateLog this package.
func SetLogger(v AllLogger) {
	logger = v
}

// SetOutput sets the output of default logger and system logger. By default, it is stderr.
func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

// SetLevel sets the level of logs below which logs will not be output.
// The default logger is LevelTrace.
// Note that this method is not concurrent-safe.
func SetLevel(lv Level) {
	logger.SetLevel(lv)
}
