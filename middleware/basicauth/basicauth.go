package basicauth

import (
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/bytepolicy"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/utils/v2"
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

var registerLogContextTagsOnce sync.Once

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	registerLogContextTagsOnce.Do(registerLogContextTags)

	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Authorization is a single-value field (RFC 9110 Section 11.6.2), so a
		// message carrying it on more than one line is malformed and refused
		// rather than read, whatever the lines are spelled like or hold. Reading
		// any one of them lets whoever wrote the other decide: the first line
		// hides a credential behind an empty one, and the first non-empty line
		// authenticates a request that a net/http layer in front, reading the
		// field with Header.Get, saw as carrying no credential at all.
		rawAuth, ok := headerlookup.Value(c, fiber.HeaderAuthorization)
		if !ok {
			return cfg.BadRequest(c)
		}
		if rawAuth == "" {
			return cfg.Unauthorized(c)
		}
		if len(rawAuth) > cfg.HeaderLimit {
			return c.SendStatus(fiber.StatusRequestHeaderFieldsTooLarge)
		}
		if containsInvalidHeaderChars(rawAuth) {
			return cfg.BadRequest(c)
		}
		auth := utils.TrimSpace(rawAuth)
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
		// containsInvalidHeaderChars left only HTAB and visible ASCII, so
		// the only remaining bytes unicode.IsSpace can match are ' ' and '\t'.
		if utils.IndexAny2(rest, ' ', '\t') != -1 {
			return cfg.BadRequest(c)
		}

		// Decode the header contents
		raw, err := base64.StdEncoding.DecodeString(rest)
		if err != nil {
			if _, ok := errors.AsType[base64.CorruptInputError](err); ok {
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
		username, password, found := utils.CutByte(creds, ':')
		if !found {
			return cfg.BadRequest(c)
		}

		if containsCTL(username) || containsCTL(password) {
			return cfg.BadRequest(c)
		}

		if cfg.Authorizer(username, password, c) {
			fiber.StoreInContext(c, usernameKey, username)
			return c.Next()
		}

		// Authentication failed
		return cfg.Unauthorized(c)
	}
}

// registerLogContextTags exposes the authenticated username under the
// ${username} tag for both middleware/logger access logs and fiberlog
// WithContext lines. The full clear-text username is intentionally written
// (not redacted) because the primary use case for ${username} is auditability:
// who hit which endpoint, when. Username strings have already passed
// containsCTL stripping, so they are safe with respect to log injection.
//
// If full usernames are PII in your jurisdiction (GDPR, CCPA, etc.), do not
// include ${username} in your log format and obtain the value via
// UsernameFromContext at the application layer where you can hash, prefix,
// or otherwise minimize it before emitting.
func registerLogContextTags() {
	logger.RegisterContextTag("username", UsernameFromContext)
}

// containsCTL reports whether s contains a Unicode control character
// (C0, DEL, or C1). The shared byte scan finds ASCII controls or the first
// non-ASCII byte, where rune decoding starts to check the C1 range.
func containsCTL(s string) bool {
	i := bytepolicy.Index(s, bytepolicy.VisibleASCII)
	if i == -1 {
		return false
	}
	return s[i] < 0x80 || containsCTLUnicode(s[i:])
}

// containsCTLUnicode is the rune-decoding slow path of containsCTL.
func containsCTLUnicode(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) != -1
}

// containsInvalidHeaderChars reports whether s holds any byte outside the
// valid header set: HTAB or visible ASCII [0x20, 0x7E]. Bytes >= 0x80 are
// invalid, so checking bytes and checking runes give the same answer, which
// lets the scan run eight bytes at a time.
//
// NOTE: the utils.IndexAny2(rest, ' ', '\t') whitespace check in New relies
// on this having rejected every byte unicode.IsSpace could otherwise match
// (>= 0x80 and C0 except HTAB); keep the two in sync.
func containsInvalidHeaderChars(s string) bool {
	return bytepolicy.Index(s, bytepolicy.HeaderValue) != -1
}

// UsernameFromContext returns the username found in the context.
// It accepts fiber.CustomCtx, fiber.Ctx, *fasthttp.RequestCtx, and context.Context.
// It returns an empty string if the username does not exist.
func UsernameFromContext(ctx any) string {
	if username, ok := fiber.ValueFromContext[string](ctx, usernameKey); ok {
		return username
	}

	return ""
}
