package encryptcookie

import (
	"errors"
	"slices"

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
		decryptRequestCookies(c, &cfg)

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

// requestCookie is one entry of the Cookie header as the decrypt pass read it.
type requestCookie struct {
	name  string
	value string
	// except marks a name listed in Config.Except: value is the raw cookie
	// and is written back unchanged.
	except bool
	// keep is false when the Decryptor rejected the value.
	keep bool
}

// decryptRequestCookies replaces each request cookie with its decrypted value
// and drops the ones the Decryptor rejects.
//
// A Cookie header may carry the same name more than once (RFC 6265 Section
// 5.4, e.g. set under two Paths or Domains), and fasthttp's cookie API works by
// name: SetCookie rewrites the first entry with that name, DelCookie removes
// every entry with it, and DelCookie swaps the last entry into the hole, so a
// deletion reorders what remains. The header is therefore read in full first,
// and mutated only afterwards. When nothing failed and no name repeats, each
// value is written back in place. Otherwise the cookies are cleared and the
// survivors re-added in their original order, which keeps exactly one entry
// per name: the first value that decrypts, matching what c.Cookies returns
// for plaintext cookies. Names in Except keep every entry as it arrived.
func decryptRequestCookies(c fiber.Ctx, cfg *Config) {
	header := &c.Request().Header
	cookies := make([]requestCookie, 0, 8)
	rebuild := false

	for key, value := range header.Cookies() {
		name := string(key)
		if isDisabled(name, cfg.Except) {
			cookies = append(cookies, requestCookie{name: name, value: string(value), except: true, keep: true})
			continue
		}
		decrypted, err := cfg.Decryptor(name, string(value), cfg.Key)
		if err != nil {
			rebuild = true
			cookies = append(cookies, requestCookie{name: name})
			continue
		}
		if !rebuild && hasCookieName(cookies, name) {
			rebuild = true
		}
		cookies = append(cookies, requestCookie{name: name, value: decrypted, keep: true})
	}

	if !rebuild {
		for i := range cookies {
			if !cookies[i].except {
				header.SetCookie(cookies[i].name, cookies[i].value)
			}
		}
		return
	}

	header.DelAllCookies()
	written := make([]string, 0, len(cookies))
	for i := range cookies {
		ck := &cookies[i]
		switch {
		case !ck.keep:
			continue
		case ck.except:
			// Add appends a Cookie entry and keeps the existing ones, so a
			// repeated excepted name survives as it arrived.
			header.Add(fiber.HeaderCookie, ck.name+"="+ck.value)
		case slices.Contains(written, ck.name):
			continue
		default:
			header.SetCookie(ck.name, ck.value)
			written = append(written, ck.name)
		}
	}
}

// hasCookieName reports whether cookies holds a kept, non-excepted entry named name.
func hasCookieName(cookies []requestCookie, name string) bool {
	for i := range cookies {
		if cookies[i].keep && !cookies[i].except && cookies[i].name == name {
			return true
		}
	}
	return false
}
