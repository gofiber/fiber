// Package bytepolicy defines which bytes the control-character checks of
// BasicAuth, request IDs and quoted strings reject. Each policy has a word mask
// that marks, on bit 7, every lane of an 8-byte word it rejects, and a byte
// predicate for the scalar tail. Each caller keeps its own scan loop around
// them: a shared index function cost up to 1.8x on arm64.
package bytepolicy

import (
	"github.com/gofiber/utils/v2/swar"
)

// NonVisibleASCIIMask marks the lanes of w outside visible ASCII [0x20, 0x7E].
func NonVisibleASCIIMask(w uint64) uint64 {
	return swar.MatchRangeMask(w, 0x20, 0x7e) ^ swar.HighBits
}

// IsNonVisibleASCII reports whether c is outside visible ASCII [0x20, 0x7E].
func IsNonVisibleASCII(c byte) bool {
	return c < 0x20 || c > 0x7e
}

// NonVisibleASCIIOrTabMask marks the lanes of w that are neither visible ASCII
// nor HTAB. Bytes >= 0x80 are marked, although RFC 9110 field values allow them.
func NonVisibleASCIIOrTabMask(w uint64) uint64 {
	return (swar.MatchRangeMask(w, 0x20, 0x7e) | swar.MatchByteMask(w, '\t')) ^ swar.HighBits
}

// IsNonVisibleASCIIOrTab reports whether c is neither visible ASCII nor HTAB.
func IsNonVisibleASCIIOrTab(c byte) bool {
	return (c < 0x20 && c != '\t') || c > 0x7e
}

// QuotedStringEscapeMask marks the lanes of w holding a byte quotedstring.Escape
// rewrites: '\\', '"', any C0 control (HTAB included) or DEL. Bytes >= 0x80 pass.
func QuotedStringEscapeMask(w uint64) uint64 {
	return swar.MatchByteMask(w, '\\') | swar.MatchByteMask(w, '"') |
		swar.MatchRangeMask(w, 0x00, 0x1f) | swar.MatchByteMask(w, 0x7f)
}

// IsQuotedStringEscape reports whether quotedstring.Escape rewrites c.
func IsQuotedStringEscape(c byte) bool {
	return c == '\\' || c == '"' || c < 0x20 || c == 0x7f
}
