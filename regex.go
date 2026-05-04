// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import "regexp"

// RegexCompiler defines the interface for regex pattern compilation and matching.
// This abstraction allows alternative regex engines to be used for routing constraints.
// The default implementation uses Go's standard library regexp package.
type RegexCompiler interface {
	// MatchString reports whether the string s contains any match of the regex pattern.
	MatchString(s string) bool

	// FindAllStringSubmatch returns a slice of all successive matches of the expression.
	// The return value is a slice of slices of strings. Each slice contains the full match
	// followed by any captured submatches. A return value of nil indicates no match.
	FindAllStringSubmatch(s string, n int) [][]string
}

// RegexEngine provides methods for creating compiled regex patterns.
// Implementations can provide optimized regex engines (e.g., coregex) as alternatives
// to the standard library regexp package.
type RegexEngine interface {
	// MustCompile compiles a regex pattern and panics if the pattern is invalid.
	// This behavior matches regexp.MustCompile for drop-in compatibility.
	MustCompile(pattern string) RegexCompiler
}

// stdlibRegexEngine is the default regex engine using Go's standard library regexp.
type stdlibRegexEngine struct{}

// MustCompile compiles a regex pattern using the standard library.
func (stdlibRegexEngine) MustCompile(pattern string) RegexCompiler {
	return &stdlibRegexCompiler{
		Regexp: regexp.MustCompile(pattern),
	}
}

// stdlibRegexCompiler wraps *regexp.Regexp to implement RegexCompiler.
type stdlibRegexCompiler struct {
	*regexp.Regexp
}

// DefaultRegexEngine is the default regex engine implementation using Go's standard library.
// This can be replaced with alternative implementations (e.g., coregex) for better performance.
var DefaultRegexEngine RegexEngine = stdlibRegexEngine{}
