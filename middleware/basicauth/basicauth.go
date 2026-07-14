package basicauth

import (
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/utils/v2"
	"github.com/gofiber/utils/v2/swar"
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

// asciiCTLMask marks the lanes of w holding ASCII control bytes (C0 or DEL).
// Lanes >= 0x80 are never marked; callers must route words containing them
// to the Unicode slow path first.
func asciiCTLMask(w uint64) uint64 {
	return swar.MatchRangeMask(w, 0x00, 0x1f) | swar.MatchByteMask(w, 0x7f)
}

// containsCTL reports whether s contains a Unicode control character
// (C0, DEL, or C1). ASCII spans are scanned eight bytes at a time; the
// first word holding a byte >= 0x80 defers the rest to the rune loop,
// which handles the multi-byte C1 range. The handoff happens only where
// every preceding byte is ASCII, i.e. on a rune boundary.
func containsCTL(s string) bool {
	n := len(s)
	i := 0
	for ; i+swar.WordLen <= n; i += swar.WordLen {
		w := swar.Load8(s, i)
		if w&swar.HighBits != 0 {
			return containsCTLUnicode(s[i:])
		}
		if asciiCTLMask(w) != 0 {
			return true
		}
	}
	if i == n {
		return false
	}
	if n >= swar.WordLen {
		// One overlapping word: the re-checked bytes are known ASCII and
		// clean, so any hit lands in the new bytes — still a rune boundary.
		w := swar.Load8(s, n-swar.WordLen)
		if w&swar.HighBits != 0 {
			return containsCTLUnicode(s[n-swar.WordLen:])
		}
		return asciiCTLMask(w) != 0
	}
	for ; i < n; i++ {
		c := s[i]
		if c >= 0x80 {
			return containsCTLUnicode(s[i:])
		}
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// containsCTLUnicode is the rune-decoding slow path of containsCTL.
func containsCTLUnicode(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) != -1
}

// validHeaderMask marks the lanes of w holding bytes from the valid header
// set: HTAB or visible ASCII [0x20, 0x7E]. A word is fully valid iff the
// mask equals swar.HighBits.
func validHeaderMask(w uint64) uint64 {
	return swar.MatchRangeMask(w, 0x20, 0x7e) | swar.MatchByteMask(w, '\t')
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
	n := len(s)
	i := 0
	for ; i+swar.WordLen <= n; i += swar.WordLen {
		if validHeaderMask(swar.Load8(s, i)) != swar.HighBits {
			return true
		}
	}
	if i == n {
		return false
	}
	if n >= swar.WordLen {
		// Finish with one overlapping word; re-checking bytes that already
		// passed cannot change the outcome.
		return validHeaderMask(swar.Load8(s, n-swar.WordLen)) != swar.HighBits
	}
	for ; i < n; i++ {
		if c := s[i]; (c < 0x20 && c != '\t') || c >= 0x7f {
			return true
		}
	}
	return false
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
