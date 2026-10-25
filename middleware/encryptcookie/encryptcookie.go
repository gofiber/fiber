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
// every attribute it arrived with. valueStart and valueEnd are the byte range
// of the value inside raw, recorded once at read time so the rewrite splices
// the encrypted value in without parsing the line a second time.
type responseCookie struct {
	name string
	raw  string
	// value is the plaintext read from raw, then the value to write back.
	value string
	// valueStart and valueEnd delimit value inside raw.
	valueStart int
	valueEnd   int
	// except marks a name listed in Config.Except: raw is added back as it
	// arrived.
	except bool
	// encrypt is false for an excepted name and for a line whose name-value
	// segment carries no '=', a line RFC 6265 Section 5.2 says to ignore. Such
	// a line is dropped in the rebuild, because the value it carries is the one
	// this middleware is meant to seal and there is no name to seal it under.
	encrypt bool
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
// name repeats. Names in Except keep the value they arrived with, and the value
// is read with fasthttp's own rules (only in the first ';'-delimited segment,
// spaces trimmed, one surrounding DQUOTE pair stripped), so a quoted or padded
// value round-trips the way main did.
//
// A line whose name-value segment carries no '=', the shape fasthttp stores for
// a cookie with an empty key, has no name to seal its value under. It is dropped
// instead of written back, so the value it carries cannot reach the client in
// clear text.
//
// When the Encryptor rejects a value, the response cookies that are not in
// Except are dropped for the same reason.
func encryptResponseCookies(c fiber.Ctx, cfg *Config, stackErr error) error {
	header := &c.Response().Header
	cookies := make([]responseCookie, 0, 4)
	encryptable := false

	for name, raw := range header.Cookies() {
		line := string(raw)
		entry := responseCookie{name: string(name), raw: line}
		if isDisabled(entry.name, cfg.Except) {
			entry.except = true
		} else if parts, ok := splitResponseCookie(line); ok {
			entry.name = parts.name
			entry.value = parts.value
			entry.valueStart = parts.start
			entry.valueEnd = parts.end
			entry.encrypt = true
			encryptable = true
		} else {
			// The name-value segment carries no '=', so there is no name to
			// seal the value under. The line is dropped in the rebuild below
			// rather than written back, so its value cannot reach the client in
			// clear text.
			encryptable = true
		}

		cookies = append(cookies, entry)
	}

	// Nothing to encrypt: every entry is excepted or carries no name-value
	// pair, so the header is left exactly as it arrived.
	if !encryptable {
		return stackErr
	}

	for i := range cookies {
		if !cookies[i].encrypt {
			continue
		}

		encrypted, err := cfg.Encryptor(cookies[i].name, cookies[i].value, cfg.Key)
		if err != nil {
			// Rebuild with only the excepted lines, in their original order,
			// so the error path keeps the same order guarantee as the success
			// path and never leaves a refused value in clear text.
			header.DelAllCookies()
			for j := range cookies {
				if cookies[j].except {
					header.Add(fiber.HeaderSetCookie, cookies[j].raw)
				}
			}

			return errors.Join(stackErr, err)
		}
		// ';' separates the attributes of a Set-Cookie line, so a value that
		// carries one would inject attributes. fasthttp's SetValue strips it
		// the same way, so a custom Encryptor cannot change the cookie shape.
		cookies[i].value = strings.ReplaceAll(encrypted, ";", " ")
	}

	header.DelAllCookies()
	for i := range cookies {
		switch {
		case cookies[i].encrypt:
			header.Add(fiber.HeaderSetCookie, responseCookieWithValue(cookies[i]))
		case cookies[i].except:
			header.Add(fiber.HeaderSetCookie, cookies[i].raw)
		}
	}

	return stackErr
}

// cookieValueRange is the name and plaintext value of one Set-Cookie field
// value, with the byte range of the value inside the line so the value can be
// replaced without parsing the line a second time.
type cookieValueRange struct {
	name  string
	value string
	start int
	end   int
}

// splitResponseCookie reads the name and the plaintext value of the Set-Cookie
// field value raw. The second result is false when the name-value segment
// carries no '=', a line RFC 6265 Section 5.2 says to ignore. The value is
// taken only from the segment before the first ';', the way fasthttp's
// cookieScanner reads it, so an '=' inside an attribute (Path=/x) cannot be
// mistaken for the value.
func splitResponseCookie(raw string) (cookieValueRange, bool) {
	segmentEnd := len(raw)
	if i := strings.IndexByte(raw, ';'); i >= 0 {
		segmentEnd = i
	}

	eq := strings.IndexByte(raw[:segmentEnd], '=')
	if eq < 0 {
		return cookieValueRange{}, false
	}

	return cookieValueRange{
		name:  strings.Trim(raw[:eq], " "),
		value: trimCookieValue(raw[eq+1 : segmentEnd]),
		start: eq + 1,
		end:   segmentEnd,
	}, true
}

// trimCookieValue removes leading and trailing spaces and one surrounding
// DQUOTE pair, the same trim fasthttp's cookieScanner applies to a Set-Cookie
// value.
func trimCookieValue(value string) string {
	value = strings.Trim(value, " ")
	if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}

	return value
}

// responseCookieValue returns the plaintext value of the Set-Cookie field value
// raw, read with the same trimming fasthttp uses.
func responseCookieValue(raw string) string {
	value, ok := splitResponseCookie(raw)
	if !ok {
		return ""
	}

	return value.value
}

// responseCookieWithValue returns the Set-Cookie field value of ck with its
// value replaced, so every attribute and the position of the entry are kept.
func responseCookieWithValue(ck responseCookie) string {
	return ck.raw[:ck.valueStart] + ck.value + ck.raw[ck.valueEnd:]
}
