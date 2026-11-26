package wiretarget_test

import (
	"bufio"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/wiretarget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// answerFor hands app a request line exactly as written, which net/http's
// request builders would reject or rewrite, and returns what wiretarget.Routed answered.
func answerFor(t *testing.T, app *fiber.App, target string) string {
	t.Helper()

	var req fasthttp.Request
	raw := fiber.MethodGet + " " + target + " HTTP/1.1\r\nHost: example.com\r\n\r\n"
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	var fctx fasthttp.RequestCtx
	fctx.Init(&req, nil, nil)
	app.Handler()(&fctx)
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode(), target)
	return string(fctx.Response.Body())
}

func echoRouted(c fiber.Ctx) error {
	return c.SendString(wiretarget.Routed(c))
}

func Test_Routed(t *testing.T) {
	t.Parallel()
	t.Run("default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/*", echoRouted)
		for target, want := range map[string]string{
			"/":                       "/",
			"/a%2Fb/%41?q=%2F&x=1":    "/a%2Fb/A?q=%2F&x=1",
			"/a//b":                   "/a//b",
			"/a/./b/../c":             "/a/c",
			"/%2e%2e/a":               "/a",
			"/a%2fb":                  "/a%2Fb",
			"/%zz":                    "/%25zz",
			"/a%zzb%2Fc%":             "/a%25zzb%2Fc%25",
			"/trailing%2":             "/trailing%252",
			"/%":                      "/%25",
			"/a?":                     "/a",
			"/a?%zz=%zz":              "/a?%zz=%zz",
			"/caf%C3%A9?x=caf%C3%A9":  "/caf%C3%A9?x=caf%C3%A9",
			"/a?x=1#frag":             "/a?x=1",
			"/sp%20ace/%7Etilde?x=+y": "/sp%20ace/~tilde?x=+y",
			"/UPPER/%5C?X=1":          "/UPPER/%5C?X=1",
			// bytes a path may not carry raw are written as their escapes
			"/a\\b":                 "/a%5Cb",
			"/a|b{c}":               "/a%7Cb%7Bc%7D",
			"/q\"^<>`[]":            "/q%22%5E%3C%3E%60%5B%5D",
			"/\xc3\xbcber":          "/%C3%BCber",
			"/sub!$&'()*+,;=:@~-._": "/sub!$&'()*+,;=:@~-._",
		} {
			require.Equal(t, want, answerFor(t, app, target), target)
		}
	})

	t.Run("unescape path", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{UnescapePath: true})
		app.Get("/*", echoRouted)
		for target, want := range map[string]string{
			"/a%2Fb/%41%20c?q=%2F": "/a/b/A%20c?q=%2F",
			"/a%2F..%2Fb":          "/b",
			"/a%2F%2Fb":            "/a//b",
			"/caf%C3%A9":           "/caf%C3%A9",
			"/%zz":                 "/%25zz",
			"/%252e%252e/a":        "/%252e%252e/a",
			"/a%3Fb%23c%25d":       "/a%3Fb%23c%25d",
			"/admin;x/secret":      "/admin%3Bx/secret",
			"/admin%3Bx/b;c=d":     "/admin%3Bx/b%3Bc=d",
		} {
			require.Equal(t, want, answerFor(t, app, target), target)
		}
	})

	t.Run("query read by a handler", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			_ = c.Query("a")
			return c.Next()
		})
		app.Get("/*", echoRouted)
		// parsed arguments would be serialized again as "a=+b&c=%3Bd"
		require.Equal(t, "/x?a=%20b&c=;d&sig=AbC%2F", answerFor(t, app, "/x?a=%20b&c=;d&sig=AbC%2F"))
	})

	t.Run("query arguments changed by a handler", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			c.Request().URI().QueryArgs().Set("k", "v")
			return c.Next()
		})
		app.Get("/*", echoRouted)
		require.Equal(t, "/a?x=1&k=v", answerFor(t, app, "/a?x=1"))
	})

	t.Run("path overridden by a handler", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			c.Path("/rewritten/%41")
			return c.Next()
		})
		app.Get("/*", echoRouted)
		require.Equal(t, "/rewritten/A?x=1", answerFor(t, app, "/original?x=1"))
	})

	t.Run("immutable", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{Immutable: true})
		app.Get("/*", echoRouted)
		require.Equal(t, "/a%2Fb/A?q=1", answerFor(t, app, "/a%2Fb/%41?q=1"))
	})
}

