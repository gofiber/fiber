package encryptcookie

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// responseCookie is one entry of the Set-Cookie header as the encrypt pass read
// it. key is the name fasthttp stores the line under, name is the one the
// caller wrote, line is the field value as it arrived, and ck holds the parsed
// form of a line that still has to be sealed. An entry with no line and no ck
// is dropped.
type responseCookie struct {
	ck   *fasthttp.Cookie
	key  string
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

		// A response can set the same name more than once, and the order of the
		// lines decides which value the browser keeps, so every line is sealed
		// on its own instead of once per name. The name is read from the first
		// ';' segment, the same segment the value lives in, so Except matches
		// the name the caller wrote.
		var cookies []responseCookie

		for key, raw := range header.Cookies() {
			line := string(raw)
			segment, _, _ := strings.Cut(line, ";")
			name, _, found := strings.Cut(segment, "=")
			name = utils.Trim(name, ' ')

			cookie := responseCookie{key: string(key), name: name, line: line}
			switch {
			case isDisabled(name, cfg.Except):
				// Except keeps the line exactly as it arrived.

			case !found || name == "":
				// A line with no name, or with an empty one, has nothing to
				// seal its value under, so it is not sent instead of going out
				// in clear text. The Except check above can still keep it.
				cookie.line = ""

			default:
				// fasthttp's parser is the one the request side reads with, so
				// a quoted or padded value is sealed as the plain text it
				// stands for. It rejects a value holding a quote or a
				// backslash, and such a line cannot be sealed under its name,
				// so it is not sent either.
				cookie.ck = fasthttp.AcquireCookie()
				if parseErr := cookie.ck.ParseBytes(raw); parseErr != nil {
					fasthttp.ReleaseCookie(cookie.ck)
					cookie.ck, cookie.line = nil, ""
				}
			}

			cookies = append(cookies, cookie)
		}

		defer releaseResponseCookies(cookies)

		var encErr error
		for i := range cookies {
			cookie := &cookies[i]
			if cookie.ck == nil {
				continue
			}

			// name is the same '=' segment the parser read as the key, so it is
			// the name the request side decrypts under, which the default
			// helpers bind as AAD.
			encryptedValue, sealErr := cfg.Encryptor(cookie.name, string(cookie.ck.Value()), cfg.Key)
			if sealErr != nil {
				encErr = sealErr
				break
			}

			// SetValue replaces every ';', which would otherwise start a new
			// attribute and let the value add one.
			cookie.ck.SetValue(encryptedValue)
		}

		// Write once, in the order the lines arrived. A repeated name or a
		// dropped line needs the whole header built again, while a unique name
		// goes back into its own slot without touching the others.
		switch {
		case encErr != nil:
			header.DelAllCookies()
			for i := range cookies {
				if cookies[i].ck == nil && cookies[i].line != "" {
					header.Add(fiber.HeaderSetCookie, cookies[i].line)
				}
			}

			return errors.Join(err, encErr)

		case rebuildResponseCookies(cookies):
			header.DelAllCookies()
			for i := range cookies {
				cookie := &cookies[i]
				switch {
				case cookie.ck != nil:
					header.Add(fiber.HeaderSetCookie, string(cookie.ck.Cookie()))
				case cookie.line != "":
					header.Add(fiber.HeaderSetCookie, cookie.line)
				}
			}

		default:
			for i := range cookies {
				if cookies[i].ck != nil {
					header.SetCookie(cookies[i].ck)
				}
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

// releaseResponseCookies returns every parsed line to the pool.
func releaseResponseCookies(cookies []responseCookie) {
	for i := range cookies {
		if cookies[i].ck != nil {
			fasthttp.ReleaseCookie(cookies[i].ck)
		}
	}
}

// rebuildResponseCookies reports whether the Set-Cookie header has to be built
// again from scratch: a line is dropped, or two lines share a name so they
// cannot be written back one by one.
func rebuildResponseCookies(cookies []responseCookie) bool {
	dropped := false

	for i := range cookies {
		if cookies[i].ck == nil && cookies[i].line == "" {
			dropped = true
		}

		for j := i + 1; j < len(cookies); j++ {
			if cookies[i].key == cookies[j].key {
				return true
			}
		}
	}

	return dropped
}
