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

// Test_Redirect_CaptureInsideAuthority asserts that a capture spliced into the
// target's own authority cannot cut that authority short and pick a different
// host.
//
// A target like "https://$1.assets.example.com/" is a plausible way to route
// per tenant, and the author means $1 to be a label. A capture holding
// "evil.com/x" composed "https://evil.com/x.assets.example.com/", whose
// authority ends at that slash — so the browser went to evil.com.
func Test_Redirect_CaptureInsideAuthority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"slash escapes the label", "https://$1.assets.example.com/", "/cdn/evil.com/x", ""},
		{"slash escapes a mid-authority label", "https://assets.$1.com", "/cdn/evil.com/x", ""},
		{"protocol relative target", "//$1.assets.example.com/", "/cdn/evil.com/x", ""},
		{"question mark escapes", "https://$1.assets.example.com/", "/cdn/evil.com%3Fa", ""},

		// An "@" makes everything before it userinfo, so the host becomes the
		// capture's — no authority-ending byte required.
		{"at sign makes the host userinfo", "https://$1.assets.example.com/", "/cdn/a@evil.com", ""},
		{"at sign at the end of the authority", "https://cdn.example.com$1", "/cdn/@evil.com", ""},
		// A capture that does not open a new component extends the host the
		// author wrote.
		{"bare value extends the host", "https://cdn.example.com$1", "/cdn/x", ""},

		// A clean label still composes.
		{"clean label", "https://$1.assets.example.com/", "/cdn/images", "https://images.assets.example.com/"},
		{"clean label protocol relative", "//$1.assets.example.com/", "/cdn/images", "//images.assets.example.com/"},

		// A capture in the path may hold slashes freely — it cannot reach the
		// authority, which the target fixed.
		{"path capture keeps slashes", "https://cdn.example.com/$1", "/cdn/a/b/c", "https://cdn.example.com/a/b/c"},
		{"path capture with a host-like value", "https://cdn.example.com/$1", "/cdn/evil.com/x", "https://cdn.example.com/evil.com/x"},

		// A target handing the whole authority to the capture would let the
		// request choose the host, so the rule never fires. New warns why.
		{"whole authority is the capture", "https://$1", "/cdn/anything/x", ""},
		{"protocol relative whole authority", "//$1", "/cdn/anything", ""},
		{"scheme with no authority", "https:$1", "/cdn//evil.com", ""},
		// Separators alone pin nothing, so a value opening a path does not end
		// an authority the author never wrote: "///evil.com." is read host-first.
		{"separator only before the capture", "//$1.", "/cdn/evil.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{
				Rules:      map[string]string{"/cdn/*": tc.target},
				StatusCode: fiber.StatusFound,
			}))
			app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)

			if tc.want == "" {
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must not fire")
				require.Empty(t, resp.Header.Get("Location"))
				return
			}
			require.Equal(t, fiber.StatusFound, resp.StatusCode)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_CaptureEndingTheAuthority covers a target whose placeholder
// ends its authority, which is a natural way to keep the request's own path:
// the capture must open a new component rather than extend the host.
func Test_Redirect_CaptureEndingTheAuthority(t *testing.T) {
	t.Parallel()

	// "/cdn*" leaves the leading slash in the capture, so $1 supplies the path.
	app := fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{"/cdn*": "https://cdn.example.com$1"},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	get := func(target string) (int, string) {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, target, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		return resp.StatusCode, resp.Header.Get("Location")
	}

	status, location := get("/cdn/foo.png")
	require.Equal(t, fiber.StatusFound, status, "a capture that opens a path must still redirect")
	require.Equal(t, "https://cdn.example.com/foo.png", location)

	status, location = get("/cdn/a/b/c")
	require.Equal(t, fiber.StatusFound, status)
	require.Equal(t, "https://cdn.example.com/a/b/c", location)

	// The capture carries the '@' into the path, past the authority, so the
	// host is still the author's.
	status, location = get("/cdn/@evil.com")
	require.Equal(t, fiber.StatusFound, status)
	require.Equal(t, "https://cdn.example.com/@evil.com", location)
}

func Test_AuthorityChunks(t *testing.T) {
	t.Parallel()

	requireNoAuthorityChunks(t, "https://cdn.example.com/$1", "no token in the authority")
	requireNoAuthorityChunks(t, "/$1", "no authority at all")
	requireNoAuthorityChunks(t, "mailto:$1", "a scheme with no // has no authority")
	requireNoAuthorityChunks(t, "https://cost$.example.com/", "a bare $ is literal text")

	require.Equal(t, []authorityChunk{
		{text: "$1", placeholder: true},
		{text: ".assets.example.com"},
	}, authorityChunks("https://$1.assets.example.com/x"))

	require.Equal(t, []authorityChunk{
		{text: "cdn.example.com"},
		{text: "$1", placeholder: true},
	}, authorityChunks("https://cdn.example.com$1"))

	require.Equal(t, []authorityChunk{
		{text: "assets."},
		{text: "$1", placeholder: true},
		{text: ".com"},
	}, authorityChunks("https://assets.$1.com"))

	// A target that is nothing but the token: the author picked the
	// destination outright, and authorityHolds leaves it alone.
	require.Equal(t, []authorityChunk{{text: "$1", placeholder: true}}, authorityChunks("https://$1"))
	require.Equal(t, []authorityChunk{{text: "$12", placeholder: true}}, authorityChunks("//$12"))
}

