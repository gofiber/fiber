package encryptcookie

import (
	"errors"
	"strings"

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
		return encryptResponseCookies(c, &cfg, err)
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

// responseCookie is one entry of the Set-Cookie headers as the encrypt pass
// read it. raw keeps the whole field value, so an entry is written back with
// every attribute it arrived with.
type responseCookie struct {
	name string
	raw  string
	// value is the field value on its own, then the value to write back.
	value string
	// except marks a name listed in Config.Except: raw is added back as it
	// arrived.
	except bool
}

// encryptResponseCookies replaces the value of every response cookie with its
// encrypted value and returns the error the rest of the stack collected.
//
// A response may carry the same cookie name more than once (RFC 6265 Section
// 5.4, the shape a server sends when one cookie lives on two Paths), and
// fasthttp's cookie store is line based while Cookie and SetCookie resolve a
// name to the FIRST entry that has it. Reading and writing one name at a time
// therefore encrypts that first entry once per duplicate and writes the other
// entries straight through in clear text. The lines are read in full first and
// the store is rebuilt from them afterwards, so each entry is encrypted
// exactly once and keeps its own attributes and position, however often its
// name repeats. Names in Except keep the value they arrived with.
//
// When the Encryptor rejects a value, the response cookies that are not in
// Except are dropped, so a value it refused can never leave the response in
// clear text.
func encryptResponseCookies(c fiber.Ctx, cfg *Config, stackErr error) error {
	header := &c.Response().Header
	cookies := make([]responseCookie, 0, 4)

	for name, raw := range header.Cookies() {
		key := string(name)
		cookies = append(cookies, responseCookie{
			name:   key,
			raw:    string(raw),
			value:  responseCookieValue(string(raw)),
			except: isDisabled(key, cfg.Except),
		})
	}

	for i := range cookies {
		if cookies[i].except {
			continue
		}

		encrypted, err := cfg.Encryptor(cookies[i].name, cookies[i].value, cfg.Key)
		if err != nil {
			for j := range cookies {
				if !cookies[j].except {
					header.DelCookie(cookies[j].name)
				}
			}

			return errors.Join(stackErr, err)
		}
		cookies[i].value = encrypted
	}

	header.DelAllCookies()
	for i := range cookies {
		if cookies[i].except {
			header.Add(fiber.HeaderSetCookie, cookies[i].raw)
			continue
		}
		header.Add(fiber.HeaderSetCookie, responseCookieWithValue(cookies[i].raw, cookies[i].value))
	}

	return stackErr
}

// responseCookieValue returns the value of the Set-Cookie field value raw,
// which is everything after the first '=' and before the first ';'.
func responseCookieValue(raw string) string {
	_, value, found := strings.Cut(raw, "=")
	if !found {
		return ""
	}

	if end := strings.IndexByte(value, ';'); end >= 0 {
		value = value[:end]
	}

	return value
}

// responseCookieWithValue returns the Set-Cookie field value raw with only its
// value replaced, so every attribute of the entry is kept as it was.
func responseCookieWithValue(raw, value string) string {
	name, rest, found := strings.Cut(raw, "=")
	if !found {
		return raw
	}

	if end := strings.IndexByte(rest, ';'); end >= 0 {
		return name + "=" + value + rest[end:]
	}

	return name + "=" + value
}
