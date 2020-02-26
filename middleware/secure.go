package middleware

import (
	"fmt"

	"github.com/gofiber/fiber"
)

// Usage
// app.Use(middleware.Secure())

// SecureConfig ...
type SecureConfig struct {
	// Optional. Default value "1; mode=block".
	XSSProtection string `yaml:"xss_protection"`

	// Optional. Default value "nosniff".
	ContentTypeNosniff string `yaml:"content_type_nosniff"`

	// Optional. Default value "SAMEORIGIN".
	// Possible values:
	// - "SAMEORIGIN" - The page can only be displayed in a frame on the same origin as the page itself.
	// - "DENY" - The page cannot be displayed in a frame, regardless of the site attempting to do so.
	// - "ALLOW-FROM uri" - The page can only be displayed in a frame on the specified origin.
	XFrameOptions string `yaml:"x_frame_options"`

	// Optional. Default value 0.
	HSTSMaxAge int `yaml:"hsts_max_age"`

	// Optional. Default value false.
	HSTSExcludeSubdomains bool `yaml:"hsts_exclude_subdomains"`

	// Optional. Default value "".
	ContentSecurityPolicy string `yaml:"content_security_policy"`

	// Optional. Default value false.
	CSPReportOnly bool `yaml:"csp_report_only"`

	// Optional.  Default value false.
	HSTSPreloadEnabled bool `yaml:"hsts_preload_enabled"`

	// Optional. Default value "".
	ReferrerPolicy string `yaml:"referrer_policy"`
}

// Secure ...
func Secure(config ...SecureConfig) func(*fiber.Ctx) {
	// Init config
	var cfg SecureConfig
	// Set config if provided
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.XSSProtection == "" {
		cfg.XSSProtection = "1; mode=block"
	}
	if cfg.ContentTypeNosniff == "" {
		cfg.ContentTypeNosniff = "nosniff"
	}
	if cfg.XFrameOptions == "" {
		cfg.XFrameOptions = "SAMEORIGIN"
	}
	// Return middleware handler
	return func(c *fiber.Ctx) {
		if cfg.XSSProtection != "" {
			c.Set(fiber.HeaderXXSSProtection, cfg.XSSProtection)
		}
		if cfg.ContentTypeNosniff != "" {
			c.Set(fiber.HeaderXContentTypeOptions, cfg.ContentTypeNosniff)
		}
		if cfg.XFrameOptions != "" {
			c.Set(fiber.HeaderXFrameOptions, cfg.XFrameOptions)
		}
		if (c.Secure() || (c.Get(fiber.HeaderXForwardedProto) == "https")) && cfg.HSTSMaxAge != 0 {
			subdomains := ""
			if !cfg.HSTSExcludeSubdomains {
				subdomains = "; includeSubdomains"
			}
			if cfg.HSTSPreloadEnabled {
				subdomains = fmt.Sprintf("%s; preload", subdomains)
			}
			c.Set(fiber.HeaderStrictTransportSecurity, fmt.Sprintf("max-age=%d%s", cfg.HSTSMaxAge, subdomains))
		}
		if cfg.ContentSecurityPolicy != "" {
			if cfg.CSPReportOnly {
				c.Set(fiber.HeaderContentSecurityPolicyReportOnly, cfg.ContentSecurityPolicy)
			} else {
				c.Set(fiber.HeaderContentSecurityPolicy, cfg.ContentSecurityPolicy)
			}
		}
		if cfg.ReferrerPolicy != "" {
			c.Set(fiber.HeaderReferrerPolicy, cfg.ReferrerPolicy)
		}
		c.Next()
	}
}
