package wiretarget

import (
	"bufio"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// answerFor hands app a request line exactly as written, which net/http's
// request builders would reject or rewrite, and returns what Routed answered.
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
	return c.SendString(Routed(c))
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

	for _, target := range []string{
		"/", "/a/b", "/a/b?x=%2F&y=//", "/a%252Fb", "/%252E%252E/a", "/a/b/",
		"/a;b/c", "/m;x=1;y=2", "/..x;y/a", "/a/.b;c", "/a%255C", "/a?q=\\&r=%5C",
	} {
		require.True(t, SegmentsAsRouted(target), target)
	}
	// "%2E" is an escape the router decodes, so one still in the routed path
	// was forged and URL.Path would read "%2E%2E" as "..". A backslash is a
	// separator to WHATWG URL parsers and IIS, and servlet containers resolve
	// a dot segment once they strip its path parameters.
	for _, target := range []string{
		"//", "//admin", "/a//b", "/a/b//", "/public/..%2Fadmin", "/a%2fb", "/a%2Fb?x=1", "/%2E%2E/a",
		"/admin\\secret", "/public/..\\admin", "/a%5Cb", "/public/..%5cadmin",
		"/public/..;/admin", "/public/.;/admin", "/public/..;jsessionid=x/admin", "/a/..;", "/a/.;x",
	} {
		require.False(t, SegmentsAsRouted(target), target)
	}
}

func Test_ParsesAsPath(t *testing.T) {
	t.Parallel()

	host := []byte("example.com")
	for _, target := range []string{"/", "/a://b", "/a//b://c", "//evil.example/x", "//u:pw@evil.example/a:/b", "/x?u=http://y"} {
		require.True(t, ParsesAsPath(target, host), target)
	}
	for _, target := range []string{"//u:pw@evil.example/a://b", "//evil.example/x?u=http://y", "//?x=1://"} {
		require.False(t, ParsesAsPath(target, host), target)
	}
	// Without a Host header fasthttp reads any target that begins with "//"
	// as an authority.
	require.False(t, ParsesAsPath("//evil.example/x", nil))
	require.True(t, ParsesAsPath("/x", nil))
	require.True(t, ParsesAsPath("/a//b", nil))
}

// Test_Routed_RootsUnrootedPath pins that a path override without a leading
// slash, as a rewrite of "/go/*" to "$1" produces, is handed on as a path
// rather than as the absolute URL a request line would read it as.
func Test_Routed_RootsUnrootedPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/go/*", func(c fiber.Ctx) error {
		c.Path(c.Params("*"))
		return c.SendString(Routed(c))
	})
	require.Equal(t, "/http://u:pw@evil.example/x", answerFor(t, app, "/go/http://u:pw@evil.example/x"))
	require.Equal(t, "/evil.example/x?q=1", answerFor(t, app, "/go/evil.example/x?q=1"))
}

func Test_HasForgedEscape(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"/public/%2e%2e/admin", "/%2E", "/a%41", "/%7e", "/x%5F?q=%41", "/a%25%41"} {
		require.True(t, HasForgedEscape(target), target)
	}
	// every unreserved character, digits and "-" included
	for _, c := range []byte("09azAZ-._~") {
		target := "/x/%" + strings.ToUpper(hex.EncodeToString([]byte{c}))
		require.True(t, HasForgedEscape(target), target)
		require.True(t, HasForgedEscape(strings.ToLower(target)), target)
	}
	for _, target := range []string{"/", "/a/b", "/a%20b", "/a%2Fb", "/a%25b", "/%2541", "/a%3Fb", "/%", "/%2", "/%zz", "/a?q=%41", "/a%25/%2"} {
		require.False(t, HasForgedEscape(target), target)
	}
}

func Test_SegmentsAsRouted_ForgedEscape(t *testing.T) {
	t.Parallel()

	require.False(t, SegmentsAsRouted("/public/%2e%2e/admin/secret"))
	require.False(t, SegmentsAsRouted("/public/%70rivate"))
	require.True(t, SegmentsAsRouted("/public/%2541/a%20b?x=%2e"))
}

func Test_SurvivesNormalization(t *testing.T) {
	t.Parallel()

	// An escape of a byte fasthttp escapes again when it writes the path
	// comes out as it went in, whatever the case of its hex digits.
	for _, target := range []string{
		"/", "/a/b", "/a/b?q=%2F&x=//", "/a.b/c-d_e~f", "/a%20b", "/%25", "/caf%C3%A9",
		"/a%7Cb%7bc", "/a%3Fb%23c", "/%21%27%28%29%2A",
	} {
		require.True(t, SurvivesNormalization(target), target)
	}
	// An escape of a byte it writes raw is decoded into that byte, as is a
	// backslash Windows resolves dot segments around, an empty segment is
	// merged and a "%" that begins no escape is written as "%25".
	for _, target := range []string{
		"/public/..%2Fadmin", "/a//b", "//x", "/..%3B/admin", "/a%40b", "/a%2Cb", "/%41", "/%2e%2e/x",
		"/a%5Cb", "/a%", "/a%2", "/a%zz",
	} {
		require.False(t, SurvivesNormalization(target), target)
	}
}

func Test_ParsesAsPath_Unrooted(t *testing.T) {
	t.Parallel()

	host := []byte("example.com")
	for _, target := range []string{"", "http://u:pw@evil.example/x", "evil.example/x", "*"} {
		require.False(t, ParsesAsPath(target, host), target)
	}
	require.True(t, ParsesAsPath("/http://u:pw@evil.example/x", host))
	require.True(t, ParsesAsPath("/http://u:pw@evil.example/x", nil))
}