// Test_Redirect_CaptureBoundedByTheTarget covers a token that closes the
// authority but not the target — a port, or a host prefix with a path after it.
//
// Such a token is bounded by the author's own text, so it takes the content
// check rather than the "must open the next component" rule. Treating it like a
// token at the end of the target made every real value fail that rule, and the
// affected rules silently stopped redirecting.
func Test_Redirect_CaptureBoundedByTheTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"port", "https://cdn.example.com:$1/health", "/t/8080", "https://cdn.example.com:8080/health"},

		// The value is still part of the authority, so it may not restructure it.
		{"at sign in a port", "https://cdn.example.com:$1/health", "/t/80@evil.com", ""},

		// A capture that ends the host is refused even where the target
		// continues past it: "evil.com" would compose "example.comevil.com",
		// a domain someone can register. What follows in the target makes no
		// difference, because the host is already decided by then.
		{"host extended before a path", "https://example.com$1/health", "/t/evil.com", ""},
		{"host prefix with a path after", "https://tenant-$1/app", "/t/evil.com", ""},
		{"slash in a host prefix", "https://tenant-$1/app", "/t/evil.com%2Fx", ""},
		// Opening a path closes the host at the author's own text.
		{"capture opens a path", "https://example.com$1/health", "/t/%2Ffoo", "https://example.com/foo/health"},

		// A token right after the port colon ends the target, but a port cannot
		// extend a host and the URL parser rejects a non-numeric one outright,
		// so a digit run is honored.
		{"port at the end of the target", "https://cdn.example.com:$1", "/t/8080", "https://cdn.example.com:8080"},
		{"non-numeric port", "https://cdn.example.com:$1", "/t/80@evil.com", ""},
		{"port extended into a host", "https://cdn.example.com:$1", "/t/8080.evil.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{
				Rules:      map[string]string{"/t/*": tc.target},
				StatusCode: fiber.StatusFound,
			}))
			app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)

			if tc.want == "" {
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must not fire")
				require.Empty(t, resp.Header.Get("Location"))
				return
			}
			require.Equal(t, fiber.StatusFound, resp.StatusCode)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_TargetLetsRequestPickHost pins which target shapes hand the destination
// to the request. "https:$1" is the awkward one: it names no authority for the
// chunk check to guard and no origin for keepSameOrigin to hold it to, yet a
// captured "//evil.com" still composes "https://evil.com".
// Test_Redirect_CaptureClosingTheHost covers the two ways a capture can end up
// naming the host even though it is not the last chunk of the target's
// authority.
//
// A trailing token that resolves to empty contributes nothing, so the one
// before it closes the host; and a literal made only of the separators that may
// trail a hostname pins nothing, since "evil.com." is the same host as
// "evil.com" with the DNS root spelled out. Judging each token by its position
// alone let "https://$1$2" with an empty $2 treat $1 as an interior label, and
// the request named the host outright.
func Test_Redirect_CaptureClosingTheHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"empty trailing capture", "/a/*x*", "https://$1$2", "/a/evil.comx", ""},
		{"empty trailing capture after a dot", "/a/*x*", "https://$1.$2", "/a/evil.comx", ""},

		// An empty trailing capture is fine where the author's own text still
		// closes the host.
		{"author text closes the host", "/a/*x*", "https://$1.assets.example.com$2", "/a/tenantx", "https://tenant.assets.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				Rules:      map[string]string{tc.pattern: tc.target},
				StatusCode: fiber.StatusFound,
			}))
			app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)

			if tc.want == "" {
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must not fire")
				require.Empty(t, resp.Header.Get("Location"))
				return
			}
			require.Equal(t, fiber.StatusFound, resp.StatusCode)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_TenthCaptureInAuthority pins that the guard judges what the
// Replacer will actually splice in.
//
// strings.Replacer matches its patterns in the order they were given, so "$10"
// is consumed as "$1" followed by a literal "0". Resolving the token by index
// instead checked the tenth capture while the first was the one reaching the
// host, and a rule with ten or more wildcards was an open redirect straight
// past the guard.
func Test_Redirect_TenthCaptureInAuthority(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{"/t/*/*/*/*/*/*/*/*/*/*": "https://$10.cdn.example.com/"},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet,
		"/t/evil.com/x/b/c/d/e/f/g/h/i/tenant", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must not fire")
	require.Empty(t, resp.Header.Get("Location"))
}

func Test_TargetLetsRequestPickHost(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		target string
		want   bool
	}{
		{"https://$1", true},
		{"//$1", true},
		{"https:$1", true},
		{"mailto:$1", true},

		{"https://$1.example.com", false},
		{"https://cdn.example.com$1", false},
		{"https://cdn.example.com/$1", false},
		{"/$1", false},
		{"$1", false},
		{"https://cdn.example.com", false},
		{"mailto:someone@example.com", false},
	} {
		require.Equal(t, tc.want, targetLetsRequestPickHost(tc.target, authorityChunks(tc.target)), "target %q", tc.target)
	}
}

func Test_AuthoritySpan(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in         string
		start, end int
	}{
		{"https://cdn.example.com/$1", 8, 23},
		{"https://$1.example.com", 8, 22},
		{"//cdn.example.com/a", 2, 17},
		{"//cdn.example.com", 2, 17},
		{"https://cdn.example.com", 8, 23},
		{"https://cdn.example.com?q=1", 8, 23},
		{`https://cdn.example.com\x`, 8, 23},
		// No authority: a path-only target, or a scheme with no "//".
		{"/$1", 0, 0},
		{"$1", 0, 0},
		{"mailto:someone@example.com", 0, 0},
		{"", 0, 0},
	} {
		start, end := authoritySpan(tc.in)
		require.Equal(t, tc.start, start, "start for %q", tc.in)
		require.Equal(t, tc.end, end, "end for %q", tc.in)
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

func requireNoAuthorityChunks(t *testing.T, target, msg string) {
	t.Helper()
	require.Nil(t, authorityChunks(target), msg)
}
