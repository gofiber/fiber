package envvar

import (
	"os"

	"github.com/gofiber/fiber/v3"
)

const hAllow = fiber.MethodGet + ", " + fiber.MethodHead

// EnvVar captures environment variables that are exposed through the
// middleware response.
type EnvVar struct {
	Vars map[string]string `json:"vars"`
}

func (envVar *EnvVar) set(key, val string) {
	envVar.Vars[key] = val
}

// New creates a handler that returns configured environment variables as a
// JSON response.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(c fiber.Ctx) error {
		method := c.Method()
		if method != fiber.MethodGet && method != fiber.MethodHead {
			c.Set(fiber.HeaderAllow, hAllow)
			return fiber.ErrMethodNotAllowed
		}

		envVar := newEnvVar(cfg)
		varsByte, err := c.App().Config().JSONEncoder(envVar)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(varsByte)
	}
}

func newEnvVar(cfg Config) *EnvVar {
	vars := &EnvVar{Vars: make(map[string]string)}

	if len(cfg.ExportVars) == 0 {
		// do not expose environment variables when no configuration
		// is supplied to prevent accidental information disclosure
		return vars
	}

	for key, defaultVal := range cfg.ExportVars {
		vars.set(key, defaultVal)
		if envVal, exists := os.LookupEnv(key); exists {
			vars.set(key, envVal)
		}
	}

	return vars
}
