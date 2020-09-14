package json

import (
	"bytes"
	"math"
	"reflect"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2/internal/encoding/ascii"
)

// All spaces characters defined in the json specification.
const (
	sp = ' '
	ht = '\t'
	nl = '\n'
	cr = '\r'
)

const (
	escape = '\\'
	quote  = '"'
)

func skipSpaces(b []byte) []byte {
	b, _ = skipSpacesN(b)
	return b
}

func skipSpacesN(b []byte) ([]byte, int) {
	for i := range b {
		switch b[i] {
		case sp, ht, nl, cr:
		default:
			return b[i:], i
		}
	}
	return nil, 0
}

// parseInt parses a decimanl representation of an int64 from b.
//
// The function is equivalent to calling strconv.ParseInt(string(b), 10, 64) but
// it prevents Go from making a memory allocation for converting a byte slice to
// a string (escape analysis fails due to the error returned by strconv.ParseInt).
//
// Because it only works with base 10 the function is also significantly faster
// than strconv.ParseInt.
func parseInt(b []byte, t reflect.Type) (int64, []byte, error) {
	var value int64
	var count int

	if len(b) == 0 {
		return 0, b, syntaxError(b, "cannot decode integer from an empty input")
	}

	if b[0] == '-' {
		const max = math.MinInt64
		const lim = max / 10

		if len(b) == 1 {
			return 0, b, syntaxError(b, "cannot decode integer from '-'")
		}

		if len(b) > 2 && b[1] == '0' && '0' <= b[2] && b[2] <= '9' {
			return 0, b, syntaxError(b, "invalid leading character '0' in integer")
		}

		for _, d := range b[1:] {
			if !(d >= '0' && d <= '9') {
				if count == 0 {
					b, err := inputError(b, t)
					return 0, b, err
				}
				break
			}

			if value < lim {
				return 0, b, unmarshalOverflow(b, t)
			}

			value *= 10
			x := int64(d - '0')

			if value < (max + x) {
				return 0, b, unmarshalOverflow(b, t)
			}

			value -= x
			count++
		}

		count++
	} else {
		const max = math.MaxInt64
		const lim = max / 10

		if len(b) > 1 && b[0] == '0' && '0' <= b[1] && b[1] <= '9' {
			return 0, b, syntaxError(b, "invalid leading character '0' in integer")
		}

		for _, d := range b {
			if !(d >= '0' && d <= '9') {
				if count == 0 {
					b, err := inputError(b, t)
					return 0, b, err
				}
				break
			}
			x := int64(d - '0')

			if value > lim {
				return 0, b, unmarshalOverflow(b, t)
			}

			if value *= 10; value > (max - x) {
				return 0, b, unmarshalOverflow(b, t)
			}

			value += x
			count++
		}
	}

	if count < len(b) {
		switch b[count] {
		case '.', 'e', 'E': // was this actually a float?
			v, r, err := parseNumber(b)
			if err != nil {
				v, r = b[:count+1], b[count+1:]
			}
			return 0, r, unmarshalTypeError(v, t)
		}
	}

	return value, b[count:], nil
}

// parseUint is like parseInt but for unsigned integers.
func parseUint(b []byte, t reflect.Type) (uint64, []byte, error) {
	const max = math.MaxUint64
	const lim = max / 10

	var value uint64
	var count int

	if len(b) == 0 {
		return 0, b, syntaxError(b, "cannot decode integer value from an empty input")
	}

	if len(b) > 1 && b[0] == '0' && '0' <= b[1] && b[1] <= '9' {
		return 0, b, syntaxError(b, "invalid leading character '0' in integer")
	}

	for _, d := range b {
		if !(d >= '0' && d <= '9') {
			if count == 0 {
				b, err := inputError(b, t)
				return 0, b, err
			}
			break
		}
		x := uint64(d - '0')

		if value > lim {
			return 0, b, unmarshalOverflow(b, t)
		}

		if value *= 10; value > (max - x) {
			return 0, b, unmarshalOverflow(b, t)
		}

		value += x
		count++
	}

	if count < len(b) {
		switch b[count] {
		case '.', 'e', 'E': // was this actually a float?
			v, r, err := parseNumber(b)
			if err != nil {
				v, r = b[:count+1], b[count+1:]
			}
			return 0, r, unmarshalTypeError(v, t)
		}
	}

	return value, b[count:], nil
}

