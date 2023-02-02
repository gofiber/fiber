package recover //nolint:predeclared // TODO: Rename to some non-builtin

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
)

func defaultStackTraceHandler(_ *fiber.Ctx, e interface{}) {
	_, _ = os.Stderr.WriteString(fmt.Sprintf("panic: %v\n%s\n", e, debug.Stack())) //nolint:errcheck // This will never fail
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *fiber.Ctx) (err error) { //nolint:nonamedreturns // Uses recover() to overwrite the error
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Catch panics
		defer func() {
			if r := recover(); r != nil {
				if cfg.EnableStackTrace {
					cfg.StackTraceHandler(c, r)
				}

				var ok bool
				if err, ok = r.(error); !ok {
					// Set error that will call the global error handler
					err = fmt.Errorf("%v", r)
				}
			}
		}()

		// Return err if exist, else move to next handler
		return c.Next()
	}
}
