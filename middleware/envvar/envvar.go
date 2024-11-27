package envvar

import (
	"os"
	"strings"
	"github.com/gofiber/fiber/v3"
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

// New initializes a new middleware handler with the given config.
func New(config ...Config) fiber.Handler {
	cfg := Config{}
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c fiber.Ctx) error {
		// Restrict to GET requests only
		if c.Method() != fiber.MethodGet {
			return c.SendStatus(fiber.StatusMethodNotAllowed)
		}

		// Construct EnvVar and encode as JSON
		envVar := newEnvVar(cfg)
		varsByte, err := c.App().Config().JSONEncoder(envVar)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}

		// Set content type and send response
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(varsByte)
	}
}

// newEnvVar creates and populates an EnvVar instance based on the config.
func newEnvVar(cfg Config) *EnvVar {
	envVar := &EnvVar{Vars: make(map[string]string)}

	if len(cfg.ExportVars) > 0 {
		// Populate explicitly defined export variables
		for key, defaultVal := range cfg.ExportVars {
			if envVal, exists := os.LookupEnv(key); exists {
				envVar.set(key, envVal)
			} else {
				envVar.set(key, defaultVal)
			}
		}
	} else {
		// Exclude specified variables from all environment variables
		excludeVars := cfg.ExcludeVars
		for _, env := range os.Environ() {
			// Use strings.IndexByte for performance on splitting
			if idx := strings.IndexByte(env, '='); idx > 0 {
				key := env[:idx]
				if _, excluded := excludeVars[key]; !excluded {
					envVar.set(key, env[idx+1:])
				}
			}
		}
	}

	return envVar
}
