package redirect

import (
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/urlnorm"
	"github.com/stretchr/testify/require"
)

// testApp builds an app running the middleware over rules, with a fall-through
// route so a request no rule redirects answers 200 "fell through".
func testApp(rules map[string]string, unescape bool) *fiber.App {
	app := fiber.New(fiber.Config{UnescapePath: unescape})
	app.Use(New(Config{Rules: rules, StatusCode: fiber.StatusFound}))
	app.Get("/*", func(c fiber.Ctx) error {
		return c.SendString("fell through")
	})
	return app
}

// get sends one GET to app and returns the response status and Location.
func get(t *testing.T, app *fiber.App, path string) (status int, location string) { //nolint:nonamedreturns // names document the pair
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Location")
}

// requireWin asserts that one GET for path redirects to want.
func requireWin(t *testing.T, rules map[string]string, path, want string) {
	t.Helper()
	status, location := get(t, testApp(rules, false), path)
	require.Equal(t, fiber.StatusFound, status, "request %q", path)
	require.Equal(t, want, location, "request %q", path)
}

// requireRule builds a one-rule app and asserts where the request lands:
// redirected to want, or — want "" — fallen through with no Location.
func requireRule(t *testing.T, unescape bool, pattern, target, request, want string) {
	t.Helper()
	status, location := get(t, testApp(map[string]string{pattern: target}, unescape), request)
	if want == "" {
		require.Equal(t, fiber.StatusOK, status, "the rule must not fire on %q", request)
		require.Empty(t, location)
		return
	}
	require.Equal(t, fiber.StatusFound, status, "request %q", request)
	require.Equal(t, want, location, "request %q", request)
}

func Test_Redirect(t *testing.T) {
	app := fiber.New()

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
			status, location := get(t, app, tt.url)
			require.Equal(t, tt.statusCode, status)
			require.Equal(t, tt.redirectTo, location)
		})
	}
}

// Test_Redirect_StartAnchor verifies a rule only matches from the start of the path (issue #4476).
func Test_Redirect_StartAnchor(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{"/old": "/new"},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Get("/very/old", func(c fiber.Ctx) error { return c.SendString("not redirected") })

	// The rule matches the whole path and redirects.
	status, location := get(t, app, "/old")
	require.Equal(t, fiber.StatusMovedPermanently, status)
	require.Equal(t, "/new", location)

	// A path that only ends with the rule path must not be redirected.
	status, location = get(t, app, "/very/old")
	require.Equal(t, fiber.StatusOK, status)
	require.Empty(t, location)
}

