package csrf

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// KeyLookup is a string in the form of "<source>:<key>" that is used
	// to extract token from the request.
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	// - "cookie:<name>"
	//
	// Optional. Default: "header:X-CSRF-Token"
	KeyLookup string

	// Cookie settings to pass the CSRF token to the client on GET
	// requests.
	//
	// Optional.
	Cookie *fiber.Cookie

	// Expiration is the duration before csrf token will expire
	//
	// Optional. Default: 1 * time.Hour
	Expiration time.Duration

	// Store is used to store the state of the middleware
	//
	// Optional. Default: memory.New()
	Storage fiber.Storage

	// Context key to store generated CSRF token into context.
	// If left empty, token will not be stored in context.
	//
	// Optional. Default: ""
	ContextKey string

	// KeyGenerator creates a new CSRF token
	//
	// Optional. Default: utils.UUID
	KeyGenerator func() string

	// Deprecated, please use Expiration
	CookieExpires time.Duration

	// Deprecated, please use KeyLookup
	TokenLookup string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	KeyLookup: "header:X-Csrf-Token",
	Cookie: &fiber.Cookie{
		Name:     "_csrf",
		SameSite: "Strict",
	},
	Expiration:   1 * time.Hour,
	KeyGenerator: utils.UUID,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.TokenLookup != "" {
		fmt.Println("[CSRF] TokenLookup is deprecated, please use KeyLookup")
		cfg.KeyLookup = cfg.TokenLookup
	}
	if int(cfg.CookieExpires.Seconds()) >= 0 {
		fmt.Println("[CSRF] CookieExpires is deprecated, please use Expiration")
		cfg.Expiration = cfg.CookieExpires
	}
	if cfg.KeyLookup == "" {
		cfg.KeyLookup = ConfigDefault.KeyLookup
	}
	if int(cfg.Expiration.Seconds()) <= 0 {
		cfg.Expiration = ConfigDefault.Expiration
	}
	if cfg.Cookie != nil {
		if cfg.Cookie.Name == "" {
			cfg.Cookie.Name = ConfigDefault.Cookie.Name
		}
		if cfg.Cookie.SameSite == "" {
			cfg.Cookie.SameSite = ConfigDefault.Cookie.SameSite
		}
	} else {
		cfg.Cookie = ConfigDefault.Cookie
	}
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}

	return cfg
}
