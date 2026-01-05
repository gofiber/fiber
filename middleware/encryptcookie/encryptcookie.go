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
		var cookiesToDelete [][]byte

		for key, value := range c.Request().Header.Cookies() {
			keyString := string(key)
			if !isDisabled(keyString, cfg.Except) {
				decryptedValue, err := cfg.Decryptor(keyString, string(value), cfg.Key)
				if err != nil {
					cookiesToDelete = append(cookiesToDelete, key)
				} else {
					c.Request().Header.SetCookie(keyString, decryptedValue)
				}
			}
		}

		// Delete cookies that failed to decrypt - outside the loop to avoid mutation during iteration
		for _, key := range cookiesToDelete {
			c.Request().Header.DelCookieBytes(key)
		}

		// Continue stack
		err := c.Next()

		// Encrypt response cookies
		for key := range c.Response().Header.Cookies() {
			keyString := string(key)
			if !isDisabled(keyString, cfg.Except) {
				cookieValue := fasthttp.Cookie{}
				cookieValue.SetKeyBytes(key)
				if c.Response().Header.Cookie(&cookieValue) {
					encryptedValue, encErr := cfg.Encryptor(keyString, string(cookieValue.Value()), cfg.Key)
					if encErr != nil {
						return errors.Join(err, encErr)
					}

					cookieValue.SetValue(encryptedValue)
					c.Response().Header.SetCookie(&cookieValue)
				}
			}
		}

		return err
	}
}
