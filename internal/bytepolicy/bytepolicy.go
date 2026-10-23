package bytepolicy

import (
	"github.com/gofiber/utils/v2/swar"
)

type Policy uint8

const (
	VisibleASCII Policy = iota
	HeaderValue
	QuotedString
)

func Index(s string, policy Policy) int {
	n := len(s)
	if n >= swar.WordLen {
		for i := 0; i < n; i += swar.WordLen {
			offset := min(i, n-swar.WordLen)
			w := swar.Load8(s, offset)
			invalid := swar.MatchRangeMask(w, 0x20, 0x7e) ^ swar.HighBits
			switch policy {
			case VisibleASCII:
			case HeaderValue:
				invalid &^= swar.MatchByteMask(w, '\t')
			case QuotedString:
				// Mask bits sit on each lane's bit 7, so &^w clears the lanes of bytes >= 0x80, which pass raw.
				invalid = invalid&^w | swar.MatchByteMask(w, '\\') | swar.MatchByteMask(w, '"')
			}
			if invalid != 0 {
				return offset + swar.FirstLane(invalid)
			}
		}
		return -1
	}
	for i := range n {
		c := s[i]
		if policy == QuotedString {
			if c < 0x20 || c == 0x7f || c == '\\' || c == '"' {
				return i
			}
		} else if (c < 0x20 || c > 0x7e) && (policy != HeaderValue || c != '\t') {
			return i
		}
	}
	return -1
}
