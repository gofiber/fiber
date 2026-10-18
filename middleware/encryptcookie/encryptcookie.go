package encryptcookie

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Decrypt request cookies
		cookiesToDelete := make([][]byte, 0, 4)
		// A Cookie header may carry the same name more than once (RFC 6265
		// Section 5.4, e.g. the same name set under two Paths or Domains).
		// Header.Cookie, and so c.Cookies, returns the first of those, while
		// SetCookie rewrites that first entry and DelCookie removes every entry
		// with the name. Handle each name once, on its first occurrence, so a
		// later duplicate can neither replace the value the handler reads nor
		// get it deleted.
		seen := make([][]byte, 0, 4)

		for key, value := range c.Request().Header.Cookies() {
			keyString := string(key)
			if isDisabled(keyString, cfg.Except) || containsBytes(seen, key) {
				continue
			}
			seen = append(seen, key)

			decryptedValue, err := cfg.Decryptor(keyString, string(value), cfg.Key)
			if err != nil {
				cookiesToDelete = append(cookiesToDelete, key)
			} else {
				c.Request().Header.SetCookie(keyString, decryptedValue)
			}
		}

		// Delete cookies that failed to decrypt - outside the loop to avoid mutation during iteration
		for _, key := range cookiesToDelete {
			c.Request().Header.DelCookieBytes(key)
		}

		// Continue stack
		err := c.Next()

		// Encrypt response cookies
		responseCookieKeys := make([]string, 0, 4)
		for key := range c.Response().Header.Cookies() {
			responseCookieKeys = append(responseCookieKeys, string(key))
		}

		for _, keyString := range responseCookieKeys {
			if isDisabled(keyString, cfg.Except) {
				continue
			}

			cookieValue := fasthttp.Cookie{}
			cookieValue.SetKey(keyString)
			if c.Response().Header.Cookie(&cookieValue) {
				encryptedValue, encErr := cfg.Encryptor(keyString, string(cookieValue.Value()), cfg.Key)
				if encErr != nil {
					for _, responseCookieKey := range responseCookieKeys {
						if !isDisabled(responseCookieKey, cfg.Except) {
							c.Response().Header.DelCookie(responseCookieKey)
						}
					}

					return errors.Join(err, encErr)
				}

				cookieValue.SetValue(encryptedValue)
				c.Response().Header.SetCookie(&cookieValue)
			}
		}

		return err
	}
}
