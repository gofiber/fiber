package fiber

import (
	"github.com/gofiber/fiber/v3/log"
)

// preforkLogger adapts fiber's logger to the fasthttp prefork Logger interface.
type preforkLogger struct{}

// Printf implements the fasthttp prefork Logger interface.
func (preforkLogger) Printf(format string, args ...any) {
	log.Infof(format, args...)
}
