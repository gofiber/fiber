// Package mediatype canonicalizes the case-insensitive parts of a request's
// Content-Type, so the case-sensitive parsers downstream — fasthttp's form and
// multipart readers above all — see the value they expect.
package mediatype

import (
	"bytes"

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
		i++ // step over the '='
		if i < len(ct) && ct[i] == '"' {
			i++
			for i < len(ct) && ct[i] != '"' {
				if ct[i] == '\\' && i+1 < len(ct) {
					i++
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