func Test_SegmentsAsRouted(t *testing.T) {
	t.Parallel()
	const both = wiretarget.AllowSlashes | wiretarget.AllowParameters
	all := []wiretarget.Allowance{0, wiretarget.AllowSlashes, wiretarget.AllowParameters, both}

	for _, tc := range []struct {
		name    string
		targets []string
		passes  []wiretarget.Allowance // the allowances under which every target is let through
	}{
		{
			name: "plain",
			targets: []string{
				"/", "/a/b", "/a/b?x=%2F&y=//", "/a%252Fb", "/%252E%252E/a", "/a/b/", "/a%255C", "/a?q=\\&r=%5C",
				// a ";" in the query is no path parameter, and an escape of a
				// reserved character, "%" included, is no forged escape
				"/a/b?x=1;y=2", "/a?;", "/public/%2541/a%20b?x=%2e",
			},
			passes: all,
		},
		{
			// A servlet container strips the parameters from every segment, so
			// "/admin;x/secret" is "/admin/secret" there.
			name: "path parameter",
			targets: []string{
				"/a;b/c", "/m;x=1;y=2", "/..x;y/a", "/a/.b;c", "/admin;x/secret", "/admin;/secret",
				"/a/b;jsessionid=1", "/a;x/b;y?q=1", "/a;", "/;jsessionid=1", "/a/;x", "/;x;y",
			},
			passes: []wiretarget.Allowance{wiretarget.AllowParameters, both},
		},
		{
			// Stripped, a segment of parameters alone is an empty segment:
			// "/;/admin/secret" is "//admin/secret", merged into "/admin/secret".
			name:    "parameters alone before another segment",
			targets: []string{"/;/admin/secret", "/;x/admin", "/a/;x/b", "/a/;/", "/;x;y/b", "/a/;jsessionid=1/b"},
			passes:  []wiretarget.Allowance{both},
		},
		{
			name:    "escaped slash and a parameter",
			targets: []string{"/a%2Fb;c", "/group%2Fname;x", "/a%2F;x/b", "/a%2fb;c"},
			passes:  []wiretarget.Allowance{both},
		},
		{
			name:    "escaped slash or empty segment",
			targets: []string{"//", "//admin", "/a//b", "/a/b//", "/public/..%2Fadmin", "/a%2fb", "/a%2Fb?x=1"},
			passes:  []wiretarget.Allowance{wiretarget.AllowSlashes, both},
		},
		{
			// "%2E" is an escape the router decodes, so one still in the routed
			// path was forged and URL.Path would read "%2E%2E" as "..". A
			// backslash is a separator to WHATWG URL parsers and IIS, and
			// servlet containers resolve a dot segment once they strip its
			// path parameters.
			name: "never",
			targets: []string{
				"/%2E%2E/a", "/public/%2e%2e/admin", "/public/%70rivate",
				"/admin\\secret", "/public/..\\admin", "/a%5Cb", "/public/..%5cadmin",
				"/public/..;/admin", "/public/.;/admin", "/public/..;jsessionid=x/admin", "/a/..;", "/a/.;x",
				"/..;/a", "/.;x/a", "/a;x/..;/b",
				// behind an escaped slash, the dot segment is one to a reader
				// that decodes it; the escaped slash alone is let through
				"/a%2F..;/b", "/%2F..;/b", "/a%2F.;x/b", "/a%2f..;/b", "/a%2F..;",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, allow := range all {
				want := slices.Contains(tc.passes, allow)
				for _, target := range tc.targets {
					require.Equal(t, want, wiretarget.SegmentsAsRouted(target, allow), "%s allowing %d", target, allow)
				}
			}
		})
	}
}

