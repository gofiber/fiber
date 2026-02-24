package recover //nolint:predeclared // TODO: Rename to some non-builtin

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
)

func defaultStackTraceHandler(_ fiber.Ctx, e any) {
	fmt.Fprintf(os.Stderr, "panic: %v\n\n%s\n", e, debug.Stack())
}

func defaultErrorCustomizer(_ fiber.Ctx, r any) error {
	if err, ok := r.(error); ok {
		return err
	}
	return fmt.Errorf("%v", r)
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c fiber.Ctx) (err error) { //nolint:nonamedreturns // Uses recover() to overwrite the error
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

				// Set error that will call the global error handler
				err = cfg.ErrorCustomizer(c, r)
			}
		}()

		// Return err if exist, else move to next handler
		return c.Next()
	}
}
