package envvar

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// ExportVars specifies the environment variables that should export
	ExportVars map[string]string
	// ExcludeVars specifies the environment variables that should not export
	ExcludeVars map[string]string
}

type EnvVar struct {
	Vars map[string]string `json:"vars"`
}

func (envVar *EnvVar) set(key, val string) {
	envVar.Vars[key] = val
}

func New(config ...Config) fiber.Handler {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet {
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

	if len(cfg.ExportVars) > 0 {
		for key, defaultVal := range cfg.ExportVars {
			vars.set(key, defaultVal)
			if envVal, exists := os.LookupEnv(key); exists {
				vars.set(key, envVal)
			}
		}
	} else {
		const numElems = 2
		for _, envVal := range os.Environ() {
			keyVal := strings.SplitN(envVal, "=", numElems)
			if _, exists := cfg.ExcludeVars[keyVal[0]]; !exists {
				vars.set(keyVal[0], keyVal[1])
			}
		}
	}

	return vars
}
