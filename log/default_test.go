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

func Test_DefaultLogger(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Trace("trace work")
	Debug("received work order")
	Info("starting work")
	Warn("work may fail")
	Error("work failed")
	Panic("work panic")
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
	Panicf("%s panic", work)

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
	WithContext(ctx).Errorf("%s failed", work)
	WithContext(ctx).Panicf("%s panic", work)

	require.Equal(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n"+
		"[Panic] work panic\n", string(w.b))
}

func Test_LogfKeyAndValues(t *testing.T) {
	tests := []struct {
		name          string
		level         Level
		format        string
		fmtArgs       []any
		keysAndValues []any
		wantOutput    string
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

	require.Equal(t, "default_test.go:169: [Info] \ndefault_test.go:170: [Info] \n", string(w.b))
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