// parseUintHex parses a hexadecimanl representation of a uint64 from b.
//
// The function is equivalent to calling strconv.ParseUint(string(b), 16, 64) but
// it prevents Go from making a memory allocation for converting a byte slice to
// a string (escape analysis fails due to the error returned by strconv.ParseUint).
//
// Because it only works with base 16 the function is also significantly faster
// than strconv.ParseUint.
func parseUintHex(b []byte) (uint64, []byte, error) {
	const max = math.MaxUint64
	const lim = max / 0x10

	var value uint64
	var count int

	if len(b) == 0 {
		return 0, b, syntaxError(b, "cannot decode hexadecimal value from an empty input")
	}

parseLoop:
	for i, d := range b {
		var x uint64

		switch {
		case d >= '0' && d <= '9':
			x = uint64(d - '0')

		case d >= 'A' && d <= 'F':
			x = uint64(d-'A') + 0xA

		case d >= 'a' && d <= 'f':
			x = uint64(d-'a') + 0xA

		default:
			if i == 0 {
				return 0, b, syntaxError(b, "expected hexadecimal digit but found '%c'", d)
			}
			break parseLoop
		}

		if value > lim {
			return 0, b, syntaxError(b, "hexadecimal value out of range")
		}

		if value *= 0x10; value > (max - x) {
			return 0, b, syntaxError(b, "hexadecimal value out of range")
		}

		value += x
		count++
	}

	return value, b[count:], nil
}

func parseNull(b []byte) ([]byte, []byte, error) {
	if hasNullPrefix(b) {
		return b[:4], b[4:], nil
	}
	if len(b) < 4 {
		return nil, b[len(b):], unexpectedEOF(b)
	}
	return nil, b, syntaxError(b, "expected 'null' but found invalid token")
}

func parseTrue(b []byte) ([]byte, []byte, error) {
	if hasTruePrefix(b) {
		return b[:4], b[4:], nil
	}
	if len(b) < 4 {
		return nil, b[len(b):], unexpectedEOF(b)
	}
	return nil, b, syntaxError(b, "expected 'true' but found invalid token")
}

func parseFalse(b []byte) ([]byte, []byte, error) {
	if hasFalsePrefix(b) {
		return b[:5], b[5:], nil
	}
	if len(b) < 5 {
		return nil, b[len(b):], unexpectedEOF(b)
	}
	return nil, b, syntaxError(b, "expected 'false' but found invalid token")
}

func parseNumber(b []byte) (v, r []byte, err error) {
	if len(b) == 0 {
		r, err = b, unexpectedEOF(b)
		return
	}

	i := 0
	// sign
	if b[i] == '-' {
		i++
	}

	if i == len(b) {
		r, err = b[i:], syntaxError(b, "missing number value after sign")
		return
	}

	if b[i] < '0' || b[i] > '9' {
		r, err = b[i:], syntaxError(b, "expected digit but got '%c'", b[i])
		return
	}

	// integer part
	if b[i] == '0' {
		i++
		if i == len(b) || (b[i] != '.' && b[i] != 'e' && b[i] != 'E') {
			v, r = b[:i], b[i:]
			return
		}
		if '0' <= b[i] && b[i] <= '9' {
			r, err = b[i:], syntaxError(b, "cannot decode number with leading '0' character")
			return
		}
	}

	for i < len(b) && '0' <= b[i] && b[i] <= '9' {
		i++
	}

	// decimal part
	if i < len(b) && b[i] == '.' {
		i++
		decimalStart := i

		for i < len(b) {
			if c := b[i]; !('0' <= c && c <= '9') {
				if i == decimalStart {
					r, err = b[i:], syntaxError(b, "expected digit but found '%c'", c)
					return
				}
				break
			}
			i++
		}

		if i == decimalStart {
			r, err = b[i:], syntaxError(b, "expected decimal part after '.'")
			return
		}
	}

	// exponent part
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++

		if i < len(b) {
			if c := b[i]; c == '+' || c == '-' {
				i++
			}
		}

		if i == len(b) {
			r, err = b[i:], syntaxError(b, "missing exponent in number")
			return
		}

		exponentStart := i

		for i < len(b) {
			if c := b[i]; !('0' <= c && c <= '9') {
				if i == exponentStart {
					err = syntaxError(b, "expected digit but found '%c'", c)
					return
				}
				break
			}
			i++
		}
	}

	v, r = b[:i], b[i:]
	return
}

func parseUnicode(b []byte) (rune, int, error) {
	if len(b) < 4 {
		return 0, 0, syntaxError(b, "unicode code point must have at least 4 characters")
	}

	u, r, err := parseUintHex(b[:4])
	if err != nil {
		return 0, 0, syntaxError(b, "parsing unicode code point: %s", err)
	}

	if len(r) != 0 {
		return 0, 0, syntaxError(b, "invalid unicode code point")
	}

	return rune(u), 4, nil
}

