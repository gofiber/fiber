package logtemplate

import (
	"github.com/gofiber/utils/v2"
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
// or -1 if none is present.
//
// The set it looks for — C0 controls except HTAB, plus DEL, with bytes >= 0x80
// never matching — is exactly the one utils.IndexControlExceptTab scans for, so
// this is a thin alias over that helper rather than a second SWAR loop to keep
// in step. utils unrolls two words per branch and folds the whole test into one
// arithmetic mask, which measures ~28% faster on a 12-byte path and ~45% faster
// on a 120-byte user agent than the single-word loop this replaced.
func IndexControlByte[S ~string | ~[]byte](s S) int {
	return utils.IndexControlExceptTab(s)
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
