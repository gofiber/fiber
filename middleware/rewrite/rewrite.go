package rewrite

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Initialize
	cfg.rulesRegex = map[*regexp.Regexp]string{}
	for k, v := range cfg.Rules {
		k = strings.ReplaceAll(k, "*", "(.*)")
		k += "$"
		cfg.rulesRegex[regexp.MustCompile(k)] = v
	}
	// Middleware function
	return func(c *fiber.Ctx) error {
		// Next request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Rewrite
		for k, v := range cfg.rulesRegex {
			replacer := captureTokens(k, c.Path())
			if replacer != nil {
				c.Path(replacer.Replace(v))
				break
			}
		}
		return c.Next()
	}
}

// https://github.com/labstack/echo/blob/master/middleware/rewrite.go
func captureTokens(pattern *regexp.Regexp, input string) *strings.Replacer {
	groups := pattern.FindAllStringSubmatch(input, -1)
	if groups == nil {
		return nil
	}
	values := groups[0][1:]
	replace := make([]string, 2*len(values))
	for i, v := range values {
		j := 2 * i
		replace[j] = "$" + strconv.Itoa(i+1)
		replace[j+1] = v
	}
	return strings.NewReplacer(replace...)
}
