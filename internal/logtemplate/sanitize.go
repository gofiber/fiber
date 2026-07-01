package logtemplate

import (
	"slices"
	"strings"
)

// IsControlByte reports whether b is an ASCII control byte that must not pass
// through to a log line. Tab is preserved because operators frequently use it
// for delimiting structured fields. CR, LF, NUL, and the other C0/DEL bytes
// are replaced — they are the bytes attackers use to forge log lines (log
// injection) or corrupt terminal output via ANSI escape sequences.
func IsControlByte(b byte) bool {
	if b == '\t' {
		return false
	}
	return b < 0x20 || b == 0x7f
}

// WriteSanitized writes p to output with ASCII control bytes replaced by
// spaces. Tabs are preserved. The replacement is done in a single pass so the
// hot path stays alloc-free for inputs that are already clean (the common
// case): clean inputs forward directly to output.Write.
func WriteSanitized(output Buffer, p []byte) (int, error) {
	if !needsControlSanitize(p) {
		return output.Write(p)
	}
	scrubbed := make([]byte, len(p))
	for i, b := range p {
		if IsControlByte(b) {
			scrubbed[i] = ' '
		} else {
			scrubbed[i] = b
		}
	}
	return output.Write(scrubbed)
}

// WriteSanitizedString writes s to output with ASCII control bytes replaced by
// spaces. It mirrors WriteSanitized for string inputs.
func WriteSanitizedString(output Buffer, s string) (int, error) {
	if !needsControlSanitizeString(s) {
		return output.WriteString(s)
	}
	scrubbed := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if IsControlByte(b) {
			scrubbed[i] = ' '
		} else {
			scrubbed[i] = b
		}
	}
	return output.Write(scrubbed)
}

func needsControlSanitize(p []byte) bool {
	return slices.ContainsFunc(p, IsControlByte)
}

func needsControlSanitizeString(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return r < 0x80 && IsControlByte(byte(r)) //nolint:gosec // G115: rune is checked to be < 0x80 before conversion
	}) >= 0
}
