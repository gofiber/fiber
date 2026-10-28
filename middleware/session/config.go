package session

import (
	"reflect"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	internalcookie "github.com/gofiber/fiber/v3/internal/cookie"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// Config defines the configuration for the session middleware.
type Config struct {
	// Storage interface for storing session data.
	//
	// Optional. Default: memory.New()
	Storage fiber.Storage

	// Store defines a pre-built session store, for example one shared with
	// other handlers or one that has custom types registered.
	//
	// Optional. Default: nil, which creates a store from this Config.
	Store *Store

	// Next defines a function to skip this middleware when it returns true.
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// ErrorHandler defines a function to handle errors.
	//
	// Optional. Default: nil
	ErrorHandler func(fiber.Ctx, error)

	// KeyGenerator generates the session key.
	//
	// Optional. Default: utils.SecureToken
	KeyGenerator func() string

	// CookieDomain defines the domain of the session cookie.
	//
	// Optional. Default: ""
	CookieDomain string

	// CookiePath defines the path of the session cookie.
	//
	// Optional. Default: ""
	CookiePath string

	// CookieSameSite specifies the SameSite attribute of the cookie.
	// Accepted values are "Disabled", "Lax", "Strict", and "None", case-insensitively.
	//
	// Optional. Default: "Lax"
	CookieSameSite string

	// StorageKeyPrefix is put in front of the "session:" namespace of the key every
	// session is stored under ("<prefix>session:<ID>"), so that several
	// applications, environments or stores can share one storage without their
	// sessions meeting. End it with a separator such as ":", so that no prefix,
	// followed by "session:", is the start of another. The ID the client holds is
	// unchanged.
	//
	// Store.Reset clears the whole storage, not only the keys under the prefix.
	//
	// Optional. Default: ""
	StorageKeyPrefix string

	// Extractor is used to extract the session ID from the request.
	// See: https://docs.gofiber.io/guide/extractors
	//
	// Optional. Default: extractors.FromCookie("session_id")
	Extractor extractors.Extractor

	// IdleTimeout defines the maximum duration of inactivity before the session expires.
	//
	// Note: The idle timeout is updated on each `Save()` call. If a middleware handler is used, `Save()` is called automatically.
	//
	// Optional. Default: 30 * time.Minute
	IdleTimeout time.Duration

	// AbsoluteTimeout defines the maximum duration of the session before it expires.
	//
	// If set to 0, the session will not have an absolute timeout, and will expire after the idle timeout.
	//
	// Optional. Default: 0
	AbsoluteTimeout time.Duration

	// CookieSecure specifies if the session cookie should be secure.
	//
	// Optional. Default: false
	CookieSecure bool

	// CookieHTTPOnly specifies if the session cookie should be HTTP-only.
	//
	// Optional. Default: false
	CookieHTTPOnly bool

	// CookieSessionOnly determines if the cookie should expire when the browser session ends.
	//
	// If true, the cookie will be deleted when the browser is closed.
	// Note: This will not delete the session data from the store.
	//
	// Optional. Default: false
	CookieSessionOnly bool

	// SkipEmptySessions keeps a session that is new and holds nothing a handler
	// put there out of the storage and out of the response: no record is written
	// and no cookie is set for a visitor whose handlers never use the session.
	//
	// Without it, every request that carries no valid session creates a record
	// that lives until the idle timeout, so a client that sends no cookies, such
	// as a health check or a crawler, fills the storage as fast as it sends
	// requests. A Reset that leaves nothing behind, such as a logout, expires
	// the cookie and stores no new session either.
	//
	// A handler that wants a session to exist must store something in it, if only
	// a flag. The option applies to the session middleware; a session from
	// Store.Get is saved when the handler calls Save.
	//
	// Optional. Default: false
	SkipEmptySessions bool

	// HashStorageKeys stores each session under the SHA-256 digest of its ID,
	// base64url-encoded, instead of under the ID itself (after the prefix and
	// the "session:" namespace, which stay). Whoever can read the
	// storage, through a backup, a replica or a leaked dump, then holds digests
	// that cannot be sent as a session cookie. The ID the client holds is
	// unchanged. It does not make a guessable ID from a custom KeyGenerator safe.
	//
	// Turning it on or off changes the key of every session, so sessions stored
	// before the change are not found afterwards.
	//
	// Optional. Default: false
	HashStorageKeys bool
}

// ConfigDefault provides the default configuration.
var ConfigDefault = Config{
	IdleTimeout:    30 * time.Minute,
	KeyGenerator:   utils.SecureToken,
	Extractor:      extractors.FromCookie("session_id"),
	CookieSameSite: fiber.CookieSameSiteLaxMode,
}

// DefaultErrorHandler logs the error and sends a 500 status code.
//
// Parameters:
//   - c: The Fiber context.
//   - err: The error to handle.
//
// Usage:
//
//	DefaultErrorHandler(c, err)
func DefaultErrorHandler(c fiber.Ctx, err error) {
	log.Errorf("session error: %v", err)
	if sendErr := c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error"); sendErr != nil {
		log.Errorf("failed to send error response: %v", sendErr)
	}
}

