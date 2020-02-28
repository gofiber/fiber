package middleware

import (
	"fmt"

	"github.com/gofiber/fiber"
)

// SecureConfig ...
type SecureConfig struct {
	// Optional. Default value "1; mode=block".
	XSSProtection string
	// Optional. Default value "nosniff".
	ContentTypeNosniff string
	// Optional. Default value "SAMEORIGIN". Possible values: "SAMEORIGIN", "DENY", "ALLOW-FROM uri"
	XFrameOptions string
	// Optional. Default value 0.
	HSTSMaxAge int
	// Optional. Default value false.
	HSTSExcludeSubdomains bool
	// Optional. Default value "".
	ContentSecurityPolicy string
	// Optional. Default value false.
	CSPReportOnly bool
	// Optional.  Default value false.
	HSTSPreloadEnabled bool
	// Optional. Default value "".
	ReferrerPolicy string
}

// Secure ...
func Secure(config ...SecureConfig) func(*fiber.Ctx) {
	// Init config
	var cfg SecureConfig
	// Set config if provided
	if len(config) > 0 {
		cfg = config[0]
	}
	// Set config default options
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
