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

// StripParams returns ct without its parameters and surrounding whitespace.
func StripParams(ct []byte) []byte {
	if i := bytes.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return utils.TrimSpace(ct)
}

// Params yields the trimmed name and value of each parameter in ct that has an "=".
func Params(ct []byte) iter.Seq2[[]byte, []byte] {
	return func(yield func(name, value []byte) bool) {
		_, params, ok := utils.CutByte(ct, ';')
		if !ok {
			return
		}
		for len(params) > 0 {
			param := params
			if end := paramEnd(params); end >= 0 {
				param, params = params[:end], params[end+1:]
			} else {
				params = nil
			}
			name, value, ok := utils.CutByte(param, '=')
			if !ok {
				continue
			}
			if !yield(utils.TrimSpace(name), utils.TrimSpace(value)) {
				return
			}
		}
	}
}

// paramEnd returns the index of the ';' that ends the first parameter, or -1.
func paramEnd(params []byte) int {
	// A DQUOTE opens a quoted-string only at value position, so a stray quote in
	// a bare value cannot swallow the parameters after it.
	inQuotes, escaped, expectValue := false, false, false
	for i, c := range params {
		switch {
		case escaped:
			escaped = false
		case inQuotes:
			switch c {
			case '\\':
				escaped = true
			case '"':
				inQuotes = false
			}
		case c == '=':
			expectValue = true
		case c == '"' && expectValue:
			inQuotes, expectValue = true, false
		case c == ';':
			return i
		case c != ' ' && c != '\t':
			expectValue = false
		}
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
