package log

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const work = "work"

func initDefaultLogger() {
	logger = &defaultLogger{
		stdlog: log.New(os.Stderr, "", 0),
		depth:  4,
	}
}

type byteSliceWriter struct {
	b []byte
}

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func Test_WithContextCaller(t *testing.T) {
	logger = &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.Lshortfile),
		depth:  4,
	}

	var w byteSliceWriter
	SetOutput(&w)
	ctx := context.TODO()

	WithContext(ctx).Info("")
	Info("")

	require.Equal(t, "default_test.go:41: [Info] \ndefault_test.go:42: [Info] \n", string(w.b))
}

func Test_DefaultLogger(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Trace("trace work")
	Debug("received work order")
	Info("starting work")
	Warn("work may fail")
	Error("work failed")

	require.Panics(t, func() {
		Panic("work panic")
	})

	require.Equal(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n"+
		"[Panic] work panic\n", string(w.b))
}

func Test_DefaultFormatLogger(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Tracef("trace %s", work)
	Debugf("received %s order", work)
	Infof("starting %s", work)
	Warnf("%s may fail", work)
	Errorf("%s failed", work)

	require.Panics(t, func() {
		Panicf("%s panic", work)
	})

	require.Equal(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n"+
		"[Panic] work panic\n", string(w.b))
}

func Test_CtxLogger(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	ctx := context.Background()

	WithContext(ctx).Tracef("trace %s", work)
	WithContext(ctx).Debugf("received %s order", work)
	WithContext(ctx).Infof("starting %s", work)
	WithContext(ctx).Warnf("%s may fail", work)
	WithContext(ctx).Errorf("%s failed %d", work, 50)

	require.Panics(t, func() {
		WithContext(ctx).Panicf("%s panic", work)
	})

	require.Equal(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed 50\n"+
		"[Panic] work panic\n", string(w.b))
}

type testContextKey struct{}

func Test_WithContextExtractor(t *testing.T) {
	// Save and restore global extractors
	saved := contextExtractors
	defer func() { contextExtractors = saved }()
	contextExtractors = nil

	RegisterContextExtractor(func(ctx context.Context) (string, any, bool) {
		if v, ok := ctx.Value(testContextKey{}).(string); ok && v != "" {
			return "request-id", v, true
		}
		return "", nil, false
	})

	t.Run("Info with context field", func(t *testing.T) {
		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}
		ctx := context.WithValue(context.Background(), testContextKey{}, "abc-123")
		l.WithContext(ctx).Info("hello")

		require.Equal(t, "[Info] request-id=abc-123 hello\n", buf.String())
	})

	t.Run("Infof with context field", func(t *testing.T) {
		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}
		ctx := context.WithValue(context.Background(), testContextKey{}, "abc-123")
		l.WithContext(ctx).Infof("hello %s", "world")

		require.Equal(t, "[Info] request-id=abc-123 hello world\n", buf.String())
	})

	t.Run("Infow with context field", func(t *testing.T) {
		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}
		ctx := context.WithValue(context.Background(), testContextKey{}, "abc-123")
		l.WithContext(ctx).Infow("hello", "key", "value")

		require.Equal(t, "[Info] request-id=abc-123 hello key=value\n", buf.String())
	})

	t.Run("no context field when value absent", func(t *testing.T) {
		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}
		ctx := context.Background()
		l.WithContext(ctx).Info("hello")

		require.Equal(t, "[Info] hello\n", buf.String())
	})

	t.Run("no context field without WithContext", func(t *testing.T) {
		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}
		l.Info("hello")

		require.Equal(t, "[Info] hello\n", buf.String())
	})

	t.Run("empty key extractor is skipped", func(t *testing.T) {
		// Save and restore extractors for this subtest
		savedInner := contextExtractors
		defer func() { contextExtractors = savedInner }()

		// Add an extractor that returns ok=true but key=""
		RegisterContextExtractor(func(_ context.Context) (string, any, bool) {
			return "", "should-not-appear", true
		})

		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}
		ctx := context.WithValue(context.Background(), testContextKey{}, "abc-123")
		l.WithContext(ctx).Info("hello")

		require.Equal(t, "[Info] request-id=abc-123 hello\n", buf.String())
	})
}

