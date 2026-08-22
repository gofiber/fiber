package redirect

import (
	"github.com/gofiber/fiber/v3"
)

// Rule is one redirect: the path pattern to match and the target to send the
// client to. The values captured in asterisk can be retrieved by index e.g.
// $1, $2 and so on.
type Rule struct {
	// From is the path pattern, where "*" matches a run of any length.
	From string

	// To is the redirect target, where "$1", "$2" and so on stand for what the
	// asterisks of From captured.
	To string
}

// Config defines the config for middleware.
type Config struct {
	// Filter defines a function to skip middleware.
	// Optional. Default: nil
	Next func(fiber.Ctx) bool

	// Rules defines the URL path rewrite rules. The values captured in asterisk can be
	// retrieved by index e.g. $1, $2 and so on.
	//
	// Deprecated: Use RuleList instead. A map has no order, so the rule answering
	// a path two rules both match is decided by a documented heuristic rather
	// than by the author. Retained for backward compatibility with existing
	// configurations.
	Rules map[string]string

	// RuleList defines the URL path rewrite rules, tried in the order given:
	// the first rule whose From matches the request path wins, as routes do.
	// Put the specific rules before the catch-alls.
	// Required. Example:
	// {From: "/old", To: "/new"},
	// {From: "/api/*", To: "/$1"},
	// {From: "/js/*", To: "/public/javascript/$1"},
	// {From: "/users/*/orders/*", To: "/user/$1/order/$2"},
	RuleList []Rule

	rulesRegex []compiledRule

	// The status code when redirecting
	// This is ignored if Redirect is disabled
	// Optional. Default: 302 Temporary Redirect
	StatusCode int
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	StatusCode: fiber.StatusFound,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	if len(cfg.Rules) > 0 && len(cfg.RuleList) > 0 {
		panic("redirect: set either Rules (deprecated) or RuleList, not both")
	}

	// Set default values
	if cfg.StatusCode == 0 {
		cfg.StatusCode = ConfigDefault.StatusCode
	}

	return cfg
}
