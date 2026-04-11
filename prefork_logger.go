package fiber

import (
	"github.com/gofiber/fiber/v3/log"
)

// PreforkLoggerInterface defines a logger for the prefork process manager.
// Compatible with fasthttp/prefork.Logger.
type PreforkLoggerInterface interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...any)
}

// preforkLogger adapts Fiber's logger to the PreforkLoggerInterface.
type preforkLogger struct{}

func (preforkLogger) Printf(format string, args ...any) {
	log.Infof(format, args...)
}
