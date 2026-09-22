package openapi

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// groupTag derives an operation tag from the group a route was registered
// through. A named group contributes the last segment of its name, so
// app.Group("/users").Name("users.") tags with "users"; otherwise the last
// static segment of the prefix does, skipping parameters and version markers
// such as "v1", so app.Group("/api/v1/users/:id") also tags with "users".
func groupTag(route *fiber.Route) string {
	if name := route.GroupName(); name != "" {
		name = utils.TrimRight(name, '.')
		if i := strings.LastIndexByte(name, '.'); i >= 0 {
			name = name[i+1:]
		}
		if name != "" {
			return name
		}
	}

	prefix := route.GroupPrefix()
	for prefix != "" {
		segment := prefix
		if i := strings.LastIndexByte(prefix, '/'); i >= 0 {
			segment, prefix = prefix[i+1:], prefix[:i]
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

// isParamSegment reports whether a path segment is a route parameter or
// wildcard rather than a literal.
func isParamSegment(segment string) bool {
	switch segment[0] {
	case ':', '*', '+':
		return true
	default:
		return false
	}
}

// isVersionSegment reports whether a path segment is a version marker such as
// "v1" or "v2.1", which names an API revision rather than a resource.
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

// handlerSummary derives an operation summary from the name of the route's
// final handler, so listUsers documents as "List users". A closure or a method
// value carries no usable name and yields "".
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

// summaryFromFuncName turns a runtime function name into a sentence: the
// package path and receiver are dropped, a method value's "-fm" suffix and a
// generic instantiation's type list are stripped, and the identifier is split
// into words. Anonymous functions ("func1", "1") produce "".
func summaryFromFuncName(name string) string {
	if i := strings.IndexByte(name, '['); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSuffix(name, "-fm")
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
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
			_ = b.WriteByte(word[0] &^ 0x20)                //nolint:errcheck // strings.Builder.WriteByte never returns an error
			_, _ = b.WriteString(strings.ToLower(word[1:])) //nolint:errcheck // strings.Builder.WriteString never returns an error
		default:
			_, _ = b.WriteString(strings.ToLower(word)) //nolint:errcheck // strings.Builder.WriteString never returns an error
		}
	}
	return b.String()
}

// isAnonymousFuncName reports whether the last name segment belongs to a
// function literal: "func1", the "1" of a nested "func1.1", or a compiler
// wrapper such as "deferwrap1".
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

// splitIdentifier breaks a Go identifier into words at underscores and camel
// case boundaries, keeping acronyms whole: GetUserByID yields Get, User, By, ID.
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
		// A word starts at an upper-case letter that follows a lower-case
		// letter or digit, or that ends an acronym (the "S" of "HTTPServer").
		// A plural acronym ("IDs", "URLs") stays whole.
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

// isPluralAcronymEnd reports whether the upper-case letter at i is the last of
// an acronym that only a plural "s" follows, as in "IDs" or "URLs".
func isPluralAcronymEnd(name string, i int) bool {
	return name[i+1] == 's' && (i+2 == len(name) || isUpper(name[i+2]) || name[i+2] == '_')
}

// isAcronym reports whether a word is written in upper case, such as ID or
// HTTP, optionally with a plural "s", and so keeps its case in a sentence.
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

// statusHasNoBody reports whether a response status never carries content per
// RFC 9110: informational, 204 No Content, 205 Reset Content and 304 Not
// Modified. Such a response documents no media type by default.
func statusHasNoBody(code string) bool {
	switch code {
	case "204", "205", "304":
		return true
	default:
		return len(code) == 3 && code[0] == '1'
	}
}