// Test_Redirect_SameOriginTargets verifies a captured path segment cannot take a path-only target off this origin.
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

		// A target naming its own authority is the author's call, so it is left as configured.
		{"absolute target", "/ext/*", "https://cdn.example.com/$1", "/ext/a", "https://cdn.example.com/a"},
		{"protocol relative target", "/pr/*", "//cdn.example.com/$1", "/pr/a", "//cdn.example.com/a"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, tc.pattern, tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_SameOriginTargets_Unescaped covers the same guard once UnescapePath decodes the capture.
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

		// A space is percent-encoded rather than removed, so an interior one forms no authority.
		{"interior space", "/api/*", "/$1", "/api/%20/evil.com", "/ /evil.com"},
		{"ordinary capture", "/api/*", "/$1", "/api/users", "/users"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, true, tc.pattern, tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_CaptureInsideAuthority asserts a capture in the target's authority cannot pick another host.
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

		// An "@" makes everything before it userinfo, so the host becomes the capture's.
		{"at sign makes the host userinfo", "https://$1.assets.example.com/", "/cdn/a@evil.com", ""},
		{"at sign at the end of the authority", "https://cdn.example.com$1", "/cdn/@evil.com", ""},
		// A capture that does not open a new component extends the host the author wrote.
		{"bare value extends the host", "https://cdn.example.com$1", "/cdn/x", ""},

		// A clean label still composes.
		{"clean label", "https://$1.assets.example.com/", "/cdn/images", "https://images.assets.example.com/"},
		{"clean label protocol relative", "//$1.assets.example.com/", "/cdn/images", "//images.assets.example.com/"},

		// A capture in the path may hold slashes freely; the target fixed the authority.
		{"path capture keeps slashes", "https://cdn.example.com/$1", "/cdn/a/b/c", "https://cdn.example.com/a/b/c"},
		{"path capture with a host-like value", "https://cdn.example.com/$1", "/cdn/evil.com/x", "https://cdn.example.com/evil.com/x"},

		// A target handing the whole authority to the capture never fires, and New warns why.
		{"whole authority is the capture", "https://$1", "/cdn/anything/x", ""},
		{"protocol relative whole authority", "//$1", "/cdn/anything", ""},
		{"scheme with no authority", "https:$1", "/cdn//evil.com", ""},
		// Separators alone pin nothing: "///evil.com." is read host-first, so a value opening a path ends no authority.
		{"separator only before the capture", "//$1.", "/cdn/evil.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, true, "/cdn/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_CaptureInUserinfo covers a capture before an "@" the author wrote, where the value is only userinfo.
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

		// Past the author's "@" the capture is a host label again.
		{"label after the at sign", "https://user@$1.example.com/", "/cdn/a%3Ab", ""},
		{"clean label after the at sign", "https://user@$1.example.com/", "/cdn/images", "https://user@images.example.com/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, true, "/cdn/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_CaptureEndingTheAuthority covers a placeholder ending the authority, which must open a new component.
func Test_Redirect_CaptureEndingTheAuthority(t *testing.T) {
	t.Parallel()

	// "/cdn*" leaves the leading slash in the capture, so $1 supplies the path.
	const target = "https://cdn.example.com$1"
	requireRule(t, false, "/cdn*", target, "/cdn/foo.png", "https://cdn.example.com/foo.png")
	requireRule(t, false, "/cdn*", target, "/cdn/a/b/c", "https://cdn.example.com/a/b/c")
	// The capture carries the "@" into the path, so the host is still the author's.
	requireRule(t, false, "/cdn*", target, "/cdn/@evil.com", "https://cdn.example.com/@evil.com")
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

	// A literal between two captures is open on the left, unlike one starting the authority.
	require.Equal(t, []authorityChunk{
		{text: "$1", placeholder: true},
		{text: "xyz:"},
		{text: "$2", placeholder: true},
	}, authorityChunks("https://$1xyz:$2"))

	require.Equal(t, []authorityChunk{
		{text: "xyz.example.com", pins: true},
		{text: "$1", placeholder: true},
	}, authorityChunks("https://xyz.example.com$1"))

	// A target that is nothing but the token: the author picked the destination outright.
	require.Equal(t, []authorityChunk{{text: "$1", placeholder: true}}, authorityChunks("https://$1"))
	require.Equal(t, []authorityChunk{{text: "$12", placeholder: true}}, authorityChunks("//$12"))
}

// Test_Redirect_CaptureBoundedByTheTarget covers a token that closes the authority but not the target.
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

		// A capture ending the host is refused even where the target continues: "evil.com" composes "example.comevil.com".
		{"host extended before a path", "https://example.com$1/health", "/t/evil.com", ""},
		{"host prefix with a path after", "https://tenant-$1/app", "/t/evil.com", ""},
		{"slash in a host prefix", "https://tenant-$1/app", "/t/evil.com%2Fx", ""},
		// Opening a path closes the host at the author's own text.
		{"capture opens a path", "https://example.com$1/health", "/t/%2Ffoo", "https://example.com/foo/health"},

		// A token after the port colon ends the target, but the parser rejects a non-numeric port, so digits are honored.
		{"port at the end of the target", "https://cdn.example.com:$1", "/t/8080", "https://cdn.example.com:8080"},
		{"non-numeric port", "https://cdn.example.com:$1", "/t/80@evil.com", ""},
		{"port extended into a host", "https://cdn.example.com:$1", "/t/8080.evil.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, true, "/t/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_CaptureClosingTheHost covers a capture naming the host without being the target's last chunk.
func Test_Redirect_CaptureClosingTheHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		request string
		want    string // "" means the rule must not fire
	}{
		{"empty trailing capture", "https://$1$2", "/a/evil.comx", ""},
		{"empty trailing capture after a dot", "https://$1.$2", "/a/evil.comx", ""},

		// An empty trailing capture is fine where the author's own text still closes the host.
		{"author text closes the host", "https://$1.assets.example.com$2", "/a/tenantx", "https://tenant.assets.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, "/a/*x*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_TenthCaptureInAuthority pins the guard on what the Replacer splices in, "$10" being "$1" then "0".
func Test_Redirect_TenthCaptureInAuthority(t *testing.T) {
	t.Parallel()

	requireRule(t, false, "/t/*/*/*/*/*/*/*/*/*/*", "https://$10.cdn.example.com/",
		"/t/evil.com/x/b/c/d/e/f/g/h/i/tenant", "")
}

// Test_Redirect_ExtraSlashesBeforeTheCapture covers a target opening its authority with more than two slashes.
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

		// A host the author wrote after the run is still their choice.
		{"fixed host after the run", "///static/$1", "/go/x", "///static/x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, "/go/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_OverlappingRulesAreDeterministic pins that two patterns matching one path resolve the same every run.
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

	resp, err := build().Test(httptest.NewRequest(fiber.MethodGet, "/cdn/ax", http.NoBody))
	require.NoError(t, err)
	first := resp.Header.Get("Location")
	require.NotEmpty(t, first)

	// Rebuilding the middleware re-walks the rules map, which is where the randomization came from.
	for range 20 {
		resp, err := build().Test(httptest.NewRequest(fiber.MethodGet, "/cdn/ax", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, first, resp.Header.Get("Location"))
	}
}

// Test_Redirect_PortDoesNotPinTheHost covers a capture followed only by a port: "evil.com:8080" is still evil.com.
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

			requireRule(t, false, tc.pattern, tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_MoreSpecificRuleWins pins that the rule pinning more of the path is tried first, not "/*" before "/old/*".
func Test_Redirect_MoreSpecificRuleWins(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/*":     "/home",
		"/old/*": "/new/$1",
	}
	requireWin(t, rules, "/old/thing", "/new/thing")
	// The catch-all still covers everything the specific rule does not.
	requireWin(t, rules, "/other", "/home")
}

// Test_TargetLetsRequestPickHost pins which target shapes hand the destination to the request, "https:$1" among them.
func Test_TargetLetsRequestPickHost(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		target string
		want   bool
	}{
		{"https://$1", true},
		{"//$1", true},
		{"https:$1", true},
		// The missing-solidus step gives every special scheme but file an authority: "ws:evil.com" is ws://evil.com.
		{"ws:$1", true},
		{"wss:$1", true},
		{"ftp:$1", true},
		{"WS:$1", true},
		{"FTP:$1", true},
		// file has no missing-solidus reading, but a captured "//evil.com" opens an authority for it too.
		{"file:$1", true},
		// Any scheme at all: "//" is what opens the authority.
		{"mailto:$1", true},
		{"myapp:$1", true},
		{"custom:$1", true},
		// Author text between the colon and the capture leaves no authority for the value to open.
		{"myapp:fixed/$1", false},
		{"https:fixed/$1", false},
		// A port, a captured port, a second capture and a trailing dot pin nothing, so the value names the host.
		{"https://$1:8080", true},
		{"https://$1:8080/health", true},
		{"https://$1:$2", true},
		{"https://$1$2", true},
		{"//$1.", true},
		{"//.$1", true},
		// Text before an "@" is a username, not a host.
		{"https://example.com@$1", true},
		{"https://user:pw@$1", true},
		// A capture inside the brackets is refused either side, a bracketed address running most significant group first.
		{"https://[$1]", true},
		{"https://[$1]:8080/", true},
		{"https://[$1::1]", true},
		{"https://[2001:db8::$1]", true},
		{"https://[2001:db8::$1]:8080", true},

		// A scheme with no authority syntax has no host to hijack, so it is not refused.
		{"mailto:$1@example.com", false},
		{"myapp:$1@example.com", false},

		{"https://$1.example.com", false},
		{"https://cdn.example.com$1", false},
		{"https://cdn.example.com/$1", false},
		// The same rules spelled without the "//", which a special scheme reads the same way.
		{"https:$1.example.com", false},
		{"https:cdn.example.com$1", false},
		{"https:cdn.$1.com", false},
		{"ws:$1.example.com", false},
		{"ftp:cdn.example.com$1", false},
		{"HTTPS:$1.example.com", false},
		// And where nothing but a port or a dot sits beside it, refused in either spelling.
		{"https:$1:8080", true},
		{"ws:$1.", true},
		{"https:example.com@$1", true},
		// The author's host text can sit either side of the capture.
		{"https://$1@example.com", false},
		{"https://cdn.example.com:$1", false},
		{"https://tenant-$1.example.com", false},
		// A bracketed literal the author wrote in full still pins the host, so a captured port beside it is theirs to allow.
		{"https://[::1]:$1", false},
		{"https://[::1]:8080/$1", false},
		{"https://[::]:$1", false},
		{"https://[2001:db8::1]:$2/$1", false},
		// A bracket in userinfo is an ordinary character, not a host delimiter.
		{"https://[$1]@example.com", false},
		{"https://us[er@$1.example.com", false},
		{"https://us[er@example.com:$1", false},
		// Empty brackets pin no host, so the capture names it outright.
		{"https://[]$1", true},
		{"https://[:]:$1", true},
		// Only an "@" outside the brackets ends the userinfo, so a capture among them still chooses the address.
		{"https://[$1@::1]", true},
		{"https://[2001:db8::$1@a]", true},
		{"https://x@y@[$1::1]", true},
		// Past the closing bracket the "@" does end the userinfo, so the capture only reaches it.
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

// Test_PinsHost pins the three things that are not author-written host text: a port, trailing separators, and userinfo.
func Test_PinsHost(t *testing.T) {
	t.Parallel()

	// wantClosed is the answer when a label separator, an "@" or the start of the authority stands to the left.
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

		// An IPv6 literal's own colons sit inside the brackets, so the port is what follows the closing one.
		{"[::1]", true},
		{"[::1]:", true},
		{"[::1]:8080", true},
		{"[2001:db8::1]", true},
		// The unspecified address is still a complete one the author wrote.
		{"[::]", true},
		{"[::]:", true},
		// An opener with no closer is a fragment of an address some capture split, so it pins nothing.
		{"[2001:db8::", false},
		{"[::", false},
		{"[abc", false},
		{"[fe80", false},
		// Decided by the fragment rule alone: it is neither dotless nor a hex tail.
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
		// The brackets are punctuation, so "https://[$1]:8080" pins no more than "https://$1:8080".
		{"[", false},
		{"]", false},
		{"]:8080", false},
		// Brackets holding no address pin nothing, and counting them composed a location no client parses.
		{"[]", false},
		{"[:]", false},
		{"[.]", false},
		{"[[]", false},
		// Holding a hex digit is not the same as being an address.
		{"[zzz1]", false},
		{"[evil.com1]", false},
		{"[a b]", false},
		// Brackets hold IPv6 only, so an IPv4 address is no host there — but the IPv4-mapped spelling is.
		{"[127.0.0.1]", false},
		{"[1.2.3.4]", false},
		{"[::ffff:127.0.0.1]", true},
		{"[0:0:0:0:0:0:0:1]", true},
		// A zone ID is not accepted in a URL host, by this or by the parser.
		{"[fe80::1%25eth0]", false},
		// Whitespace is not host text: the parser deletes a tab and percent-encodes a space.
		{" ", false},
		{"\t", false},
		{"\n", false},
		{" \t ", false},
		// Nor is anything UTS #46 deletes, nor what it folds onto a plain ".".
		{"\u00ad", false}, // soft hyphen
		{"\ufeff", false}, // zero width no-break space
		{"\u200b", false}, // zero width space
		{"\u2060", false}, // word joiner
		{"\ufe0f", false}, // variation selector
		{"\u3002", false}, // ideographic full stop
		{"\uff0e", false}, // fullwidth full stop
		{"\uff61", false}, // halfwidth ideographic full stop
		{"\u00ad\u200b", false},
		// A control character is stripped from either end and forbidden in the middle, so it pins nothing wherever it sits.
		{"\x00", false},
		{"\x01", false},
		{"\x1f", false},
		{"\x7f", false},
		// The parser percent-decodes a host first, so an escape pins only what it stands for.
		{"%2E", false},       // "."
		{"%2e", false},       // same, lowercased
		{"%C2%AD", false},    // soft hyphen
		{"%E3%80%82", false}, // ideographic full stop
		{"%EF%BB%BF", false}, // BOM
		// "A" is a hex tail a capture can turn into a number with "0x", so it pins only behind a label separator.
		{"%41", false},
		{"%41.example.com", true},
		// A stray "%" is literal to the parser, not an error — though dotless.
		{"100%", false},
		{"a%zz", false},
		// A host whose last label reads as a number is an IPv4 address, and only a complete one pins a host.
		{".1", false},
		{".0", false},
		{".0.0.1", false},
		{".0x1", false},
		// A hex tail is one a capture can open a number with, by supplying the "0x", or finish a dangling escape.
		{"cafe", false},
		{"beef", false},
		{"e", false},
		{"ad", false},
		{"x", false},
		{"xcafe", false},
		// Invalid UTF-8 is not host text: the mapping turns it into U+FFFD.
		{"\xff.example.com", false},
		{"%FF.example.com", false},
		{"\xed\xa0\x80.example.com", false},

		// Nor does any other dotless tail: a captured "evil." left "https://$1xyz" reaching evil.xyz.
		{"cdn", false},
		{"xyz", false},
		{"tenant-", false},
		{"com", false},
		// A trailing dot separates no label and the trim takes it off, so these are dotless too.
		{"xyz.", false},
		{"com.", false},
		// A leading "." closes the label, so an all-hex suffix is a suffix: ".de" pins Germany.
		{".de", true},
		{".be", true},
		{".cc", true},
		{".ad", true},
		{".cafe", true},
		{".e", true},
		// Even a whole address: open on the left the capture reaches its first octet, which is the network.
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		// A name is still a name, whatever letters it is made of.
		{"abc.def", true},
		{".example.com", true},
		{"example1.com", true},
		// An internationalized label is host text, in either spelling.
		{"\u4f8b\u3048.jp", true},
		{"xn--r8jz45g.jp", true},
		// A closer with no opener is the tail of a split address, and an IPv6 tail is the low bits.
		{"::1]", false},
		{":db8::1]", false},
	} {
		require.Equal(t, tc.want, pinsHost(tc.literal, true), "literal %q", tc.literal)
	}

	// The closed-on-the-left branch, where a dotless tail the capture cannot reach pins the host.
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

		// A bare number is an address rather than a name, abbreviated spellings included: "1" is 0.0.0.1.
		{"1", true},
		{"0x1", true},

		{"", false},
		{".", false},
		{"[example.com", false},
		// Closed on the left, the decode, map and trim steps each answer for themselves.
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
		// The trim has to run on the front too, or a last label of " 1" reads as no number.
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

	// The query is appended after the location is made same-origin, so it survives the collapse.
	requireRule(t, false, "/api/*", "/$1", "/api//evil.com?a=1&b=2", "/evil.com?a=1&b=2")
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
	app := fiber.New()
	app.Use(New(Config{
		Next:       func(fiber.Ctx) bool { return true },
		Rules:      map[string]string{"/default": "google.com"},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	status, _ := get(t, app, "/default")
	require.Equal(t, fiber.StatusOK, status)

	// Case 2 : Next function always returns false
	app = fiber.New()
	app.Use(New(Config{
		Next:       func(fiber.Ctx) bool { return false },
		Rules:      map[string]string{"/default": "google.com"},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	status, location := get(t, app, "/default")
	require.Equal(t, fiber.StatusMovedPermanently, status)
	require.Equal(t, "google.com", location)
}

func Test_NoRules(t *testing.T) {
	// Case 1: No rules with default route defined
	app := fiber.New()
	app.Use(New(Config{StatusCode: fiber.StatusMovedPermanently}))
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	status, _ := get(t, app, "/default")
	require.Equal(t, fiber.StatusOK, status)

	// Case 2: No rules and no default route defined
	app = fiber.New()
	app.Use(New(Config{StatusCode: fiber.StatusMovedPermanently}))

	status, _ = get(t, app, "/default")
	require.Equal(t, fiber.StatusNotFound, status)
}

func Test_DefaultConfig(t *testing.T) {
	// Case 1: Default config and no default route
	app := fiber.New()
	app.Use(New())

	status, _ := get(t, app, "/default")
	require.Equal(t, fiber.StatusNotFound, status)

	// Case 2: Default config and default route
	app = fiber.New()
	app.Use(New())
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	status, _ = get(t, app, "/default")
	require.Equal(t, fiber.StatusOK, status)
}

func Test_RegexRules(t *testing.T) {
	// Case 1: Rules regex is empty
	app := fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	status, _ := get(t, app, "/default")
	require.Equal(t, fiber.StatusOK, status)

	// Case 2: Rules regex map contains valid regex and well-formed replacement URLs
	app = fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{"/default": "google.com"},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	status, location := get(t, app, "/default")
	require.Equal(t, fiber.StatusMovedPermanently, status)
	require.Equal(t, "google.com", location)

	// Case 3: Test invalid regex throws panic
	app = fiber.New()
	require.Panics(t, func() {
		app.Use(New(Config{
			Rules:      map[string]string{"(": "google.com"},
			StatusCode: fiber.StatusMovedPermanently,
		}))
	})
}

func requireNoAuthorityChunks(t *testing.T, target, msg string) {
	t.Helper()
	require.Nil(t, authorityChunks(target), msg)
}

// Test_Redirect_TargetIsReadAsTheClientWillRead pins the guard on the target the parser sees, tabs, LFs and CRs gone.
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

			requireRule(t, true, "/r/*", tc.target, tc.request, "")
		})
	}
}

// Test_Redirect_TargetWhitespaceLeavesWorkingRulesAlone is the other half: a target that never had any must be left alone.
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

			requireRule(t, false, "/r/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_HostTextTheParserDeletes pins that the code points UTS #46 deletes cannot make a target look host-pinned.
func Test_Redirect_HostTextTheParserDeletes(t *testing.T) {
	t.Parallel()

	for _, suffix := range []string{"\u00ad", "\ufeff", "\u200b", "\u2060", "\ufe0f", "\u3002", "\uff0e", "\uff61"} {
		t.Run(strconv.QuoteToASCII(suffix), func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, "/r/*", "https://$1"+suffix, "/r/evil.com", "")
		})
	}
}

// Test_Redirect_InternationalizedHostsStillPin is the other half: mapping must not cost a rule whose host is non-ASCII.
func Test_Redirect_InternationalizedHostsStillPin(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ target, want string }{
		{"https://$1.\u4f8b\u3048.jp/", "https://t.\u4f8b\u3048.jp/"},
		{"https://$1.xn--r8jz45g.jp/", "https://t.xn--r8jz45g.jp/"},
		{"https://$1.example.com/", "https://t.example.com/"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, "/r/*", tc.target, "/r/t", tc.want)
		})
	}
}

// Test_Redirect_ControlCharacterPinsNoHost covers the leading or trailing run of controls and spaces a client removes.
func Test_Redirect_ControlCharacterPinsNoHost(t *testing.T) {
	t.Parallel()

	for _, sep := range []string{"\x00", "\x01", "\x1f", "\x7f"} {
		t.Run(strconv.QuoteToASCII(sep), func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, "/r/*x*", "https://$1"+sep+"$2", "/r/evil.comx", "")
		})
	}
}

// Test_Redirect_PercentEscapePinsOnlyWhatItDecodesTo covers the escaped spelling: "%2E" is a ".", "%C2%AD" a soft hyphen.
func Test_Redirect_PercentEscapePinsOnlyWhatItDecodesTo(t *testing.T) {
	t.Parallel()

	for _, suffix := range []string{"%2E", "%2e", "%C2%AD", "%E3%80%82", "%EF%BB%BF"} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, "/r/*", "https://$1"+suffix, "/r/evil.com", "")
		})
	}

	// An escape that decodes to real host text still pins one.
	requireRule(t, false, "/r/*", "https://$1%41.example.com", "/r/t", "https://t%41.example.com")
}

// Test_Redirect_NumericSuffixPinsNoHost covers the IPv4 reading, where "https://$1.1" composed loopback from "127.0.0".
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

			requireRule(t, false, "/r/*", tc.target, tc.request, "")
		})
	}
}

// Test_Redirect_CompleteAddressStillPins is the other half: an address written in full is the author's.
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

			requireRule(t, false, "/r/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_DotlessTailPinsNoHost covers a literal with no dot, only the tail of a label the capture opens.
func Test_Redirect_DotlessTailPinsNoHost(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ rule, target, request string }{
		{"/r/*", "https://$1x", "/r/127.0.0"},
		{"/r/*", "https://$1x", "/r/10.0.0"},
		{"/r/*", "https://$1cafe", "/r/0x"},
		{"/r/*", "https://$1beef", "/r/0x"},
		// The head can simply end with a dot, leaving the author's text a bare final label.
		{"/r/*", "https://$1xyz", "/r/evil."},
		{"/r/*", "https://$1com", "/r/evil."},
		{"/r/*", "https://$1io", "/r/evil."},
		// The literal sits between two captures, the one place a literal is open on the left without starting the authority.
		{"/r/*/*", "https://$1xyz:$2", "/r/evil./8080"},
		{"/r/*/*", "https://$1cafe:$2", "/r/0x/8080"},
		// Its only dot is trailing, and a trailing dot separates nothing from what precedes it.
		{"/r/*", "https://$1xyz.", "/r/evil."},
		{"/r/*", "https://$1com.", "/r/evil."},
		// The address rule only sees these once the port is cut off and the last label taken after the trim.
		{"/r/*", "https://$110.0.0.1:8080", "/r/0"},
		{"/r/*", "https://$110.0.0.1.", "/r/0"},
	} {
		t.Run(tc.target+" "+tc.request, func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, tc.rule, tc.target, tc.request, "")
		})
	}
}

// Test_Redirect_ClosedLabelStillPins is the other half: an "@", an own dot or a non-hex byte keeps the capture out.
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

			requireRule(t, false, "/r/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_CarriesQueryOntoTargetsOwnQuery pins how the request's query joins a target's own query or fragment.
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

			requireRule(t, false, "/old", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_NoSlashSpecialSchemeAuthorityIsGuarded holds "https:host" to the same rules as "https://host".
func Test_Redirect_NoSlashSpecialSchemeAuthorityIsGuarded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		path   string
		want   string
	}{
		{"an @ makes the author's host userinfo", "https:cdn.example.com$1", "/r/@evil.com", ""},
		{"a leading dot extends the host into another domain", "https:cdn.example.com$1", "/r/.evil.com", ""},
		{"ws is special too", "ws:cdn.example.com$1", "/r/@evil.com", ""},
		{"ftp is special too", "ftp:cdn.example.com$1", "/r/@evil.com", ""},
		// The scheme is case-insensitive, so the reading does not change.
		{"the scheme's own case does not matter", "HTTPS:cdn.example.com$1", "/r/@evil.com", ""},
		// A label under the author's host is what the rule is for.
		{"an interior label still redirects", "https:$1.example.com", "/r/tenant", "https:tenant.example.com"},
		{"a capture between two literals still redirects", "https:cdn.$1.com", "/r/example", "https:cdn.example.com"},
		// The "/" ends the authority, so the capture is a path segment and an "@" in it reaches no host.
		{"past the first slash the capture is a path", "https:cdn.example.com/$1", "/r/@evil.com", "https:cdn.example.com/@evil.com"},
		// mailto is not special: with no "//" it has no authority, so the capture is an addressee.
		{"a scheme that is not special has no implied authority", "mailto:$1@example.com", "/r/user", "mailto:user@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, false, "/r/*", tc.target, tc.path, tc.want)
		})
	}
}

// Test_Redirect_RuleOrderIsBySpecificity pins the order overlapping rules are tried in.
func Test_Redirect_RuleOrderIsBySpecificity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rules map[string]string
		path  string
		want  string
	}{
		{
			// Same prefix, so only what each pins past the wildcard separates them.
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
			// The prefix comparison still decides first: an anchored prefix is the stronger claim.
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
			// The two pin the same prefix and total, and a wildcard matches a run of any length, so the class is narrower.
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
			// Two rules that both carry a wildcard are still told apart by the width of everything beside it.
			name:  "a class past a wildcard is read like any other",
			rules: map[string]string{`/p/*[a-z]`: "/wide", `/p/*[ab]`: "/narrow"},
			path:  "/p/za",
			want:  "/narrow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireWin(t, tc.rules, tc.path, tc.want)
		})
	}
}

// Test_LiteralLengths pins how a rule's pinned length is measured, which is what orders two rules that overlap.
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
		// The prefix stops at the first wildcard; the total counts what follows it too.
		{rule: "/cdn/*", prefixLen: 5, totalLen: 5},
		{rule: "/cdn/*x", prefixLen: 5, totalLen: 6},
		{rule: "/p/*.png", prefixLen: 3, totalLen: 6},
		{rule: "*", prefixLen: 0, totalLen: 0},
		{rule: "/a/*/b/*", prefixLen: 3, totalLen: 6},

		// A key is compiled as a regexp, so its metacharacters pin nothing: "/api/[a-z]+" outranked "/api/users".
		{rule: "/api/[a-z]+", prefixLen: 5, totalLen: 5},
		{rule: "/api/users", prefixLen: 10, totalLen: 10},
		{rule: "/(a|b)/x", prefixLen: 1, totalLen: 4},
		// A group is an alternation too, so it pins what its widest branch pins.
		{rule: "/api/[a-z](specific|x)", prefixLen: 5, totalLen: 6},
		// The "?:" of a non-capturing group is syntax, and a group every branch of which pins the same text pins it too.
		{rule: "/p/(?:ab)", prefixLen: 5, totalLen: 5},
		{rule: "/p/(?:a|aa)", prefixLen: 4, totalLen: 4},
		{rule: "/p/(?:ab|ba)", prefixLen: 3, totalLen: 5},
		// A class escape matches any byte of its class, so it pins none of them and ends the prefix.
		{rule: `/api/\d+`, prefixLen: 5, totalLen: 5},
		{rule: `/api/\w`, prefixLen: 5, totalLen: 5},
		// A quantifier's bounds are syntax, and one allowing none takes its atom back: "/api/a{0,1}" matches "/api/" too.
		{rule: "/api/a{0,1}", prefixLen: 5, totalLen: 5},
		{rule: "/api/a{2,3}", prefixLen: 6, totalLen: 6},
		{rule: "/api/a?b", prefixLen: 5, totalLen: 6},
		{rule: "/api/(ab)?c", prefixLen: 5, totalLen: 6},
		// An unclosed brace is an ordinary byte to the regexp parser.
		{rule: "/api/a{", prefixLen: 6, totalLen: 6},
		// "." matches any byte, so "/api/user." pins no more than "/api/user".
		{rule: "/api/user.", prefixLen: 9, totalLen: 9},
		// Escaped, it matches itself and pins the byte it stands for; the backslash adds nothing.
		{rule: `/p/a\.png`, prefixLen: 8, totalLen: 8},
		// A complete escape names one character however it is spelled: "/p/\x{61}" is "/p/a".
		{rule: `/p/\x{61}`, prefixLen: 4, totalLen: 4},
		{rule: `/p/\x61`, prefixLen: 4, totalLen: 4},
		{rule: `/p/\141`, prefixLen: 4, totalLen: 4},
		{rule: `/p/\t`, prefixLen: 4, totalLen: 4},
		// An incomplete one names nothing, so it ends the prefix — and such a rule never compiles anyway.
		{rule: `/p/\x{61`, prefixLen: 3, totalLen: 5},
		// An anchor asserts a position and consumes nothing, so it pins no path.
		{rule: "/p/a$", prefixLen: 4, totalLen: 4},
		{rule: "/p/[a-z]$", prefixLen: 3, totalLen: 3},
		// An anchor asserts a position and consumes nothing, so it neither pins a byte nor ends what follows.
		{rule: "^/p/a", prefixLen: 4, totalLen: 4},
		{rule: `\A/p/a`, prefixLen: 4, totalLen: 4},
		// A class matches one byte whatever it lists, so listing more alternatives buys no specificity.
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

// Test_Redirect_ComposedPort covers one port built from several captures, each asked for digits as the composition would be.
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

			requireRule(t, false, "/r/*/*", "https://example.com:$1$2", tc.request, tc.want)
		})
	}
}

// Test_Redirect_ExactRuleBeatsPattern covers ordering between a rule written in regexp syntax and an exact one.
func Test_Redirect_ExactRuleBeatsPattern(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/api/[a-z]+": "/broad",
		"/api/users":  "/exact",
	}
	for _, tc := range []struct{ request, want string }{
		{"/api/users", "/exact"},
		{"/api/other", "/broad"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_ExactRuleBeatsDottedPattern is the same for the metacharacter that looks like path text: ".".
func Test_Redirect_ExactRuleBeatsDottedPattern(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/api/user.": "/broad",
		"/api/users": "/exact",
	}
	for _, tc := range []struct{ request, want string }{
		{"/api/users", "/exact"},
		{"/api/userx", "/broad"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_NonSpecialSchemeAuthority covers a custom deep link, where "\" is an ordinary authority byte.
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

			requireRule(t, true, "/p/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_ThirdSlashUnderNonSpecialScheme covers an authority the author left empty with a third slash.
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

		// A special scheme skips them, so the capture is the host and the rule still hands the destination away.
		{"special scheme still skips them", "https:///$1", "/p/evil.com", ""},
		{"two slashes still open an authority", "myapp://$1", "/p/evil.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireRule(t, true, "/p/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_WiderClassDoesNotOutrank covers two rules differing only in how many alternatives their class lists.
func Test_Redirect_WiderClassDoesNotOutrank(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/api/[abcdefghijklmnopqrstuvwxyz]": "/broad",
		"/api/[ab]":                         "/narrow",
	}
	for _, tc := range []struct{ request, want string }{
		// Both match, and they tie on pinned length, so the deterministic key order decides.
		{"/api/a", "/narrow"},
		// Only the wider class matches this one.
		{"/api/z", "/broad"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_AlternationRankedByItsWidestBranch covers a rule only as specific as its least specific branch.
func Test_Redirect_AlternationRankedByItsWidestBranch(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/very/specific|/x": "/alt",
		"/x":                "/exact",
	}
	for _, tc := range []struct{ request, want string }{
		// Both match, and the exact rule wins: they pin the same byte and the alternation matches strictly more paths.
		{"/x", "/exact"},
		// The branch only the alternation matches still reaches it.
		{"/very/specific", "/alt"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_GroupedAlternationRankedByItsWidestBranch covers an alternation written inside a group.
func Test_Redirect_GroupedAlternationRankedByItsWidestBranch(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/p/[a-z](reports|x.*)": "/grouped",
		"/p/[a-z]xy":            "/narrow",
	}
	for _, tc := range []struct{ request, want string }{
		// Both match "/p/axy", and the grouped rule pins only the byte its "x.*" branch does.
		{"/p/axy", "/narrow"},
		// What only the grouped rule matches still reaches it.
		{"/p/areports", "/grouped"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_OptionalQuantifierDoesNotOutrankExactRule covers "{0,1}", whose bounds were counted as path bytes.
func Test_Redirect_OptionalQuantifierDoesNotOutrankExactRule(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/api/ab{0,1}": "/maybe",
		"/api/ab":      "/exact",
	}
	for _, tc := range []struct{ request, want string }{
		{"/api/ab", "/exact"},
		// What only the quantified rule matches still reaches it.
		{"/api/a", "/maybe"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_AnchorPinsNoPath covers an explicit "$", which asserts a position and consumes nothing.
func Test_Redirect_AnchorPinsNoPath(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/p/[a-z]$": "/class",
		"/p/[a]":    "/exact",
	}
	for _, tc := range []struct{ request, want string }{
		{"/p/a", "/exact"},
		{"/p/b", "/class"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_UserinfoDelimitersFollowTheScheme covers a capture in the userinfo of a non-special target.
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

			requireRule(t, true, "/r/*", tc.target, "/r/user%5Cname", tc.want)
		})
	}
}

// Test_Redirect_FileAuthorityOpensOnBackslashes covers "file:/$1", where a captured "\evil.com/share" names a host.
func Test_Redirect_FileAuthorityOpensOnBackslashes(t *testing.T) {
	t.Parallel()

	requireRule(t, true, "/r/*", "file:/$1", "/r/%5Cevil.com/share", "")

	// A value that stays a path still redirects, as does a third slash, which closes the authority empty like "file:///tmp".
	for _, tc := range []struct{ request, want string }{
		{"/r/tmp/report", "file:/tmp/report"},
		{"/r/%5C%5Cevil.com/share", `file:/\\evil.com/share`},
	} {
		requireRule(t, true, "/r/*", "file:/$1", tc.request, tc.want)
	}

	// The span itself, which is what the runtime guard asks of the composition.
	for _, target := range []string{`file:/\evil.com/share`, `file:\\evil.com/share`, `file:\/evil.com`, "file://evil.com"} {
		start, end := authoritySpan(target)
		require.Equal(t, "evil.com", target[start:end], target)
	}
	for _, target := range []string{`file:/\\evil.com`, "file:///tmp", `file:\\\evil.com`} {
		start, end := authoritySpan(target)
		require.Equal(t, start, end, target)
	}
}

// Test_Redirect_AnchorsBindEveryBranch covers a top-level "|", where concatenated anchors made "^/a|/b$".
func Test_Redirect_AnchorsBindEveryBranch(t *testing.T) {
	t.Parallel()

	app := testApp(map[string]string{"/a|/b": "/moved"}, false)
	for _, tc := range []struct {
		request string
		want    int
	}{
		{"/a", fiber.StatusFound},
		{"/b", fiber.StatusFound},
		{"/a-extra", fiber.StatusOK},
		{"/extra/b", fiber.StatusOK},
	} {
		status, _ := get(t, app, tc.request)
		require.Equal(t, tc.want, status, tc.request)
	}
}

// Test_Redirect_NestedAlternationLosesTheTieBreak covers two rules that tie on both specificity measures.
func Test_Redirect_NestedAlternationLosesTheTieBreak(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		"/p/[a-z](x|y)": "/wide",
		"/p/[a-z]x":     "/narrow",
	}
	for _, tc := range []struct{ request, want string }{
		{"/p/ax", "/narrow"},
		{"/p/ay", "/wide"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}

	// A class counts its breadth here too, so the two differ by the group alone.
	require.Equal(t, 52, patternWidth("/p/[a-z](x|y)"))
	require.Equal(t, 26, patternWidth("/p/[a-z]x"))
	require.Equal(t, 1, patternWidth("/p/[a]"))
	require.Equal(t, 256, patternWidth("/p/."))
	require.Equal(t, 2, patternWidth("/very/specific|/x"))
	require.Equal(t, 4, patternWidth("(a|b)(c|d)"))

	// The wildcard is ranked on its own rather than counted here, so the width goes on separating two rules that carry one.
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

// Test_Redirect_FewerWildcardsWin ties every other measure, leaving the wildcard count to separate the pair.
func Test_Redirect_FewerWildcardsWin(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		"/p/*a*b": "https://attacker.example/$1/$2",
		"/p/*ab":  "/safe/$1",
	}, "/p/xxab?secret=top", "/safe/xx?secret=top")
	require.Equal(t, 2, wildcardRank("/p/*a*b"))
	require.Equal(t, 1, wildcardRank("/p/*ab"))
	// Both expand to a single alternative, so the width leaves them tied and the count decides.
	require.Equal(t, patternWidth("/p/*a*b"), patternWidth("/p/*ab"))
}

// Test_Redirect_WildcardCountYieldsToWidth covers the pair the count alone gets backwards.
func Test_Redirect_WildcardCountYieldsToWidth(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/([a]*|[c]*)`: "/narrow",
		`/p/[a-d]*`:      "/broad",
	}, "/p/axyz", "/narrow")

	// Every earlier measure ties, and the wildcard count points the wrong way.
	require.Equal(t, literalPrefixLen(`/p/[a-d]*`), literalPrefixLen(`/p/([a]*|[c]*)`))
	require.Equal(t, literalLen(`/p/[a-d]*`), literalLen(`/p/([a]*|[c]*)`))
	require.Equal(t, carriesRun(`/p/[a-d]*`), carriesRun(`/p/([a]*|[c]*)`))
	require.Greater(t, wildcardRank(`/p/([a]*|[c]*)`), wildcardRank(`/p/[a-d]*`))
	require.Less(t, patternWidth(`/p/([a]*|[c]*)`), patternWidth(`/p/[a-d]*`))
}

// Test_Redirect_OptionalAtomWidensARule covers "/p/*a?", which one wildcard makes look narrower than "/p/*[a]*".
func Test_Redirect_OptionalAtomWidensARule(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/*[a]*`: "/narrow",
		`/p/*a?`:   "/broad",
	}, "/p/a", "/narrow")

	// The optional atom is the one measure that separates them, and "{0,1}" spells the same thing.
	require.Greater(t, wildcardRank(`/p/*[a]*`), wildcardRank(`/p/*a?`))
	require.Less(t, patternWidth(`/p/*[a]*`), patternWidth(`/p/*a?`))
	require.Equal(t, 2, patternWidth("/p/a?"))
	require.Equal(t, 2, patternWidth("/p/a{0,1}"))
	require.Equal(t, 27, patternWidth("/p/[a-z]?"))
	// An escape names an atom too, and it is the one the "?" takes back.
	require.Equal(t, 2, patternWidth(`/p/\.?`))
}

// Test_Redirect_UnboundedRepetitionIsWidest covers the quantifier that runs on: "/p/[ab]+" matches every length.
func Test_Redirect_UnboundedRepetitionIsWidest(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[a][ab]?`: "/narrow",
		`/p/[ab]+`:    "/broad",
	}, "/p/a", "/narrow")

	// A run without an end is ranked, not measured, so the width separates two rules that both run on.
	require.Equal(t, 1, carriesRun(`/p/[ab]+`))
	require.Equal(t, 1, carriesRun(`/p/[a]{2,}`))
	require.Equal(t, 0, carriesRun(`/p/[a][ab]?`))
	require.Equal(t, 0, carriesRun(`/p/[a]{2,3}`))

	// Every count a bounded quantifier permits is a set of paths of its own, and they add: "{2,3}" matches "aa" and "aaa".
	require.Equal(t, 3, patternWidth(`/p/[a][ab]?`))
	require.Equal(t, 6, patternWidth(`/p/[ab]{1,2}`))
	require.Equal(t, 2, patternWidth(`/p/[a]{2,3}`))
	require.Equal(t, 1, patternWidth(`/p/[a]{2}`))
	// Braces spelling no number are literal text, and a range with no upper bound or a backwards one bounds nothing.
	require.Equal(t, 1, patternWidth(`/p/{id}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{1,x}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{3,1}`))
}

// Test_Redirect_FiniteRepetitionIsCounted covers the bounded quantifier: "/p/[ab]{1,2}" matches six paths.
func Test_Redirect_FiniteRepetitionIsCounted(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[a][ab]?`:  "/narrow",
		`/p/[ab]{1,2}`: "/broad",
	}, "/p/a", "/narrow")
}

// Test_Redirect_UnboundedRulesKeepTheirBreadth covers two rules that both run on: "/p/[z]+" and "/p/[a-z]+".
func Test_Redirect_UnboundedRulesKeepTheirBreadth(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[z]+`:   "/narrow",
		`/p/[a-z]+`: "/broad",
	}, "/p/z", "/narrow")
}

// Test_Redirect_EscapedByteIsOneMember covers the class escape spelling one byte: "[\.]" lists the dot alone.
func Test_Redirect_EscapedByteIsOneMember(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[\.]`:  "/narrow",
		`/p/[.-/]`: "/broad",
	}, "/p/.", "/narrow")

	// One byte named, however it is spelled; a set only when it stands for one.
	require.Equal(t, 1, patternWidth(`/p/[\.]`))
	require.Equal(t, 1, patternWidth(`/p/[\x61]`))
	require.Equal(t, 2, patternWidth(`/p/[.-/]`))
	require.Equal(t, setMemberWidth, patternWidth(`/p/[\d]`))
}

// Test_Redirect_SetMemberOutweighsAListedByte covers a member standing for a set: "[[:alpha:]]" contains "[a]".
func Test_Redirect_SetMemberOutweighsAListedByte(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[a]`:         "/narrow",
		`/p/[[:alpha:]]`: "/broad",
	}, "/p/a", "/narrow")

	// However the set is spelled, and whatever it holds.
	require.Equal(t, setMemberWidth, patternWidth(`/p/[[:alpha:]]`))
	require.Equal(t, setMemberWidth, patternWidth(`/p/[[:digit:]]`))
}

// Test_Redirect_SaturatedWidthSurvivesAQuantifier covers a width that reached the clamp before its last atom.
func Test_Redirect_SaturatedWidthSurvivesAQuantifier(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[ab][a-z][a-z][a-z][a-z]`:         "/narrow",
		`/p/[a-z][a-z][a-z][a-z][a-z][ab]{0}`: "/broad",
	}, "/p/aaaaa", "/narrow")

	// The clamp holds through the quantifier rather than being divided back.
	require.Equal(t, maxPatternWidth, patternWidth(`/p/[a-z][a-z][a-z][a-z][a-z][ab]{0}`))
	require.Less(t, patternWidth(`/p/[ab][a-z][a-z][a-z][a-z]`), maxPatternWidth)
	// What a quantifier permits is still counted where nothing saturated.
	require.Equal(t, 1, patternWidth(`/p/[ab]{0}`))
	require.Equal(t, 4, patternWidth(`/p/[ab]{2}`))
}

// Test_Redirect_SignedBoundIsNoQuantifier covers braces Go reads as literal text: "a{-0}" repeats nothing.
func Test_Redirect_SignedBoundIsNoQuantifier(t *testing.T) {
	t.Parallel()

	require.Equal(t, literalPrefixLen(`/p/a....`), literalPrefixLen(`/p/a{-0}`))
	require.Less(t, patternWidth(`/p/a{-0}`), patternWidth(`/p/a....`))

	// Text pins what it spells: the bytes between the braces are path, not syntax.
	require.Equal(t, literalLen(`/p/a-0`), literalLen(`/p/a{-0}`))
	require.Greater(t, literalLen(`/p/a{-0}`), literalLen(`/p/a....`))

	// Digits alone name a count; a sign, a space or a stray byte do not.
	require.Equal(t, 2, patternWidth(`/p/[ab]{1}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{+1}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{ 1}`))
	// A missing lower bound names no count either: Go reads "{,3}" as text.
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{,3}`))
	// Nor does a count too large to hold, which Go refuses for its size anyway.
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{99999999999999999999}`))
}

// Test_Redirect_NonGreedyMarkerIsNoQuantifier covers the "?" that only says a repetition is not greedy.
func Test_Redirect_NonGreedyMarkerIsNoQuantifier(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[b]+?`: "/narrow",
		`/p/[ab]+`: "/broad",
	}, "/p/b", "/narrow")

	// The marker leaves the width where the repetition left it — but a rule's "*" expands to a group the "?" does quantify.
	require.Equal(t, patternWidth(`/p/[b]+`), patternWidth(`/p/[b]+?`))
	require.Equal(t, patternWidth(`/p/[ab]{1,2}`), patternWidth(`/p/[ab]{1,2}?`))
	require.Equal(t, patternWidth(`/p/[ab]?`), patternWidth(`/p/[ab]??`))
	// A "?" of its own still counts the alternative where the atom is absent.
	require.Equal(t, 3, patternWidth(`/p/[ab]?`))
}

// Test_Redirect_OversizedBoundIsNoQuantifier covers the count Go refuses for its size, measured before compiling.
func Test_Redirect_OversizedBoundIsNoQuantifier(t *testing.T) {
	t.Parallel()

	// Counted up to Go's own limit, and read as text above it.
	require.Equal(t, maxPatternWidth, patternWidth(`/p/[ab]{`+strconv.Itoa(maxRepeatCount)+`}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{`+strconv.Itoa(maxRepeatCount+1)+`}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{1000000000}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]{1,1000000000}`))

	// Which is what Go says of them too: the rule above the limit is refused.
	_, err := regexp.Compile(`^(?:/p/[ab]{` + strconv.Itoa(maxRepeatCount) + `})$`)
	require.NoError(t, err)
	_, err = regexp.Compile(`^(?:/p/[ab]{` + strconv.Itoa(maxRepeatCount+1) + `})$`)
	require.Error(t, err)
}

// Test_Redirect_PosixClassNamesItsOwnStars covers "[[:alpha:]*]", whose "]" closes the POSIX name rather than the class.
func Test_Redirect_PosixClassNamesItsOwnStars(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/*[[:alpha:]*][[:alpha:]*]`: "/narrow",
		`/p/*[\pL(.*)A-z]`:             "/broad",
	}, "/p/aa", "/narrow")

	// One wildcard, and a class counted whole however its members are spelled.
	require.Equal(t, 1, wildcardRank(`/p/*[[:alpha:]*][[:alpha:]*]`))
	require.Equal(t, 0, wildcardRank(`/p/[[:alpha:]*]`))
	// The name counts as a set, the star beside it as the one byte it lists.
	require.Equal(t, setMemberWidth+1, patternWidth(`/p/[[:alpha:]*]`))
	// A member standing after the name pins nothing, being a member still.
	require.Equal(t, 3, literalLen(`/p/[[:alpha:]a]`))
	// A name is one only inside a class: "[:alpha:]" alone is a class, so the star after it is Fiber's wildcard.
	require.Equal(t, 1, wildcardRank(`/p/[:alpha:]*`))
	// A name that never closes is no name, and the star it lists is a member like any other.
	require.Equal(t, 0, wildcardRank(`/p/[[:alpha*]`))
}

