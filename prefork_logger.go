package fiber

import "github.com/gofiber/fiber/v3/log"

// preforkLogger adapts Fiber's logger to fasthttp prefork's Logger interface.
type preforkLogger struct{}

func (preforkLogger) Printf(format string, args ...any) {
	log.Infof(format, args...)
}
