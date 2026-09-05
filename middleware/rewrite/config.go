package rewrite

import (
	"github.com/gofiber/fiber/v3"
)

// Rule is one rewrite: the path pattern to match and the path to rewrite it
// to. The values captured in asterisk can be retrieved by index e.g. $1, $2
// and so on.
type Rule struct {
	// From is the path pattern, where "*" matches a run of any bytes but a
	// newline and every other byte stands for itself. There is no escape for a
	// literal "*".
	From string

	// To is the new path, where "$1", "$2" and so on stand for what the
	// asterisks of From captured.
	To string
}

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip middleware.
	// Optional. Default: nil
	Next func(fiber.Ctx) bool

	// Rules defines the URL path rewrite rules. The values captured in asterisk can be
	// retrieved by index e.g. $1, $2 and so on.
	//
	// Deprecated: Use RuleList instead. A map has no order, so the rule
	// answering a path two rules both match is decided by a documented heuristic
	// rather than by the author. Retained for backward compatibility with
	// existing configurations.
	Rules map[string]string

	// RuleList defines the URL path rewrite rules. They are tried in the order
	// given and the first rule whose From matches the request path wins, as
	// routes do; nothing reorders the list. Put the specific rules before the
	// catch-alls. Set this or the deprecated Rules, not both.
	//
	// Example:
	// {From: "/old", To: "/new"},
	// {From: "/api/*", To: "/$1"},
	// {From: "/js/*", To: "/public/javascript/$1"},
	// {From: "/users/*/orders/*", To: "/user/$1/order/$2"},
	RuleList []Rule
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return Config{}
	}

	// Override default config
	cfg := config[0]

	if len(cfg.Rules) > 0 && len(cfg.RuleList) > 0 {
		panic("rewrite: set either Rules (deprecated) or RuleList, not both")
	}

	return cfg
}