// Test_Redirect_UnterminatedQuoteRuleIsRejected pins an unclosed "\Q": it quotes the rest, so the key never compiles.
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

// Test_Redirect_HexEscapeOutranksAClass covers "\x{61}", which names the one character "a" rather than a class.
func Test_Redirect_HexEscapeOutranksAClass(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		`/p/\x{61}`: "/exact",
		"/p/[a-z]":  "/class",
	}
	for _, tc := range []struct{ request, want string }{
		{"/p/a", "/exact"},
		{"/p/b", "/class"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_CaptureEndsAuthorityPastADeletedByte covers a captured "\t/ok", whose tab the parser deletes.
func Test_Redirect_CaptureEndsAuthorityPastADeletedByte(t *testing.T) {
	t.Parallel()

	app := testApp(map[string]string{"/r/*": "https://example.com$1"}, true)
	status, location := get(t, app, "/r/%09%2Fok")
	require.Equal(t, fiber.StatusFound, status)

	// What the client reads is example.com, whatever bytes the parser drops on the way there.
	ref, err := url.Parse(strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(location))
	require.NoError(t, err)
	require.Equal(t, "example.com", ref.Host)
	require.Equal(t, "/ok", ref.Path)

	// A value that really does extend the host is still refused.
	status, _ = get(t, app, "/r/%09evil")
	require.Equal(t, fiber.StatusOK, status)
}

// Test_Redirect_BareFileSchemeIsAPath covers a "file:" target with no slashes, which names no authority.
func Test_Redirect_BareFileSchemeIsAPath(t *testing.T) {
	t.Parallel()

	requireRule(t, false, "/r/*", "file:tmp$1", "/r/report", "file:tmpreport")

	// A bare "file:" target holds no authority span, so it is opaque-path and the composed location is what gets checked.
	start, end := authoritySpan("file:tmp/report")
	require.Equal(t, 0, start)
	require.Equal(t, 0, end)
	start, end = authoritySpan("file://host/report")
	require.NotEqual(t, start, end)
}

// Test_Redirect_AuthorPinnedShortIPv4 covers "127.1", which the URL parser reads as 127.0.0.1 where net.ParseIP does not.
func Test_Redirect_AuthorPinnedShortIPv4(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"127.1", "0x7f.1", "2130706433", "127.0.0.1"} {
		requireRule(t, false, "/r/*", "https://"+host+":$1", "/r/8080", "https://"+host+":8080")
	}

	// A host that is not an address still hands the host away when the capture closes it, so "https://$1" stays refused.
	require.False(t, isIPv4Host("127.0.0.256"))
	require.False(t, isIPv4Host("1.2.3.4.5"))
	require.False(t, isIPv4Host("0x1g"))
	require.True(t, isIPv4Host("127.0.0.1."))
}

// Test_Redirect_ClassEscapeDoesNotOutrankExactRule covers "\d", which matches any digit rather than the byte "d" escaped.
func Test_Redirect_ClassEscapeDoesNotOutrankExactRule(t *testing.T) {
	t.Parallel()

	rules := map[string]string{
		`/api/\d+`: "/digits",
		"/api/1":   "/one",
	}
	for _, tc := range []struct{ request, want string }{
		{"/api/1", "/one"},
		{"/api/27", "/digits"},
	} {
		requireWin(t, rules, tc.request, tc.want)
	}
}

// Test_Redirect_FileSchemeEmptyAuthority covers "file", whose "file:///$1" is the empty authority of a local path.
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

			requireRule(t, true, "/p/*", tc.target, tc.request, tc.want)
		})
	}
}

