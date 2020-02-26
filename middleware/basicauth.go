package middleware

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gofiber/fiber"
)

// Usage
// app.Use(middleware.BasicAuth(func(user, pass string) bool {
//   if user == "john" && pass == "doe" {
//     return true
//   }
//   return false
// }))

// BasicAuth ...
func BasicAuth(validator func(string, string) bool, realm ...string) func(*fiber.Ctx) {
	const (
		basic = "Basic "
		plen  = len(basic)
	)
	var realmToken = `"Restricted"`
	if len(realm) > 0 {
		realmToken = strconv.Quote(realm[0])
	}

	return func(c *fiber.Ctx) {
		// Get Authorization header
		auth := c.Get(fiber.HeaderAuthorization)
		// Case insensitive prefix match.
		if len(auth) > plen && strings.EqualFold(auth[:plen], basic) {
			// Decode auth string
			raw, err := base64.StdEncoding.DecodeString(auth[plen:])
			if err == nil {
				// Convert to string
				cred := string(raw)
				// Find semicolumn position
				semi := strings.IndexByte(cred, ':')
				if semi > -1 {
					// Pass user & pass to validator func
					if validator(cred[:semi], cred[semi+1:]) {
						// Success!
						c.Next()
						return
					}
				}
			}
		}
		// Return 401 to pop-up login box
		c.Status(401).Set(fiber.HeaderWWWAuthenticate, "Basic realm="+realmToken)
	}
}
