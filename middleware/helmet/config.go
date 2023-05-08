package helmet

import (
	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip middleware.
	// Optional. Default: nil
	Next func(*fiber.Ctx) bool

	// XSSProtection
	// Optional. Default value "0".
	XSSProtection string

	// ContentTypeNosniff
	// Optional. Default value "nosniff".
	ContentTypeNosniff string

	// XFrameOptions
	// Optional. Default value "SAMEORIGIN".
	// Possible values: "SAMEORIGIN", "DENY", "ALLOW-FROM uri"
	XFrameOptions string

	// HSTSMaxAge
	// Optional. Default value 0.
	HSTSMaxAge int

	// HSTSExcludeSubdomains
	// Optional. Default value false.
	HSTSExcludeSubdomains bool

	// ContentSecurityPolicy
	// Optional. Default value "".
	ContentSecurityPolicy string

	// CSPReportOnly
	// Optional. Default value false.
	CSPReportOnly bool

	// HSTSPreloadEnabled
	// Optional. Default value false.
	HSTSPreloadEnabled bool

	// ReferrerPolicy
	// Optional. Default value "ReferrerPolicy".
	ReferrerPolicy string

	// Permissions-Policy
	// Optional. Default value "".
	PermissionPolicy string

	// Cross-Origin-Embedder-Policy
	// Optional. Default value "require-corp".
	CrossOriginEmbedderPolicy string

	// Cross-Origin-Opener-Policy
	// Optional. Default value "same-origin".
	CrossOriginOpenerPolicy string

	// Cross-Origin-Resource-Policy
	// Optional. Default value "same-origin".
	CrossOriginResourcePolicy string

	// Origin-Agent-Cluster
	// Optional. Default value "?1".
	OriginAgentCluster string

	// X-DNS-Prefetch-Control
	// Optional. Default value "off".
	XDNSPrefetchControl string

	// X-Download-Options
	// Optional. Default value "noopen".
	XDownloadOptions string

	// X-Permitted-Cross-Domain-Policies
	// Optional. Default value "none".
	XPermittedCrossDomain string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	XSSProtection:             "0",
	ContentTypeNosniff:        "nosniff",
	XFrameOptions:             "SAMEORIGIN",
	ReferrerPolicy:            "no-referrer",
	CrossOriginEmbedderPolicy: "require-corp",
	CrossOriginOpenerPolicy:   "same-origin",
	CrossOriginResourcePolicy: "same-origin",
	OriginAgentCluster:        "?1",
	XDNSPrefetchControl:       "off",
	XDownloadOptions:          "noopen",
	XPermittedCrossDomain:     "none",
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
	if cfg.XSSProtection == "" {
		cfg.XSSProtection = ConfigDefault.XSSProtection
	}

	if cfg.ContentTypeNosniff == "" {
		cfg.ContentTypeNosniff = ConfigDefault.ContentTypeNosniff
	}

	if cfg.XFrameOptions == "" {
		cfg.XFrameOptions = ConfigDefault.XFrameOptions
	}

	if cfg.ReferrerPolicy == "" {
		cfg.ReferrerPolicy = ConfigDefault.ReferrerPolicy
	}

	if cfg.CrossOriginEmbedderPolicy == "" {
		cfg.CrossOriginEmbedderPolicy = ConfigDefault.CrossOriginEmbedderPolicy
	}

	if cfg.CrossOriginOpenerPolicy == "" {
		cfg.CrossOriginOpenerPolicy = ConfigDefault.CrossOriginOpenerPolicy
	}

	if cfg.CrossOriginResourcePolicy == "" {
		cfg.CrossOriginResourcePolicy = ConfigDefault.CrossOriginResourcePolicy
	}

	if cfg.OriginAgentCluster == "" {
		cfg.OriginAgentCluster = ConfigDefault.OriginAgentCluster
	}

	if cfg.XDNSPrefetchControl == "" {
		cfg.XDNSPrefetchControl = ConfigDefault.XDNSPrefetchControl
	}

	if cfg.XDownloadOptions == "" {
		cfg.XDownloadOptions = ConfigDefault.XDownloadOptions
	}

	if cfg.XPermittedCrossDomain == "" {
		cfg.XPermittedCrossDomain = ConfigDefault.XPermittedCrossDomain
	}

	return cfg
}
