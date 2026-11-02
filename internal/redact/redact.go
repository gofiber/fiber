// Package redact provides common helpers for masking sensitive values in log
// output. It is internal because the redaction policy is not part of Fiber's
// public API.
package redact

const (
	// MinLength is the inclusive lower bound on input length for which the
	// redacted form keeps a leading clear-text prefix. Values shorter than
	// this are fully masked. Eight bytes is enough that a 4-byte prefix
	// never reveals more than half of a typical opaque token.
	MinLength = 8
	// PrefixLength is the number of leading bytes kept when the input is at
	// least MinLength long.
	PrefixLength = 4
	// Mask is appended after the visible prefix (or returned alone when the
	// input is too short to keep a prefix).
	Mask = "****"
)

// Placeholder replaces a whole value (a storage key, a configured origin)
// in log and error output when value redaction is enabled.
const Placeholder = "[redacted]"

// Value returns Placeholder when enabled is true and value unchanged
// otherwise.
func Value(value string, enabled bool) string { //nolint:revive // flag-parameter: enabled is the middleware's inverted DisableValueRedaction setting
	if enabled {
		return Placeholder
	}
	return value
}

// Masker returns a function that applies Value with the given enabled flag.
func Masker(enabled bool) func(string) string {
	return func(value string) string {
		return Value(value, enabled)
	}
}

// Prefix returns a redacted form of value. Empty input is returned unchanged.
// Inputs shorter than MinLength are fully masked. Inputs at least MinLength
// long return the leading PrefixLength bytes followed by Mask. Operations are
// byte-wise; callers must not feed user-controlled UTF-8 they intend to
// truncate by rune boundaries.
func Prefix(value string) string {
	if value == "" {
		return ""
	}
	if len(value) < MinLength {
		return Mask
	}
	return value[:PrefixLength] + Mask
}