func Test_LogfKeyAndValues(t *testing.T) {
	tests := []struct {
		name          string
		format        string
		wantOutput    string
		fmtArgs       []any
		keysAndValues []any
		level         Level
	}{
		{
			name:          "test logf with debug level and key-values",
			level:         LevelDebug,
			format:        "",
			fmtArgs:       nil,
			keysAndValues: []any{"name", "Bob", "age", 30},
			wantOutput:    "[Debug] name=Bob age=30\n",
		},
		{
			name:          "test logf with info level and key-values",
			level:         LevelInfo,
			format:        "",
			fmtArgs:       nil,
			keysAndValues: []any{"status", "ok", "code", 200},
			wantOutput:    "[Info] status=ok code=200\n",
		},
		{
			name:          "test logf with warn level and key-values",
			level:         LevelWarn,
			format:        "",
			fmtArgs:       nil,
			keysAndValues: []any{"error", "not found", "id", 123},
			wantOutput:    "[Warn] error=not found id=123\n",
		},
		{
			name:          "test logf with format and key-values",
			level:         LevelWarn,
			format:        "test",
			fmtArgs:       nil,
			keysAndValues: []any{"error", "not found", "id", 123},
			wantOutput:    "[Warn] test error=not found id=123\n",
		},
		{
			name:          "test logf with one key",
			level:         LevelWarn,
			format:        "",
			fmtArgs:       nil,
			keysAndValues: []any{"error"},
			wantOutput:    "[Warn] error=KEYVALS UNPAIRED\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := &defaultLogger{
				stdlog: log.New(&buf, "", 0),
				level:  tt.level,
				depth:  4,
			}
			l.privateLogw(tt.level, tt.format, tt.keysAndValues)
			require.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func Test_SetLevel(t *testing.T) {
	setLogger := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  4,
	}

	setLogger.SetLevel(LevelTrace)
	require.Equal(t, LevelTrace, setLogger.level)
	require.Equal(t, LevelTrace.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelDebug)
	require.Equal(t, LevelDebug, setLogger.level)
	require.Equal(t, LevelDebug.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelInfo)
	require.Equal(t, LevelInfo, setLogger.level)
	require.Equal(t, LevelInfo.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelWarn)
	require.Equal(t, LevelWarn, setLogger.level)
	require.Equal(t, LevelWarn.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelError)
	require.Equal(t, LevelError, setLogger.level)
	require.Equal(t, LevelError.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelFatal)
	require.Equal(t, LevelFatal, setLogger.level)
	require.Equal(t, LevelFatal.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelPanic)
	require.Equal(t, LevelPanic, setLogger.level)
	require.Equal(t, LevelPanic.toString(), setLogger.level.toString())

	setLogger.SetLevel(8)
	require.Equal(t, 8, int(setLogger.level))
	require.Equal(t, "[?8] ", setLogger.level.toString())
}

func Test_Logger(t *testing.T) {
	underlyingLogger := log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds)
	setLogger := &defaultLogger{
		stdlog: underlyingLogger,
		depth:  4,
	}

	require.Equal(t, underlyingLogger, setLogger.Logger())

	logger := setLogger.Logger()
	logger.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	require.Equal(t, log.LstdFlags|log.Lshortfile|log.Lmicroseconds, setLogger.stdlog.Flags())
}

func Test_Debugw(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	msg := "debug work"
	keysAndValues := []any{"key1", "value1", "key2", "value2"}

	Debugw(msg, keysAndValues...)

	require.Equal(t, "[Debug] debug work key1=value1 key2=value2\n", string(w.b))
}

func Test_Infow(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	msg := "info work"
	keysAndValues := []any{"key1", "value1", "key2", "value2"}

	Infow(msg, keysAndValues...)

	require.Equal(t, "[Info] info work key1=value1 key2=value2\n", string(w.b))
}

func Test_Warnw(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	msg := "warning work"
	keysAndValues := []any{"key1", "value1", "key2", "value2"}

	Warnw(msg, keysAndValues...)

	require.Equal(t, "[Warn] warning work key1=value1 key2=value2\n", string(w.b))
}

func Test_Errorw(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	msg := "error work"
	keysAndValues := []any{"key1", "value1", "key2", "value2"}

	Errorw(msg, keysAndValues...)

	require.Equal(t, "[Error] error work key1=value1 key2=value2\n", string(w.b))
}