// Test_Redirect_LiteralBracesHideNoRun covers "{x*}", whose braces bound no repetition and hide no run.
func Test_Redirect_LiteralBracesHideNoRun(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(a|{x[c]})`: "/narrow",
		`/p/(a|{x*})`:   "/broad",
	}, "/p/a", "/narrow")

	// The run is seen wherever it stands, and a "+" between the braces is one too.
	require.Equal(t, 1, wildcardRank(`/p/(a|{x*})`))
	require.Equal(t, 2, carriesRun(`/p/(a|{x*})`))
	require.Equal(t, 1, carriesRun(`/p/(a|{x+})`))
	require.Equal(t, 0, carriesRun(`/p/(a|{x[c]})`))

	// A bound the braces do spell still names a repetition rather than text.
	require.Equal(t, 1, carriesRun(`/p/[ab]x{2,}`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]x{2}`))
}

// Test_Redirect_LiteralBracesAreMeasured covers the same braces in the width and the literal length.
func Test_Redirect_LiteralBracesAreMeasured(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(a|{x[c]})`:   "/narrow",
		`/p/(a|{x[a-z]})`: "/broad",
	}, "/p/a", "/narrow")

	require.Equal(t, 26, patternWidth(`/p/{x[a-z]}`))
	require.Equal(t, 1, patternWidth(`/p/{x[c]}`))

	// And what they pin is pinned: the "x" between them is a byte of the path.
	require.Equal(t, literalLen(`/p/x`), literalLen(`/p/{x[c]}`))

	// Go reads the braces the same way, so the rule compiles and matches them.
	require.True(t, regexp.MustCompile(`^(?:/p/{x[a-z]})$`).MatchString("/p/{xc}"))
}

// Test_Redirect_WidthSaturatesRatherThanWraps covers the product of two widths, which wrapped negative on a 32-bit int.
func Test_Redirect_WidthSaturatesRatherThanWraps(t *testing.T) {
	t.Parallel()

	require.Equal(t, maxPatternWidth, scaledWidth(math.MaxInt, 2))
	require.Equal(t, maxPatternWidth, scaledWidth(maxPatternWidth, maxPatternWidth))
	require.Equal(t, 6, scaledWidth(2, 3))
	require.Equal(t, 0, scaledWidth(0, 3))
	require.Equal(t, 0, scaledWidth(3, 0))

	// The pair that wrapped: a repetition reached on an already saturated width.
	broad := patternWidth(`/p/[a-z][a-z][a-z][a-z][a-z][a-zA-Z]{1,2}`)
	require.Equal(t, maxPatternWidth, broad)
	require.Greater(t, broad, patternWidth(`/p/[a][a-z][a-z][a-z][a-z][a]`))
	require.Equal(t, maxPatternWidth, repeated(maxPatternWidth, 2756, 1, 1))
}

// Test_Redirect_WildcardOutrunsANamedRun covers a run repeating what it names beside one repeating anything.
func Test_Redirect_WildcardOutrunsANamedRun(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(a|aa)+`: "/narrow",
		`/p/*a`:      "/broad",
	}, "/p/a?token=secret", "/narrow?token=secret")

	// Three grades, and the pair the width could not tell apart sits across two.
	require.Equal(t, 0, carriesRun(`/p/[ab]a`))
	require.Equal(t, 1, carriesRun(`/p/(a|aa)+`))
	require.Equal(t, 2, carriesRun(`/p/*a`))
	require.Less(t, patternWidth(`/p/*a`), patternWidth(`/p/(a|aa)+`))

	// A rule carrying both runs is graded by the broader of the two.
	require.Equal(t, 2, carriesRun(`/p/*[ab]+`))
}

