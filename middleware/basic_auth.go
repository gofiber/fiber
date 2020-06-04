package middleware

import (
	"github.com/gofiber/fiber"
)

// Config ...
type (
	BasicAuthConfig struct {
		// Filter defines a function to skip middleware.
		// Optional. Default: nil
		Filter func(*fiber.Ctx) bool
		// Users defines the allowed credentials
		// Required. Default: map[string]string{}
		Users map[string]string
		// Realm is a string to define realm attribute of BasicAuth.
		// the realm identifies the system to authenticate against
		// and can be used by clients to save credentials
		// Optional. Default: "Restricted".
		Realm string
		// Authorizer defines a function you can pass
		// to check the credentials however you want.
		// It will be called with a username and password
		// and is expected to return true or false to indicate
		// that the credentials were approved or not.
		// Optional. Default: nil.
		Authorizer BasicAuthAuthorizer
		// Unauthorized defines the response body for unauthorized responses.
		// Optional. Default: nil
		Unauthorized func(*fiber.Ctx)
	}
	// BasicAuthValidator defines a function to validate BasicAuth credentials.
	BasicAuthAuthorizer func(*fiber.Ctx, string, string) bool
)

// Recover will recover from panics and calls the ErrorHandler
func BasicAuth(fn BasicAuthAuthorizer) fiber.Handler {
	var cfg = BasicAuthConfig{
		Authorizer: fn,
	}
	return BasicAuthWithConfig(cfg)
}

func BasicAuthWithConfig(config ...BasicAuthConfig) fiber.Handler {
	return func(ctx *fiber.Ctx) {
		ctx.Next()
	}
}
