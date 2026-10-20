// Package quotedstring encodes values placed inside HTTP quoted strings.
package quotedstring

import (
	"github.com/gofiber/fiber/v3/internal/bytepolicy"
	"github.com/valyala/bytebufferpool"
)

// Escape encodes raw for placement inside an RFC 9110 quoted-string. Quotes
// and backslashes use quoted-pair escaping, LF and CR use their conventional
// backslash forms, and other C0 controls plus DEL are percent-encoded. HTAB is
// encoded even though RFC 9110 permits it as qdtext. Non-ASCII bytes pass
// through verbatim. The returned value excludes the surrounding quotes.
// Escape is a serializer, not input validation; callers apply their own
// rejection or sanitization policy before calling it.
func Escape(raw string) string {
	end := bytepolicy.Index(raw, bytepolicy.QuotedString)
	if end == -1 {
		return raw
	}

	const hex = "0123456789ABCDEF"
	bb := bytebufferpool.Get()
	defer bytebufferpool.Put(bb)

	bb.B = append(bb.B, raw[:end]...)
	for i := end; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '\\' || c == '"':
			bb.B = append(bb.B, '\\', c)
		case c == '\n':
			bb.B = append(bb.B, '\\', 'n')
		case c == '\r':
			bb.B = append(bb.B, '\\', 'r')
		case c < 0x20 || c == 0x7f:
			bb.B = append(bb.B, '%', hex[c>>4], hex[c&0x0f])
		default:
			bb.B = append(bb.B, c)
		}
	}

	return string(bb.B)
}
