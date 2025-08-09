package keyauth

import (
	"errors"

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
	// Optional. Default: 401 Invalid or expired API Key
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
	ErrorHandler: func(c fiber.Ctx, err error) error {
		if errors.Is(err, ErrMissingOrMalformedAPIKey) {
			return c.Status(fiber.StatusUnauthorized).SendString(err.Error())
		}
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid or expired API Key")
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

	return cfg
}
