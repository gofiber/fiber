package envvar

// Config defines the config for middleware.
type Config struct {
	// ExportVars specifies the environment variables that should export
	ExportVars map[string]string
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	ExportVars: map[string]string{},
}

func configDefault(config ...Config) Config {
	if len(config) == 0 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.ExportVars == nil {
		cfg.ExportVars = ConfigDefault.ExportVars
	}

	return cfg
}