// Test_Redirect_PropertyNameIsNotPath covers "\p{Greek}", whose braces name the property rather than bounding a repetition.
func Test_Redirect_PropertyNameIsNotPath(t *testing.T) {
	t.Parallel()

	status, location := get(t, testApp(map[string]string{
		`/p/[\x{3B1}]`: "/narrow",
		`/p/\p{Greek}`: "/broad",
	}, true), "/p/%CE%B1?token=x")
	require.Equal(t, fiber.StatusFound, status)
	require.Equal(t, "/narrow?token=x", location)

	// The property pins nothing, however long its name and however it is spelled.
	require.Equal(t, literalLen(`/p/`), literalLen(`/p/\p{Greek}`))
	require.Equal(t, literalLen(`/p/`), literalLen(`/p/\pL`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/\p{Greek}`))

	// Where "\x{3B1}" names one character, and pins it.
	require.Greater(t, literalLen(`/p/\x{3B1}`), literalLen(`/p/\p{Greek}`))

	// A property naming nothing is the two bytes it spells, closed or not.
	require.Equal(t, literalLen(`/p/`), literalLen(`/p/\p`))
	require.Equal(t, literalLen(`/p/Greek`), literalLen(`/p/\p{Greek`))
}

// Test_Redirect_QuotedTextIsNoQuantifier covers "\Qa?\E", whose "?" is a byte of the path rather than a quantifier.
func Test_Redirect_QuotedTextIsNoQuantifier(t *testing.T) {
	t.Parallel()

	status, location := get(t, testApp(map[string]string{
		`/p/\Qa?\E`:  "/narrow",
		`/p/[a][?x]`: "/broad",
	}, true), "/p/a%3F?token=x")
	require.Equal(t, fiber.StatusFound, status)
	require.Equal(t, "/narrow?token=x", location)

	// Quoted text pins what it spells and spells one path, quantifiers and all.
	require.Equal(t, literalLen(`/p/a\?`), literalLen(`/p/\Qa?\E`))
	require.Equal(t, 1, patternWidth(`/p/\Qa?\E`))
	require.Equal(t, 1, patternWidth(`/p/\Q[ab]{2}\E`))
	require.Equal(t, 0, carriesRun(`/p/\Qa+\E`))

	// And the quote runs to the end of the rule where no "\E" closes it, so New panics on such a key.
	require.Equal(t, literalLen(`/p/ab`), literalLen(`/p/\Qab`))
	require.Panics(t, func() {
		New(Config{Rules: map[string]string{`/p/\Qab`: "/x"}})
	})
}

// Test_Redirect_ClassCountsAMemberOnce covers a class repeating what it lists, counted once per member.
func Test_Redirect_ClassCountsAMemberOnce(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[^\dABC]`:  "/narrow",
		`/p/[^\d\d\d]`: "/broad",
	}, "/p/z?token=x", "/narrow?token=x")

	// A repeated set, byte or range counts once; two overlapping ranges count what they cover between them.
	require.Equal(t, patternWidth(`/p/[\d]`), patternWidth(`/p/[\d\d\d]`))
	require.Equal(t, patternWidth(`/p/[a]`), patternWidth(`/p/[aaa]`))
	require.Equal(t, patternWidth(`/p/[a-d]`), patternWidth(`/p/[a-cb-d]`))
	require.Equal(t, patternWidth(`/p/[[:alpha:]]`), patternWidth(`/p/[[:alpha:][:alpha:]]`))

	// Two spellings of one set are still two, there being no size to compare.
	require.Greater(t, patternWidth(`/p/[\d[:digit:]]`), patternWidth(`/p/[\d]`))

	// First inside the brackets a "]" is a member, and a range running backwards names none: "]" is all "[]b-a]" lists.
	require.Equal(t, patternWidth(`/p/[x]`), patternWidth(`/p/[]]`))
	require.Equal(t, patternWidth(`/p/[]]`), patternWidth(`/p/[]]]`))
	require.Equal(t, patternWidth(`/p/[]]`), patternWidth(`/p/[]b-a]`))
}

// Test_Redirect_EmptyAtomIsNoRun covers a quantifier whose atom matches only the empty string.
func Test_Redirect_EmptyAtomIsNoRun(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(?:)+a`: "/narrow",
		`/p/b?a`:    "/broad",
	}, "/p/a?token=x", "/narrow?token=x")

	// No run, whichever quantifier repeats the empty group, and no width either.
	require.Equal(t, 0, carriesRun(`/p/(?:)+a`))
	require.Equal(t, 0, carriesRun(`/p/(){2,}a`))
	require.Equal(t, 0, carriesRun(`/p/\Q\E+a`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/(?:)?(?:)?a`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/(?:){2,4}a`))

	// A group holding something is a run again, and so is Fiber's "*": it is expanded to "(.*)" whatever stands before it.
	require.Equal(t, 1, carriesRun(`/p/(?:a)+`))
	require.Equal(t, 2, carriesRun(`/p/(?:)*a`))

	// A group's emptiness is its contents', however deeply they nest.
	require.Equal(t, 0, carriesRun(`/p/(?:(?:))+`))
	require.Equal(t, 0, carriesRun(`/p/(?:(?:(?:)))+`))
	require.Equal(t, 0, carriesRun(`/p/(?:\Q\E)+`))

	// A group holding anything at all is filled, one branch of it being enough.
	require.Equal(t, 1, carriesRun(`/p/(?:(?:)a)+`))
	require.Equal(t, 1, carriesRun(`/p/(?:|a)+`))
	require.Equal(t, 1, carriesRun(`/p/(?:(?:a))+`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/(?:)?[ab]`))
}

