package monitor

import (
	"time"

	"github.com/gofiber/fiber/v2"
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
	Next func(c *fiber.Ctx) bool

	// Custom HTML Code to Head Section(Before End)
	//
	// Optional. Default: empty
	CustomHead string

	// FontURL for specify font resource path or URL . also you can use relative path
	//
	// Optional. Default: https://fonts.googleapis.com/css2?family=Roboto:wght@400;900&display=swap
	FontURL string

	// ChartJsURL for specify ChartJS library  path or URL . also you can use relative path
	//
	// Optional. Default: https://cdn.jsdelivr.net/npm/chart.js@2.9/dist/Chart.bundle.min.js
	ChartJsURL string // TODO: Rename to "ChartJSURL" in v3

	index string
}

var ConfigDefault = Config{
	Title:      defaultTitle,
	Refresh:    defaultRefresh,
	FontURL:    defaultFontURL,
	ChartJsURL: defaultChartJSURL,
	CustomHead: defaultCustomHead,
	APIOnly:    false,
	Next:       nil,
	index: newIndex(viewBag{
		defaultTitle,
		defaultRefresh,
		defaultFontURL,
		defaultChartJSURL,
		defaultCustomHead,
	}),
}

func configDefault(config ...Config) Config {
	// Users can change ConfigDefault.Title/Refresh which then
	// become incompatible with ConfigDefault.index
	if ConfigDefault.Title != defaultTitle ||
		ConfigDefault.Refresh != defaultRefresh ||
		ConfigDefault.FontURL != defaultFontURL ||
		ConfigDefault.ChartJsURL != defaultChartJSURL ||
		ConfigDefault.CustomHead != defaultCustomHead {
		if ConfigDefault.Refresh < minRefresh {
			ConfigDefault.Refresh = minRefresh
		}
		// update default index with new default title/refresh
		ConfigDefault.index = newIndex(viewBag{
			ConfigDefault.Title,
			ConfigDefault.Refresh,
			ConfigDefault.FontURL,
			ConfigDefault.ChartJsURL,
			ConfigDefault.CustomHead,
		})
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
	if cfg.FontURL == "" {
		cfg.FontURL = defaultFontURL
	}

	if cfg.ChartJsURL == "" {
		cfg.ChartJsURL = defaultChartJSURL
	}
	if cfg.Refresh < minRefresh {
		cfg.Refresh = minRefresh
	}

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if !cfg.APIOnly {
		cfg.APIOnly = ConfigDefault.APIOnly
	}

	// update cfg.index with custom title/refresh
	cfg.index = newIndex(viewBag{
		title:      cfg.Title,
		refresh:    cfg.Refresh,
		fontURL:    cfg.FontURL,
		chartJSURL: cfg.ChartJsURL,
		customHead: cfg.CustomHead,
	})

	return cfg
}
