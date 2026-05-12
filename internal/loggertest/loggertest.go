// Package loggertest provides shared helpers for the middleware ctx-logger
// integration tests. The helpers consolidate the SetOutput / SetContextTemplate
// / cleanup boilerplate that would otherwise be duplicated across every
// middleware that registers a context tag.
package loggertest

import (
	"bytes"
	"os"
	"testing"

	fiberlog "github.com/gofiber/fiber/v3/log"
)

// CaptureContextLog redirects the default fiberlog output into a fresh buffer
// and configures the given context format. On test cleanup, the default
// logger output is restored to os.Stderr and the context template is cleared.
//
// Tests that use this helper must NOT call t.Parallel() because the helper
// mutates package-global default-logger state shared across tests.
func CaptureContextLog(tb testing.TB, format string) *bytes.Buffer {
	tb.Helper()

	var buf bytes.Buffer
	fiberlog.SetOutput(&buf)
	fiberlog.MustSetContextTemplate(fiberlog.ContextConfig{Format: format})

	tb.Cleanup(func() {
		fiberlog.MustSetContextTemplate(fiberlog.ContextConfig{})
		fiberlog.SetOutput(os.Stderr)
	})

	return &buf
}
