package encryptcookie

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// responseCookie is one entry of the Set-Cookie header as the encrypt pass read
// it. line keeps the whole field value, so an entry is written back with every
// attribute it arrived with.
type responseCookie struct {
	name string
	line string
}

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
		header := &c.Response().Header
		cookieLines := make([]responseCookie, 0, 4)
		for name, raw := range header.Cookies() {
			cookieLines = append(cookieLines, responseCookie{name: string(name), line: string(raw)})
		}

		cookies := make([]string, 0, len(cookieLines))
		encryptable := false
		for i := range cookieLines {
			ck := &cookieLines[i]
			if isDisabled(ck.name, cfg.Except) {
				cookies = append(cookies, ck.line)
				continue
			}

			encryptable = true
			// The value ends at the first ';', so an '=' inside an attribute
			// is never read as the value.
			segment, attributes, hasAttributes := strings.Cut(ck.line, ";")
			_, value, found := strings.Cut(segment, "=")
			if !found {
				// No name to seal the value under, so the line is not sent.
				continue
			}

			encryptedValue, encErr := cfg.Encryptor(ck.name, trimCookieValue(value), cfg.Key)
			if encErr != nil {
				header.DelAllCookies()
				for j := range cookieLines {
					if isDisabled(cookieLines[j].name, cfg.Except) {
						header.Add(fiber.HeaderSetCookie, cookieLines[j].line)
					}
				}

				return errors.Join(err, encErr)
			}

			// A ';' in the encrypted value would inject attributes, the same way
			// fasthttp's Cookie.SetValue strips it.
			line := ck.name + "=" + strings.ReplaceAll(encryptedValue, ";", " ")
			if hasAttributes {
				line += ";" + attributes
			}
			cookies = append(cookies, line)
		}

		if encryptable {
			header.DelAllCookies()
			for _, line := range cookies {
				header.Add(fiber.HeaderSetCookie, line)
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
	// keep is false for an entry that is dropped: one the Decryptor rejected,
	// or a later duplicate of a name that already decrypted.
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
// survivors re-added in their original order, which keeps at most one entry
// per name: the first value that decrypts, as c.Cookies returns the first
// plaintext cookie of a name. Names in Except keep every entry as it arrived.
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
		if hasCookieName(cookies, name) {
			// The name already has its value; a later duplicate is dropped
			// without decrypting it.
			rebuild = true
			cookies = append(cookies, requestCookie{name: name})
			continue
		}
		decrypted, err := cfg.Decryptor(name, string(value), cfg.Key)
		if err != nil {
			rebuild = true
			cookies = append(cookies, requestCookie{name: name})
			continue
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
	for i := range cookies {
		ck := &cookies[i]
		switch {
		case !ck.keep:
			continue
		case ck.except && hasExceptedName(cookies[:i], ck.name):
			// SetCookie would overwrite the entry already written for this
			// name. Add appends a Cookie entry and keeps the existing ones, so
			// a repeated excepted name survives as it arrived.
			header.Add(fiber.HeaderCookie, ck.name+"="+ck.value)
		default:
			// The name is absent after DelAllCookies, so SetCookie appends the
			// entry in order and stores the value verbatim.
			header.SetCookie(ck.name, ck.value)
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

// hasExceptedName reports whether cookies holds an excepted entry named name.
func hasExceptedName(cookies []requestCookie, name string) bool {
	for i := range cookies {
		if cookies[i].except && cookies[i].name == name {
			return true
		}
	}
	return false
}

// trimCookieValue trims the spaces and the single pair of quotes that fasthttp
// strips from a cookie value.
func trimCookieValue(value string) string {
	value = utils.Trim(value, ' ')
	if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
