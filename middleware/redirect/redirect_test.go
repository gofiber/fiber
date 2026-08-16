package redirect

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/urlnorm"
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

// Test_Redirect_CaptureInUserinfo covers a capture the author placed before an
// "@" they wrote themselves. The host starts after the last "@", so the value is
// userinfo: ":" separates a password there and a further "@" only lengthens it,
// while the four bytes that end an authority outright still have to stay out.
func Test_Redirect_CaptureInUserinfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"password", "https://$1@example.com/", "/cdn/user%3Apassword", "https://user:password@example.com/"},
		{"further at sign", "https://$1@example.com/", "/cdn/a%40evil.com", "https://a@evil.com@example.com/"},
		{"plain userinfo", "https://$1@example.com/", "/cdn/user", "https://user@example.com/"},

		// The four that end the authority reach the host past the "@".
		{"slash", "https://$1@example.com/", "/cdn/evil.com%2Fx", ""},
		{"backslash", "https://$1@example.com/", "/cdn/evil.com%5Cx", ""},
		{"question mark", "https://$1@example.com/", "/cdn/evil.com%3Fx", ""},
		{"fragment", "https://$1@example.com/", "/cdn/evil.com%23x", ""},

		// Past the author's "@" the capture is a host label again, so the label
		// rule applies to it as it would anywhere else in the host.
		{"label after the at sign", "https://user@$1.example.com/", "/cdn/a%3Ab", ""},
		{"clean label after the at sign", "https://user@$1.example.com/", "/cdn/images", "https://user@images.example.com/"},
	}

	// Every composed location names a host the author wrote, whatever the value
	// put in the userinfo before it.
	const wantHost = "example.com"

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

			// Whatever the value did, the host is still the author's — a label
			// they left to the capture sits under it.
			u, err := url.Parse(resp.Header.Get("Location"))
			require.NoError(t, err)
			require.True(t, u.Host == wantHost || strings.HasSuffix(u.Host, "."+wantHost), "host %q", u.Host)
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

	// pins is answered once here, so it is part of what the split produces.
	require.Equal(t, []authorityChunk{
		{text: "$1", placeholder: true},
		{text: ".assets.example.com", pins: true},
	}, authorityChunks("https://$1.assets.example.com/x"))

	require.Equal(t, []authorityChunk{
		{text: "cdn.example.com", pins: true},
		{text: "$1", placeholder: true},
	}, authorityChunks("https://cdn.example.com$1"))

	require.Equal(t, []authorityChunk{
		{text: "assets.", pins: true},
		{text: "$1", placeholder: true},
		{text: ".com", pins: true},
	}, authorityChunks("https://assets.$1.com"))

	// And a literal that pins nothing is recorded as such.
	require.Equal(t, []authorityChunk{
		{text: "$1", placeholder: true},
		{text: ":8080"},
	}, authorityChunks("https://$1:8080"))

	// A literal between two captures is open on the left, unlike one that
	// starts the authority — the only place that distinction is made.
	require.Equal(t, []authorityChunk{
		{text: "$1", placeholder: true},
		{text: "xyz:"},
		{text: "$2", placeholder: true},
	}, authorityChunks("https://$1xyz:$2"))

	require.Equal(t, []authorityChunk{
		{text: "xyz.example.com", pins: true},
		{text: "$1", placeholder: true},
	}, authorityChunks("https://xyz.example.com$1"))

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

