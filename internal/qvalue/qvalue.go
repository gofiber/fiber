// Package qvalue parses the weight parameter defined by RFC 9110 §12.4.2.
package qvalue

// Parse reports the weight encoded by a q parameter value. The qvalue grammar
// permits 0, 1, and a decimal point with one to three digits between them.
// Malformed values, including NaN, infinity and negative zero, yield 0 and
// false. Callers can use the boolean to distinguish an absent weight (which
// defaults to 1) from an explicitly malformed one (which is refused).
func Parse(value []byte) (float64, bool) {
	if len(value) == 0 {
		return 0, false
	}

	var whole uint64
	digits := 0
	for _, c := range value {
		if c < '0' || c > '9' {
			break
		}
		whole = whole*10 + uint64(c-'0')
		digits++
	}
	if digits == 0 || digits > 1 || whole > 1 {
		return 0, false
	}
	if digits == len(value) {
		return float64(whole), true
	}
	if value[digits] != '.' {
		return 0, false
	}

	frac := value[digits+1:]
	if len(frac) < 1 || len(frac) > 3 {
		return 0, false
	}
	var fraction float64
	scale := 0.1
	for _, c := range frac {
		if c < '0' || c > '9' {
			return 0, false
		}
		fraction += float64(c-'0') * scale
		scale /= 10
	}
	if whole == 1 && fraction != 0 {
		return 0, false
	}
	return float64(whole) + fraction, true
}
