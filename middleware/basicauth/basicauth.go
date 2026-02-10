package basicauth

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
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
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	var cerr base64.CorruptInputError

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
		if containsInvalidHeaderChars(rawAuth) {
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
		if len(rest) < 2 || rest[0] != ' ' || rest[1] == ' ' {
			return cfg.BadRequest(c)
		}
		rest = rest[1:]
		if strings.IndexFunc(rest, unicode.IsSpace) != -1 {
			return cfg.BadRequest(c)
		}

		// Decode the header contents
		raw, err := base64.StdEncoding.DecodeString(rest)
		if err != nil {
			if errors.As(err, &cerr) {
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
		username, password, found := strings.Cut(creds, ":")
		if !found {
			return cfg.BadRequest(c)
		}

		if containsCTL(username) || containsCTL(password) {
			return cfg.BadRequest(c)
		}

		if cfg.Authorizer(username, password, c) {
			c.Locals(usernameKey, username)
			c.SetContext(context.WithValue(c.Context(), usernameKey, username))
			return c.Next()
		}

		// Authentication failed
		return cfg.Unauthorized(c)
	}
}

func containsCTL(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) != -1
}

func containsInvalidHeaderChars(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return (r < 0x20 && r != '\t') || r == 0x7F || r >= 0x80
	}) != -1
}

// UsernameFromContext returns the username found in the context
// returns an empty string if the username does not exist
func UsernameFromContext(ctx any) string {
	if customCtx, ok := ctx.(fiber.CustomCtx); ok {
		if username, ok := customCtx.Locals(usernameKey).(string); ok {
			return username
		}
	}
	switch typed := ctx.(type) {
	case fiber.Ctx:
		if username, ok := typed.Locals(usernameKey).(string); ok {
			return username
		}
	case *fasthttp.RequestCtx:
		if username, ok := typed.UserValue(usernameKey).(string); ok {
			return username
		}
	case context.Context:
		if username, ok := typed.Value(usernameKey).(string); ok {
			return username
		}
	}
	return ""
}
