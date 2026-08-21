package redirect

import (
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
func testApp(rules []Rule, unescape bool) *fiber.App {
	app := fiber.New(fiber.Config{UnescapePath: unescape})
	app.Use(New(Config{RuleList: rules, StatusCode: fiber.StatusFound}))
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
func requireWinMap(t *testing.T, rules map[string]string, path, want string) {
	t.Helper()
	app := fiber.New()
	app.Use(New(Config{Rules: rules, StatusCode: fiber.StatusFound}))
	app.Get("/*", func(c fiber.Ctx) error {
		return c.SendString("fell through")
	})
	status, location := get(t, app, path)
	require.Equal(t, fiber.StatusFound, status, "request %q", path)
	require.Equal(t, want, location, "request %q", path)
}

func requireWin(t *testing.T, rules []Rule, path, want string) {
	t.Helper()
	status, location := get(t, testApp(rules, false), path)
	require.Equal(t, fiber.StatusFound, status, "request %q", path)
	require.Equal(t, want, location, "request %q", path)
}

// requireRule builds a one-rule app and asserts where the request lands:
// redirected to want, or fallen through with no Location where want is "".
func requireRule(t *testing.T, unescape bool, pattern, target, request, want string) {
	t.Helper()
	status, location := get(t, testApp([]Rule{{From: pattern, To: target}}, unescape), request)
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
		RuleList:   []Rule{{From: "/default", To: "google.com"}},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(New(Config{
		RuleList:   []Rule{{From: "/default/*", To: "fiber.wiki"}},
		StatusCode: fiber.StatusTemporaryRedirect,
	}))
	app.Use(New(Config{
		RuleList:   []Rule{{From: "/redirect/*", To: "$1"}},
		StatusCode: fiber.StatusSeeOther,
	}))
	app.Use(New(Config{
		RuleList:   []Rule{{From: "/pattern/*", To: "golang.org"}},
		StatusCode: fiber.StatusFound,
	}))

	app.Use(New(Config{
		RuleList:   []Rule{{From: "/", To: "/swagger"}},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(New(Config{
		RuleList:   []Rule{{From: "/params", To: "/with_params"}},
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
		RuleList:   []Rule{{From: "/old", To: "/new"}},
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
			RuleList: []Rule{
				{From: "/cdn/*", To: "/first/$1"},
				{From: "/cdn/*x", To: "/second/$1"},
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
		// Brackets hold IPv6 only, so an IPv4 address is no host there. The IPv4-mapped spelling is.
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
		// A stray "%" is literal to the parser, not an error, though dotless.
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
		RuleList:   []Rule{{From: "/default", To: "google.com"}},
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
		RuleList:   []Rule{{From: "/default", To: "google.com"}},
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
		RuleList:   []Rule{},
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
		RuleList:   []Rule{{From: "/default", To: "google.com"}},
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
			RuleList:   []Rule{{From: "(", To: "google.com"}},
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

// Test_Redirect_DeprecatedMapHeuristicIsNotExact records what the deprecated
// map cannot do. Its order is read off the path text a rule pins, so two rules
// separated only by regexp syntax can order the wrong way round: "[a-z]" spells
// more bytes than "[ab]" while matching more paths. RuleList is the answer:
// the author says which comes first and nothing has to be inferred.
func Test_Redirect_DeprecatedMapHeuristicIsNotExact(t *testing.T) {
	t.Parallel()

	const narrow, broad = `/p/*[ab]`, `/p/*[a-z]`

	// The map hands "/p/za" to the broader rule, which is not what the author
	// would have picked.
	requireWinMap(t, map[string]string{broad: "/wide", narrow: "/narrow"}, "/p/za", "/wide")

	// Ordering them explicitly settles it.
	requireWin(t, []Rule{{From: narrow, To: "/narrow"}, {From: broad, To: "/wide"}}, "/p/za", "/narrow")
}

// Test_Redirect_FirstMatchWins pins the order rules are tried in: the first
// whose pattern matches answers, as routes do, and the broader rule still takes
// every path the narrower one leaves.
func Test_Redirect_FirstMatchWins(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		narrow Rule
		broad  Rule
		shared string
		wide   string
	}{
		{"a suffix past the wildcard", Rule{From: "/old/*", To: "/new"}, Rule{From: "/*", To: "/home"}, "/old/thing", "/other"},
		{"an exact rule and a class", Rule{From: "/api/users", To: "/exact"}, Rule{From: "/api/[a-z]+", To: "/broad"}, "/api/users", "/api/other"},
		{"a dot matches any byte", Rule{From: "/api/users", To: "/exact"}, Rule{From: "/api/user.", To: "/broad"}, "/api/users", "/api/userx"},
		{"a narrower class", Rule{From: "/api/[ab]", To: "/narrow"}, Rule{From: "/api/[abcdefghijklmnopqrstuvwxyz]", To: "/broad"}, "/api/a", "/api/z"},
		{"an alternation matches every branch", Rule{From: "/x", To: "/exact"}, Rule{From: "/very/specific|/x", To: "/alt"}, "/x", "/very/specific"},
		{"a grouped alternation", Rule{From: "/p/[a-z]xy", To: "/narrow"}, Rule{From: "/p/[a-z](reports|x.*)", To: "/grouped"}, "/p/axy", "/p/areports"},
		{"an optional atom", Rule{From: "/api/ab", To: "/exact"}, Rule{From: "/api/ab{0,1}", To: "/maybe"}, "/api/ab", "/api/a"},
		{"an anchor consumes nothing", Rule{From: "/p/[a]", To: "/exact"}, Rule{From: "/p/[a-z]$", To: "/class"}, "/p/a", "/p/b"},
		{"a class escape", Rule{From: "/api/1", To: "/one"}, Rule{From: `/api/\d+`, To: "/digits"}, "/api/1", "/api/27"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// The path both match goes to whichever rule the author put first.
			requireWin(t, []Rule{tc.narrow, tc.broad}, tc.shared, tc.narrow.To)
			// What only the broader rule matches still reaches it.
			requireWin(t, []Rule{tc.narrow, tc.broad}, tc.wide, tc.broad.To)
			// Reversing the order reverses the winner on the shared path, which
			// is the whole of what author order buys.
			requireWin(t, []Rule{tc.broad, tc.narrow}, tc.shared, tc.broad.To)
		})
	}
}

// Test_Redirect_DeprecatedMapOrderIsBySpecificity pins the heuristic that orders the deprecated Rules map.
func Test_Redirect_DeprecatedMapOrderIsBySpecificity(t *testing.T) {
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireWinMap(t, tc.rules, tc.path, tc.want)
		})
	}
}

// Test_LiteralLengths pins how a rule's pinned length is measured, which is what orders two rules that overlap.
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

	app := testApp([]Rule{{From: "/a|/b", To: "/moved"}}, false)
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
func Test_Redirect_FiniteRepetitionIsCounted(t *testing.T) {
	t.Parallel()

	requireWin(t, []Rule{
		{From: `/p/[a][ab]?`, To: "/narrow"},
		{From: `/p/[ab]{1,2}`, To: "/broad"},
	}, "/p/a", "/narrow")
}

// Test_Redirect_UnboundedRulesKeepTheirBreadth covers two rules that both run on: "/p/[z]+" and "/p/[a-z]+".
func Test_Redirect_UnboundedRulesKeepTheirBreadth(t *testing.T) {
	t.Parallel()

	requireWin(t, []Rule{
		{From: `/p/[z]+`, To: "/narrow"},
		{From: `/p/[a-z]+`, To: "/broad"},
	}, "/p/z", "/narrow")
}

// Test_Redirect_EscapedByteIsOneMember covers the class escape spelling one byte: "[\.]" lists the dot alone.
func Test_Redirect_UnterminatedQuoteRuleIsRejected(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{RuleList: []Rule{{From: `/p/*\Qab*`, To: "/safe"}}})
	})
}

func Test_Redirect_HexEscapeOutranksAClass(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{From: `/p/\x{61}`, To: "/exact"},
		{From: "/p/[a-z]", To: "/class"},
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

	app := testApp([]Rule{{From: "/r/*", To: "https://example.com$1"}}, true)
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
func Test_Redirect_DeepNestingIsBounded(t *testing.T) {
	t.Parallel()

	deep := "/p/" + strings.Repeat("(?:", 16000) + "a" + strings.Repeat(")", 16000)
	require.NotPanics(t, func() {
		New(Config{RuleList: []Rule{{From: deep, To: "/x"}, {From: "/p/a", To: "/y"}}})
	})
}

// Test_Redirect_BothRuleFieldsPanic pins that a config setting the deprecated
// map and the ordered list at once is refused: the two disagree about what
// decides precedence, and picking one silently is the bug this API replaced.
func Test_Redirect_BothRuleFieldsPanic(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			Rules:    map[string]string{"/old": "/new"},
			RuleList: []Rule{{From: "/old", To: "/new"}},
		})
	})

	// Either alone is fine.
	require.NotPanics(t, func() {
		New(Config{Rules: map[string]string{"/old": "/new"}})
	})
	require.NotPanics(t, func() {
		New(Config{RuleList: []Rule{{From: "/old", To: "/new"}}})
	})
}

