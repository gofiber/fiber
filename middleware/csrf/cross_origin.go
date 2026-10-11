package csrf

import (
	"errors"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
)

// ErrCrossOriginRequest is the error NewCrossOriginProtection denies a request with.
var ErrCrossOriginRequest = errors.New("csrf: cross-origin request denied")

// CrossOriginConfig defines the config for NewCrossOriginProtection.
type CrossOriginConfig struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// ErrorHandler is executed when a request is denied.
	//
	// Optional. Default: defaultErrorHandler
	ErrorHandler fiber.ErrorHandler

	// TrustedOrigins is a list of origins allowed to send unsafe cross-origin
	// requests. It takes the same values as Config.TrustedOrigins, including
	// subdomain wildcards such as "https://*.example.com".
	//
	// No token backs this check, so a matching origin is trusted completely.
	//
	// Optional. Default: []
	TrustedOrigins []string
}

// NewCrossOriginProtection creates a tokenless CSRF middleware. It rejects
// unsafe cross-origin browser requests by their Sec-Fetch-Site and Origin
// headers, like net/http.CrossOriginProtection, and keeps no cookie or state.
func NewCrossOriginProtection(config ...CrossOriginConfig) fiber.Handler {
	var cfg CrossOriginConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = defaultErrorHandler
	}

	trusted := parseTrustedOrigins(cfg.TrustedOrigins, func(string) string { return redactedKey })

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		if err := checkCrossOrigin(c, trusted); err != nil {
			return cfg.ErrorHandler(c, err)
		}
		return c.Next()
	}
}

// checkCrossOrigin follows the decisions of net/http.CrossOriginProtection.Check.
func checkCrossOrigin(c fiber.Ctx, trusted trustedOrigins) error {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return nil
	}

	// ok is false for a repeated line: no single value to decide on.
	fetchSite, ok := headerlookup.Value(c, fiber.HeaderSecFetchSite)
	if !ok {
		return ErrCrossOriginRequest
	}
	if fetchSite == "same-origin" || fetchSite == "none" {
		return nil
	}

	origin, ok := headerlookup.Value(c, fiber.HeaderOrigin)
	if !ok {
		return ErrCrossOriginRequest
	}

	if fetchSite == "" {
		// Neither header: same-origin or not a browser.
		if origin == "" {
			return nil
		}
		// Host carries no scheme, so http to https on one host passes, as in net/http.
		// The header as sent, not c.Host(), which would also honor X-Forwarded-Host.
		if host := c.Request().Header.Host(); len(host) > 0 {
			if originURL, err := url.Parse(origin); err == nil && originURL.Host == string(host) {
				return nil
			}
		}
	}

	// Compared as sent: browsers serialize Origin in the normalized form.
	if trusted.contains(origin) {
		return nil
	}
	return ErrCrossOriginRequest
}