// configDefault sets default values for the Config struct.
//
// This function ensures that all necessary fields have sensible defaults
// if they are not explicitly set by the user.
//
// Parameters:
//   - config: Variadic parameter to override default config.
//
// Returns:
//   - Config: The configuration with defaults applied.
//
// Usage:
//
//	cfg := configDefault()
//	cfg := configDefault(customConfig)
func configDefault(config ...Config) Config {
	cfg := ConfigDefault
	if len(config) > 0 {
		// Override default config
		cfg = config[0]
	}

	// Set default values
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = ConfigDefault.IdleTimeout
	}

	// Ensure AbsoluteTimeout is greater than or equal to IdleTimeout.
	if cfg.AbsoluteTimeout > 0 && cfg.AbsoluteTimeout < cfg.IdleTimeout {
		panic("[session] AbsoluteTimeout must be greater than or equal to IdleTimeout")
	}

	// Check if we have a zero-value Extractor
	if cfg.Extractor.Extract == nil {
		cfg.Extractor = ConfigDefault.Extractor
	}

	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}

	if cfg.CookieSameSite == "" {
		cfg.CookieSameSite = ConfigDefault.CookieSameSite
	}

	if _, valid := internalcookie.ParseSameSite(cfg.CookieSameSite); !valid {
		panic("[session] CookieSameSite must be one of Disabled, Lax, Strict, or None")
	}

	if isNilStorage(cfg.Storage) {
		panic("[session] Storage must not be a typed nil")
	}

	return cfg
}

// isNilStorage reports whether storage holds a nil pointer, map, slice, channel
// or function. Such a value is not equal to nil, so it passes for a storage and
// then fails on the first request. A storage that is plainly nil is fine: it
// stands for the default.
func isNilStorage(storage fiber.Storage) bool {
	if storage == nil {
		return false
	}
	switch rv := reflect.ValueOf(storage); rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		// Non-nilable kinds (struct, int, string, ...) cannot be a typed nil.
		return false
	}
}

// warnCookiePrefixes logs a warning for each session cookie whose name carries
// a prefix that browsers hold to rules the configuration breaks. A browser
// drops such a cookie without a word, which shows up as sessions that never
// persist. Browsers match the prefixes case-insensitively (RFC 6265bis, 4.1.3).
//
// It only warns: the configurations it flags were accepted before, and a cookie
// the browser rejects is the application's to fix, not a reason to refuse to
// start.
func warnCookiePrefixes(cfg *Config) {
	sameSite, _ := internalcookie.ParseSameSite(cfg.CookieSameSite)
	secure := cfg.CookieSecure || sameSite.RequiresSecure

	cfg.Extractor.Walk(func(candidate extractors.Extractor) bool {
		if len(candidate.Chain) > 0 || candidate.Source != extractors.SourceCookie || candidate.Key == "" {
			return true
		}
		name := utilsstrings.ToLower(candidate.Key)
		switch {
		case strings.HasPrefix(name, "__host-") && (!secure || cfg.CookiePath != "/" || cfg.CookieDomain != ""):
			log.Warnf("[session] cookie %q has the __Host- prefix, which browsers accept only with Secure, Path=/ and no Domain; "+
				"it is configured with Secure=%t, Path=%q and Domain=%q, so browsers will reject it",
				candidate.Key, secure, cfg.CookiePath, cfg.CookieDomain)
		case strings.HasPrefix(name, "__secure-") && !secure:
			log.Warnf("[session] cookie %q has the __Secure- prefix, which browsers accept only with Secure; "+
				"set CookieSecure, or browsers will reject it", candidate.Key)
		default:
		}
		return true
	})
}