// Test_Redirect_SetEscapeIsMeasuredWhereverItStands covers "\w" outside a class, which matches a set all the same.
func Test_Redirect_SetEscapeIsMeasuredWhereverItStands(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[ab]{2}`: "/narrow",
		`/p/\w[ab]`:  "/broad",
	}, "/p/aa?code=secret", "/narrow?code=secret")

	// A set escape measures a set wherever it stands, and the same one either way.
	require.Equal(t, patternWidth(`/p/[\w]`), patternWidth(`/p/\w`))
	require.Equal(t, patternWidth(`/p/[\p{Greek}]`), patternWidth(`/p/\p{Greek}`))
	require.Greater(t, patternWidth(`/p/\w`), patternWidth(`/p/\.`))

	// An escape spelling one character is still one, and an assertion matches nothing at all.
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/\.`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/[ab]\b`))
	require.Less(t, patternWidth(`/p/[ab]\b`), patternWidth(`/p/[a-d]`))

	// A backslash ending the rule names nothing, so it is no set either, and no character.
	require.Equal(t, patternWidth(`/p/`), patternWidth(`/p/\`))
	_, spells := escapeByte(`/p/\`, 3, 1)
	require.False(t, spells)
}

// Test_Redirect_RepeatedAtomSeparatesTiedWidths covers the two ways a repetition leaves the width tied.
func Test_Redirect_RepeatedAtomSeparatesTiedWidths(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, narrow, broad, path string
	}{
		{"run", `/p/[b]+a?`, `/p/[ab]+`, "/p/b?token=secret"},
		{"saturated", `/p/[a-z]{5}`, `/p/[a-zA-Z]{5}`, "/p/aaaaa?token=secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireWin(t, map[string]string{tc.narrow: "/narrow", tc.broad: "/broad"}, tc.path, "/narrow?token=secret")

			// Tied on the width, and separated by the widest position each matches.
			require.Equal(t, patternWidth(tc.broad), patternWidth(tc.narrow))
			require.Less(t, widestAtom(tc.narrow), widestAtom(tc.broad))
		})
	}

	// Every rule has an answer, repeating or not, and Fiber's "*" matches any byte.
	require.Equal(t, 2, widestAtom(`/p/[ab]`))
	require.Equal(t, 2, widestAtom(`/p/[ab]?`))
	require.Equal(t, 26, widestAtom(`/p/[a-z]+`))
	require.Equal(t, 256, widestAtom(`/p/*`))
	require.Equal(t, 1, widestAtom(`/p/abc`))

	// It is read after the width, so it never moves a pair the width separates.
	require.Equal(t, 1, patternWidth(`/p/[z]+`))
	require.Equal(t, 26, patternWidth(`/p/[a-z]+`))
}

// Test_Redirect_NestedEmptyGroupIsStillEmpty covers a group whose emptiness is its contents': "(?:(?:))".
func Test_Redirect_NestedEmptyGroupIsStillEmpty(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(?:(?:))+a`: "/narrow",
		`/p/b?a`:        "/broad",
	}, "/p/a?token=x", "/narrow?token=x")

	// The width reads the nesting the same way, so a quantifier on one is none.
	require.Equal(t, patternWidth(`/p/[a]`), patternWidth(`/p/(?:(?:))?(?:(?:))?[a]`))
	require.Equal(t, patternWidth(`/p/[a]`), patternWidth(`/p/(?:(?:(?:)))?[a]`))
	require.Less(t, patternWidth(`/p/(?:(?:))?[a]`), patternWidth(`/p/[ab]`))

	// And a group holding something still widens what may be absent from it.
	require.Greater(t, patternWidth(`/p/(?:(?:a))?[a]`), patternWidth(`/p/[a]`))
}

// Test_Redirect_EscapedClassMemberIsDecoded covers members spelled as escapes: Go reads "[\x{61}-\x{7a}]" as "[a-z]".
func Test_Redirect_EscapedClassMemberIsDecoded(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/[ab]`:            "/narrow",
		`/p/[\x{61}-\x{7a}]`: "/broad",
	}, "/p/a?token=x", "/narrow?token=x")

	// A range spelled in escapes covers what the same range spelled plainly does, in every spelling Go reads as one character.
	require.Equal(t, patternWidth(`/p/[a-z]`), patternWidth(`/p/[\x{61}-\x{7a}]`))
	require.Equal(t, patternWidth(`/p/[a-z]`), patternWidth(`/p/[\x61-\x7a]`))
	require.Equal(t, patternWidth(`/p/[A-Z]`), patternWidth(`/p/[\101-\132]`))
	require.Equal(t, patternWidth(`/p/[a-z]`), patternWidth(`/p/[a-\x7a]`))

	// And one member spelled twice is still one member, however it is spelled.
	require.Equal(t, patternWidth(`/p/[a]`), patternWidth(`/p/[a\x{61}]`))
	require.Equal(t, patternWidth(`/p/[\n]`), patternWidth(`/p/[\x0a]`))

	// Every control character written by name is the byte it names.
	for _, tc := range []struct {
		named, hex string
	}{
		{`\a`, `\x07`},
		{`\f`, `\x0c`},
		{`\n`, `\x0a`},
		{`\r`, `\x0d`},
		{`\t`, `\x09`},
		{`\v`, `\x0b`},
	} {
		require.Equal(t, patternWidth(`/p/[`+tc.hex+`]`), patternWidth(`/p/[`+tc.named+tc.hex+`]`), tc.named)
	}

	// A character of more than one byte is no endpoint, and counts as the one member it is.
	require.Equal(t, patternWidth(`/p/[a]`), patternWidth(`/p/[\x{3B1}]`))
	require.Less(t, patternWidth(`/p/[\x{3B1}]`), patternWidth(`/p/[\d]`))
}

// Test_Redirect_KeyOrderIsTotal covers the comparator itself, which left three rules in a cycle.
func Test_Redirect_KeyOrderIsTotal(t *testing.T) {
	t.Parallel()

	// The cycle: all three tie down to the width, which saturates for each.
	rules := []string{
		`/p/[a-z]{5}`,
		`/p/[a-zA-Z]{5}`,
		`/p/[a-zB-Z][b-z][b-z][b-z][b-z]`,
	}
	for _, rule := range rules {
		require.Equal(t, maxPatternWidth, patternWidth(rule), rule)
		require.Equal(t, 0, carriesRun(rule), rule)
	}

	// Every ordering the three can be given sorts them the same way, so the winner is named the same however they go in.
	targets := map[string]string{rules[0]: "/narrow", rules[1]: "/broad", rules[2]: "/other"}
	want := ""
	for _, order := range [][]string{
		{rules[0], rules[1], rules[2]},
		{rules[0], rules[2], rules[1]},
		{rules[1], rules[0], rules[2]},
		{rules[1], rules[2], rules[0]},
		{rules[2], rules[0], rules[1]},
		{rules[2], rules[1], rules[0]},
	} {
		app := testApp(map[string]string{
			order[0]: targets[order[0]],
			order[1]: targets[order[1]],
			order[2]: targets[order[2]],
		}, false)
		status, got := get(t, app, "/p/aaaaa?token=x")
		require.Equal(t, fiber.StatusFound, status)
		if want == "" {
			want = got
		}
		require.Equal(t, want, got, order)
	}

	// And the narrowest of the three wins: "/p/[a-z]{5}" is a strict subset of "/p/[a-zA-Z]{5}".
	require.Equal(t, "/narrow?token=x", want)
	require.Less(t, widestAtom(rules[0]), widestAtom(rules[2]))
	require.Less(t, widestAtom(rules[2]), widestAtom(rules[1]))
}

// Test_Redirect_ZeroWidthConstructIsEmpty covers what a group can hold and still match nothing.
func Test_Redirect_ZeroWidthConstructIsEmpty(t *testing.T) {
	t.Parallel()

	for _, narrow := range []string{`/p/(?:\b)+a`, `/p/(?:a{0})+a`} {
		t.Run(narrow, func(t *testing.T) {
			t.Parallel()

			requireWin(t, map[string]string{narrow: "/narrow", `/p/b?a`: "/broad"}, "/p/a?token=x", "/narrow?token=x")

			require.Equal(t, 0, carriesRun(narrow))
		})
	}

	// An anchor asserts a position and consumes nothing, so it is empty too.
	require.Equal(t, 0, carriesRun(`/p/(?:^)+a`))
	require.Equal(t, 0, carriesRun(`/p/(?:$)+a`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/a$`))

	// An assertion pins nothing and widens nothing, wherever it stands.
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/a\b`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/a\b?`))
	require.Equal(t, literalLen(`/p/a`), literalLen(`/p/a\b`))

	// A count of none takes the atom away, and a count that permits one does not.
	require.Equal(t, 0, carriesRun(`/p/(?:[a-z]{0})+`))
	require.Equal(t, 1, carriesRun(`/p/(?:[a-z]{0,2})+`))
	require.Equal(t, 1, carriesRun(`/p/(?:[a-z]{0,})+`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/a[b-z]{0}`))
}

// Test_Redirect_GroupIsNoOnePosition covers a group's width, which is the product of the positions inside it.
func Test_Redirect_GroupIsNoOnePosition(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(?:[a-m][a-z][a-z][a-z][a-z])`: "/narrow",
		`/p/[a-z][a-z][a-z][a-z][a-z]`:     "/broad",
	}, "/p/aaaaa?token=x", "/narrow?token=x")

	// Grouping changes no position, however deeply the groups nest.
	require.Equal(t, 26, widestAtom(`/p/(?:[a-m][a-z][a-z][a-z][a-z])`))
	require.Equal(t, 26, widestAtom(`/p/[a-z][a-z][a-z][a-z][a-z]`))
	require.Equal(t, widestAtom(`/p/[a-z]a`), widestAtom(`/p/(?:(?:(?:[a-z])))a`))
	require.Equal(t, widestAtom(`/p/[a-z]`), widestAtom(`/p/(?:[a-m]|[a-z])`))
}

// Test_Redirect_NonGreedyMarkerKeepsAnEmptyAtom covers the "?" between an empty atom and its quantifier.
func Test_Redirect_NonGreedyMarkerKeepsAnEmptyAtom(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(?:(?:)+?)+a`: "/narrow",
		`/p/b?a`:          "/broad",
	}, "/p/a?token=x", "/narrow?token=x")

	// A "?" matches nothing of its own, so it neither fills a group nor empties one.
	require.Equal(t, 0, carriesRun(`/p/(?:(?:)+?)+a`))
	require.Equal(t, 0, carriesRun(`/p/(?:(?:)?)+`))
	require.Equal(t, 1, carriesRun(`/p/(?:a?)+`))
	require.Equal(t, 1, carriesRun(`/p/(?:[a-z]+?)+`))
}

// Test_Redirect_PrefixReadsQuotesAndGroups covers a "\Q...\E" span and a group whose branches begin alike.
func Test_Redirect_PrefixReadsQuotesAndGroups(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, narrow, broad, path string
	}{
		{"quoted", `/p/\Qab\E`, `/p/a.+`, "/p/ab?token=x"},
		{"grouped", `/p/(?:a|aa)`, `/p/a+`, "/p/a?token=x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireWin(t, map[string]string{tc.narrow: "/narrow", tc.broad: "/broad"}, tc.path, "/narrow?token=x")

			require.GreaterOrEqual(t, literalPrefixLen(tc.narrow), literalPrefixLen(tc.broad))
		})
	}

	// Quoted text pins what it spells, and the prefix goes on past the "\E".
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/\Qab\E`))
	require.Equal(t, literalPrefixLen(`/p/abcd`), literalPrefixLen(`/p/\Qab\Ecd`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/\Q\E`))

	// A quantifier reaches the last byte quoted, and no more of them.
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/\Qab\E?`))

	// A group pins only what every branch of it pins, and a quantifier can take the group away whole.
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(?:ab|ac)d`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?:ab|ba)`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?:ab)?c`))
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/(?:ab)+`))

	// Two spellings of one character agree, and a character no byte answers for is named by the spelling instead.
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/(?:\x{61}b|ab)`))
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/(?:\x{3B1}b|\x{3B1}b)`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?:\x{3B1}|a)`))

	// A group closes where its own nesting, classes and quotes say it does, or at the end of the rule.
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/(?:\Qa\E(?:b)[c-c])`))
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/(?:ab`))
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(?:a[)]|a[)]b)`))
}

// Test_Redirect_AlternationSeparatorIsEmpty covers the "|" between branches every one of which matches nothing.
func Test_Redirect_AlternationSeparatorIsEmpty(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/(?:a{0}|b{0})+a`: "/narrow",
		`/p/(?:b|)a`:         "/broad",
	}, "/p/a?token=x", "/narrow?token=x")

	// A group is empty where every branch of it is, and filled where any is not.
	require.Equal(t, 0, carriesRun(`/p/(?:a{0}|b{0})+a`))
	require.Equal(t, 0, carriesRun(`/p/(?:|)+a`))
	require.Equal(t, 1, carriesRun(`/p/(?:a|b{0})+`))
	require.Equal(t, 1, carriesRun(`/p/(?:a|b)+`))
}

// Test_Redirect_PrefixReadsZeroCountsAndFlags covers a "{0}" and a group setting match flags.
func Test_Redirect_PrefixReadsZeroCountsAndFlags(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, narrow, broad, path string
	}{
		{"zero-count", `/p/a{0}a`, `/p/(?:a)[ab]?`, "/p/a?token=x"},
		{"folded", `/p/a`, `/p/(?i:a)`, "/p/a?token=x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireWin(t, map[string]string{tc.narrow: "/narrow", tc.broad: "/broad"}, tc.path, "/narrow?token=x")
		})
	}

	// A count of none is skipped, where a count that permits one ends the prefix: a path may arrive without the atom.
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/a{0}a`))
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/a[c-z]{0}b`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/a{0,2}a`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/a?a`))

	// Folding says what the text inside matches, so it pins no path, scoped or not.
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?i:a)`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?i:a)b`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?i)a`))
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(?-i:a)`))
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(?m:a)`))
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(?s:a)`))

	// A group counted none times is skipped like any other atom, and a "(" that opens no group names no flags.
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/a(?:xy){0}b`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/(?P`))

	// A group naming no flags pins what it spells, a capture name among them.
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(?:a)`))
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(?P<n>a)`))
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen(`/p/(a)`))

	// Which is what Go says of them: the flagged rule matches a path the exact one does not.
	require.True(t, regexp.MustCompile(`^(?:/p/(?i:a))$`).MatchString("/p/A"))
	require.False(t, regexp.MustCompile(`^(?:/p/a)$`).MatchString("/p/A"))
}

// Test_Redirect_CaseFoldingIsNoExactText covers a group setting the "i" flag, whose text pins no path.
func Test_Redirect_CaseFoldingIsNoExactText(t *testing.T) {
	t.Parallel()

	for _, broad := range []string{`/p/(?i)a`, `/p/(?i:a)`, `/p/(?si)a`} {
		t.Run(broad, func(t *testing.T) {
			t.Parallel()

			requireWin(t, map[string]string{`/p/[a]`: "/narrow", broad: "/broad"}, "/p/a?token=x", "/narrow?token=x")

			// Broader on both counts: it pins no path, and matches two characters where the class matches one.
			require.Equal(t, literalLen(`/p/`), literalLen(broad))
			require.Greater(t, patternWidth(broad), patternWidth(`/p/[a]`))
		})
	}

	// Scoped flags end at the group's ")", where a flag-only group reaches the rest of the rule.
	require.Equal(t, literalLen(`/p/b`), literalLen(`/p/(?i:a)b`))
	require.Equal(t, literalLen(`/p/`), literalLen(`/p/(?i)ab`))
	require.Equal(t, patternWidth(`/p/[ab]`), patternWidth(`/p/(?i:a)`))
	require.Equal(t, patternWidth(`/p/(?i:a)a`), patternWidth(`/p/(?i:a)(?-i:a)`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/(?i)(?-i)a`))

	// A group naming no flags is exact text still, a capture name among them.
	require.Equal(t, literalLen(`/p/ab`), literalLen(`/p/(?:a)b`))
	require.Equal(t, literalLen(`/p/ab`), literalLen(`/p/(?P<n>a)b`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/(?:a)`))

	// Which is what Go says of them.
	require.True(t, regexp.MustCompile(`^(?:/p/(?i)a)$`).MatchString("/p/A"))
	require.False(t, regexp.MustCompile(`^(?:/p/(?i)(?-i)a)$`).MatchString("/p/A"))
}

// Test_Redirect_QuotedPipeSeparatesNoBranch covers a "|" inside quoted text, which separates no branch.
func Test_Redirect_QuotedPipeSeparatesNoBranch(t *testing.T) {
	t.Parallel()

	// No request reaches these rules — Fiber percent-encodes a "|" — so the keys the sort reads pin the ordering.
	require.Equal(t, literalPrefixLen(`/p/a\|b`), literalPrefixLen(`/p/\Qa|b\E`))
	require.Greater(t, literalPrefixLen(`/p/\Qa|b\E`), literalPrefixLen(`/p/a\|(?i:b)`))
	require.Equal(t, literalLen(`/p/a\|b`), literalLen(`/p/\Qa|b\E`))

	// A "|" outside a quote still separates branches, and one inside a class still does not.
	require.Equal(t, literalPrefixLen(`/x`), literalPrefixLen(`/p/xa|/x`))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(`/p/[a|b]`))
	require.Len(t, splitAlternation(`/p/\Qa|b\E|/x`), 2)
}

// Test_Redirect_DeepNestingIsBounded covers a rule nesting groups without end, at the square of the depth.
func Test_Redirect_DeepNestingIsBounded(t *testing.T) {
	t.Parallel()

	deep := "/p/" + strings.Repeat("(?:", 16000) + "a" + strings.Repeat(")", 16000)
	require.NotPanics(t, func() {
		New(Config{Rules: map[string]string{deep: "/x", "/p/a": "/y"}})
	})

	// Followed no further than the bound, which only ends a prefix sooner.
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen(deep))
	require.Equal(t, literalPrefixLen(`/p/a`), literalPrefixLen("/p/"+strings.Repeat("(?:", maxPinnedDepth-1)+"a"+strings.Repeat(")", maxPinnedDepth-1)))
	require.Equal(t, literalPrefixLen(`/p/`), literalPrefixLen("/p/"+strings.Repeat("(?:", maxPinnedDepth+1)+"a"+strings.Repeat(")", maxPinnedDepth+1)))
}

// Test_Redirect_QuotedTextCountedNoneTimes covers a "{0}" after a "\E", which reaches the last byte quoted and no more.
func Test_Redirect_QuotedTextCountedNoneTimes(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`/p/a\Qbc\E{0}d`: "/narrow",
		`/p/a\142(?i:d)`: "/broad",
	}, "/p/abd?token=x", "/narrow?token=x")

	// The count takes the "c" and nothing else, and the "d" after it is pinned.
	require.Equal(t, literalPrefixLen(`/p/abd`), literalPrefixLen(`/p/a\Qbc\E{0}d`))
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/a\Qbc\E?d`))
	require.Equal(t, literalPrefixLen(`/p/abcd`), literalPrefixLen(`/p/a\Qbc\Ed`))
}

// Test_Redirect_FoldingReadsWhatGoFolds covers quoted text, a "k" folding to three, and a flag moving no literal.
func Test_Redirect_FoldingReadsWhatGoFolds(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, narrow, broad string
	}{
		{"quoted", `/p/[Kk]`, `/p/(?i:\Qk\E)`},
		{"cycle", `/p/[Kk]`, `/p/(?i:k)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireWin(t, map[string]string{tc.narrow: "/narrow", tc.broad: "/broad"}, "/p/k?token=x", "/narrow?token=x")

			require.Greater(t, patternWidth(tc.broad), patternWidth(tc.narrow))
		})
	}

	// What Go folds is what is counted: a "k" to "K" and the Kelvin sign, an "a" to "A" alone, and a digit to itself.
	require.Equal(t, 3, patternWidth(`/p/(?i:k)`))
	require.Equal(t, 2, patternWidth(`/p/(?i:a)`))
	require.Equal(t, 1, patternWidth(`/p/(?i:1)`))
	require.Equal(t, 3, patternWidth(`/p/(?i:s)`))
	require.True(t, regexp.MustCompile(`^(?:/p/(?i:k))$`).MatchString("/p/\u212A"))

	// Folding reaches quoted text and escapes that spell a character, and a class is read as a floor still.
	require.Equal(t, patternWidth(`/p/(?i:k)`), patternWidth(`/p/(?i:\Qk\E)`))
	require.Equal(t, patternWidth(`/p/(?i:k)`), patternWidth(`/p/(?i:\x6b)`))
	require.Equal(t, patternWidth(`/p/(?i:kk)`), patternWidth(`/p/(?i:\Qkk\E)`))
	require.Equal(t, 2*patternWidth(`/p/[ab]`), patternWidth(`/p/(?i:[ab])`))

	// A flag that moves no literal leaves it exact: "m" moves what an anchor asserts and "s" what a "." matches.
	require.Equal(t, literalLen(`/p/a`), literalLen(`/p/(?m:a)`))
	require.Equal(t, literalLen(`/p/a`), literalLen(`/p/(?s:a)`))
	require.Equal(t, patternWidth(`/p/a`), patternWidth(`/p/(?m:a)`))
	require.Equal(t, literalLen(`/p/`), literalLen(`/p/(?mi:a)`))
}

// Test_Redirect_AnchorEndsNoPrefix covers a rule opening with an anchor, which New adds to every rule anyway.
func Test_Redirect_AnchorEndsNoPrefix(t *testing.T) {
	t.Parallel()

	requireWin(t, map[string]string{
		`^/p/a`:      "/narrow",
		`/p/(?:a|b)`: "/broad",
	}, "/p/a?token=x", "/narrow?token=x")

	// An anchor pins no byte of its own and ends nothing, wherever it stands.
	require.Equal(t, literalPrefixLen(`/p/ab`), literalPrefixLen(`/p/a\bb`))
}