// Test_Redirect_ExtraSlashesBeforeTheCapture covers targets that open their
// authority with more than two slashes.
//
// The URL parser's special-authority-ignore-slashes state skips the whole run
// before it reads the host, so "///evil.com" is evil.com. Stopping the authority
// span at the first of those slashes instead made such a target look like it had
// an empty authority: no chunks for the guard to walk, and still "absolute"
// enough that the same-origin fallback was skipped too.
func Test_Redirect_ExtraSlashesBeforeTheCapture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"three slashes", "///$1", "/go/evil.com", ""},
		{"four slashes", "////$1", "/go/evil.com", ""},
		{"slash then backslash", `//\$1`, "/go/evil.com", ""},
		{"scheme with three slashes", "https:///$1", "/go/evil.com", ""},
		{"scheme then backslash", `https://\$1`, "/go/evil.com", ""},

		// A host the author wrote after the run is still their choice, and the
		// capture only reaches the path.
		{"fixed host after the run", "///static/$1", "/go/x", "///static/x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				Rules:      map[string]string{"/go/*": tc.target},
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

// Test_Redirect_OverlappingRulesAreDeterministic pins that two patterns matching
// the same path resolve the same way on every run.
//
// The rules used to be held in a map and walked in its randomized order, so the
// winner changed run to run — and once a rule could be refused and fall through
// to the next, so did whether there was a redirect at all.
func Test_Redirect_OverlappingRulesAreDeterministic(t *testing.T) {
	t.Parallel()

	build := func() *fiber.App {
		app := fiber.New()
		app.Use(New(Config{
			Rules: map[string]string{
				"/cdn/*":  "/first/$1",
				"/cdn/*x": "/second/$1",
			},
			StatusCode: fiber.StatusFound,
		}))
		app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })
		return app
	}

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/cdn/ax", http.NoBody)
	require.NoError(t, err)
	resp, err := build().Test(req)
	require.NoError(t, err)
	first := resp.Header.Get("Location")
	require.NotEmpty(t, first)

	// Rebuilding the middleware re-walks the rules map, which is where the
	// randomization came from.
	for range 20 {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/cdn/ax", http.NoBody)
		require.NoError(t, err)
		resp, err := build().Test(req)
		require.NoError(t, err)
		require.Equal(t, first, resp.Header.Get("Location"))
	}
}

// Test_Redirect_PortDoesNotPinTheHost covers a capture followed only by a port.
//
// A port closes nothing: "evil.com:8080" is still evil.com. Counting the
// literal ":8080" as author-written host text left the capture beside it judged
// as a harmless interior label, and the request named the host outright.
func Test_Redirect_PortDoesNotPinTheHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"literal port after a capture", "/p/*", "https://$1:8080", "/p/evil.com", ""},
		{"protocol relative with a port", "/p/*", "//$1:8080", "/p/evil.com", ""},
		{"captured host and port", "/p/*/*", "https://$1:$2", "/p/evil.com/443", ""},

		// A host the author actually wrote still pins, port or no port.
		{"port after author host text", "/p/*", "https://$1.cdn.example.com:8080", "/p/images", "https://images.cdn.example.com:8080"},
		{"captured port only", "/p/*", "https://cdn.example.com:$1", "/p/8080", "https://cdn.example.com:8080"},
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

// Test_Redirect_MoreSpecificRuleWins pins that a rule pinning more of the path
// is evaluated before one that pins less.
//
// Ordering the rules made evaluation deterministic, but plain lexicographic
// order sorts "/*" ahead of "/old/*", so the catch-all would shadow the
// specific rule on every request instead of half the time.
func Test_Redirect_MoreSpecificRuleWins(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/*":     "/home",
			"/old/*": "/new/$1",
		},
		StatusCode: fiber.StatusFound,
	}))

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/old/thing", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "/new/thing", resp.Header.Get("Location"))

	// The catch-all still covers everything the specific rule does not.
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/other", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "/home", resp.Header.Get("Location"))
}

// Test_TargetLetsRequestPickHost pins which target shapes hand the destination
// to the request. "https:$1" is the awkward one: it names no authority for the
// chunk check to guard and no origin for keepSameOrigin to hold it to, yet a
// captured "//evil.com" still composes "https://evil.com".
//
// The shapes that pin only a port, a trailing dot or userinfo belong here too.
// authorityHolds refuses every value for them, so before they were listed the
// rule compiled, matched each request, was refused, and fell through — while
// warning that values opening a path or query were honored, when none were.
func Test_TargetLetsRequestPickHost(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		target string
		want   bool
	}{
		{"https://$1", true},
		{"//$1", true},
		{"https:$1", true},
		// The parser's missing-solidus step gives every special scheme but file
		// an authority, so a capture after the colon names the host with no
		// slash of its own: "ws:evil.com" is ws://evil.com.
		{"ws:$1", true},
		{"wss:$1", true},
		{"ftp:$1", true},
		{"WS:$1", true},
		{"FTP:$1", true},
		// file has no missing-solidus reading, but a captured "//evil.com" opens
		// an authority for it like any other scheme.
		{"file:$1", true},
		// Any scheme at all: "//" is what opens the authority, so which scheme
		// it is does not enter into it.
		{"mailto:$1", true},
		{"myapp:$1", true},
		{"custom:$1", true},
		// Author text between the colon and the capture leaves no authority for
		// the value to open, or one the author already filled in.
		{"myapp:fixed/$1", false},
		{"https:fixed/$1", false},
		// Nothing beside the capture is host text: a port, a captured port, a
		// second capture and a trailing dot all pin nothing, so the value names
		// the host on its own.
		{"https://$1:8080", true},
		{"https://$1:8080/health", true},
		{"https://$1:$2", true},
		{"https://$1$2", true},
		{"//$1.", true},
		{"//.$1", true},
		// Text before an "@" is a username, not a host.
		{"https://example.com@$1", true},
		{"https://user:pw@$1", true},
		// A capture inside the brackets is refused whichever side the author
		// wrote: a bracketed address runs most significant group first, so the
		// ordinary "text after the capture pins it" reading is backwards there.
		{"https://[$1]", true},
		{"https://[$1]:8080/", true},
		{"https://[$1::1]", true},
		{"https://[2001:db8::$1]", true},
		{"https://[2001:db8::$1]:8080", true},

		// A scheme with no authority syntax has no host to hijack, so it is not
		// refused — dropping it would kill a working rule and the warning would
		// say something untrue about it.
		{"mailto:$1@example.com", false},
		{"myapp:$1@example.com", false},

		{"https://$1.example.com", false},
		{"https://cdn.example.com$1", false},
		{"https://cdn.example.com/$1", false},
		// The same rules spelled without the "//". A special scheme reaches the
		// authority state either way, so the two spellings have to be read the
		// same: the capture here is a label under the author's host, and it is
		// authorityHolds that judges each value for it.
		{"https:$1.example.com", false},
		{"https:cdn.example.com$1", false},
		{"https:cdn.$1.com", false},
		{"ws:$1.example.com", false},
		{"ftp:cdn.example.com$1", false},
		{"HTTPS:$1.example.com", false},
		// And where nothing but a port or a dot sits beside it, refused in
		// either spelling.
		{"https:$1:8080", true},
		{"ws:$1.", true},
		{"https:example.com@$1", true},
		// The author's host text can sit either side of the capture, and a
		// captured port leaves it theirs.
		{"https://$1@example.com", false},
		{"https://cdn.example.com:$1", false},
		{"https://tenant-$1.example.com", false},
		// A bracketed literal the author wrote in full still pins the host, so
		// a captured port beside it is theirs to allow.
		{"https://[::1]:$1", false},
		{"https://[::1]:8080/$1", false},
		{"https://[::]:$1", false},
		{"https://[2001:db8::1]:$2/$1", false},
		// A bracket in userinfo is an ordinary character, not a host delimiter,
		// so it does not make the capture beside it interior to an address.
		{"https://[$1]@example.com", false},
		{"https://us[er@$1.example.com", false},
		{"https://us[er@example.com:$1", false},
		// Empty brackets pin no host, so the capture names it outright.
		{"https://[]$1", true},
		{"https://[:]:$1", true},
		// Only an "@" outside the brackets ends the userinfo. One inside them
		// is no delimiter to the URL parser either, so the brackets still hold
		// the host and a capture among them still chooses the address.
		{"https://[$1@::1]", true},
		{"https://[2001:db8::$1@a]", true},
		{"https://x@y@[$1::1]", true},
		// Past the closing bracket the "@" does end the userinfo, so these name
		// the host after it and the capture only reaches userinfo.
		{"https://[$1]@x", false},
		{"https://[$1::1]@a", false},
		{"https://$1@[::1]", false},
		// Whereas here the capture is the host.
		{"https://[::1]@$1", true},
		{"/$1", false},
		{"$1", false},
		{"https://cdn.example.com", false},
		{"mailto:someone@example.com", false},
	} {
		require.Equal(t, tc.want, targetLetsRequestPickHost(tc.target, authorityChunks(tc.target)), "target %q", tc.target)
	}
}

// Test_PinsHost pins the three things that are not author-written host text: a
// port, the separators that may trail a hostname, and userinfo.
func Test_PinsHost(t *testing.T) {
	t.Parallel()

	// wantClosed is the answer when a label separator, an "@" or the start of
	// the authority stands between the capture and this literal.
	for _, tc := range []struct {
		literal string
		want    bool
	}{
		{"cdn.example.com", true},
		{"cdn.example.com:", true},
		{"cdn.example.com:8080", true},
		{".example.com", true},
		{"@example.com", true},
		{"user:pw@example.com", true},

		// An IPv6 literal's own colons sit inside the brackets, so the port is
		// what follows the closing one.
		{"[::1]", true},
		{"[::1]:", true},
		{"[::1]:8080", true},
		{"[2001:db8::1]", true},
		// The unspecified address is still a complete one the author wrote.
		{"[::]", true},
		{"[::]:", true},
		// An opener with no closer is a fragment of an address some capture
		// split, so it pins nothing on its own — the mirror of "::1]" above.
		{"[2001:db8::", false},
		{"[::", false},
		{"[abc", false},
		{"[fe80", false},
		// Decided by the fragment rule alone: with a dot of its own it is not
		// dotless, and it holds no hex tail, so nothing else would refuse it.
		{"[example.com", false},

		{"", false},
		{":8080", false},
		{".", false},
		{"..", false},
		{"::", false},
		// Everything through the last "@" is userinfo, so nothing is left.
		{"example.com@", false},
		{"user:pw@", false},
		{"example.com@:8080", false},
		// The brackets around an IPv6 literal are punctuation, so a target of
		// "https://[$1]:8080" pins no more of the host than "https://$1:8080".
		{"[", false},
		{"]", false},
		{"]:8080", false},
		// Brackets holding no address pin nothing. Counting them bought a rule
		// that matched every request and composed a location no client parses.
		{"[]", false},
		{"[:]", false},
		{"[.]", false},
		{"[[]", false},
		// Holding a hex digit is not the same as being an address.
		{"[zzz1]", false},
		{"[evil.com1]", false},
		{"[a b]", false},
		// Brackets hold an IPv6 address only, so an IPv4 one is no host there
		// however well it parses alone — but the IPv4-mapped IPv6 spelling is.
		{"[127.0.0.1]", false},
		{"[1.2.3.4]", false},
		{"[::ffff:127.0.0.1]", true},
		{"[0:0:0:0:0:0:0:1]", true},
		// A zone ID is not accepted in a URL host, by this or by the parser.
		{"[fe80::1%25eth0]", false},
		// Whitespace is not host text: the parser deletes a tab outright and
		// percent-encodes a space into a host that fails to parse.
		{" ", false},
		{"\t", false},
		{"\n", false},
		{" \t ", false},
		// Nor is anything UTS #46 mapping deletes before a host is read, nor
		// the code points it folds onto a plain ".", which pins nothing either.
		{"\u00ad", false}, // soft hyphen
		{"\ufeff", false}, // zero width no-break space
		{"\u200b", false}, // zero width space
		{"\u2060", false}, // word joiner
		{"\ufe0f", false}, // variation selector
		{"\u3002", false}, // ideographic full stop
		{"\uff0e", false}, // fullwidth full stop
		{"\uff61", false}, // halfwidth ideographic full stop
		{"\u00ad\u200b", false},
		// A control character is stripped from either end of the whole input
		// before the parser reads anything, and is forbidden in a host in the
		// middle, so it pins nothing wherever it sits.
		{"\x00", false},
		{"\x01", false},
		{"\x1f", false},
		{"\x7f", false},
		// The parser percent-decodes a host before reading it, so an escape
		// pins only what it stands for.
		{"%2E", false},       // "."
		{"%2e", false},       // same, lowercased
		{"%C2%AD", false},    // soft hyphen
		{"%E3%80%82", false}, // ideographic full stop
		{"%EF%BB%BF", false}, // BOM
		// "A" is host text, but alone it is a hex tail a capture can turn into
		// a number by supplying "0x" — so it pins only where a label already
		// separates it from whatever came before.
		{"%41", false},
		{"%41.example.com", true},
		// A stray "%" is literal to the parser, not an error — though dotless,
		// so nothing here pins on its own account.
		{"100%", false},
		{"a%zz", false},
		// A host whose last label reads as a number is an IPv4 address, where
		// the author's trailing text is the low octets and the request supplies
		// the network. Only a complete address pins one.
		{".1", false},
		{".0", false},
		{".0.0.1", false},
		{".0x1", false},
		// A hex tail is one a capture can open a number with, by supplying the
		// "0x" — or finish a percent-escape it left dangling.
		{"cafe", false},
		{"beef", false},
		{"e", false},
		{"ad", false},
		{"x", false},
		{"xcafe", false},
		// Invalid UTF-8 is not host text: the mapping turns it into U+FFFD,
		// which no client accepts in a host.
		{"\xff.example.com", false},
		{"%FF.example.com", false},
		{"\xed\xa0\x80.example.com", false},

		// Nor does any other dotless tail: the head the capture supplies can
		// simply end with a dot, leaving the author's text a bare final label —
		// "https://$1xyz" reached evil.xyz from a captured "evil.".
		{"cdn", false},
		{"xyz", false},
		{"tenant-", false},
		{"com", false},
		// A trailing dot is not a label separator against what precedes it, and
		// the trim takes it off, so these are dotless too.
		{"xyz.", false},
		{"com.", false},
		// A leading "." closes the label, so an all-hex suffix is a suffix —
		// ".de" pins Germany the way ".example.com" pins that domain.
		{".de", true},
		{".be", true},
		{".cc", true},
		{".ad", true},
		{".cafe", true},
		{".e", true},
		// Even a whole address: open on the left the capture reaches its first
		// octet, which is the network.
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		// A name is still a name, whatever letters it is made of.
		{"abc.def", true},
		{".example.com", true},
		{"example1.com", true},
		// An internationalized label is host text, in either spelling.
		{"\u4f8b\u3048.jp", true},
		{"xn--r8jz45g.jp", true},
		// A closer with no opener is the tail of an address a capture split,
		// and an IPv6 tail is the low bits, not the network it routes to.
		{"::1]", false},
		{":db8::1]", false},
	} {
		require.Equal(t, tc.want, pinsHost(tc.literal, true), "literal %q", tc.literal)
	}

	// The closed-on-the-left branch: a dotless tail the capture cannot reach
	// pins the host, which is what keeps "https://[$1]@x" and
	// "https://cdn$1.example.com" compiling.
	for _, tc := range []struct {
		literal string
		want    bool
	}{
		{"cdn", true},
		{"xyz", true},
		{"cafe", true},
		{"x", true},
		{"tenant-", true},
		{"example.com", true},
		{"xyz.", true},
		{"127.0.0.1", true}, // the author wrote the whole address
		{"10.0.0.1", true},

		// A bare number is an address rather than a name, and the parser takes
		// the abbreviated spellings too: "1" is 0.0.0.1 and "0x1" the same, so
		// the author who wrote one pinned the whole host.
		{"1", true},
		{"0x1", true},

		{"", false},
		{".", false},
		{"[example.com", false},
		// Closed on the left the dotless rule no longer stands in front of the
		// decode, map and trim steps, so this is where each answers for itself:
		// a code point the mapping deletes, ones the trim drops, and delimiters
		// that only appear once the escape is decoded.
		{"\u00ad", false},
		{"\u200b", false},
		{"\u3002example.com", true}, // folds to ".example.com"
		{"\x7f", false},
		{" ", false},
		{"[", false},
		{":", false},
		{"%7F", false},
		{"%2E", false},
		{"%3A", false},
		{"%5B", false},
		// The trim has to run on the front too: keeping the space leaves a
		// last label of " 1", which reads as no number, and the address check
		// never runs on what is really the address 1.
		{" 1", true},
		{"%201", true},
		{"%3A0x", true}, // "0x" is a number too, and it is zero
	} {
		require.Equal(t, tc.want, pinsHost(tc.literal, false), "literal %q closed on the left", tc.literal)
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
		require.Equal(t, tc.want, urlnorm.AsBrowserReads(tc.in), "input %q", tc.in)
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

// Test_Redirect_TargetIsReadAsTheClientWillRead pins that the guard judges the
// target the URL parser will see, not the bytes as configured.
//
// The parser deletes every tab, LF and CR before parsing. Reading the target
// as written scored one of them as author-written host text that then vanished
// on the way out, so "https://\t$1" passed as a target naming its own host and
// a captured "/evil.com" composed "https:///evil.com" — evil.com to a browser,
// with no startup warning. A tab also defeated the leading-slash skip in
// authoritySpan, leaving the authority of "https://\t/[$1::1]" unguarded
// entirely, which reached "[beef::1]" and "[::ffff:127.0.0.1]".
func Test_Redirect_TargetIsReadAsTheClientWillRead(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		target  string
		request string
	}{
		{"tab before the host", "https://\t$1", "/r/%2Fevil.com"},
		{"newline before the host", "https://\n$1", "/r/%2Fevil.com"},
		{"carriage return before the host", "https://\r$1", "/r/%2Fevil.com"},
		{"protocol relative", "//\t$1", "/r/%2Fevil.com"},
		{"tab defeating the slash skip", "https://\t/$1", "/r/evil.com"},
		{"tab hiding a bracketed capture", "https://\t/[$1::1]", "/r/beef"},
		{"tab before a bracketed capture", "https://\t[2001:db8::$1]", "/r/8080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}}))
			app.Get("/r/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must be refused at startup")
			require.Empty(t, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_TargetWhitespaceLeavesWorkingRulesAlone is the other half: the
// normalization above must not disturb a target that never had any.
func Test_Redirect_TargetWhitespaceLeavesWorkingRulesAlone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		target, request, want string
	}{
		{"https://cdn.example.com/$1", "/r/a", "https://cdn.example.com/a"},
		{"https://$1.example.com/", "/r/tenant", "https://tenant.example.com/"},
		{"https://[::1]:$1/health", "/r/8080", "https://[::1]:8080/health"},
		{"/new/$1", "/r/a", "/new/a"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}}))

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_HostTextTheParserDeletes pins that a target cannot be made to
// look host-pinned with characters a URL parser drops before it reads the host.
//
// UTS #46 mapping runs on a domain before anything looks at it: it deletes some
// 270 code points outright and folds three more onto a plain ".". Judging the
// literal on its bytes let an invisible one stand in for a host — so
// "https://$1\u00ad" compiled with no warning, and a captured "evil.com"
// composed "https://evil.com\u00ad", which a browser resolves to evil.com.
func Test_Redirect_HostTextTheParserDeletes(t *testing.T) {
	t.Parallel()

	for _, suffix := range []string{"\u00ad", "\ufeff", "\u200b", "\u2060", "\ufe0f", "\u3002", "\uff0e", "\uff61"} {
		t.Run(strconv.QuoteToASCII(suffix), func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": "https://$1" + suffix}}))
			app.Get("/r/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/evil.com", http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must be refused at startup")
			require.Empty(t, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_InternationalizedHostsStillPin is the other half: mapping the
// literal must not cost a rule whose host is written in a non-ASCII script.
func Test_Redirect_InternationalizedHostsStillPin(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ target, want string }{
		{"https://$1.\u4f8b\u3048.jp/", "https://t.\u4f8b\u3048.jp/"},
		{"https://$1.xn--r8jz45g.jp/", "https://t.xn--r8jz45g.jp/"},
		{"https://$1.example.com/", "https://t.example.com/"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}}))

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/t", http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_ControlCharacterPinsNoHost pins the other half of what a client
// removes before reading a host.
//
// The parser strips a leading or trailing run of controls and spaces from the
// whole input before parsing, and urlnorm.AsBrowserReads does the same to the composed
// location. So a control character in the target pinned a host that was gone by
// the time the client saw it: with an empty second capture the rule below
// composed "https://evil.com\x01" and shipped "https://evil.com".
func Test_Redirect_ControlCharacterPinsNoHost(t *testing.T) {
	t.Parallel()

	for _, sep := range []string{"\x00", "\x01", "\x1f", "\x7f"} {
		t.Run(strconv.QuoteToASCII(sep), func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*x*": "https://$1" + sep + "$2"}}))
			app.Use(func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/evil.comx", http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must be refused at startup")
			require.Empty(t, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_PercentEscapePinsOnlyWhatItDecodesTo covers the escaped
// spelling of the same idea: a URL parser percent-decodes a host before it maps
// or reads it, so "%2E" is the "." this guard already knows pins nothing, and
// "%C2%AD" is a soft hyphen the mapping deletes. Judging the escape as written
// let three ordinary characters stand in for a host.
func Test_Redirect_PercentEscapePinsOnlyWhatItDecodesTo(t *testing.T) {
	t.Parallel()

	for _, suffix := range []string{"%2E", "%2e", "%C2%AD", "%E3%80%82", "%EF%BB%BF"} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": "https://$1" + suffix}}))
			app.Use(func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/evil.com", http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must be refused at startup")
			require.Empty(t, resp.Header.Get("Location"))
		})
	}

	// An escape that decodes to real host text still pins one.
	app := fiber.New()
	app.Use(New(Config{Rules: map[string]string{"/r/*": "https://$1%41.example.com"}}))
	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/t", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "https://t%41.example.com", resp.Header.Get("Location"))
}

// Test_Redirect_NumericSuffixPinsNoHost covers the IPv4 reading of the same
// inversion that makes a capture inside an IPv6 literal unjudgeable.
//
// A host whose last label reads as a number is parsed as an IPv4 address, so
// the author's trailing text is the low octets and the capture supplies the
// network. "https://$1.1" looks like a pinned suffix and composed
// "https://127.0.0.1" from a captured "127.0.0" — loopback, and 169.254.169.254
// or a private range just as easily. The IPv4 parser reads hex too, so
// "https://$1.0x1" did the same.
func Test_Redirect_NumericSuffixPinsNoHost(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ target, request string }{
		{"https://$1.1", "/r/127.0.0"},
		{"https://$1.0x1", "/r/127.0.0"},
		{"https://$1.0.0.1", "/r/127"},
		{"https://$1.0", "/r/127.0.0"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}}))
			app.Use(func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must be refused at startup")
			require.Empty(t, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_CompleteAddressStillPins is the other half: an address the
// author wrote in full is theirs, so a capture beside it is only a port or a
// path.
func Test_Redirect_CompleteAddressStillPins(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ target, request, want string }{
		{"https://127.0.0.1:$1", "/r/8080", "https://127.0.0.1:8080"},
		{"https://10.0.0.1/$1", "/r/a", "https://10.0.0.1/a"},
		{"https://$1.example.com", "/r/tenant", "https://tenant.example.com"},
		{"https://$1.abc.def", "/r/a", "https://a.abc.def"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}}))

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_DotlessTailPinsNoHost covers a literal with no dot of its own:
// it sits inside a label the capture opens on the left, so what the author
// wrote is only that label's tail and the request decides what the label is.
//
// Two tails let the request take the whole label. A tail of hex digits becomes
// a number once the value supplies "0x" — "https://$1cafe" reached
// 0.0.202.254, and "https://$1x" turned a captured "127.0.0" into 127.0.0.0 —
// and the same tails finish a percent-escape the value left open, so
// "https://$1E" composed "https://evil.com%2E" from "evil.com%2", whose
// trailing dot pins nothing. That second shape is pinned by the "E" row of
// Test_PinsHost rather than here, since net/http will not build a request
// carrying the dangling escape it needs.
func Test_Redirect_DotlessTailPinsNoHost(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ rule, target, request string }{
		{"/r/*", "https://$1x", "/r/127.0.0"},
		{"/r/*", "https://$1x", "/r/10.0.0"},
		{"/r/*", "https://$1cafe", "/r/0x"},
		{"/r/*", "https://$1beef", "/r/0x"},
		// The head can simply end with a dot, which pins nothing at all: the
		// author's text becomes a bare final label of the request's choosing.
		{"/r/*", "https://$1xyz", "/r/evil."},
		{"/r/*", "https://$1com", "/r/evil."},
		{"/r/*", "https://$1io", "/r/evil."},
		// The literal sits between two captures, the one place a literal is
		// open on the left without starting the authority.
		{"/r/*/*", "https://$1xyz:$2", "/r/evil./8080"},
		{"/r/*/*", "https://$1cafe:$2", "/r/0x/8080"},
		// Its only dot is trailing, and a trailing dot separates nothing from
		// what precedes it.
		{"/r/*", "https://$1xyz.", "/r/evil."},
		{"/r/*", "https://$1com.", "/r/evil."},
		// The address rule only sees these once the port is cut off and the
		// last label is taken after the trim; skipping either step let them
		// compile and reach 8.0.0.1 from a captured "0".
		{"/r/*", "https://$110.0.0.1:8080", "/r/0"},
		{"/r/*", "https://$110.0.0.1.", "/r/0"},
	} {
		t.Run(tc.target+" "+tc.request, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{tc.rule: tc.target}}))
			app.Use(func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must be refused at startup")
			require.Empty(t, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_ClosedLabelStillPins is the other half: a literal a capture
// cannot reach into still pins the host. An "@" starts the host, so the capture
// before it is only userinfo; a dot of the literal's own separates the label;
// and a tail holding a non-hex byte is one no prefix turns into a number.
func Test_Redirect_ClosedLabelStillPins(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ target, request, want string }{
		{"https://$1@example.com/", "/r/user", "https://user@example.com/"},
		{"https://$1.example.com", "/r/tenant", "https://tenant.example.com"},
		{"https://$1cdn.example.com", "/r/a", "https://acdn.example.com"},
		{"https://$1.de", "/r/shop", "https://shop.de"},
		{"https://$1.cafe", "/r/shop", "https://shop.cafe"},
		{"https://$1xyz.example.com", "/r/a", "https://axyz.example.com"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}}))

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_CarriesQueryOntoTargetsOwnQuery pins how the request's query
// string joins a target that already has one, or a fragment.
//
// Appending "?" + query was right only for a target holding neither. A target
// with its own query got a second "?", which a URL parser reads as one query
// string — "/new?from=old" plus "bar=2" became the single parameter
// "from=old?bar=2" — and a target with a fragment got the query after the "#",
// where it is read as part of the fragment and the request's query is lost.
func Test_Redirect_CarriesQueryOntoTargetsOwnQuery(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ target, request, want string }{
		{"/new?from=old", "/old?bar=2", "/new?from=old&bar=2"},
		{"/new#frag", "/old?bar=2", "/new?bar=2#frag"},
		{"/new?from=old#frag", "/old?bar=2", "/new?from=old&bar=2#frag"},
		{"https://cdn.example.com/p?a=1", "/old?b=2", "https://cdn.example.com/p?a=1&b=2"},

		// Unchanged where there was nothing to merge.
		{"/new", "/old?bar=2", "/new?bar=2"},
		{"/new?from=old", "/old", "/new?from=old"},
		{"/new#frag", "/old", "/new#frag"},
		{"/new", "/old", "/new"},
	} {
		t.Run(tc.target+" "+tc.request, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/old": tc.target}}))

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_NoSlashSpecialSchemeAuthorityIsGuarded pins that a target
// naming its host without the "//" is held to the same rules as one that writes
// them.
//
// A special scheme reaches the authority state either way — "https:host" and
// "https://host" name the same host — but authoritySpan looked for the slashes,
// so the no-slash spelling had no authority for the chunk checks to guard and
// no value was ever refused. "https:cdn.example.com$1" composed
// "https:cdn.example.com@evil.com", where everything before the "@" is userinfo
// and evil.com is the host the client reaches.
func Test_Redirect_NoSlashSpecialSchemeAuthorityIsGuarded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		path   string
		want   string
	}{
		{
			name:   "an @ makes the author's host userinfo",
			target: "https:cdn.example.com$1",
			path:   "/r/@evil.com",
			want:   "",
		},
		{
			name:   "a leading dot extends the host into another domain",
			target: "https:cdn.example.com$1",
			path:   "/r/.evil.com",
			want:   "",
		},
		{
			name:   "ws is special too",
			target: "ws:cdn.example.com$1",
			path:   "/r/@evil.com",
			want:   "",
		},
		{
			name:   "ftp is special too",
			target: "ftp:cdn.example.com$1",
			path:   "/r/@evil.com",
			want:   "",
		},
		{
			// The scheme is case-insensitive, so the reading does not change.
			name:   "the scheme's own case does not matter",
			target: "HTTPS:cdn.example.com$1",
			path:   "/r/@evil.com",
			want:   "",
		},
		{
			// A label under the author's host is what the rule is for.
			name:   "an interior label still redirects",
			target: "https:$1.example.com",
			path:   "/r/tenant",
			want:   "https:tenant.example.com",
		},
		{
			name:   "a capture between two literals still redirects",
			target: "https:cdn.$1.com",
			path:   "/r/example",
			want:   "https:cdn.example.com",
		},
		{
			// The "/" ends the authority, so the capture is a path segment and
			// an "@" in it reaches no host.
			name:   "past the first slash the capture is a path",
			target: "https:cdn.example.com/$1",
			path:   "/r/@evil.com",
			want:   "https:cdn.example.com/@evil.com",
		},
		{
			// mailto is not special: with no "//" it has no authority at all,
			// so the capture is an addressee and the author's host follows it.
			name:   "a scheme that is not special has no implied authority",
			target: "mailto:$1@example.com",
			path:   "/r/user",
			want:   "mailto:user@example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}, StatusCode: fiber.StatusFound}))
			app.Use(func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.path, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
			if tc.want == "" {
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "a refused value falls through")
			}
		})
	}
}

