package logger

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/stretchr/testify/require"
)

// Test_methodColor_AllMethods exercises every branch of methodColor, including
// the nil-colors short circuit and the default case.
func Test_methodColor_AllMethods(t *testing.T) {
	t.Parallel()
	colors := &fiber.DefaultColors

	require.Empty(t, methodColor(fiber.MethodGet, nil))

	cases := map[string]string{
		fiber.MethodGet:     colors.Cyan,
		fiber.MethodQuery:   colors.Cyan,
		fiber.MethodPost:    colors.Green,
		fiber.MethodPut:     colors.Yellow,
		fiber.MethodDelete:  colors.Red,
		fiber.MethodPatch:   colors.White,
		fiber.MethodHead:    colors.Magenta,
		fiber.MethodOptions: colors.Blue,
		"UNKNOWN":           colors.Reset,
	}
	for method, want := range cases {
		require.Equal(t, want, methodColor(method, colors), "method %q", method)
	}
}

// Test_statusColor_AllRanges exercises every branch of statusColor, including
// the nil-colors short circuit and each status range.
func Test_statusColor_AllRanges(t *testing.T) {
	t.Parallel()
	colors := &fiber.DefaultColors

	require.Empty(t, statusColor(fiber.StatusOK, nil))

	require.Equal(t, colors.Green, statusColor(fiber.StatusOK, colors))
	require.Equal(t, colors.Blue, statusColor(fiber.StatusMovedPermanently, colors))
	require.Equal(t, colors.Yellow, statusColor(fiber.StatusBadRequest, colors))
	require.Equal(t, colors.Red, statusColor(fiber.StatusInternalServerError, colors))
}

// Test_customLoggerWriter_InvalidLevel verifies the default branch of Write
// returns (0, nil) for a level outside the supported set. An unsupported level
// routes to the default case before loggerInstance is ever touched, so a nil
// instance is safe and we avoid mutating any shared/global logger state.
func Test_customLoggerWriter_InvalidLevel(t *testing.T) {
	t.Parallel()
	cl := &customLoggerWriter[any]{level: fiberlog.LevelFatal}

	n, err := cl.Write([]byte("ignored"))
	require.NoError(t, err)
	require.Zero(t, n)
}

func Test_loadTimestamp_Empty(t *testing.T) {
	t.Parallel()
	var v atomic.Value
	require.Empty(t, loadTimestamp(&v))
}

func Test_UnknownTagError_WithHint(t *testing.T) {
	t.Parallel()
	err := &UnknownTagError{Tag: "method", Hint: "did you mean ${method}?"}
	require.Contains(t, err.Error(), "did you mean ${method}?")
	require.Contains(t, err.Error(), `"method"`)
}

func Test_RegisterContextTag(t *testing.T) {
	t.Parallel()
	const tagName = "cov-ctx-tag"
	RegisterContextTag(tagName, func(_ any) string { return "rendered-value" })

	require.Contains(t, loggerTestLog(t, &Config{Format: "${" + tagName + "}\n"}), "rendered-value")

	require.Panics(t, func() { RegisterContextTag("", func(_ any) string { return "" }) })
	require.Panics(t, func() { RegisterContextTag("name", nil) })
}

func Test_RegisterContextTag_EmptyValue(t *testing.T) {
	t.Parallel()
	const tagName = "cov-ctx-empty"
	RegisterContextTag(tagName, func(_ any) string { return "" })

	require.Contains(t, loggerTestLog(t, &Config{Format: "[${" + tagName + "}]\n"}), "[]")
}

// Test_Logger_PreRegisteredMiddlewareTag exercises emptyLogTag: a format that
// references a built-in middleware tag (api-key) compiles and renders nothing
// when the producing middleware has not registered a value.
func Test_Logger_PreRegisteredMiddlewareTag(t *testing.T) {
	t.Parallel()
	require.Contains(t, loggerTestLog(t, &Config{Format: "[${" + fiberlog.TagAPIKey + "}]\n"}), "[]")
}

// Test_Logger_New_TimeDoneUpdater verifies the New path that starts a dedicated
// timestamp updater when TimeDone is configured (covers startTimestampUpdater).
func Test_Logger_New_TimeDoneUpdater(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	defer close(done)

	logged := loggerTestLog(t, &Config{
		Format:       "${time}\n",
		TimeFormat:   time.RFC3339Nano,
		TimeInterval: 5 * time.Millisecond,
		TimeDone:     done,
	})

	// The format always emits a trailing newline, so a non-empty buffer alone
	// would pass even if ${time} rendered empty. Trim it and assert the rendered
	// timestamp is present and well-formed, proving the updater path ran.
	rendered := strings.TrimRight(logged, "\n")
	require.NotEmpty(t, rendered)
	_, parseErr := time.Parse(time.RFC3339Nano, rendered)
	require.NoError(t, parseErr)
}
