package basicauth

import (
	"encoding/base64"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	utils "github.com/gofiber/utils/v2"
	"golang.org/x/text/unicode/norm"
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// The key for the username value stored in the context
const (
	usernameKey contextKey = iota
)

const basicScheme = "Basic"

// New creates a new middleware handler
func New(config Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config)

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get authorization header and ensure it matches the Basic scheme
		rawAuth := c.Get(fiber.HeaderAuthorization)
		if rawAuth == "" {
			return cfg.Unauthorized(c)
		}
		if len(rawAuth) > cfg.HeaderLimit {
			return c.SendStatus(fiber.StatusRequestHeaderFieldsTooLarge)
		}
		if strings.IndexFunc(rawAuth, func(r rune) bool {
			return (r < 0x20 && r != '\t') || r == 0x7F || r >= 0x80
		}) != -1 {
			return cfg.BadRequest(c)
		}
		auth := strings.Trim(rawAuth, " \t")
		if auth == "" {
			return cfg.Unauthorized(c)
		}
		if len(auth) < len(basicScheme) || !utils.EqualFold(auth[:len(basicScheme)], basicScheme) {
			return cfg.Unauthorized(c)
		}
		rest := auth[len(basicScheme):]
		if rest == "" || rest[0] != ' ' {
			return cfg.BadRequest(c)
		}
		i := 0
		for i < len(rest) && rest[i] == ' ' {
			i++
		}
		rest = rest[i:]
		if rest == "" || strings.IndexAny(rest, " \t") != -1 {
			return cfg.BadRequest(c)
		}

		// Decode the header contents
		raw, err := base64.StdEncoding.DecodeString(rest)
		if err != nil {
			if _, ok := err.(base64.CorruptInputError); ok {
				raw, err = base64.RawStdEncoding.DecodeString(rest)
			}
			if err != nil {
				return cfg.BadRequest(c)
			}
		}
		if !utf8.Valid(raw) {
			return cfg.BadRequest(c)
		}
		if !norm.NFC.IsNormal(raw) {
			raw = norm.NFC.Bytes(raw)
		}

		// Get the credentials
		var creds string
		if c.App().Config().Immutable {
			creds = string(raw)
		} else {
			creds = utils.UnsafeString(raw)
		}

		// Check if the credentials are in the correct form
		// which is "username:password".
		index := strings.Index(creds, ":")
		if index == -1 {
			return cfg.BadRequest(c)
		}

		// Get the username and password
		username := creds[:index]
		password := creds[index+1:]

		if containsCTL(username) || containsCTL(password) {
			return cfg.BadRequest(c)
		}

		if cfg.Authorizer(username, password, c) {
			c.Locals(usernameKey, username)
			return c.Next()
		}

		// Authentication failed
		return cfg.Unauthorized(c)
	}
}

func containsCTL(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// UsernameFromContext returns the username found in the context
// returns an empty string if the username does not exist
func UsernameFromContext(c fiber.Ctx) string {
	username, ok := c.Locals(usernameKey).(string)
	if !ok {
		return ""
	}
	return username
}
