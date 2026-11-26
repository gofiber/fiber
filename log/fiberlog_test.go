package log

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DefaultSystemLogger(t *testing.T) {
	t.Parallel()
	defaultL := DefaultLogger[*log.Logger]()
	require.Equal(t, logger, defaultL)
}

// Test_DefaultLoggerOtherType covers DefaultLogger asked for a logger type the
// default logger does not wrap: it returns nil.
func Test_DefaultLoggerOtherType(t *testing.T) {
	t.Parallel()
	require.Nil(t, DefaultLogger[*bytes.Buffer]())
}

// Test_FiberlogFatal covers the package-level Fatal functions: each writes
// through the logger SetLogger installed and then exits with code 1.
// conventions:ignore parallel: SetLogger swaps the package-global logger
func Test_FiberlogFatal(t *testing.T) {
	prev := logger
	t.Cleanup(func() { logger = prev })

	var exited []int
	var buf bytes.Buffer
	SetLogger(&defaultLogger{
		stdlog: log.New(&buf, "", 0),
		exit:   func(code int) { exited = append(exited, code) },
		depth:  4,
	})

	Fatal("fatal")
	Fatalf("fatal %d", 1)
	Fatalw("fatal", "k", "v")

	require.Equal(t, []int{1, 1, 1}, exited)
	require.Equal(t, "[Fatal] fatal\n[Fatal] fatal 1\n[Fatal] fatal k=v\n", buf.String())
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_SetLogger(t *testing.T) {
	setLog := &defaultLogger{stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds), depth: 6}

	SetLogger(setLog)
	require.Equal(t, logger, setLog)
}

// logLevelCases are the levels Test_Fiberlog_SetLevel sets, under the name it gives each
// and the name the SetLevel benchmarks give it.
var logLevelCases = []struct {
	name, bench string
	level       Level
}{
	{name: "debug", bench: "Test case 1", level: LevelDebug},
	{name: "info", bench: "Test case 2", level: LevelInfo},
	{name: "warn", bench: "Test case 3", level: LevelWarn},
	{name: "error", bench: "Test case 4", level: LevelError},
	{name: "fatal", bench: "Test case 5", level: LevelFatal},
}

// Test_Fiberlog_SetLevel runs serially because SetLogger swaps the package-global logger.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Fiberlog_SetLevel(t *testing.T) {
	mockLogger := &defaultLogger{}
	SetLogger(mockLogger)

	for _, tc := range logLevelCases {
		t.Run(tc.name, func(t *testing.T) {
			SetLevel(tc.level)
			require.Equal(t, tc.level, mockLogger.level)
		})
	}
}

func Benchmark_DefaultSystemLogger(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = DefaultLogger[*log.Logger]()
	}
}

func Benchmark_SetLogger(b *testing.B) {
	setLog := &defaultLogger{stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds), depth: 6}

	b.ReportAllocs()
	for b.Loop() {
		SetLogger(setLog)
	}
}

func Benchmark_Fiberlog_SetLevel(b *testing.B) {
	mockLogger := &defaultLogger{}
	SetLogger(mockLogger)

	for _, tc := range logLevelCases {
		b.ReportAllocs()
		b.Run(tc.bench, func(b *testing.B) {
			for b.Loop() {
				SetLevel(tc.level)
			}
		})
	}
}

func Benchmark_DefaultSystemLogger_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = DefaultLogger[*log.Logger]()
		}
	})
}

func Benchmark_SetLogger_Parallel(b *testing.B) {
	setLog := &defaultLogger{stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds), depth: 6}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			SetLogger(setLog)
		}
	})
}

func Benchmark_Fiberlog_SetLevel_Parallel(b *testing.B) {
	mockLogger := &defaultLogger{}
	SetLogger(mockLogger)

	for _, tc := range logLevelCases {
		b.Run(tc.bench+"_Parallel", func(bb *testing.B) {
			bb.ReportAllocs()
			bb.ResetTimer()
			bb.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					SetLevel(tc.level)
				}
			})
		})
	}
}
