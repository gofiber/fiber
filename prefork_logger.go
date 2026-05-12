package fiber

import (
	"github.com/gofiber/fiber/v3/log"
)

// PreforkLogger defines a logger for the prefork process manager.
// Compatible with fasthttp/prefork.Logger.
type PreforkLogger interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...any)
}

// preforkLogger adapts Fiber's logger to the PreforkLogger.
type preforkLogger struct{}

func (preforkLogger) Printf(format string, args ...any) {
	log.Infof(format, args...)
}
