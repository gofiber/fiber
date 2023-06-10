package log

import (
	"context"
	"io"
)

// Fatal calls the default logger's Fatal method and then os.Exit(1).
func Fatal(v ...interface{}) {
	logger.Fatal(v...)
}

// Error calls the default logger's Error method.
func Error(v ...interface{}) {
	logger.Error(v...)
}

// Warn calls the default logger's Warn method.
func Warn(v ...interface{}) {
	logger.Warn(v...)
}

// Info calls the default logger's Info method.
func Info(v ...interface{}) {
	logger.Info(v...)
}

// Debug calls the default logger's Debug method.
func Debug(v ...interface{}) {
	logger.Debug(v...)
}

// Trace calls the default logger's Trace method.
func Trace(v ...interface{}) {
	logger.Trace(v...)
}

// Panic calls the default logger's Panic method.
func Panic(v ...interface{}) {
	logger.Panic(v...)
}

// Fatalf calls the default logger's Fatalf method and then os.Exit(1).
func Fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

// Errorf calls the default logger's Errorf method.
func Errorf(format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

// Warnf calls the default logger's Warnf method.
func Warnf(format string, v ...interface{}) {
	logger.Warnf(format, v...)
}

// Infof calls the default logger's Infof method.
func Infof(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

// Debugf calls the default logger's Debugf method.
func Debugf(format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

// Tracef calls the default logger's Tracef method.
func Tracef(format string, v ...interface{}) {
	logger.Tracef(format, v...)
}

// Panicf calls the default logger's Tracef method.
func Panicf(format string, v ...interface{}) {
	logger.Panicf(format, v...)
}

// CtxFatalf calls the default logger's CtxFatalf method and then os.Exit(1).
func CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	logger.CtxFatalf(ctx, format, v...)
}

// CtxErrorf calls the default logger's CtxErrorf method.
func CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	logger.CtxErrorf(ctx, format, v...)
}

// CtxWarnf calls the default logger's CtxWarnf method.
func CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	logger.CtxWarnf(ctx, format, v...)
}

// CtxInfof calls the default logger's CtxInfof method.
func CtxInfof(ctx context.Context, format string, v ...interface{}) {
	logger.CtxInfof(ctx, format, v...)
}

// CtxDebugf calls the default logger's CtxDebugf method.
func CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	logger.CtxDebugf(ctx, format, v...)
}

// CtxTracef calls the default logger's CtxTracef method.
func CtxTracef(ctx context.Context, format string, v ...interface{}) {
	logger.CtxTracef(ctx, format, v...)
}

// CtxPanicf calls the default logger's CtxPanicf method.
func CtxPanicf(ctx context.Context, format string, v ...interface{}) {
	logger.CtxPanicf(ctx, format, v...)
}

// SetLogger sets the default logger and the system logger.
// Note that this method is not concurrent-safe and must not be called
// after the use of DefaultLogger and global functions in this package.
func SetLogger(v AllLogger) {
	logger = v
}

// SetOutput sets the output of default logger and system logger. By default, it is stderr.
func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

// SetLevel sets the level of logs below which logs will not be output.
// The default logger and system logger level is LevelTrace.
// Note that this method is not concurrent-safe.
func SetLevel(lv Level) {
	logger.SetLevel(lv)
}
