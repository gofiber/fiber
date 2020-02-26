package middleware

import (
	"encoding/base64"
	"strconv"
	"strings"

	".."
)

// package main
//
// import (
// 	"github.com/gofiber/fiber"
// 	"github.com/gofiber/fiber/middleware"
// )
//
// func validator(user, pass string) bool {
// 	if user == "john" && pass == "doe" {
// 		return true
// 	}
// 	return false
// }
//
// func main() {
// 	app := fiber.New()
// 	app.Use(middleware.BasicAuth(validator))
// 	app.Get("/", func(c *fiber.Ctx) {
// 		c.Send("Authorized!")
// 	})
// 	app.Listen(3000)
// }

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
