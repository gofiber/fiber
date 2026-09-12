// Package quotedstring encodes values placed inside HTTP quoted strings.
package quotedstring

import (
	"github.com/gofiber/utils/v2/swar"
	"github.com/valyala/bytebufferpool"
)

// escapeMask marks the lanes of w holding bytes Escape must encode: '\\',
// '"', any C0 control (including HTAB), or DEL. Lanes >= 0x80 are never
// marked; non-ASCII bytes pass through verbatim.
func escapeMask(w uint64) uint64 {
	return swar.MatchByteMask(w, '\\') | swar.MatchByteMask(w, '"') |
		swar.MatchRangeMask(w, 0x00, 0x1f) | swar.MatchByteMask(w, 0x7f)
}

// indexEscape returns the index of the first byte escapeMask matches, or -1
// when raw needs no escaping. It scans eight bytes at a time, finishing inputs
// of 8+ bytes with one overlapping word; shorter inputs are checked byte-wise.
func indexEscape(raw string) int {
	n := len(raw)
	i := 0
	for ; i+swar.WordLen <= n; i += swar.WordLen {
		if m := escapeMask(swar.Load8(raw, i)); m != 0 {
			return i + swar.FirstLane(m)
		}
	}
	if i == n {
		return -1
	}
	if n >= swar.WordLen {
		if m := escapeMask(swar.Load8(raw, n-swar.WordLen)); m != 0 {
			return n - swar.WordLen + swar.FirstLane(m)
		}
		return -1
	}
	for ; i < n; i++ {
		if c := raw[i]; c == '\\' || c == '"' || c < 0x20 || c == 0x7f {
			return i
		}
	}
	return -1
}

// Escape encodes raw for placement inside an RFC 9110 quoted-string. Quotes
// and backslashes use quoted-pair escaping, LF and CR use their conventional
// backslash forms, and other C0 controls plus DEL are percent-encoded. HTAB is
// encoded even though RFC 9110 permits it as qdtext. Non-ASCII bytes pass
// through verbatim. The returned value excludes the surrounding quotes.
// Escape is a serializer, not input validation; callers apply their own
// rejection or sanitization policy before calling it.
func Escape(raw string) string {
	end := indexEscape(raw)
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
