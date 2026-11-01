package openapi

import (
	"reflect"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// groupTag derives a tag from the route's group: its name's last segment, else the last static prefix segment.
func groupTag(route *fiber.Route) string {
	name := utils.TrimRight(route.GroupName(), '.')
	for name != "" {
		segment := name
		if before, after, found := utils.LastCutByte(name, '.'); found {
			segment, name = after, before
		} else {
			name = ""
		}
		if segment != "" && !isVersionSegment(segment) {
			return segment
		}
	}

	prefix := route.GroupPrefix()
	for prefix != "" {
		segment := prefix
		if before, after, found := utils.LastCutByte(prefix, '/'); found {
			segment, prefix = after, before
		} else {
			prefix = ""
		}
		if segment == "" || isParamSegment(segment) || isVersionSegment(segment) {
			continue
		}
		return segment
	}
	return ""
}

func isParamSegment(segment string) bool {
	switch segment[0] {
	case ':', '*', '+':
		return true
	default:
		return false
	}
}

func isVersionSegment(segment string) bool {
	if len(segment) < 2 || (segment[0] != 'v' && segment[0] != 'V') {
		return false
	}
	for i := 1; i < len(segment); i++ {
		if c := segment[i]; (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	return true
}

func handlerSummary(handlers []fiber.Handler) string {
	if len(handlers) == 0 {
		return ""
	}
	handler := handlers[len(handlers)-1]
	if handler == nil {
		return ""
	}
	fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if fn == nil {
		return ""
	}
	return summaryFromFuncName(fn.Name())
}

// stripTypeArguments removes bracketed type-argument lists from a runtime function name.
func stripTypeArguments(name string) string {
	if !strings.Contains(name, "[") {
		return name
	}
	var b strings.Builder
	b.Grow(len(name))
	depth := 0
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case c == '[':
			depth++
		case c == ']' && depth > 0:
			depth--
		case depth == 0:
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
	}
	return b.String()
}

// summaryFromFuncName turns a runtime function name into a sentence; anonymous functions yield "".
func summaryFromFuncName(name string) string {
	name = stripTypeArguments(name)
	name = strings.TrimSuffix(name, "-fm")
	if _, after, found := utils.LastCutByte(name, '.'); found {
		name = after
	}
	if name == "" || isAnonymousFuncName(name) {
		return ""
	}

	words := splitIdentifier(name)
	if len(words) == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(name) + len(words))
	for i, word := range words {
		if i > 0 {
			_ = b.WriteByte(' ') //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
		switch {
		case isAcronym(word):
			_, _ = b.WriteString(word) //nolint:errcheck // strings.Builder.WriteString never returns an error
		case i == 0:
			first, size := utf8.DecodeRuneInString(word)
			_, _ = b.WriteRune(unicode.ToUpper(first))              //nolint:errcheck // strings.Builder.WriteRune never returns an error
			_, _ = b.WriteString(utilsstrings.ToLower(word[size:])) //nolint:errcheck // strings.Builder.WriteString never returns an error
		default:
			_, _ = b.WriteString(utilsstrings.ToLower(word)) //nolint:errcheck // strings.Builder.WriteString never returns an error
		}
	}
	return b.String()
}

func isAnonymousFuncName(name string) bool {
	if name[0] >= '0' && name[0] <= '9' {
		return true
	}
	for _, prefix := range [...]string{"func", "deferwrap", "gowrap"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
			return true
		}
	}
	return false
}

func splitIdentifier(name string) []string {
	var words []string
	start := 0
	for i := range len(name) {
		c := name[i]
		if c == '_' {
			if i > start {
				words = append(words, name[start:i])
			}
			start = i + 1
			continue
		}
		if i == start || !isUpper(c) {
			continue
		}
		prev := name[i-1]
		// A word starts after a lower-case letter or digit, or ends an acronym; plural acronyms ("IDs") stay whole.
		if !isUpper(prev) || (i+1 < len(name) && isLower(name[i+1]) && !isPluralAcronymEnd(name, i)) {
			words = append(words, name[start:i])
			start = i
		}
	}
	if start < len(name) {
		words = append(words, name[start:])
	}
	return words
}

func isPluralAcronymEnd(name string, i int) bool {
	return name[i+1] == 's' && (i+2 == len(name) || isUpper(name[i+2]) || name[i+2] == '_')
}

func isAcronym(word string) bool {
	word = strings.TrimSuffix(word, "s")
	if len(word) < 2 {
		return false
	}
	for i := range len(word) {
		if !isUpper(word[i]) && !isASCIIDigit(word[i]) {
			return false
		}
	}
	return true
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

func statusHasNoBody(code string) bool {
	switch code {
	case "204", "205", "304":
		return true
	default:
		return len(code) == 3 && code[0] == '1'
	}
}
