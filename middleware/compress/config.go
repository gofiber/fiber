package compress

import (
	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Level determines the compression algorithm
	//
	// Optional. Default: LevelDefault
	// LevelDisabled:         -1
	// LevelDefault:          0
	// LevelBestSpeed:        1
	// LevelBestCompression:  2
	Level Level
}

// Level is numeric representation of compression level
type Level int

// Represents compression level that will be used in the middleware
const (
	LevelDisabled        Level = -1
	LevelDefault         Level = 0
	LevelBestSpeed       Level = 1
	LevelBestCompression Level = 2
)

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:  nil,
	Level: LevelDefault,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Level < LevelDisabled || cfg.Level > LevelBestCompression {
		cfg.Level = ConfigDefault.Level
	}
	return cfg
}