func Test_Panicw(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	msg := "panic work"
	keysAndValues := []any{"key1", "value1", "key2", "value2"}

	require.Panics(t, func() {
		Panicw(msg, keysAndValues...)
	})

	require.Equal(t, "[Panic] panic work key1=value1 key2=value2\n", string(w.b))
}

func Test_Tracew(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	msg := "trace work"
	keysAndValues := []any{"key1", "value1", "key2", "value2"}

	Tracew(msg, keysAndValues...)

	require.Equal(t, "[Trace] trace work key1=value1 key2=value2\n", string(w.b))
}

type stringKey struct {
	value string
}

func (k stringKey) String() string {
	return "key:" + k.value
}

func Test_DefaultLoggerNonStringKeys(t *testing.T) {
	t.Parallel()

	t.Run("Tracew with non-string keys", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}

		require.NotPanics(t, func() {
			l.Tracew("trace", 123, "value", stringKey{value: "alpha"}, 42)
		})

		require.Equal(t, "[Trace] trace 123=value key:alpha=42\n", buf.String())
	})

	t.Run("Infow with non-string keys", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		l := &defaultLogger{
			stdlog: log.New(&buf, "", 0),
			level:  LevelTrace,
			depth:  4,
		}

		require.NotPanics(t, func() {
			l.Infow("info", 456, "value", stringKey{value: "beta"}, 7)
		})

		require.Equal(t, "[Info] info 456=value key:beta=7\n", buf.String())
	})
}

func Benchmark_LogfKeyAndValues(b *testing.B) {
	tests := []struct {
		name          string
		format        string
		keysAndValues []any
		level         Level
	}{
		{
			name:          "test logf with debug level and key-values",
			level:         LevelDebug,
			format:        "",
			keysAndValues: []any{"name", "Bob", "age", 30},
		},
		{
			name:          "test logf with info level and key-values",
			level:         LevelInfo,
			format:        "",
			keysAndValues: []any{"status", "ok", "code", 200},
		},
		{
			name:          "test logf with warn level and key-values",
			level:         LevelWarn,
			format:        "",
			keysAndValues: []any{"error", "not found", "id", 123},
		},
		{
			name:          "test logf with format and key-values",
			level:         LevelWarn,
			format:        "test",
			keysAndValues: []any{"error", "not found", "id", 123},
		},
		{
			name:          "test logf with one key",
			level:         LevelWarn,
			format:        "",
			keysAndValues: []any{"error"},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(bb *testing.B) {
			var buf bytes.Buffer
			l := &defaultLogger{
				stdlog: log.New(&buf, "", 0),
				level:  tt.level,
				depth:  4,
			}

			bb.ReportAllocs()

			for bb.Loop() {
				l.privateLogw(tt.level, tt.format, tt.keysAndValues)
			}
		})
	}
}

func Benchmark_LogfKeyAndValues_Parallel(b *testing.B) {
	tests := []struct {
		name          string
		format        string
		keysAndValues []any
		level         Level
	}{
		{
			name:          "debug level with key-values",
			level:         LevelDebug,
			format:        "",
			keysAndValues: []any{"name", "Bob", "age", 30},
		},
		{
			name:          "info level with key-values",
			level:         LevelInfo,
			format:        "",
			keysAndValues: []any{"status", "ok", "code", 200},
		},
		{
			name:          "warn level with key-values",
			level:         LevelWarn,
			format:        "",
			keysAndValues: []any{"error", "not found", "id", 123},
		},
		{
			name:          "warn level with format and key-values",
			level:         LevelWarn,
			format:        "test",
			keysAndValues: []any{"error", "not found", "id", 123},
		},
		{
			name:          "warn level with one key",
			level:         LevelWarn,
			format:        "",
			keysAndValues: []any{"error"},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(bb *testing.B) {
			bb.ReportAllocs()
			bb.ResetTimer()
			bb.RunParallel(func(pb *testing.PB) {
				var buf bytes.Buffer
				l := &defaultLogger{
					stdlog: log.New(&buf, "", 0),
					level:  tt.level,
					depth:  4,
				}
				for pb.Next() {
					l.privateLogw(tt.level, tt.format, tt.keysAndValues)
				}
			})
		})
	}
}
