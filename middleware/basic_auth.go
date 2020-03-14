package middleware

import (
	"encoding/base64"
	"log"
	"strings"

	"github.com/gofiber/fiber"
)

// BasicAuthConfig defines the config for BasicAuth middleware
type BasicAuthConfig struct {
	// Skip defines a function to skip middleware.
	// Optional. Default: nil
	Skip func(*fiber.Ctx) bool
	// Users defines the allowed credentials
	// Required. Default: map[string]string{}
	Users map[string]string
	// Realm is a string to define realm attribute of BasicAuth.
	// Optional. Default: "Restricted".
	Realm string
}

// BasicAuthConfigDefault is the default BasicAuth middleware config.
var BasicAuthConfigDefault = BasicAuthConfig{
	Skip:  nil,
	Users: map[string]string{},
	Realm: "Restricted",
}

// BasicAuth ...
func BasicAuth(config ...BasicAuthConfig) func(*fiber.Ctx) {
	log.Println("Warning: middleware.BasicAuth() is deprecated since v1.8.3, please use github.com/gofiber/basicauth")
	// Init config
	var cfg BasicAuthConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	// Set config default values
	if cfg.Users == nil {
		cfg.Users = BasicAuthConfigDefault.Users
	}
	if cfg.Realm == "" {
		cfg.Realm = BasicAuthConfigDefault.Realm
	}
	// Return middleware handler
	return func(c *fiber.Ctx) {
		// Skip middleware if Skip returns true
		if cfg.Skip != nil && cfg.Skip(c) {
			c.Next()
			return
		}
		// Get authorization header
		auth := c.Get(fiber.HeaderAuthorization)
		// Check if characters are provided
		if len(auth) > 6 && strings.ToLower(auth[:5]) == "basic" {
			// Try to decode
			if raw, err := base64.StdEncoding.DecodeString(auth[6:]); err == nil {
				// Convert to string
				cred := string(raw)
				// Find semicolumn
				for i := 0; i < len(cred); i++ {
					if cred[i] == ':' {
						// Split into user & pass
						user := cred[:i]
						pass := cred[i+1:]
						// If exist & match in Users, we let him pass
						if cfg.Users[user] == pass {
							c.Next()
							return
						}
					}
				}
			}
		}
		// Authentication required
		c.Set(fiber.HeaderWWWAuthenticate, "basic realm="+cfg.Realm)
		c.SendStatus(401)
	}
}
