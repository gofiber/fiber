package keyauth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// SuccessHandler defines a function which is executed for a valid key.
	//
	// Optional. Default: c.Next()
	SuccessHandler fiber.Handler

	// ErrorHandler defines a function which is executed for an invalid key.
	// It may be used to define a custom error.
	//
	// Optional. Default: 401 Missing or invalid API Key
	ErrorHandler fiber.ErrorHandler

	// Validator is a function to validate the key.
	//
	// Required.
	Validator func(c fiber.Ctx, key string) (bool, error)

	// Realm defines the protected area for WWW-Authenticate responses.
	// This is used to set the `WWW-Authenticate` header when authentication fails.
	//
	// Optional. Default value "Restricted".
	Realm string

	// Challenge defines the full `WWW-Authenticate` header value used when
	// the middleware responds with 401 and no Authorization scheme is
	// present.
	//
	// Optional. Default: `ApiKey realm="<Realm>"` when no Authorization scheme
	// is configured.
	Challenge string

	// Error is the RFC 6750 `error` parameter appended to Bearer
	// `WWW-Authenticate` challenges when validation fails. Allowed values
	// are `invalid_request`, `invalid_token`, or `insufficient_scope`.
	//
	// Optional. Default: "".
	Error string

	// ErrorDescription is the RFC 6750 `error_description` parameter
	// appended to Bearer `WWW-Authenticate` challenges when validation
	// fails. This field requires that `Error` is also set.
	//
	// Optional. Default: "".
	ErrorDescription string

	// ErrorURI is the RFC 6750 `error_uri` parameter appended to Bearer
	// `WWW-Authenticate` challenges when validation fails. This field
	// requires that `Error` is also set.
	//
	// Optional. Default: "".
	ErrorURI string

	// Scope is the RFC 6750 `scope` parameter appended to Bearer
	// challenges when the `error` is `insufficient_scope`. This field
	// requires that `Error` is set to `insufficient_scope`.
	//
	// Optional. Default: "".
	Scope string

	// Extractor is a function to extract the key from the request.
	//
	// Optional. Default: FromAuthHeader("Authorization", "Bearer")
	Extractor Extractor
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	SuccessHandler: func(c fiber.Ctx) error {
		return c.Next()
	},
	ErrorHandler: func(c fiber.Ctx, _ error) error {
		return c.Status(fiber.StatusUnauthorized).SendString(ErrMissingOrMalformedAPIKey.Error())
	},
	Realm:     "Restricted",
	Extractor: FromAuthHeader(fiber.HeaderAuthorization, "Bearer"),
}

// configDefault is a helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		panic("fiber: keyauth middleware requires a validator function")
	}
	cfg := config[0]

	// Require a validator function
	if cfg.Validator == nil {
		panic("fiber: keyauth middleware requires a validator function")
	}

	// Set default values
	if cfg.Extractor.Extract == nil {
		cfg.Extractor = ConfigDefault.Extractor
	}
	if cfg.Realm == "" {
		cfg.Realm = ConfigDefault.Realm
	}
	if cfg.SuccessHandler == nil {
		cfg.SuccessHandler = ConfigDefault.SuccessHandler
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = ConfigDefault.ErrorHandler
	}

	if len(getAuthSchemes(cfg.Extractor)) == 0 && cfg.Challenge == "" {
		cfg.Challenge = fmt.Sprintf("ApiKey realm=%q", cfg.Realm)
	}

	if cfg.Error != "" {
		switch cfg.Error {
		case "invalid_request", "invalid_token", "insufficient_scope":
		default:
			panic("fiber: keyauth unsupported error token")
		}
	}
	if cfg.ErrorDescription != "" && cfg.Error == "" {
		panic("fiber: keyauth error_description requires error")
	}
	if cfg.ErrorURI != "" {
		if cfg.Error == "" {
			panic("fiber: keyauth error_uri requires error")
		}
		if u, err := url.Parse(cfg.ErrorURI); err != nil || !u.IsAbs() {
			panic("fiber: keyauth error_uri must be absolute")
		}
	}
	if cfg.Error == "insufficient_scope" {
		if cfg.Scope == "" {
			panic("fiber: keyauth insufficient_scope requires scope")
		}
		for scope := range strings.SplitSeq(cfg.Scope, " ") {
			if scope == "" || !isScopeToken(scope) {
				panic("fiber: keyauth scope contains invalid token")
			}
		}
	} else if cfg.Scope != "" {
		panic("fiber: keyauth scope requires insufficient_scope error")
	}

	return cfg
}

func isScopeToken(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x21 || c > 0x7e || c == '"' || c == '\\' {
			return false
		}
	}
	return s != ""
}
