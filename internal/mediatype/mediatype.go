// Package mediatype canonicalizes the case-insensitive parts of a request's
// Content-Type, so the case-sensitive parsers downstream — fasthttp's form and
// multipart readers above all — see the value they expect.
package mediatype

import (
	"bytes"

	"github.com/gofiber/utils/v2"
	utilsbytes "github.com/gofiber/utils/v2/bytes"
	"github.com/valyala/fasthttp"
)

// NormalizeRequestContentType lowercases the case-insensitive parts of a
// request's Content-Type in place. The fold must land on the request's own
// bytes: fasthttp locates the boundary and the urlencoded body case-sensitively.
func NormalizeRequestContentType(h *fasthttp.RequestHeader) []byte {
	ct := h.ContentType()

	// Nothing to fold is both the common and the repeat case: after the first
	// FormValue the header is already lowercase. Uppercase confined to a parameter
	// value still takes the full walk, which leaves it alone.
	if !hasUpper(ct) {
		return ct
	}

	i := bytes.IndexByte(ct, ';')
	if i == -1 {
		utilsbytes.UnsafeToLower(ct)
		return ct
	}
	utilsbytes.UnsafeToLower(ct[:i])

	for i < len(ct) {
		i++ // step over the ';'
		for i < len(ct) && (ct[i] == ' ' || ct[i] == '\t') {
			i++
		}

		nameStart := i
		for i < len(ct) && ct[i] != '=' && ct[i] != ';' {
			i++
		}
		utilsbytes.UnsafeToLower(ct[nameStart:i])
		if i >= len(ct) || ct[i] == ';' {
			continue
		}

		// Step over the value untouched: a quoted-string may contain ';' (RFC 9110
		// §5.6.6), and a quoted-pair may contain '"'. Ending the string at an
		// escaped quote would fold the rest as parameters, and fasthttp matches
		// "boundary" case-sensitively, so lowercasing a decoy inside a value is
		// what makes it the one fasthttp picks: on
		// `X="\"; BOUNDARY=bogus"; BOUNDARY=Real` it took bogus over Real.
		i++ // step over the '='
		if i < len(ct) && ct[i] == '"' {
			i++
			for i < len(ct) && ct[i] != '"' {
				if ct[i] == '\\' && i+1 < len(ct) {
					i++ // the escaped byte cannot close the string
				}
				i++
			}
			if i < len(ct) {
				i++ // closing quote
			}
		}
		for i < len(ct) && ct[i] != ';' {
			i++
		}
	}

	return ct
}

// Form media types, duplicated from package fiber because that package imports
// this one.
const (
	applicationForm = "application/x-www-form-urlencoded"
	multipartForm   = "multipart/form-data"
)

// IsForm reports whether ct names one of the two media types fasthttp's form
// parsers handle. Callers gate NormalizeRequestContentType on it, since the fold
// lands on the request's own bytes. Compared folded, parameters ignored.
func IsForm(ct []byte) bool {
	if i := bytes.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = bytes.TrimRight(ct, " \t")

	name := utils.UnsafeString(ct)
	return utils.EqualFold(name, applicationForm) || utils.EqualFold(name, multipartForm)
}

// hasUpper reports whether b holds an ASCII uppercase byte.
func hasUpper(b []byte) bool {
	for _, c := range b {
		if c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
}
