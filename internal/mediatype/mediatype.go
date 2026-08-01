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
// The fold has to land on the request's own bytes rather than on a copy:
// fasthttp locates the multipart boundary and the urlencoded form body with
// case-sensitive comparisons (Request.MultipartFormBoundary matches a
// lowercase "boundary=", Request.PostArgs a lowercase media type), as does
// binder.FormBinding — so a perfectly legal "Multipart/Form-Data" or
// "BOUNDARY=" would otherwise parse as an empty form.
//
// Both the media type and the parameter *names* are case-insensitive
// (RFC 9110 Sections 8.3.1 and 5.6.6) and are folded. Parameter *values* are
// left untouched: a multipart boundary is case-sensitive, and folding it
// detaches the header from the body it describes.
//
// It lives here rather than in package fiber because every entry point that
// reaches those parsers has to apply it — Ctx's own form accessors, Bind, and
// the binder package, which callers may drive directly — and one copy is what
// keeps them from drifting.
func NormalizeRequestContentType(h *fasthttp.RequestHeader) []byte {
	ct := h.ContentType()

	// Nothing to fold at all is both the common case and the repeat case: a
	// handler reading twenty form fields calls FormValue twenty times, and
	// after the first the header is already lowercase. Answering those with one
	// byte scan and no writes keeps the accessor cheap to call in a loop. A
	// header whose only uppercase sits in a parameter value still falls through
	// to the full walk, which leaves that value alone — slower than it needs to
	// be, never wrong.
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
		// A backslash is not treated as an escape, even though RFC 9110 allows
		// quoted-pair. fasthttp's own boundary scanner splits the parameter list
		// on ';' with no quoting rules whatsoever, so honoring an escape here
		// only creates inputs the two disagree about: an unterminated `\"` would
		// swallow the rest of the list and leave a later "BOUNDARY=" unfolded,
		// while fasthttp went on to look for it. Ending the string at the first
		// unescaped-or-not quote keeps this scanner no more permissive than the
		// parser it exists to feed.
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
// Every caller of NormalizeRequestContentType that is about to read a form
// gates on this first. The fold lands on the request's own bytes, so running it
// on, say, a JSON request would rewrite a header the caller may still hold a
// view into — to no purpose, because no form parser is going to read it.
//
// Media types are case-insensitive (RFC 9110 Section 8.3.1), so compare folded,
// and ignore any parameters: a multipart boundary is not part of the name.
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
