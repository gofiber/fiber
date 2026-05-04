// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockRegexCompiler is a mock implementation of RegexCompiler for testing
type mockRegexCompiler struct {
	*regexp.Regexp
	matchCalled bool
}

func (m *mockRegexCompiler) MatchString(s string) bool {
	m.matchCalled = true
	return m.Regexp.MatchString(s)
}

// mockRegexEngine is a mock implementation of RegexEngine for testing
type mockRegexEngine struct {
	lastPattern   string
	compileCalled bool
}

func (m *mockRegexEngine) MustCompile(pattern string) RegexCompiler {
	m.compileCalled = true
	m.lastPattern = pattern
	return &mockRegexCompiler{
		Regexp: regexp.MustCompile(pattern),
	}
}

// Test_RegexEngine_Default verifies the default regex engine works correctly
func Test_RegexEngine_Default(t *testing.T) {
	t.Parallel()

	// Test that DefaultRegexEngine is set
	require.NotNil(t, DefaultRegexEngine)

	// Test compilation
	compiler := DefaultRegexEngine.MustCompile(`\d+`)
	require.NotNil(t, compiler)

	// Test matching
	require.True(t, compiler.MatchString("123"))
	require.False(t, compiler.MatchString("abc"))

	// Test FindAllStringSubmatch
	compiler = DefaultRegexEngine.MustCompile(`(\w+)@(\w+)\.(\w+)`)
	matches := compiler.FindAllStringSubmatch("test@example.com", -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 4)
	require.Equal(t, "test@example.com", matches[0][0])
	require.Equal(t, "test", matches[0][1])
	require.Equal(t, "example", matches[0][2])
	require.Equal(t, "com", matches[0][3])
}

// Test_RegexEngine_CustomEngine verifies that a custom regex engine can be used
func Test_RegexEngine_CustomEngine(t *testing.T) {
	t.Parallel()

	mockEngine := &mockRegexEngine{}

	// Create app with custom regex engine
	app := New(Config{
		RegexEngine: mockEngine,
	})

	// Register a route with regex constraint
	app.Get("/api/:id<regex(\\d+)>", func(c Ctx) error {
		return c.SendString("matched")
	})

	// Verify the mock engine was used during route registration
	require.True(t, mockEngine.compileCalled, "MustCompile should have been called")
	require.Equal(t, `\d+`, mockEngine.lastPattern, "Pattern should match")

	// Test the route
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/123", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	// Test with non-matching pattern
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/api/abc", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}

// Test_RoutePatternMatch_WithRegex verifies RoutePatternMatch works with regex constraints
func Test_RoutePatternMatch_WithRegex(t *testing.T) {
	t.Parallel()

	// Test with default engine
	require.True(t, RoutePatternMatch("/api/123", "/api/:id<regex(\\d+)>"))
	require.False(t, RoutePatternMatch("/api/abc", "/api/:id<regex(\\d+)>"))

	// Test with custom config
	mockEngine := &mockRegexEngine{}
	require.True(t, RoutePatternMatch("/api/123", "/api/:id<regex(\\d+)>", Config{
		RegexEngine: mockEngine,
	}))
	require.True(t, mockEngine.compileCalled, "MustCompile should have been called")
}

// Test_RegexEngine_NilDefaultsToStdlib verifies that nil RegexEngine defaults to stdlib
func Test_RegexEngine_NilDefaultsToStdlib(t *testing.T) {
	t.Parallel()

	// Create app without specifying RegexEngine (should default)
	app := New()

	// Verify it's set to the default
	require.NotNil(t, app.config.RegexEngine)
	require.Equal(t, DefaultRegexEngine, app.config.RegexEngine)

	// Register a route with regex constraint
	app.Get("/api/:id<regex(\\d+)>", func(c Ctx) error {
		return c.SendString("matched")
	})

	// Test the route works
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/123", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// Test_RegexEngine_ComplexPattern tests complex regex patterns
func Test_RegexEngine_ComplexPattern(t *testing.T) {
	t.Parallel()

	app := New()

	// Test date pattern
	app.Get("/date/:date<regex(\\d{4}-\\d{2}-\\d{2})>", func(c Ctx) error {
		return c.SendString("date: " + c.Params("date"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/date/2024-01-15", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/date/2024-1-5", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}
