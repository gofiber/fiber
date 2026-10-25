package encryptcookie

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// requestCookie is one entry of the Cookie header as the decrypt pass read it.
type requestCookie struct {
	name   string
	value  string
	except bool
	keep   bool
}

// responseCookie is one entry of the Set-Cookie header as the encrypt pass read it.
type responseCookie struct {
	name       string
	raw        string
	value      string
	valueStart int
	valueEnd   int
	except     bool
	encrypt    bool
}

type cookieValueRange struct {
	name  string
	value string
	start int
	end   int
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
				encryptable = true
			}
			cookies = append(cookies, entry)
		}

		if !encryptable {
			return err
		}

		for i := range cookies {
			if !cookies[i].encrypt {
				continue
			}

			encrypted, encErr := cfg.Encryptor(cookies[i].name, cookies[i].value, cfg.Key)
			if encErr != nil {
				header.DelAllCookies()
				for j := range cookies {
					if cookies[j].except {
						header.Add(fiber.HeaderSetCookie, cookies[j].raw)
					}
				}
				return errors.Join(err, encErr)
			}
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

		return err
	}
}

// decryptRequestCookies replaces each request cookie with its decrypted value
// and drops the ones the Decryptor rejects.
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
			header.Add(fiber.HeaderCookie, ck.name+"="+ck.value)
		default:
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

// splitResponseCookie reads the name and plaintext value of raw Set-Cookie.
func splitResponseCookie(raw string) (cookieValueRange, bool) {
	segmentEnd := len(raw)
	if i := strings.IndexByte(raw, 59); i >= 0 { // 59 is ''';''''
		segmentEnd = i
	}

	eq := strings.IndexByte(raw[:segmentEnd], 61) // 61 is '''=''''
	if eq < 0 {
		return cookieValueRange{}, false
	}

	return cookieValueRange{
		name:  utils.Trim(raw[:eq], ' '),
		value: trimCookieValue(raw[eq+1 : segmentEnd]),
		start: eq + 1,
		end:   segmentEnd,
	}, true
}

// trimCookieValue removes leading/trailing spaces and one surrounding DQUOTE pair.
func trimCookieValue(value string) string {
	value = utils.Trim(value, ' ')
	if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return value
}

// responseCookieValue returns the plaintext value of raw Set-Cookie.
func responseCookieValue(raw string) string {
	parts, ok := splitResponseCookie(raw)
	if !ok {
		return ""
	}
	return parts.value
}

// responseCookieWithValue returns the Set-Cookie line with value replaced.
func responseCookieWithValue(ck responseCookie) string {
	return ck.raw[:ck.valueStart] + ck.value + ck.raw[ck.valueEnd:]
}
