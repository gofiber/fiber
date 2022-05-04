package monitor

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Metrics page title
	//
	// Optional. Default: "Fiber Monitor"
	Title string

	// Whether the service should expose only the monitoring API.
	//
	// Optional. Default: false
	APIOnly bool

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// defaultTitle substituted with Title in indexHtml
	index string
}

var prevTitle = defaultTitle

var ConfigDefault = Config{
	Title:   defaultTitle,
	APIOnly: false,
	Next:    nil,
	index:   indexHtml,
}

func newIndex(title string) string {
	if title == defaultTitle {
		return indexHtml
	}
	return strings.ReplaceAll(indexHtml, defaultTitle, title)
}

func configDefault(config ...Config) Config {
	// Users can change ConfigDefault.Title which then
	// becomes incompatible with ConfigDefault.index
	if prevTitle != ConfigDefault.Title {
		prevTitle = ConfigDefault.Title
		// update default index with new default title
		ConfigDefault.index = newIndex(prevTitle)
	}

	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Title == "" || cfg.Title == ConfigDefault.Title {
		cfg.Title = ConfigDefault.Title
		cfg.index = ConfigDefault.index
	} else {
		// update cfg.index with new title
		cfg.index = newIndex(cfg.Title)
	}

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if !cfg.APIOnly {
		cfg.APIOnly = ConfigDefault.APIOnly
	}

	return cfg
}
