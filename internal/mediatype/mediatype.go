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
// request's Content-Type in place and returns the full header value.
//
// The fold must land on the request's own bytes: fasthttp locates the multipart
// boundary and the urlencoded body with case-sensitive comparisons, as does
// binder.FormBinding, so a legal "Multipart/Form-Data" parsed as an empty form.
//
// Media type and parameter names are case-insensitive (RFC 9110 Sections 8.3.1
// and 5.6.6) and are folded; parameter values are not — a boundary is
// case-sensitive, and folding it detaches the header from its body.
//
// Here rather than in package fiber because every entry point reaching those
// parsers has to apply it, and one copy keeps them from drifting.
func NormalizeRequestContentType(h *fasthttp.RequestHeader) []byte {
	ct := h.ContentType()

	// Nothing to fold is both the common and the repeat case: after the first
	// FormValue the header is already lowercase, so answer those with one byte
	// scan and no writes. Uppercase confined to a parameter value still takes
	// the full walk, which leaves it alone — slower than needed, never wrong.
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

		// Step over the value without touching it. A quoted-string may
		// contain ';' (RFC 9110 Section 5.6.6), so it has to be consumed as a
		// unit or the next parameter name would be mislocated.
		//
		// A backslash is not an escape here, though RFC 9110 allows quoted-pair:
		// fasthttp's boundary scanner splits on ';' with no quoting rules, so
		// honoring one only creates inputs the two disagree about. Ending at the
		// first quote keeps this no more permissive than the parser it feeds.
		i++ // step over the '='
		if i < len(ct) && ct[i] == '"' {
			i++
			for i < len(ct) && ct[i] != '"' {
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
// parsers handle.
//
// Callers gate NormalizeRequestContentType on this: the fold lands on the
// request's own bytes, so running it on a JSON request rewrites a header the
// caller may hold a view into, to no purpose.
//
// Compared folded (RFC 9110 Section 8.3.1), parameters ignored.
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
