package envvar

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"os"
	"strings"
)

// Config defines the config for middleware.
type Config struct {
	// Path specifies the path of handler that exposes environment variables
	Path string
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

func (envVar *EnvVar) get(key string) string {
	return envVar.Vars[key]
}

func New(config ...Config) fiber.Handler {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = Config{Path: "/envvars"}
	}

	return func(c *fiber.Ctx) error {
		path := c.Path()
		if len(path) < len(cfg.Path) || !strings.HasPrefix(path, cfg.Path) {
			return c.Next()
		}

		if path == cfg.Path {
			envVar := newEnvVar(cfg)
			varsByte, err := c.App().Config().JSONEncoder(envVar)
			if err != nil {
				c.Response().SetBodyRaw(utils.UnsafeBytes(err.Error()))
				c.Response().SetStatusCode(fiber.StatusInternalServerError)
				return nil
			}
			c.Response().SetBodyRaw(varsByte)
			c.Response().SetStatusCode(fiber.StatusOK)
			c.Response().Header.Set("Content-Type", "application/json; charset=utf-8")
			return nil
		}

		return c.Next()
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
		for _, envVal := range os.Environ() {
			keyVal := strings.Split(envVal, "=")
			if _, exists := cfg.ExcludeVars[keyVal[0]]; !exists {
				vars.set(keyVal[0], keyVal[1])
			}
		}
	}

	return vars
}
