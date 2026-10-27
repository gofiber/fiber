// Package mediatype parses media types and their parameters (RFC 9110 Section
// 8.3.1), and canonicalizes the case-insensitive parts of a request's
// Content-Type, so the case-sensitive parsers downstream — fasthttp's form and
// multipart readers above all — see the value they expect.
package mediatype

import (
	"bytes"
	"iter"

	"github.com/gofiber/utils/v2"
	utilsbytes "github.com/gofiber/utils/v2/bytes"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/http/httpguts"
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
		// Params tolerates SP/HTAB before a value; preserve the same quoted
		// region here even though RFC 9110 forbids whitespace around '='.
		for i < len(ct) && (ct[i] == ' ' || ct[i] == '\t') {
			i++
		}
		if i < len(ct) && ct[i] == '"' {
			end := quotedParamEnd(ct, i)
			if end < 0 {
				return ct
			}
			i = end
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

// StripParams returns ct without its parameters and surrounding whitespace.
func StripParams(ct []byte) []byte {
	if i := bytes.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return utils.TrimSpace(ct)
}

// IsToken reports whether b is a token (RFC 9110 Section 5.6.2): one or more tchar.
func IsToken(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		if !httpguts.IsTokenRune(rune(c)) {
			return false
		}
	}
	return true
}

// Params yields the trimmed name and value of each parameter in ct that has an "=".
func Params(ct []byte) iter.Seq2[[]byte, []byte] {
	return func(yield func(name, value []byte) bool) {
		// CutByte and bytes.Cut do not inline here; keep the entry scan inline.
		params := ct
		semi := bytes.IndexByte(params, ';')
		if semi < 0 {
			return
		}
		params = params[semi+1:]
		for len(params) > 0 {
			if params[0] == ';' {
				params = params[1:]
				continue
			}
			param := params
			end := bytes.IndexByte(params, ';')
			if end > 0 && bytes.IndexByte(params[:end], '"') >= 0 {
				eq := bytes.IndexByte(params[:end], '=')
				open := eq + 1
				for open < end && (params[open] == ' ' || params[open] == '\t') {
					open++
				}
				// Only the first '=' starts a value; later '=' and stray quotes
				// in bare values must not hide subsequent parameters.
				if eq >= 0 && open < end && params[open] == '"' {
					end = quotedParamEnd(params, open)
				}
			}
			if end >= 0 {
				param, params = params[:end], params[end+1:]
			} else {
				params = nil
			}
			eq := bytes.IndexByte(param, '=')
			if eq < 0 {
				continue
			}
			if !yield(utils.TrimSpace(param[:eq]), utils.TrimSpace(param[eq+1:])) {
				return
			}
		}
	}
}

// quotedParamEnd returns the first ';' after the quoted value opened at b[open],
// or -1 if the parameter consumes the rest of b.
func quotedParamEnd(b []byte, open int) int {
	end := bytes.IndexByte(b[open+1:], '"')
	if end < 0 {
		return -1
	}
	end += open + 1
	// A quote closes the string only after an even run of backslashes.
	escaped := false
	for j := end - 1; j > open && b[j] == '\\'; j-- {
		escaped = !escaped
	}
	if escaped {
		// Avoid repeated IndexByte calls for escape-heavy values.
		for end++; end < len(b); end++ {
			if b[end] == '\\' {
				end++
				continue
			}
			if b[end] == '"' {
				break
			}
		}
		if end >= len(b) {
			return -1
		}
	}
	end++
	if end == len(b) {
		return -1
	}
	if b[end] == ';' {
		return end
	}
	if semi := bytes.IndexByte(b[end:], ';'); semi >= 0 {
		return end + semi
	}
	return -1
}

// Unescape resolves the quoted-pairs in v; ok is false if v ends in a lone backslash, which is kept.
func Unescape(v []byte) ([]byte, bool) {
	i := bytes.IndexByte(v, '\\')
	switch i {
	case -1:
		return v, true
	case len(v) - 1:
		return v, false
	}
	out := make([]byte, 0, len(v))
	out = append(out, v[:i]...)
	for ; i < len(v); i++ {
		if v[i] == '\\' {
			if i == len(v)-1 {
				return append(out, '\\'), false
			}
			i++
		}
		out = append(out, v[i])
	}
	return out, true
}
