package logtemplate

import (
	"github.com/gofiber/utils/v2/swar"
)

// WriteSanitized writes p to output with ASCII control bytes replaced by
// spaces. Tabs are preserved. Clean inputs (the common case) forward
// directly to output.Write with no allocation; dirty inputs are scrubbed
// into a copy starting at the first control byte.
func WriteSanitized(output Buffer, p []byte) (int, error) {
	idx := IndexControlByte(p)
	if idx == -1 {
		return output.Write(p)
	}
	return output.Write(ScrubControls(p, idx))
}

// WriteSanitizedString is WriteSanitized for strings, keeping the clean
// fast path on output.WriteString.
func WriteSanitizedString(output Buffer, s string) (int, error) {
	idx := IndexControlByte(s)
	if idx == -1 {
		return output.WriteString(s)
	}
	return output.Write(ScrubControls(s, idx))
}

// ScrubControls returns a copy of s with every byte IsControlByte matches
// replaced by a space. idx is the index of the first such byte, so the scan
// starts there and the clean prefix is copied untouched.
//
// A negative idx — what IndexControlByte returns for clean input — is clamped
// to 0 rather than panicking, so ScrubControls(s, IndexControlByte(s)) is
// safe even though the callers here take the clean fast path instead.
func ScrubControls[S ~string | ~[]byte](s S, idx int) []byte {
	if idx < 0 {
		idx = 0
	}
	scrubbed := make([]byte, len(s))
	copy(scrubbed, s)
	for i := idx; i < len(scrubbed); i++ {
		if IsControlByte(scrubbed[i]) {
			scrubbed[i] = ' '
		}
	}
	return scrubbed
}

// IndexControlByte returns the index of the first byte IsControlByte matches,
// or -1 if none is present. It scans eight bytes at a time; inputs of 8+
// bytes finish with one overlapping word, shorter ones byte-wise.
func IndexControlByte[S ~string | ~[]byte](s S) int {
	n := len(s)
	i := 0
	for ; i+swar.WordLen <= n; i += swar.WordLen {
		if m := controlScrubMask(swar.Load8(s, i)); m != 0 {
			return i + swar.FirstLane(m)
		}
	}
	if i == n {
		return -1
	}
	if n >= swar.WordLen {
		if m := controlScrubMask(swar.Load8(s, n-swar.WordLen)); m != 0 {
			return n - swar.WordLen + swar.FirstLane(m)
		}
		return -1
	}
	for ; i < n; i++ {
		if IsControlByte(s[i]) {
			return i
		}
	}
	return -1
}

// controlScrubMask marks the lanes of w holding bytes IsControlByte matches:
// C0 controls except HTAB, plus DEL. Bytes >= 0x80 are never marked.
func controlScrubMask(w uint64) uint64 {
	return (swar.MatchRangeMask(w, 0x00, 0x1f) &^ swar.MatchByteMask(w, '\t')) | swar.MatchByteMask(w, 0x7f)
}

// IsControlByte reports whether b is an ASCII control byte that must not pass
// through to a log line. Tab is preserved because operators frequently use it
// for delimiting structured fields. CR, LF, NUL, and the other C0/DEL bytes
// are replaced — they are the bytes attackers use to forge log lines or
// corrupt terminal output via ANSI escape sequences.
func IsControlByte(b byte) bool {
	if b == '\t' {
		return false
	}
	return b < 0x20 || b == 0x7f
}
