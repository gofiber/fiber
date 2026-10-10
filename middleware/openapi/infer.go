package openapi

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// groupTag derives an operation tag from the route's group: the last segment of its name, else the
// last static prefix segment, skipping parameters and version markers like "v1".
func groupTag(route *fiber.Route) string {
	if name := route.GroupName(); name != "" {
		name = utils.TrimRight(name, '.')
		if _, after, found := utils.LastCutByte(name, '.'); found {
			name = after
		}
		if name != "" {
			return name
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

// isVersionSegment reports whether a segment is a version marker such as "v1" or "v2.1".
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

// handlerSummary derives a summary from the final handler's name (listUsers -> "List users"); function literals yield "".
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

// summaryFromFuncName turns a runtime function name into a sentence, stripping package, receiver, "-fm" and type lists. Anonymous functions yield "".
func summaryFromFuncName(name string) string {
	name, _, _ = utils.CutByte(name, '[')
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
			_ = b.WriteByte(word[0] &^ 0x20)                     //nolint:errcheck // strings.Builder.WriteByte never returns an error
			_, _ = b.WriteString(utilsstrings.ToLower(word[1:])) //nolint:errcheck // strings.Builder.WriteString never returns an error
		default:
			_, _ = b.WriteString(utilsstrings.ToLower(word)) //nolint:errcheck // strings.Builder.WriteString never returns an error
		}
	}
	return b.String()
}

// isAnonymousFuncName reports whether the last name segment is a function literal or compiler wrapper.
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

// splitIdentifier splits a Go identifier at underscores and camel-case boundaries, keeping acronyms whole.
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
		// A word starts at an upper-case letter after a lower-case letter or digit, or ending an acronym; plural acronyms ("IDs") stay whole.
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

// isAcronym reports whether a word is upper case (optionally plural), so it keeps its case in a sentence.
func isAcronym(word string) bool {
	word = strings.TrimSuffix(word, "s")
	if len(word) < 2 {
		return false
	}
	for i := range len(word) {
		if !isUpper(word[i]) && (word[i] < '0' || word[i] > '9') {
			return false
		}
	}
	return true
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

// statusHasNoBody reports whether RFC 9110 forbids content for the status (1xx, 204, 205, 304).
func statusHasNoBody(code string) bool {
	switch code {
	case "204", "205", "304":
		return true
	default:
		return len(code) == 3 && code[0] == '1'
	}
}
