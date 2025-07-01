package encryptcookie

import (
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
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
		c.Request().Header.Cookies()(func(key, value []byte) bool {
			keyString := string(key)
			if !isDisabled(keyString, cfg.Except) {
				decryptedValue, err := cfg.Decryptor(string(value), cfg.Key)
				if err != nil {
					c.Request().Header.DelCookieBytes(key)
				} else {
					c.Request().Header.SetCookie(string(key), decryptedValue)
				}
			}
			return true
		})

		// Continue stack
		err := c.Next()

		// Encrypt response cookies
		c.Response().Header.Cookies()(func(key, _ []byte) bool {
			keyString := string(key)
			if !isDisabled(keyString, cfg.Except) {
				cookieValue := fasthttp.Cookie{}
				cookieValue.SetKeyBytes(key)
				if c.Response().Header.Cookie(&cookieValue) {
					encryptedValue, err := cfg.Encryptor(string(cookieValue.Value()), cfg.Key)
					if err != nil {
						panic(err)
					}

					cookieValue.SetValue(encryptedValue)
					c.Response().Header.SetCookie(&cookieValue)
				}
			}
			return true
		})

		return err
	}
}