// Test_Redirect_RuleOrderIsBySpecificity pins the order overlapping rules are
// tried in. A map range gave no order at all, so which rule won changed run to
// run; ordering by the literal prefix alone fixed the "/*" versus "/old/*" case
// but left two rules that pin the same prefix tied, and the lexicographic
// fallback then put the broader one first — where it matches everything the
// narrower one would, leaving that rule dead.
func Test_Redirect_RuleOrderIsBySpecificity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rules map[string]string
		path  string
		want  string
	}{
		{
			// Same prefix, so only what each pins past the wildcard separates
			// them.
			name:  "a suffix past the wildcard is more specific",
			rules: map[string]string{"/cdn/*": "/broad", "/cdn/*x": "/narrow"},
			path:  "/cdn/foox",
			want:  "/narrow",
		},
		{
			name:  "and the broader rule still takes what the narrower does not",
			rules: map[string]string{"/cdn/*": "/broad", "/cdn/*x": "/narrow"},
			path:  "/cdn/foo",
			want:  "/broad",
		},
		{
			name:  "an extension is a suffix like any other",
			rules: map[string]string{"/p/*": "/any", "/p/*.png": "/img"},
			path:  "/p/q.png",
			want:  "/img",
		},
		{
			// The prefix comparison still decides first: an anchored prefix is
			// the stronger claim, so the catch-all does not shadow this.
			name:  "a longer prefix outranks a longer total",
			rules: map[string]string{"/*": "/catchall", "/old/*": "/new"},
			path:  "/old/z",
			want:  "/new",
		},
		{
			name:  "a deeper prefix wins too",
			rules: map[string]string{"/a/*": "/one", "/a/b/*": "/two"},
			path:  "/a/b/c",
			want:  "/two",
		},
		{
			// The two pin the same prefix and the same total, and a wildcard
			// matches a run of any length, so the class is the narrower claim.
			name:  "a character class outranks a wildcard",
			rules: map[string]string{"/api/*": "/wild/$1", "/api/[ab]": "/class"},
			path:  "/api/a?token=secret",
			want:  "/class?token=secret",
		},
		{
			name:  "the wildcard still takes paths outside the class",
			rules: map[string]string{"/api/*": "/wild/$1", "/api/[ab]": "/class"},
			path:  "/api/z",
			want:  "/wild/z",
		},
		{
			// Two rules that both carry a wildcard are still told apart by the
			// width of everything beside it.
			name:  "a class past a wildcard is read like any other",
			rules: map[string]string{`/p/*[a-z]`: "/wide", `/p/*[ab]`: "/narrow"},
			path:  "/p/za",
			want:  "/narrow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Rules: tc.rules, StatusCode: fiber.StatusFound}))
			app.Use(func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.path, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_LiteralLengths pins how a rule's pinned length is measured, which is
// what orders two rules that overlap.
func Test_LiteralLengths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rule      string
		prefixLen int
		totalLen  int
	}{
		// No wildcard at all: the whole rule is pinned.
		{rule: "/exact", prefixLen: 6, totalLen: 6},
		{rule: "/", prefixLen: 1, totalLen: 1},
		{rule: "", prefixLen: 0, totalLen: 0},
		// The prefix stops at the first wildcard; the total counts what
		// follows it too.
		{rule: "/cdn/*", prefixLen: 5, totalLen: 5},
		{rule: "/cdn/*x", prefixLen: 5, totalLen: 6},
		{rule: "/p/*.png", prefixLen: 3, totalLen: 6},
		{rule: "*", prefixLen: 0, totalLen: 0},
		{rule: "/a/*/b/*", prefixLen: 3, totalLen: 6},

		// A key is compiled as a regexp, so its metacharacters match something
		// other than themselves and pin nothing. Counting them as path bytes put
		// "/api/[a-z]+" ahead of "/api/users", shadowing the exact rule outright.
		{rule: "/api/[a-z]+", prefixLen: 5, totalLen: 5},
		{rule: "/api/users", prefixLen: 10, totalLen: 10},
		{rule: "/(a|b)/x", prefixLen: 1, totalLen: 4},
		// A group is an alternation too, so it pins what its widest branch pins.
		{rule: "/api/[a-z](specific|x)", prefixLen: 5, totalLen: 6},
		// The "?:" of a non-capturing group is syntax, not path.
		{rule: "/p/(?:ab)", prefixLen: 3, totalLen: 5},
		// A class escape matches any byte of its class, so it pins none of them
		// and ends the prefix where it stands.
		{rule: `/api/\d+`, prefixLen: 5, totalLen: 5},
		{rule: `/api/\w`, prefixLen: 5, totalLen: 5},
		// A quantifier's bounds are syntax, and one allowing none of what it
		// follows takes that back: "/api/a{0,1}" matches "/api/" too.
		{rule: "/api/a{0,1}", prefixLen: 5, totalLen: 5},
		{rule: "/api/a{2,3}", prefixLen: 6, totalLen: 6},
		{rule: "/api/a?b", prefixLen: 5, totalLen: 6},
		{rule: "/api/(ab)?c", prefixLen: 5, totalLen: 6},
		// An unclosed brace is an ordinary byte to the regexp parser.
		{rule: "/api/a{", prefixLen: 6, totalLen: 6},
		// "." matches any byte, so "/api/user." pins no more than "/api/user".
		{rule: "/api/user.", prefixLen: 9, totalLen: 9},
		// Escaped, it matches itself and pins the byte it stands for. Counted in
		// what a path is pinned to, so the backslash adds nothing to either.
		{rule: `/p/a\.png`, prefixLen: 8, totalLen: 8},
		// A complete escape names one character however it is spelled, so it is
		// as specific as writing that character: "/p/\x{61}" is "/p/a".
		{rule: `/p/\x{61}`, prefixLen: 4, totalLen: 4},
		{rule: `/p/\x61`, prefixLen: 4, totalLen: 4},
		{rule: `/p/\141`, prefixLen: 4, totalLen: 4},
		{rule: `/p/\t`, prefixLen: 4, totalLen: 4},
		// An incomplete one names nothing, so it ends the prefix where it
		// stands. Only the prefix is pinned here: such a rule does not compile,
		// so nothing downstream of the sort ever sees it.
		{rule: `/p/\x{61`, prefixLen: 3, totalLen: 5},
		// An anchor asserts a position and consumes nothing, so it pins no path.
		{rule: "/p/a$", prefixLen: 4, totalLen: 4},
		{rule: "/p/[a-z]$", prefixLen: 3, totalLen: 3},
		{rule: "^/p/a", prefixLen: 0, totalLen: 4},
		// A class matches one byte whatever it lists, so listing more
		// alternatives buys no specificity.
		{rule: "/api/[abcdefghijklmnopqrstuvwxyz]", prefixLen: 5, totalLen: 5},
		{rule: "/api/[ab]", prefixLen: 5, totalLen: 5},
		{rule: "/api/[a-z]x", prefixLen: 5, totalLen: 6},
		// An alternation is only as specific as its least specific branch.
		{rule: "/very/specific|/x", prefixLen: 2, totalLen: 2},
		// A "|" inside a group or a class separates no top-level branch.
		{rule: "/p/[a|b]", prefixLen: 3, totalLen: 3},
	}

	for _, tc := range tests {
		t.Run(tc.rule, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.prefixLen, literalPrefixLen(tc.rule), "literalPrefixLen")
			require.Equal(t, tc.totalLen, literalLen(tc.rule), "literalLen")
		})
	}
}