// Test_Serve_UnescapePathParameters pins that under UnescapePath Serve judges the path parameters of the path the
// router decoded, which is the one the handler reads: the request line it is given spells every ";" as "%3B", a name
// to SegmentsAsRouted. A "." or ".." segment with parameters and a segment of parameters alone before another are not
// served, whether the ";" arrived raw or as "%3B"; a parameter on a named segment or alone in the last one is.
func Test_Serve_UnescapePathParameters(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{UnescapePath: true})
	app.Get("/*", func(c fiber.Ctx) error {
		read := ""
		handler := func(fctx *fasthttp.RequestCtx) { read = string(fctx.RequestURI()) + " " + string(fctx.Path()) }
		if wiretarget.Serve(c, handler) == wiretarget.NotFound {
			read = "not found"
		}
		return c.SendString(read)
	})
	for _, target := range []string{
		"/public/..;/admin", "/public/..%3B/admin", "/public/..%3b/admin", "/public/.;x/admin", "/public/%2e%2E%3Bx/admin",
		"/public/..;jsessionid=x/admin", "/a/..;", "/a/.%3Bx", "/..%3B/a", "/a;x/..%3B/b", "/a%2F..%3B/b",
		"/;/admin", "/%3B/admin", "/;x/admin", "/public/%3Bx/admin", "/public%2F;x/admin", "/a/%3Bx;y/",
	} {
		require.Equal(t, "not found", answerFor(t, app, target), target)
	}
	for target, read := range map[string]string{
		"/admin;x/secret":    "/admin%3Bx/secret /admin;x/secret",
		"/admin%3Bx/b;c=d":   "/admin%3Bx/b%3Bc=d /admin;x/b;c=d",
		"/..x;y/a/.b%3Bc":    "/..x%3By/a/.b%3Bc /..x;y/a/.b;c",
		"/app/;jsessionid=x": "/app/%3Bjsessionid=x /app/;jsessionid=x",
		"/app/%3B":           "/app/%3B /app/;",
		"/a/..%253B/b":       "/a/..%253B/b /a/..%3B/b", // an escaped "%" is no escape of a ";"
		"/a/../b?x=..;/c":    "/b?x=..;/c /b",
	} {
		require.Equal(t, read, answerFor(t, app, target), target)
	}
}

func Test_ParsesAsPath(t *testing.T) {
	t.Parallel()
	host := []byte("example.com")
	for _, target := range []string{"/", "/a://b", "/a//b://c", "//evil.example/x", "//u:pw@evil.example/a:/b", "/x?u=http://y"} {
		require.True(t, wiretarget.ParsesAsPath(target, host), target)
	}
	for _, target := range []string{"//u:pw@evil.example/a://b", "//evil.example/x?u=http://y", "//?x=1://"} {
		require.False(t, wiretarget.ParsesAsPath(target, host), target)
	}
	// Without a Host header fasthttp reads any target that begins with "//"
	// as an authority.
	require.False(t, wiretarget.ParsesAsPath("//evil.example/x", nil))
	require.True(t, wiretarget.ParsesAsPath("/x", nil))
	require.True(t, wiretarget.ParsesAsPath("/a//b", nil))
}