func parseStringFast(b []byte) ([]byte, []byte, bool, error) {
	if len(b) < 2 {
		return nil, b[len(b):], false, unexpectedEOF(b)
	}
	if b[0] != '"' {
		return nil, b, false, syntaxError(b, "expected '\"' at the beginning of a string value")
	}

	n := bytes.IndexByte(b[1:], '"') + 2
	if n <= 1 {
		return nil, b[len(b):], false, syntaxError(b, "missing '\"' at the end of a string value")
	}
	if bytes.IndexByte(b[1:n], '\\') < 0 && ascii.ValidPrint(b[1:n]) {
		return b[:n], b[n:], false, nil
	}

	for i := 1; i < len(b); i++ {
		switch b[i] {
		case '\\':
			if i++; i < len(b) {
				switch b[i] {
				case '"', '\\', '/', 'n', 'r', 't', 'f', 'b':
				case 'u':
					_, n, err := parseUnicode(b[i+1:])
					if err != nil {
						return nil, b, false, err
					}
					i += n
				default:
					return nil, b, false, syntaxError(b, "invalid character '%c' in string escape code", b[i])
				}
			}

		case '"':
			return b[:i+1], b[i+1:], true, nil

		default:
			if b[i] < 0x20 {
				return nil, b, false, syntaxError(b, "invalid character '%c' in string escape code", b[i])
			}
		}
	}

	return nil, b[len(b):], false, syntaxError(b, "missing '\"' at the end of a string value")
}

func parseString(b []byte) ([]byte, []byte, error) {
	s, b, _, err := parseStringFast(b)
	return s, b, err
}

func parseStringUnquote(b []byte, r []byte) ([]byte, []byte, bool, error) {
	s, b, escaped, err := parseStringFast(b)
	if err != nil {
		return s, b, false, err
	}

	s = s[1 : len(s)-1] // trim the quotes

	if !escaped {
		return s, b, false, nil
	}

	if r == nil {
		r = make([]byte, 0, len(s))
	}

	for len(s) != 0 {
		i := bytes.IndexByte(s, '\\')

		if i < 0 {
			r = appendCoerceInvalidUTF8(r, s)
			break
		}

		r = appendCoerceInvalidUTF8(r, s[:i])
		s = s[i+1:]

		c := s[0]
		switch c {
		case '"', '\\', '/':
			// simple escaped character
		case 'n':
			c = '\n'

		case 'r':
			c = '\r'

		case 't':
			c = '\t'

		case 'b':
			c = '\b'

		case 'f':
			c = '\f'

		case 'u':
			s = s[1:]

			r1, n1, err := parseUnicode(s)
			if err != nil {
				return r, b, true, err
			}
			s = s[n1:]

			if utf16.IsSurrogate(r1) {
				if !hasPrefix(s, `\u`) {
					r1 = unicode.ReplacementChar
				} else {
					r2, n2, err := parseUnicode(s[2:])
					if err != nil {
						return r, b, true, err
					}
					if r1 = utf16.DecodeRune(r1, r2); r1 != unicode.ReplacementChar {
						s = s[2+n2:]
					}
				}
			}

			r = appendRune(r, r1)
			continue

		default: // not sure what this escape sequence is
			return r, b, false, syntaxError(s, "invalid character '%c' in string escape code", c)
		}

		r = append(r, c)
		s = s[1:]
	}

	return r, b, true, nil
}

func appendRune(b []byte, r rune) []byte {
	n := len(b)
	b = append(b, 0, 0, 0, 0)
	return b[:n+utf8.EncodeRune(b[n:], r)]
}

func appendCoerceInvalidUTF8(b []byte, s []byte) []byte {
	c := [4]byte{}

	for _, r := range string(s) {
		b = append(b, c[:utf8.EncodeRune(c[:], r)]...)
	}

	return b
}

