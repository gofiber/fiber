package redirect

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_Redirect(t *testing.T) {
	app := *fiber.New()

	app.Use(New(Config{
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/default/*": "fiber.wiki",
		},
		StatusCode: fiber.StatusTemporaryRedirect,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/redirect/*": "$1",
		},
		StatusCode: fiber.StatusSeeOther,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/pattern/*": "golang.org",
		},
		StatusCode: fiber.StatusFound,
	}))

	app.Use(New(Config{
		Rules: map[string]string{
			"/": "/swagger",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/params": "/with_params",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Get("/api/*", func(c fiber.Ctx) error {
		return c.SendString("API")
	})

	app.Get("/new", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	tests := []struct {
		name       string
		url        string
		redirectTo string
		statusCode int
	}{
		{
			name:       "should be returns status StatusFound without a wildcard",
			url:        "/default",
			redirectTo: "google.com",
			statusCode: fiber.StatusMovedPermanently,
		},
		{
			name:       "should be returns status StatusTemporaryRedirect  using wildcard",
			url:        "/default/xyz",
			redirectTo: "fiber.wiki",
			statusCode: fiber.StatusTemporaryRedirect,
		},
		{
			name:       "should be returns status StatusSeeOther without set redirectTo to use the default",
			url:        "/redirect/github.com/gofiber/redirect",
			redirectTo: "github.com/gofiber/redirect",
			statusCode: fiber.StatusSeeOther,
		},
		{
			name:       "should return the status code default",
			url:        "/pattern/xyz",
			redirectTo: "golang.org",
			statusCode: fiber.StatusFound,
		},
		{
			name:       "access URL without rule",
			url:        "/new",
			statusCode: fiber.StatusOK,
		},
		{
			name:       "redirect to swagger route",
			url:        "/",
			redirectTo: "/swagger",
			statusCode: fiber.StatusMovedPermanently,
		},
		{
			name:       "no redirect to swagger route",
			url:        "/api/",
			statusCode: fiber.StatusOK,
		},
		{
			name:       "no redirect to swagger route #2",
			url:        "/api/test",
			statusCode: fiber.StatusOK,
		},
		{
			name:       "redirect with query params",
			url:        "/params?query=abc",
			redirectTo: "/with_params?query=abc",
			statusCode: fiber.StatusMovedPermanently,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tt.url, http.NoBody)
			require.NoError(t, err)
			req.Header.Set("Location", "github.com/gofiber/redirect")
			resp, err := app.Test(req)

			require.NoError(t, err)
			require.Equal(t, tt.statusCode, resp.StatusCode)
			require.Equal(t, tt.redirectTo, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_StartAnchor verifies that a rule only matches from the start of
// the path, so a request is not redirected by a rule whose path it merely ends
// with (issue #4476).
func Test_Redirect_StartAnchor(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/old": "/new",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Get("/very/old", func(c fiber.Ctx) error {
		return c.SendString("not redirected")
	})

	// The rule matches the whole path and redirects.
	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/old", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "/new", resp.Header.Get("Location"))

	// A path that only ends with the rule path must not be redirected.
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/very/old", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Location"))
}

// Test_Redirect_SameOriginTargets verifies that captured path segments cannot
// turn a path-only target into a redirect off this origin.
//
// The path arrives with its slash runs intact, so the documented rule
// "/api/*" -> "/$1" composed "Location: //evil.com" from a request for
// "/api//evil.com" — a network-path reference the browser follows to evil.com —
// and "/redirect/*" -> "$1" composed an outright absolute redirect from
// "/redirect/https://evil.com".
func Test_Redirect_SameOriginTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		target  string
		request string
		want    string
	}{
		{"protocol relative", "/api/*", "/$1", "/api//evil.com", "/evil.com"},
		{"long slash run", "/redirect/*", "$1", "/redirect///evil.com", "/evil.com"},
		{"absolute url", "/redirect/*", "$1", "/redirect/https://evil.com", "/https://evil.com"},
		{"non fetch scheme", "/redirect/*", "$1", "/redirect/javascript:x", "/javascript:x"},

		// Same-origin composition is untouched.
		{"ordinary capture", "/api/*", "/$1", "/api/users", "/users"},
		{"capture below a prefix", "/old/*", "/new/$1", "/old//evil.com", "/new//evil.com"},
		{"relative reference", "/g", "google.com", "/g", "google.com"},

		// A target that names its own authority is the author's call, so it is
		// left exactly as configured.
		{"absolute target", "/ext/*", "https://cdn.example.com/$1", "/ext/a", "https://cdn.example.com/a"},
		{"protocol relative target", "/pr/*", "//cdn.example.com/$1", "/pr/a", "//cdn.example.com/a"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				Rules:      map[string]string{tc.pattern: tc.target},
				StatusCode: fiber.StatusFound,
			}))

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusFound, resp.StatusCode)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_SameOriginTargets_Unescaped covers the same guard when
// UnescapePath decodes the capture before it is spliced in.
//
// Two rewrites reach a Location before anything navigates: a recipient strips
// leading and trailing whitespace from the field value (RFC 9110 Section 5.5),
// and the WHATWG URL parser removes every ASCII tab, LF and CR before parsing.
// Checking the composed bytes alone missed both, so " //evil.com" and
// "/\t/evil.com" still reached evil.com.
func Test_Redirect_SameOriginTargets_Unescaped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		target  string
		request string
		want    string
	}{
		{"leading space", "/r/*", "$1", "/r/%20//evil.com", "/evil.com"},
		{"leading tab", "/r/*", "$1", "/r/%09//evil.com", "/evil.com"},
		{"tab before scheme", "/r/*", "$1", "/r/%09https://evil.com", "/https://evil.com"},
		{"interior tab", "/api/*", "/$1", "/api/%09/evil.com", "/evil.com"},

		// A space is not removed by the URL parser — it gets percent-encoded —
		// so an interior one cannot form an authority and is left alone.
		{"interior space", "/api/*", "/$1", "/api/%20/evil.com", "/ /evil.com"},
		{"ordinary capture", "/api/*", "/$1", "/api/users", "/users"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{
				Rules:      map[string]string{tc.pattern: tc.target},
				StatusCode: fiber.StatusFound,
			}))

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusFound, resp.StatusCode)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

func Test_AsBrowserReads(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{" //evil.com", "//evil.com"},
		{"/\t/evil.com", "//evil.com"},
		{"/\r\n/evil.com", "//evil.com"},
		{"\t\t//evil.com", "//evil.com"},
		{"/a/b  ", "/a/b"},
		{"/ /evil.com", "/ /evil.com"}, // an interior space survives
		{"/clean/path", "/clean/path"},
		{"", ""},
		{" \t ", ""},
	} {
		require.Equal(t, tc.want, asBrowserReads(tc.in), "input %q", tc.in)
	}
}

func Test_Redirect_SameOriginTargets_QueryPreserved(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{"/api/*": "/$1"},
		StatusCode: fiber.StatusFound,
	}))

	// The query is appended after the location is made same-origin, so it
	// survives the collapse untouched.
	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/api//evil.com?a=1&b=2", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "/evil.com?a=1&b=2", resp.Header.Get("Location"))
}

func Test_SchemeEnd(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want int
	}{
		{"https://evil.com", 5},
		{"javascript:x", 10},
		{"a:", 1},
		{"h+t-t.p1://x", 8},
		{"", -1},
		{":", -1},          // a scheme cannot be empty
		{"1http://x", -1},  // nor start with a digit
		{"/https://x", -1}, // already rooted, so not a scheme
		{"//evil.com", -1},
		{"google.com", -1}, // no colon at all
		{"/a/b?x=1:2", -1}, // the colon is past the path separator
	} {
		require.Equal(t, tc.want, schemeEnd(tc.in), "input %q", tc.in)
	}
}

func Test_Next(t *testing.T) {
	// Case 1 : Next function always returns true
	app := *fiber.New()
	app.Use(New(Config{
		Next: func(fiber.Ctx) bool {
			return true
		},
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Case 2 : Next function always returns false
	app = *fiber.New()
	app.Use(New(Config{
		Next: func(fiber.Ctx) bool {
			return false
		},
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)

	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "google.com", resp.Header.Get("Location"))
}

func Test_NoRules(t *testing.T) {
	// Case 1: No rules with default route defined
	app := *fiber.New()

	app.Use(New(Config{
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Case 2: No rules and no default route defined
	app = *fiber.New()

	app.Use(New(Config{
		StatusCode: fiber.StatusMovedPermanently,
	}))

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_DefaultConfig(t *testing.T) {
	// Case 1: Default config and no default route
	app := *fiber.New()

	app.Use(New())

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	// Case 2: Default config and default route
	app = *fiber.New()

	app.Use(New())
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_RegexRules(t *testing.T) {
	// Case 1: Rules regex is empty
	app := *fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Case 2: Rules regex map contains valid regex and well-formed replacement URLs
	app = *fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "google.com", resp.Header.Get("Location"))

	// Case 3: Test invalid regex throws panic
	app = *fiber.New()
	require.Panics(t, func() {
		app.Use(New(Config{
			Rules: map[string]string{
				"(": "google.com",
			},
			StatusCode: fiber.StatusMovedPermanently,
		}))
	})
}