// Test_Redirect_DeprecatedMapStillRedirects pins that the deprecated field goes
// on working, since it has to for the whole of v3.
func Test_Redirect_DeprecatedMapStillRedirects(t *testing.T) {
	t.Parallel()

	requireWinMap(t, map[string]string{"/api/*": "/$1"}, "/api/users", "/users")
	requireWinMap(t, map[string]string{"/old": "/new"}, "/old", "/new")
}

// Test_ShadowedRules pins which rule orders leave a rule dead. Reported rather
// than reordered: the order is the author's, so the answer is to tell them.
func Test_ShadowedRules(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		rules   []Rule
		shadows bool
	}{
		{"catch-all first", []Rule{{From: "/*", To: "/a"}, {From: "/old/*", To: "/b"}}, true},
		{"catch-all last", []Rule{{From: "/old/*", To: "/b"}, {From: "/*", To: "/a"}}, false},
		{"wildcard eats the exact rule", []Rule{{From: "/api/*", To: "/a"}, {From: "/api/users", To: "/b"}}, true},
		{"exact rule first", []Rule{{From: "/api/users", To: "/b"}, {From: "/api/*", To: "/a"}}, false},
		{"more wildcards eat fewer", []Rule{{From: "/p/*a*b", To: "/a"}, {From: "/p/*ab", To: "/b"}}, true},
		{"fewer wildcards first", []Rule{{From: "/p/*ab", To: "/b"}, {From: "/p/*a*b", To: "/a"}}, false},
		{"a suffix under a wildcard", []Rule{{From: "/p/*", To: "/a"}, {From: "/p/*.png", To: "/b"}}, true},
		{"a deeper path under a wildcard", []Rule{{From: "/users/*", To: "/a"}, {From: "/users/*/orders/*", To: "/b"}}, true},
		{"disjoint rules shadow nothing", []Rule{{From: "/a/*", To: "/x"}, {From: "/b/*", To: "/y"}}, false},
		{"one rule shadows nothing", []Rule{{From: "/a/*", To: "/x"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			compiled := make([]compiledRule, 0, len(tc.rules))
			for _, rule := range tc.rules {
				compiled = append(compiled, compiledRule{
					from:    rule.From,
					pattern: regexp.MustCompile("^(?:" + strings.ReplaceAll(rule.From, "*", "(.*)") + ")$"),
				})
			}
			require.Equal(t, tc.shadows, shadowedRules(compiled, nil))
		})
	}
}