func parseObject(b []byte) ([]byte, []byte, error) {
	if len(b) < 2 {
		return nil, b[len(b):], unexpectedEOF(b)
	}

	if b[0] != '{' {
		return nil, b, syntaxError(b, "expected '{' at the beginning of an object value")
	}

	var err error
	var a = b
	var n = len(b)
	var i = 0

	b = b[1:]
	for {
		b = skipSpaces(b)

		if len(b) == 0 {
			return nil, b, syntaxError(b, "cannot decode object from empty input")
		}

		if b[0] == '}' {
			j := (n - len(b)) + 1
			return a[:j], a[j:], nil
		}

		if i != 0 {
			if len(b) == 0 {
				return nil, b, syntaxError(b, "unexpected EOF after object field value")
			}
			if b[0] != ',' {
				return nil, b, syntaxError(b, "expected ',' after object field value but found '%c'", b[0])
			}
			b = skipSpaces(b[1:])
			if len(b) == 0 {
				return nil, b, unexpectedEOF(b)
			}
			if b[0] == '}' {
				return nil, b, syntaxError(b, "unexpected trailing comma after object field")
			}
		}

		_, b, err = parseString(b)
		if err != nil {
			return nil, b, err
		}
		b = skipSpaces(b)

		if len(b) == 0 {
			return nil, b, syntaxError(b, "unexpected EOF after object field key")
		}
		if b[0] != ':' {
			return nil, b, syntaxError(b, "expected ':' after object field key but found '%c'", b[0])
		}
		b = skipSpaces(b[1:])

		_, b, err = parseValue(b)
		if err != nil {
			return nil, b, err
		}

		i++
	}
}

func parseArray(b []byte) ([]byte, []byte, error) {
	if len(b) < 2 {
		return nil, b[len(b):], unexpectedEOF(b)
	}

	if b[0] != '[' {
		return nil, b, syntaxError(b, "expected '[' at the beginning of array value")
	}

	var err error
	var a = b
	var n = len(b)
	var i = 0

	b = b[1:]
	for {
		b = skipSpaces(b)

		if len(b) == 0 {
			return nil, b, syntaxError(b, "missing closing ']' after array value")
		}

		if b[0] == ']' {
			j := (n - len(b)) + 1
			return a[:j], a[j:], nil
		}

		if i != 0 {
			if len(b) == 0 {
				return nil, b, syntaxError(b, "unexpected EOF after array element")
			}
			if b[0] != ',' {
				return nil, b, syntaxError(b, "expected ',' after array element but found '%c'", b[0])
			}
			b = skipSpaces(b[1:])
			if len(b) == 0 {
				return nil, b, unexpectedEOF(b)
			}
			if b[0] == ']' {
				return nil, b, syntaxError(b, "unexpected trailing comma after object field")
			}
		}

		_, b, err = parseValue(b)
		if err != nil {
			return nil, b, err
		}

		i++
	}
}

func parseValue(b []byte) ([]byte, []byte, error) {
	if len(b) != 0 {
		switch b[0] {
		case '{':
			return parseObject(b)
		case '[':
			return parseArray(b)
		case '"':
			return parseString(b)
		case 'n':
			return parseNull(b)
		case 't':
			return parseTrue(b)
		case 'f':
			return parseFalse(b)
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return parseNumber(b)
		default:
			return nil, b, syntaxError(b, "invalid character '%c' looking for beginning of value", b[0])
		}
	}
	return nil, b, syntaxError(b, "unexpected end of JSON input")
}

func hasNullPrefix(b []byte) bool {
	return len(b) >= 4 && string(b[:4]) == "null"
}

func hasTruePrefix(b []byte) bool {
	return len(b) >= 4 && string(b[:4]) == "true"
}

func hasFalsePrefix(b []byte) bool {
	return len(b) >= 5 && string(b[:5]) == "false"
}

func hasPrefix(b []byte, s string) bool {
	return len(b) >= len(s) && s == string(b[:len(s)])
}

func hasLeadingSign(b []byte) bool {
	return len(b) > 0 && (b[0] == '+' || b[0] == '-')
}

func hasLeadingZeroes(b []byte) bool {
	if hasLeadingSign(b) {
		b = b[1:]
	}
	return len(b) > 1 && b[0] == '0' && '0' <= b[1] && b[1] <= '9'
}

func appendToLower(b, s []byte) []byte {
	if ascii.Valid(s) { // fast path for ascii strings
		i := 0

		for j := range s {
			c := s[j]

			if 'A' <= c && c <= 'Z' {
				b = append(b, s[i:j]...)
				b = append(b, c+('a'-'A'))
				i = j + 1
			}
		}

		return append(b, s[i:]...)
	}

	for _, r := range string(s) {
		b = appendRune(b, foldRune(r))
	}

	return b
}

func foldRune(r rune) rune {
	if r = unicode.SimpleFold(r); 'A' <= r && r <= 'Z' {
		r = r + ('a' - 'A')
	}
	return r
}