// Test_Redirect_ComposedPort covers one port built from several captures. Each
// is asked for digits, which is the same question asked of the composition, so
// the rule holds — requiring every one to sit directly after the colon left it
// dead for any input at all.
func Test_Redirect_ComposedPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request string
		want    string
	}{
		{"both captures digits", "/r/80/80", "https://example.com:8080"},
		{"uneven digits", "/r/8/443", "https://example.com:8443"},
		{"second capture is not a port", "/r/80/x", ""},
		{"first capture is not a port", "/r/x/80", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				Rules:      map[string]string{"/r/*/*": "https://example.com:$1$2"},
				StatusCode: fiber.StatusFound,
			}))
			app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)

			if tc.want == "" {
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must not fire")
				return
			}
			require.Equal(t, fiber.StatusFound, resp.StatusCode)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_ExactRuleBeatsPattern covers ordering between a rule written
// with regexp syntax and an exact one it overlaps. The metacharacters pin no
// path, so counting them as pinned text made the broad rule win every time.
func Test_Redirect_ExactRuleBeatsPattern(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/api/[a-z]+": "/broad",
			"/api/users":  "/exact",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		{"/api/users", "/exact"},
		{"/api/other", "/broad"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_ExactRuleBeatsDottedPattern is the same ordering question as
// Test_Redirect_ExactRuleBeatsPattern for the one metacharacter that looks like
// path text. A "." matches any byte, so it pins nothing and must not tie with
// the literal it resembles and then win on the lexicographic fallback.
func Test_Redirect_ExactRuleBeatsDottedPattern(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/api/user.": "/broad",
			"/api/users": "/exact",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		{"/api/users", "/exact"},
		{"/api/userx", "/broad"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_NonSpecialSchemeAuthority covers targets under a scheme the
// WHATWG URL Standard does not call special — a custom deep link.
//
// Two things differ there. A "\" is an ordinary authority byte rather than a
// delimiter, so accepting one as opening the path let a value reach past the
// author's host into a new one. And a scheme followed by an opaque path names
// no host at all, so text after the capture only looked like it pinned one:
// "myapp:$1@example.com" against "//evil.com/x" composed a host of evil.com.
func Test_Redirect_NonSpecialSchemeAuthority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		// The capture writes the "//" that opens an authority the target had none of.
		{"capture opens an authority", "myapp:$1@example.com", "/p/%2F%2Fevil.com%2Fx", ""},
		{"capture opens one with the target's slash", "myapp:$1/@evil.com", "/p/%2F", ""},
		// Ordinary userinfo under the same shape still composes.
		{"plain userinfo", "myapp:$1@example.com", "/p/user", "myapp:user@example.com"},
		{"mailto", "mailto:$1@example.com", "/p/bob", "mailto:bob@example.com"},

		// A backslash does not end a non-special authority, so it reaches the host.
		{"backslash in a non-special authority", "myapp://example.com$1", "/p/%5C@evil.com", ""},
		{"backslash written by the author", `myapp://example.com\$1`, "/p/evil.com", ""},
		{"slash still opens the path", "myapp://example.com$1", "/p/%2Fok", "myapp://example.com/ok"},

		// Under a special scheme the parser folds it, so it still opens the path.
		{"backslash under a special scheme", "https://example.com$1", "/p/%5Cok", `https://example.com\ok`},
		{"slash under a special scheme", "https://example.com$1", "/p/%2Fok", "https://example.com/ok"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{
				Rules:      map[string]string{"/p/*": tc.target},
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

// Test_Redirect_ThirdSlashUnderNonSpecialScheme covers a target whose authority
// the author left empty by writing a third slash.
//
// Only a special scheme reaches "special authority ignore slashes" and skips
// past it; under any other one that slash terminates an empty authority, so the
// capture is path. Skipping it regardless read the capture as the host, and the
// rule was dropped at startup for handing the host to the request.
func Test_Redirect_ThirdSlashUnderNonSpecialScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"empty authority leaves the capture in the path", "myapp:///$1", "/p/evil.com", "myapp:///evil.com"},
		{"deeper path under an empty authority", "myapp:///x/$1", "/p/evil.com", "myapp:///x/evil.com"},

		// A special scheme skips them, so the capture is the host and the rule
		// still hands the destination away.
		{"special scheme still skips them", "https:///$1", "/p/evil.com", ""},
		{"two slashes still open an authority", "myapp://$1", "/p/evil.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{
				Rules:      map[string]string{"/p/*": tc.target},
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

			// The authority stayed empty, so nothing the value supplied is a host.
			u, err := url.Parse(resp.Header.Get("Location"))
			require.NoError(t, err)
			require.Empty(t, u.Host)
		})
	}
}

// Test_Redirect_WiderClassDoesNotOutrank covers two rules that differ only in
// how many alternatives their character class lists. A class matches one byte
// either way, so the longer list pins no more path — counting its members made
// the broader rule win every time and left the narrower one dead.
func Test_Redirect_WiderClassDoesNotOutrank(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/api/[abcdefghijklmnopqrstuvwxyz]": "/broad",
			"/api/[ab]":                         "/narrow",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		// Both match, and they now tie on pinned length, so the deterministic
		// key order decides rather than the length of the class.
		{"/api/a", "/narrow"},
		// Only the wider class matches this one.
		{"/api/z", "/broad"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_AlternationRankedByItsWidestBranch covers a rule that can match
// by more than one branch. It is only as specific as the least specific of them,
// so crediting everything before the "|" put it ahead of the exact rule it ties
// with and shadowed that rule outright.
func Test_Redirect_AlternationRankedByItsWidestBranch(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/very/specific|/x": "/alt",
			"/x":                "/exact",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		// Both match, and the exact rule wins: they pin the same one byte, and
		// the alternation matches strictly more paths.
		{"/x", "/exact"},
		// The branch only the alternation matches still reaches it.
		{"/very/specific", "/alt"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_GroupedAlternationRankedByItsWidestBranch covers an alternation
// written inside a group, which is one just the same. Crediting it with the
// literals of every branch made the rule that matches more paths sort as the
// more specific one, so it answered every request they share.
func Test_Redirect_GroupedAlternationRankedByItsWidestBranch(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/p/[a-z](reports|x.*)": "/grouped",
			"/p/[a-z]xy":            "/narrow",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		// Both match "/p/axy". The grouped rule pins only the one byte its "x.*"
		// branch does, so the rule pinning "xy" answers it.
		{"/p/axy", "/narrow"},
		// What only the grouped rule matches still reaches it.
		{"/p/areports", "/grouped"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_OptionalQuantifierDoesNotOutrankExactRule covers "{0,1}", whose
// bounds were counted as path bytes. That put the rule matching "/api/" as well
// ahead of the exact "/api/a", which it then answered for.
func Test_Redirect_OptionalQuantifierDoesNotOutrankExactRule(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/api/ab{0,1}": "/maybe",
			"/api/ab":      "/exact",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		{"/api/ab", "/exact"},
		// What only the quantified rule matches still reaches it.
		{"/api/a", "/maybe"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_AnchorPinsNoPath covers an explicit "$" in a rule. It asserts a
// position and consumes nothing, so counting it as a path byte put the broader
// class rule ahead of the exact one on a path they both match.
func Test_Redirect_AnchorPinsNoPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/p/[a-z]$": "/class",
			"/p/[a]":    "/exact",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		{"/p/a", "/exact"},
		{"/p/b", "/class"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_UserinfoDelimitersFollowTheScheme covers a capture in the
// userinfo of a non-special target. A backslash ends the authority only under a
// special scheme, so refusing one outright dropped a redirect whose host the
// author had pinned — "myapp://user\name@example.com" still reaches example.com.
func Test_Redirect_UserinfoDelimitersFollowTheScheme(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		target string
		want   string
	}{
		{name: "non-special takes the backslash as userinfo", target: "myapp://$1@example.com", want: `myapp://user\name@example.com`},
		{name: "special folds it into a slash", target: "https://$1@example.com", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{Rules: map[string]string{"/r/*": tc.target}, StatusCode: fiber.StatusFound}))
			app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/user%5Cname", http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}

// Test_Redirect_FileAuthorityOpensOnBackslashes covers "file:/$1", which looks
// like it names no authority. The parser folds a backslash into a slash on the
// way to the file host, so a captured "\evil.com/share" composes a location
// whose host is evil.com — on Windows, a UNC path to a server of its choosing.
func Test_Redirect_FileAuthorityOpensOnBackslashes(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{UnescapePath: true})
	app.Use(New(Config{Rules: map[string]string{"/r/*": "file:/$1"}, StatusCode: fiber.StatusFound}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/%5Cevil.com/share", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Location"))

	// A value that stays a path still redirects. So does a third slash, which
	// closes the authority empty exactly as "file:///tmp" does.
	for _, tc := range []struct{ request, want string }{
		{"/r/tmp/report", "file:/tmp/report"},
		{"/r/%5C%5Cevil.com/share", `file:/\\evil.com/share`},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}

	// The span itself, which is what the runtime guard asks of the composition:
	// two slashes in any spelling open the host, and a third closes it empty.
	for _, target := range []string{`file:/\evil.com/share`, `file:\\evil.com/share`, `file:\/evil.com`, "file://evil.com"} {
		start, end := authoritySpan(target)
		require.Equal(t, "evil.com", target[start:end], target)
	}
	for _, target := range []string{`file:/\\evil.com`, "file:///tmp", `file:\\\evil.com`} {
		start, end := authoritySpan(target)
		require.Equal(t, start, end, target)
	}
}

// Test_Redirect_AnchorsBindEveryBranch covers a rule holding a top-level "|".
// Concatenating the anchors made "^/a|/b$", which anchors the start of one
// branch and the end of the other, so a path only sharing an edge matched.
func Test_Redirect_AnchorsBindEveryBranch(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Rules: map[string]string{"/a|/b": "/moved"}, StatusCode: fiber.StatusFound}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct {
		request string
		want    int
	}{
		{"/a", fiber.StatusFound},
		{"/b", fiber.StatusFound},
		{"/a-extra", fiber.StatusOK},
		{"/extra/b", fiber.StatusOK},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, tc.want, resp.StatusCode, tc.request)
	}
}

// Test_Redirect_NestedAlternationLosesTheTieBreak covers two rules that tie on
// both specificity measures. The wider one matches a path the other does not, so
// key order deciding between them left the exact rule dead.
func Test_Redirect_NestedAlternationLosesTheTieBreak(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/p/[a-z](x|y)": "/wide",
			"/p/[a-z]x":     "/narrow",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		{"/p/ax", "/narrow"},
		{"/p/ay", "/wide"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}

	// A class counts its breadth here too, so the two differ by the group alone.
	require.Equal(t, 52, patternWidth("/p/[a-z](x|y)"))
	require.Equal(t, 26, patternWidth("/p/[a-z]x"))
	require.Equal(t, 1, patternWidth("/p/[a]"))
	require.Equal(t, 256, patternWidth("/p/."))
	require.Equal(t, 2, patternWidth("/very/specific|/x"))
	require.Equal(t, 4, patternWidth("(a|b)(c|d)"))

	// The wildcard is ranked on its own rather than counted here, so the width
	// goes on separating two rules that both carry one.
	require.Equal(t, 1, wildcardRank("/p/*"))
	require.Equal(t, 0, wildcardRank("/p/[a-z]"))
	require.Greater(t, patternWidth("/p/*[a-z]"), patternWidth("/p/*[ab]"))

	// A star that names itself is no wildcard: quoted, or listed by a class.
	require.Equal(t, 0, wildcardRank(`/p/\Q*\E`))
	require.Equal(t, 0, wildcardRank(`/p/\Qab*`))
	require.Equal(t, 0, wildcardRank("/p/[*]"))
	require.Equal(t, 1, wildcardRank(`/p/\Qab\E*`))
	require.Equal(t, 1, wildcardRank(`/p/\d*`))
}

// Test_Redirect_FewerWildcardsWin ties every other specificity measure to
// ensure a broader multi-wildcard rule cannot shadow a narrower rule.
func Test_Redirect_FewerWildcardsWin(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/p/*a*b": "https://attacker.example/$1/$2",
			"/p/*ab":  "/safe/$1",
		},
		StatusCode: fiber.StatusFound,
	}))

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/p/xxab?secret=top", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusFound, resp.StatusCode)
	require.Equal(t, "/safe/xx?secret=top", resp.Header.Get("Location"))
	require.Equal(t, 2, wildcardRank("/p/*a*b"))
	require.Equal(t, 1, wildcardRank("/p/*ab"))
	// Both expand to a single alternative, so the width leaves them tied and
	// the count is all that stands between them.
	require.Equal(t, patternWidth("/p/*a*b"), patternWidth("/p/*ab"))
}

// Test_Redirect_WildcardCountYieldsToWidth covers the pair the count alone gets
// backwards: "/p/([a]*|[c]*)" carries two wildcards to "/p/[a-d]*"'s one and yet
// matches half as many paths, so reading the count before the width handed every
// "/p/a..." request to the broader rule.
func Test_Redirect_WildcardCountYieldsToWidth(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			`/p/([a]*|[c]*)`: "/narrow",
			`/p/[a-d]*`:      "/broad",
		},
		StatusCode: fiber.StatusFound,
	}))

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/p/axyz", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusFound, resp.StatusCode)
	require.Equal(t, "/narrow", resp.Header.Get("Location"))

	// Every earlier measure ties, and the wildcard count points the wrong way.
	require.Equal(t, literalPrefixLen(`/p/[a-d]*`), literalPrefixLen(`/p/([a]*|[c]*)`))
	require.Equal(t, literalLen(`/p/[a-d]*`), literalLen(`/p/([a]*|[c]*)`))
	require.Equal(t, carriesWildcard(`/p/[a-d]*`), carriesWildcard(`/p/([a]*|[c]*)`))
	require.Greater(t, wildcardRank(`/p/([a]*|[c]*)`), wildcardRank(`/p/[a-d]*`))
	require.Less(t, patternWidth(`/p/([a]*|[c]*)`), patternWidth(`/p/[a-d]*`))
}

// Test_Redirect_UnterminatedQuoteRuleIsRejected pins what an unclosed "\Q" does:
// Go reads it as quoting the rest of the rule, which swallows the ")" anchoring
// the key, so the rule never compiles and never reaches the ordering the count
// decides. The count is still carried past the quoted tail, since the wildcards
// standing before it are expanded either way.
func Test_Redirect_UnterminatedQuoteRuleIsRejected(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			Rules:      map[string]string{`/p/*\Qab*`: "/safe"},
			StatusCode: fiber.StatusFound,
		})
	})

	require.Equal(t, 1, wildcardRank(`/p/*\Qab*`))
	require.Equal(t, 2, wildcardRank(`/p/*\Qab\E*`))
}

// Test_Redirect_HexEscapeOutranksAClass covers "\x{61}", which names the one
// character "a". Reading every letter-led escape as a class scored it no higher
// than "[a-z]", and the tie left key order to pick the broader rule.
func Test_Redirect_HexEscapeOutranksAClass(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			`/p/\x{61}`: "/exact",
			"/p/[a-z]":  "/class",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		{"/p/a", "/exact"},
		{"/p/b", "/class"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_CaptureEndsAuthorityPastADeletedByte covers a captured "\t/ok".
// The parser deletes the tab, so the value ends the authority at the author's
// host and the client reaches "/ok" — judging the tab as the first byte instead
// read the value as extending the host, and the redirect was refused.
func Test_Redirect_CaptureEndsAuthorityPastADeletedByte(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{UnescapePath: true})
	app.Use(New(Config{
		Rules:      map[string]string{"/r/*": "https://example.com$1"},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/%09%2Fok", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusFound, resp.StatusCode)

	// What the client reads is example.com, whatever bytes the parser drops on
	// the way there.
	location := resp.Header.Get("Location")
	ref, err := url.Parse(strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(location))
	require.NoError(t, err)
	require.Equal(t, "example.com", ref.Host)
	require.Equal(t, "/ok", ref.Path)

	// A value that really does extend the host is still refused.
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/%09evil", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test_Redirect_BareFileSchemeIsAPath covers a "file:" target with no slashes,
// which names no authority: the parser leaves "file state" for "path state"
// unless one follows, so what stands there is path. Reading it as a host made a
// value that is only a path name one, and the redirect was skipped at runtime.
func Test_Redirect_BareFileSchemeIsAPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Rules: map[string]string{"/r/*": "file:tmp$1"}, StatusCode: fiber.StatusFound}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/report", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusFound, resp.StatusCode)
	require.Equal(t, "file:tmpreport", resp.Header.Get("Location"))

	// A bare "file:" target holds no authority span, so it is opaque-path and the
	// composed location is what gets checked for one.
	start, end := authoritySpan("file:tmp/report")
	require.Equal(t, 0, start)
	require.Equal(t, 0, end)
	start, end = authoritySpan("file://host/report")
	require.NotEqual(t, start, end)
}

// Test_Redirect_AuthorPinnedShortIPv4 covers "127.1", which the URL parser reads
// as 127.0.0.1. Judging it by net.ParseIP said the author had pinned no host and
// dropped a rule whose host they had written in full.
func Test_Redirect_AuthorPinnedShortIPv4(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"127.1", "0x7f.1", "2130706433", "127.0.0.1"} {
		app := fiber.New()
		app.Use(New(Config{
			Rules:      map[string]string{"/r/*": "https://" + host + ":$1"},
			StatusCode: fiber.StatusFound,
		}))
		app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/r/8080", http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, host)
		require.Equal(t, "https://"+host+":8080", resp.Header.Get("Location"), host)
	}

	// A host that is not an address still hands the host away when the capture
	// closes it, so "https://$1" stays refused.
	require.False(t, isIPv4Host("127.0.0.256"))
	require.False(t, isIPv4Host("1.2.3.4.5"))
	require.False(t, isIPv4Host("0x1g"))
	require.True(t, isIPv4Host("127.0.0.1."))
}

// Test_Redirect_ClassEscapeDoesNotOutrankExactRule covers "\d", which matches any
// digit. Reading it as the byte "d" escaped counted two path bytes that are not
// there, putting the rule that matches every digit ahead of the exact one.
func Test_Redirect_ClassEscapeDoesNotOutrankExactRule(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			`/api/\d+`: "/digits",
			"/api/1":   "/one",
		},
		StatusCode: fiber.StatusFound,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

	for _, tc := range []struct{ request, want string }{
		{"/api/1", "/one"},
		{"/api/27", "/digits"},
	} {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusFound, resp.StatusCode, tc.request)
		require.Equal(t, tc.want, resp.Header.Get("Location"), tc.request)
	}
}

// Test_Redirect_FileSchemeEmptyAuthority covers "file", which is special — it
// folds backslashes — but reaches its host through "file slash state" rather
// than the ignore-slashes one every other special scheme takes. So "file:///$1"
// has the empty authority of a local path, and skipping that third slash read
// the capture as the host and dropped the rule at startup.
func Test_Redirect_FileSchemeEmptyAuthority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"local path composes", "file:///$1", "/p/tmp/report", "file:///tmp/report"},
		{"deeper local path", "file:///var/$1", "/p/log", "file:///var/log"},
		// Two slashes still open an authority, so the capture is the host there.
		{"capture as the file host", "file://$1/x", "/p/evil.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: true})
			app.Use(New(Config{
				Rules:      map[string]string{"/p/*": tc.target},
				StatusCode: fiber.StatusFound,
			}))
			app.Get("/*", func(c fiber.Ctx) error { return c.SendString("fell through") })

			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tc.request, http.NoBody)
			require.NoError(t, err)
			resp, err := app.Test(req)
			require.NoError(t, err)

			if tc.want == "" {
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "the rule must not fire")
				return
			}
			require.Equal(t, fiber.StatusFound, resp.StatusCode)
			require.Equal(t, tc.want, resp.Header.Get("Location"))
		})
	}
}
