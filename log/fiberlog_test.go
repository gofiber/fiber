package log

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DefaultSystemLogger(t *testing.T) {
	t.Parallel()
	defaultL := DefaultLogger()
	require.Equal(t, logger, defaultL)
}

func Test_SetLogger(t *testing.T) {
	t.Parallel()
	setLog := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  6,
	}

	SetLogger(setLog)
	require.Equal(t, logger, setLog)
}

func Test_Fiberlog_SetLevel(t *testing.T) {
	mockLogger := &defaultLogger{}
	SetLogger(mockLogger)

	// Test cases
	testCases := []struct {
		name     string
		level    Level
		expected Level
	}{
		{
			name:     "Test case 1",
			level:    LevelDebug,
			expected: LevelDebug,
		},
		{
			name:     "Test case 2",
			level:    LevelInfo,
			expected: LevelInfo,
		},
		{
			name:     "Test case 3",
			level:    LevelWarn,
			expected: LevelWarn,
		},
		{
			name:     "Test case 4",
			level:    LevelError,
			expected: LevelError,
		},
		{
			name:     "Test case 5",
			level:    LevelFatal,
			expected: LevelFatal,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SetLevel(tc.level)
			require.Equal(t, tc.expected, mockLogger.level)
		})
	}
}