// Test_OrderedRules pins each key the deprecated map is sorted by, and that a
// RuleList is handed back in the order the author wrote it.
func Test_OrderedRules(t *testing.T) {
	t.Parallel()

	t.Run("a rule list keeps the author's order", func(t *testing.T) {
		t.Parallel()

		list := []Rule{{From: "/*", To: "/broad"}, {From: "/api/users", To: "/exact"}}
		require.Equal(t, list, orderedRules(Config{RuleList: list}))
	})

	for _, tc := range []struct {
		name  string
		rules map[string]string
		want  []string
	}{
		{
			// A rule with no wildcard at all pins the whole of what it spells.
			name:  "more pinned before the wildcard wins",
			rules: map[string]string{"/api/*": "/a", "/api/users": "/b"},
			want:  []string{"/api/users", "/api/*"},
		},
		{
			name:  "then more pinned in total",
			rules: map[string]string{"/cdn/*": "/a", "/cdn/*x": "/b"},
			want:  []string{"/cdn/*x", "/cdn/*"},
		},
		{
			name:  "then fewer wildcards",
			rules: map[string]string{"/p/*a*b": "/a", "/p/*ab": "/b"},
			want:  []string{"/p/*ab", "/p/*a*b"},
		},
		{
			// Nothing else separates them, so the key itself keeps it total.
			name:  "then the key itself",
			rules: map[string]string{"/p/b*": "/b", "/p/a*": "/a"},
			want:  []string{"/p/a*", "/p/b*"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rules := orderedRules(Config{Rules: tc.rules})
			got := make([]string, 0, len(rules))
			for _, rule := range rules {
				got = append(got, rule.From)
				require.Equal(t, tc.rules[rule.From], rule.To)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

// Test_ShadowedRulesIsBounded pins that the scan gives up rather than walking
// every pair of a rule list long enough for the cost to show.
func Test_ShadowedRulesIsBounded(t *testing.T) {
	t.Parallel()

	shadowing := func(n int) []compiledRule {
		rules := make([]compiledRule, 0, n)
		rules = append(rules, compiledRule{from: "/*", pattern: regexp.MustCompile(`^(?:(.*))$`)})
		for i := 1; i < n; i++ {
			from := "/p" + strconv.Itoa(i)
			rules = append(rules, compiledRule{from: from, pattern: regexp.MustCompile("^(?:" + from + ")$")})
		}
		return rules
	}

	require.True(t, shadowedRules(shadowing(maxShadowChecked), nil))
	require.False(t, shadowedRules(shadowing(maxShadowChecked+1), nil))
}

// benchRules builds n non-overlapping rules, the last of which is the one a
// benchmark request matches.
func benchRules(n int) []Rule {
	rules := make([]Rule, 0, n)
	for i := range n {
		id := strconv.Itoa(i)
		rules = append(rules, Rule{From: "/old" + id + "/*", To: "/new" + id + "/$1"})
	}
	return rules
}

// Benchmark_Redirect_New measures building the middleware, which is where rule
// order is settled. It is parameterized over the rule count because that is
// what a per-rule startup cost shows up in.
func Benchmark_Redirect_New(b *testing.B) {
	for _, n := range []int{1, 10, 100, 500} {
		b.Run(strconv.Itoa(n)+"-rules", func(b *testing.B) {
			rules := benchRules(n)
			b.ReportAllocs()
			for b.Loop() {
				New(Config{RuleList: rules})
			}
		})
	}
}

// Benchmark_Redirect_NewDeprecatedMap is the same over the deprecated map,
// which has to be ordered before it can be used.
func Benchmark_Redirect_NewDeprecatedMap(b *testing.B) {
	for _, n := range []int{1, 10, 100, 500} {
		b.Run(strconv.Itoa(n)+"-rules", func(b *testing.B) {
			rules := make(map[string]string, n)
			for _, rule := range benchRules(n) {
				rules[rule.From] = rule.To
			}
			b.ReportAllocs()
			for b.Loop() {
				New(Config{Rules: rules})
			}
		})
	}
}

// Benchmark_Redirect_Handler measures the request path over the target shapes
// that cost different amounts to compose.
func Benchmark_Redirect_Handler(b *testing.B) {
	for _, tc := range []struct {
		name    string
		request string
		rules   []Rule
	}{
		{name: "static", rules: []Rule{{From: "/old", To: "/new"}}, request: "/old"},
		{name: "one-capture", rules: []Rule{{From: "/api/*", To: "/v2/$1"}}, request: "/api/users"},
		{name: "two-captures", rules: []Rule{{From: "/users/*/orders/*", To: "/user/$1/order/$2"}}, request: "/users/7/orders/9"},
		{name: "guarded-host", rules: []Rule{{From: "/cdn/*", To: "https://$1.cdn.example.com/"}}, request: "/cdn/images"},
		{name: "with-query", rules: []Rule{{From: "/api/*", To: "/v2/$1"}}, request: "/api/users?a=1&b=2"},
		{name: "miss", rules: benchRules(50), request: "/nothing/here"},
		{name: "last-of-50", rules: benchRules(50), request: "/old49/thing"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			app := fiber.New()
			app.Use(New(Config{RuleList: tc.rules}))
			app.Get("/*", func(c fiber.Ctx) error {
				return c.SendString("fell through")
			})

			b.ReportAllocs()
			for b.Loop() {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tc.request, http.NoBody))
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close() //nolint:errcheck // the body is empty and the error says nothing here
			}
		})
	}
}
