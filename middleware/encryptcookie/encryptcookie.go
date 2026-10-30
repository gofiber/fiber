package encryptcookie

import (
	"errors"
	"slices"

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

		if encErr := encryptResponseCookies(c, &cfg); encErr != nil {
			return errors.Join(err, encErr)
		}

		return err
	}
}

// encryptResponseCookies rewrites each Set-Cookie line on its own: a lookup by name
// misses a repeated name, and a re-serialize drops what fasthttp cannot parse.
func encryptResponseCookies(c fiber.Ctx, cfg *Config) error {
	header := &c.Response().Header

	lines := make([]string, 0, 4)
	for _, line := range header.Cookies() {
		lines = append(lines, string(line))
	}

	var encErr error
	for i, line := range lines {
		name, value, attributes := splitSetCookie(line)
		if isDisabled(name, cfg.Except) {
			continue
		}

		var encrypted string
		if encrypted, encErr = cfg.Encryptor(name, value, cfg.Key); encErr != nil {
			lines = slices.DeleteFunc(lines, func(l string) bool {
				n, _, _ := splitSetCookie(l)
				return !isDisabled(n, cfg.Except)
			})
			break
		}

		lines[i] = name + "=" + encrypted + attributes
	}

	header.DelAllCookies()
	for _, line := range lines {
		header.Add(fiber.HeaderSetCookie, line)
	}

	return encErr
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
	// A scan of cookies per entry would be quadratic in what the client sends
	seen := make(map[string]struct{})
	rebuild := false

	for key, value := range header.Cookies() {
		name := string(key)
		if isDisabled(name, cfg.Except) {
			cookies = append(cookies, requestCookie{name: name, value: string(value), except: true, keep: true})
			continue
		}
		if _, ok := seen[name]; ok {
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
		seen[name] = struct{}{}
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
	clear(seen)
	for i := range cookies {
		ck := &cookies[i]
		if !ck.keep {
			continue
		}
		if _, ok := seen[ck.name]; ok {
			// Only an excepted name repeats here. SetCookie would overwrite the
			// entry already written; Add appends one, so it survives as it arrived.
			header.Add(fiber.HeaderCookie, ck.name+"="+ck.value)
			continue
		}
		seen[ck.name] = struct{}{}
		// The name is absent after DelAllCookies, so SetCookie appends the
		// entry in order and stores the value verbatim.
		header.SetCookie(ck.name, ck.value)
	}
}
