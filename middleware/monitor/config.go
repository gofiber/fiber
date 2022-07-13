package monitor

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Metrics page title
	//
	// Optional. Default: "Fiber Monitor"
	Title string

	// Refresh period
	//
	// Optional. Default: 3 seconds
	Refresh time.Duration

	// Whether the service should expose only the monitoring API.
	//
	// Optional. Default: false
	APIOnly bool

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// customized indexHtml
	index string
}

var ConfigDefault = Config{
	Title:   defaultTitle,
	Refresh: defaultRefresh,
	APIOnly: false,
	Next:    nil,
	index:   newIndex(defaultTitle, defaultRefresh),
}

func configDefault(config ...Config) Config {
	// Users can change ConfigDefault.Title/Refresh which then
	// become incompatible with ConfigDefault.index
	if ConfigDefault.Title != defaultTitle || ConfigDefault.Refresh != defaultRefresh {

		if ConfigDefault.Refresh < minRefresh {
			ConfigDefault.Refresh = minRefresh
		}
		// update default index with new default title/refresh
		ConfigDefault.index = newIndex(ConfigDefault.Title, ConfigDefault.Refresh)
	}

	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Title == "" {
		cfg.Title = ConfigDefault.Title
	}

	if cfg.Refresh == 0 {
		cfg.Refresh = ConfigDefault.Refresh
	}

	if cfg.Title == ConfigDefault.Title && cfg.Refresh == ConfigDefault.Refresh {
		cfg.index = ConfigDefault.index
	} else {
		if cfg.Refresh < minRefresh {
			cfg.Refresh = minRefresh
		}
		// update cfg.index with custom title/refresh
		cfg.index = newIndex(cfg.Title, cfg.Refresh)
	}

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if !cfg.APIOnly {
		cfg.APIOnly = ConfigDefault.APIOnly
	}

	return cfg
}
