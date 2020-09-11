package basicauth

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Users defines the allowed credentials
	//
	// Required. Default: map[string]string{}
	Users map[string]string

	// Realm is a string to define realm attribute of BasicAuth.
	// the realm identifies the system to authenticate against
	// and can be used by clients to save credentials
	//
	// Optional. Default: "Restricted".
	Realm string

	// Authorizer defines a function you can pass
	// to check the credentials however you want.
	// It will be called with a username and password
	// and is expected to return true or false to indicate
	// that the credentials were approved or not.
	//
	// Optional. Default: nil.
	Authorizer func(string, string) bool

	// Unauthorized defines the response body for unauthorized responses.
	// By default it will return with a 401 Unauthorized and the correct WWW-Auth header
	//
	// Optional. Default: nil
	Unauthorized fiber.Handler

	// ContextUser is the key to store the username in Locals
	//
	// Optional. Default: "user"
	ContextUser string

	// ContextPass is the key to store the password in Locals
	//
	// Optional. Default: "pass"
	ContextPass string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	Users:        map[string]string{},
	Realm:        "Restricted",
	Authorizer:   nil,
	Unauthorized: nil,
	ContextUser:  "user",
	ContextPass:  "pass",
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.Next == nil {
			cfg.Next = ConfigDefault.Next
		}
		if cfg.Users == nil {
			cfg.Users = ConfigDefault.Users
		}
		if cfg.Realm == "" {
			cfg.Realm = ConfigDefault.Realm
		}
		if cfg.Authorizer == nil {
			cfg.Authorizer = func(user, pass string) bool {
				user, exist := cfg.Users[user]
				if !exist {
					return false
				}
				return user == pass
			}
		}
		if cfg.Unauthorized == nil {
			cfg.Unauthorized = func(c *fiber.Ctx) error {
				c.Set(fiber.HeaderWWWAuthenticate, "basic realm="+cfg.Realm)
				return c.SendStatus(fiber.StatusUnauthorized)
			}
		}
		if cfg.ContextUser == "" {
			cfg.ContextUser = ConfigDefault.ContextUser
		}
		if cfg.ContextPass == "" {
			cfg.ContextPass = ConfigDefault.ContextPass
		}
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get authorization header
		auth := c.Get(fiber.HeaderAuthorization)

		// Check if header is valid
		if len(auth) > 6 && strings.ToLower(auth[:5]) == "basic" {

			// Try to decode
			if raw, err := base64.StdEncoding.DecodeString(auth[6:]); err == nil {

				// Convert to string
				cred := utils.GetString(raw)

				// Find semicolumn
				for i := 0; i < len(cred); i++ {
					if cred[i] == ':' {
						// Split into user & pass
						user := cred[:i]
						pass := cred[i+1:]

						// If exist & match in Users, we let him pass
						if cfg.Authorizer(user, pass) {
							c.Locals(cfg.ContextUser, user)
							c.Locals(cfg.ContextPass, pass)
							return c.Next()
						}
					}
				}
			}
		}
		// Authentication failed
		return cfg.Unauthorized(c)
	}
}