// Test_Routed_RootsUnrootedPath pins that a path override without a leading
// slash, as a rewrite of "/go/*" to "$1" produces, is handed on as a path
// rather than as the absolute URL a request line would read it as.
func Test_Routed_RootsUnrootedPath(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/go/*", func(c fiber.Ctx) error {
		c.Path(c.Params("*"))
		return c.SendString(wiretarget.Routed(c))
	})
	require.Equal(t, "/http://u:pw@evil.example/x", answerFor(t, app, "/go/http://u:pw@evil.example/x"))
	require.Equal(t, "/evil.example/x?q=1", answerFor(t, app, "/go/evil.example/x?q=1"))
}

func Test_HasForgedEscape(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"/public/%2e%2e/admin", "/%2E", "/a%41", "/%7e", "/x%5F?q=%41", "/a%25%41"} {
		require.True(t, wiretarget.HasForgedEscape(target), target)
	}
	// every unreserved character, digits and "-" included
	for _, c := range []byte("09azAZ-._~") {
		target := "/x/%" + strings.ToUpper(hex.EncodeToString([]byte{c}))
		require.True(t, wiretarget.HasForgedEscape(target), target)
		require.True(t, wiretarget.HasForgedEscape(strings.ToLower(target)), target)
	}
	for _, target := range []string{"/", "/a/b", "/a%20b", "/a%2Fb", "/a%25b", "/%2541", "/a%3Fb", "/%", "/%2", "/%zz", "/a?q=%41", "/a%25/%2"} {
		require.False(t, wiretarget.HasForgedEscape(target), target)
	}
}

// Test_HasStrayPercent pins the "%" a path may not carry: one that is not followed by two hex digits.
func Test_HasStrayPercent(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{
		"": false, "/": false, "/a/b": false, "/a%41": false, "/a%2f": false, "/a%2F%2f": false, "/%25": false,
		"/%25%25": false, "/a%25b%20c": false, "/%C3%BCber": false,
		"/%": true, "/a%": true, "/%z": true, "/%zz": true, "/a%2": true, "/a%2/b": true, "/%4g": true, "/%g4": true,
		"/100%/x": true, "/%%41": true, "/a%25%": true, "/%41%": true, "/%41%4": true, "/%41%zz": true,
	} {
		require.Equal(t, want, wiretarget.HasStrayPercent(path), path)
	}
}

// Test_HasStrayPercent_MatchesNetHTTP pins that HasStrayPercent finds what net/http's server answers 400 for: it reads
// the request target with url.ParseRequestURI, which refuses a "%" that is not followed by two hex digits, and nothing
// else in a path made of these bytes. Every path of up to six of them is compared.
func Test_HasStrayPercent_MatchesNetHTTP(t *testing.T) {
	t.Parallel()
	const alphabet = "/%a2Fg"
	checked := 0
	var walk func(path string)
	walk = func(path string) {
		_, err := url.ParseRequestURI(path)
		if stray := wiretarget.HasStrayPercent(path); stray != (err != nil) {
			t.Fatalf("%q: HasStrayPercent=%t, url.ParseRequestURI=%v", path, stray, err)
		}
		if err != nil {
			var escapeErr url.EscapeError
			require.ErrorAs(t, err, &escapeErr, path)
		}
		checked++
		if len(path) == 6 {
			return
		}
		for i := range len(alphabet) {
			walk(path + alphabet[i:i+1])
		}
	}
	walk("/")
	require.Greater(t, checked, 5000)
}

// Test_StrayPercent pins that the routed path of the request is what is looked at, and that it is not under
// UnescapePath, where the router has decoded every escape and "/100%" and "/100%25" are one path.
func Test_StrayPercent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		config                       fiber.Config
		name, target, override, want string
	}{
		{name: "stray", target: "/100%/x", want: "true"},
		{name: "escape", target: "/100%25/x", want: "false"},
		{name: "stray in the query", target: "/x?a=%zz&b=%", want: "false"},
		{name: "unescape path", target: "/100%/x", config: fiber.Config{UnescapePath: true}, want: "false"},
		{name: "path overridden by a handler", target: "/x", override: "/100%/x", want: "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(tc.config)
			app.Get("/*", func(c fiber.Ctx) error {
				if tc.override != "" {
					c.Path(tc.override)
				}
				return c.SendString(strconv.FormatBool(wiretarget.StrayPercent(c)))
			})
			require.Equal(t, tc.want, answerFor(t, app, tc.target))
		})
	}
}

func Test_SurvivesNormalization(t *testing.T) {
	t.Parallel()
	// An escape of a byte fasthttp escapes again when it writes the path
	// comes out as it went in, whatever the case of its hex digits.
	for _, target := range []string{
		"/", "/a/b", "/a/b?q=%2F&x=//", "/a.b/c-d_e~f", "/a%20b", "/%25", "/caf%C3%A9",
		"/a%7Cb%7bc", "/a%3Fb%23c", "/%21%27%28%29%2A",
	} {
		require.True(t, wiretarget.SurvivesNormalization(target), target)
	}
	// An escape of a byte it writes raw is decoded into that byte, as is a
	// backslash Windows resolves dot segments around, an empty segment is
	// merged and a "%" that begins no escape is written as "%25".
	for _, target := range []string{
		"/public/..%2Fadmin", "/a//b", "//x", "/..%3B/admin", "/a%40b", "/a%2Cb", "/%41", "/%2e%2e/x",
		"/a%5Cb", "/a%", "/a%2", "/a%zz",
	} {
		require.False(t, wiretarget.SurvivesNormalization(target), target)
	}
}

func Test_ParsesAsPath_Unrooted(t *testing.T) {
	t.Parallel()
	host := []byte("example.com")
	for _, target := range []string{"", "http://u:pw@evil.example/x", "evil.example/x", "*"} {
		require.False(t, wiretarget.ParsesAsPath(target, host), target)
	}
	require.True(t, wiretarget.ParsesAsPath("/http://u:pw@evil.example/x", host))
	require.True(t, wiretarget.ParsesAsPath("/http://u:pw@evil.example/x", nil))
}

func Test_NoPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		line    string
		connect bool
		want    bool
	}{
		{"*", false, true},
		{"*", true, true},
		{"example.com:443", true, true},
		{"[::1]:443", true, true},
		{"[2001:db8::1]:8443", true, true},
		{"example.com:443", false, false},
		{"/a", true, false},
		{"", true, false},
		{"/*", false, false},
		{"//admin/x", true, false},
		{"admin%2Fx", true, false},
		{"host:443?x=1", true, false},
		{"host:443#x", true, false},
		{"a\\b:443", true, false},
		{"a;b:443", true, false},
		{"user@host:443", true, false},
		{"example.com", true, true},
		{"192.0.2.1:443", true, true},
		{"[::1]", true, true},
		{"[::ffff:192.0.2.1]:443", true, true},
		// no authority: a byte a path may not carry raw, or brackets that hold no IPv6 address and a port
		{"\u00fcber", true, false},
		{"\u00fcber.txt", true, false},
		{"\u00fcber:443", true, false},
		{"a b", true, false},
		{"a|b", true, false},
		{"a[1]", true, false},
		{"a{b}", true, false},
		{"[a|b]:443", true, false},
		{"[g]", true, false},
		{"[a:b]", true, false},
		{"[1]", true, false},
		{"[]", true, false},
		{"[192.0.2.1]:443", true, false},
		{"[::1", true, false},
		{"[::1]x", true, false},
		{"[::1]:x", true, false},
		{"[::1]:443|", true, false},
		{"[::1][::1]", true, false},
		{"x[::1]:443", true, false},
		{"[::1]:443", false, false},
	} {
		require.Equal(t, tc.want, wiretarget.NoPath(tc.line, tc.connect), "%q connect=%t", tc.line, tc.connect)
	}
}

func Test_SingleSpelling(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{
		"/":                                    true,
		"/R&D/x":                               true,
		"/user@corp/x":                         true,
		"/a:b/c;d=e,f+g!h*i'(j)":               true,
		"/caf%C3%A9":                           true,
		"/my%20dir/x":                          true,
		"/a%3Fb%23c%25d":                       true,
		"/%zz":                                 true, // begins no escape: not judged here, see HasStrayPercent
		"/100%":                                true,
		"/R%26D/x":                             false,
		"/user%40corp/x":                       false,
		"/a%3Ab":                               false,
		"/a%21b%24c%27d%28e%29%2A%2B%2C%3B%3D": false,
		"/café":                                false,
		"/a b":                                 false,
		"/a|b":                                 false,
		"/caf%c3%a9":                           false, // lower-case digits: the router writes upper case
	} {
		require.Equal(t, want, wiretarget.SingleSpelling(path), path)
	}
}

func Test_RoutedAsMatched(t *testing.T) {
	t.Parallel()
	const refused = "refused"
	unescape := fiber.Config{UnescapePath: true}
	for _, tc := range []struct {
		config                       fiber.Config
		name, target, override, want string
	}{
		{name: "plain", target: "/a:b/c;d=e,f+g!h*i'(j)@k/?q=1", want: "/a:b/c;d=e,f+g!h*i'(j)@k/?q=1"},
		{name: "escape", target: "/caf%C3%A9/my%20dir/R%26D/100%25", want: "/caf%C3%A9/my%20dir/R%26D/100%25"},
		{name: "lower-case escape", target: "/caf%c3%a9", want: "/caf%C3%A9"}, // the router writes upper case
		{name: "raw UTF-8", target: "/caf\xc3\xa9", want: refused},
		{name: "escape and raw byte", target: "/caf%C3\xa9", want: refused},
		{name: "raw delimiter", target: "/a|b", want: refused},
		{name: "raw bracket", target: "/a[1]", want: refused},
		{name: "backslash", target: "/a\\b", want: refused},
		{name: "stray percent", target: "/100%/x", want: refused},
		{name: "cut escape", target: "/a%2", want: refused},
		{name: "raw byte in the query", target: "/x?a=caf\xc3\xa9|%", want: "/x?a=caf\xc3\xa9|%"},
		{name: "raw UTF-8 with unescape path", target: "/caf\xc3\xa9", config: unescape, want: "/caf%C3%A9"},
		{name: "escape with unescape path", target: "/caf%C3%A9", config: unescape, want: "/caf%C3%A9"},
		{name: "stray percent with unescape path", target: "/100%/x", config: unescape, want: "/100%25/x"},
		{name: "path overridden by a handler", target: "/x", override: "/My Docs/x", want: refused},
		{name: "escaped path overridden by a handler", target: "/x", override: "/My%20Docs/x", want: "/My%20Docs/x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(tc.config)
			app.Get("/*", func(c fiber.Ctx) error {
				if tc.override != "" {
					c.Path(tc.override)
				}
				if target, ok := wiretarget.RoutedAsMatched(c); ok {
					return c.SendString(target)
				}
				return c.SendString(refused)
			})
			require.Equal(t, tc.want, answerFor(t, app, tc.target))
		})
	}
}

// tlsConn is a connection that fasthttp takes for a TLS one: it has the methods of a *tls.Conn it looks for.
type tlsConn struct{ net.Conn }

func (tlsConn) Handshake() error                     { return nil }
func (tlsConn) ConnectionState() tls.ConnectionState { return tls.ConnectionState{} }

// Test_ReadAsOriginForm_KeepsTheScheme pins that the URI that reads a target that begins with "//" as a path is the
// one of the connection: https over TLS. fasthttp reads such a target as a path itself when the Host header holds a
// host, and the scheme is then the one it took from the connection (a request made by hand has none to take); the
// URI is rewritten for a request without a host.
func Test_ReadAsOriginForm_KeepsTheScheme(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		conn                         net.Conn
		name, host, scheme, wantHost string
	}{
		{name: "plain", host: "example.com", scheme: "http", wantHost: "example.com"},
		{name: "plain, no Host header", scheme: "http"},
		{name: "TLS, no Host header", scheme: "https", conn: tlsConn{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			const target = "//a/b?x=http://y"
			fctx := &fasthttp.RequestCtx{}
			fctx.Init2(tc.conn, nil, true)
			fctx.Request.Header.SetHost(tc.host)
			fctx.Request.SetRequestURI(target)
			wiretarget.ReadAsOriginForm(fctx, []byte(target))
			uri := fctx.Request.URI()
			require.Equal(t, tc.scheme, string(uri.Scheme()))
			require.Equal(t, tc.wantHost, string(uri.Host()))
			require.Equal(t, "//a/b", string(uri.PathOriginal()))
			require.Equal(t, "x=http://y", string(uri.QueryString()))
		})
	}
}

func Test_Serve_KeepsQueryArgsChange(t *testing.T) {
	t.Parallel()
	for name, handler := range map[string]any{
		"net/http": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.WriteString(w, r.RequestURI)
			assert.NoError(t, err)
		}),
		"fasthttp": fasthttp.RequestHandler(func(fctx *fasthttp.RequestCtx) { fctx.SetBody(fctx.RequestURI()) }),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			var after string
			app.Use(func(c fiber.Ctx) error {
				c.Request().URI().QueryArgs().Del("as_user")
				err := c.Next()
				after = c.Query("as_user") + "|" + c.OriginalURL() + "|" + wiretarget.Routed(c)
				return err
			})
			app.Get("/*", handler)
			require.Equal(t, "/a?year=2026", answerFor(t, app, "/a?as_user=alice&year=2026"))
			require.Equal(t, "|/a?as_user=alice&year=2026|/a?year=2026", after)
		})
	}
}
