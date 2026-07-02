package logtemplate

import (
	"slices"
	"strings"
)

// WriteSanitized writes p to output with ASCII control bytes replaced by
// spaces. Tabs are preserved. The replacement is done in a single pass so the
// hot path stays alloc-free for inputs that are already clean (the common
// case): clean inputs forward directly to output.Write.
//
// Use it whenever untrusted, caller-controlled bytes (request paths, headers,
// bodies, query values, ...) are written to a log line. Letting raw CR/LF
// through would allow an attacker to forge additional log entries (log
// injection / CRLF injection).
func WriteSanitized(output Buffer, p []byte) (int, error) {
	if !NeedsControlSanitize(p) {
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

// WriteSanitizedString is the string counterpart of WriteSanitized.
func WriteSanitizedString(output Buffer, s string) (int, error) {
	if !NeedsControlSanitizeString(s) {
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

// NeedsControlSanitize reports whether p contains any byte that
// WriteSanitized would replace.
func NeedsControlSanitize(p []byte) bool {
	return slices.ContainsFunc(p, IsControlByte)
}

// NeedsControlSanitizeString reports whether s contains any byte that
// WriteSanitizedString would replace.
func NeedsControlSanitizeString(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return r < 0x80 && IsControlByte(byte(r)) //nolint:gosec // G115: integer overflow conversion rune -> byte
	}) >= 0
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
