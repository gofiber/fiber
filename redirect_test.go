// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func assertFlashCookieCleared(t *testing.T, setCookie string) {
	t.Helper()

	setCookie = strings.ToLower(setCookie)
	require.Contains(t, setCookie, FlashCookieName+"=")
	require.True(t, strings.Contains(setCookie, "max-age=0") || strings.Contains(setCookie, "max-age=-1"))
	require.Contains(t, setCookie, "path=/")
}

// go test -run Test_Redirect_To
func Test_Redirect_To(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err := c.Redirect().To("http://default.com")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "http://default.com", string(c.Response().Header.Peek(HeaderLocation)))

	err = c.Redirect().Status(301).To("http://example.com")
	require.NoError(t, err)
	require.Equal(t, 301, c.Response().StatusCode())
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_To_WithFlashMessages
func Test_Redirect_To_WithFlashMessages(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().With("success", "2").With("success", "1").With("message", "test", 2).To("http://example.com")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(t, err)
	_, err = msgs.UnmarshalMsg(decoded)
	require.NoError(t, err)

	require.Len(t, msgs, 2)
	require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 2, isOldInput: false})
}

// go test -run Test_Redirect_Route_WithParams
func Test_Redirect_Route_WithParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
	})
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithParams_WithQueries
func Test_Redirect_Route_WithParams_WithQueries(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
		Queries: map[string]string{"data[0][name]": "john", "data[0][age]": "10", "test": "doe"},
	})
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())

	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	require.NoError(t, err, "url.Parse(location)")
	require.Equal(t, "/user/fiber", location.Path)
	require.Equal(t, url.Values{"data[0][name]": []string{"john"}, "data[0][age]": []string{"10"}, "test": []string{"doe"}}, location.Query())
}

// go test -run Test_Redirect_Route_WithQueries_SpecialChars
func Test_Redirect_Route_WithQueries_SpecialChars(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params:  Map{"name": "fiber"},
		Queries: map[string]string{"q": "a&b=c"},
	})
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())

	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	require.NoError(t, err, "url.Parse(location)")
	require.Equal(t, "/user/fiber", location.Path)
	// The value must survive as a single query parameter, not be split into
	// `q=a` and `b=c`.
	require.Equal(t, url.Values{"q": []string{"a&b=c"}}, location.Query())
}

// go test -run Test_Redirect_Route_ParamCannotMoveTheQuery
func Test_Redirect_Route_ParamCannotMoveTheQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		param string
		want  string
	}{
		{
			name:  "plain",
			param: "fiber",
			want:  "/user/fiber?q=1",
		},
		{
			// A parameter cannot introduce an earlier query string.
			name:  "param opens a query",
			param: "a?b=2",
			want:  "/user/a%3Fb=2?q=1",
		},
		{
			// A parameter cannot turn the query into a fragment.
			name:  "param opens a fragment",
			param: "a#b",
			want:  "/user/a%23b?q=1",
		},
		{
			name:  "param opens both",
			param: "a?b=2#c",
			want:  "/user/a%3Fb=2%23c?q=1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get("/user/:name", func(c Ctx) error {
				return c.JSON(c.Params("name"))
			}).Name("user")
			c := app.AcquireCtx(&fasthttp.RequestCtx{})

			err := c.Redirect().Route("user", RedirectConfig{
				Params:  Map{"name": tc.param},
				Queries: map[string]string{"q": "1"},
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, string(c.Response().Header.Peek(HeaderLocation)))
		})
	}
}

// go test -run Test_Redirect_Route_ParamCannotLeaveTheOrigin
func Test_Redirect_Route_ParamCannotLeaveTheOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		param string
		want  string
	}{
		{
			name:  "embedded slash",
			param: "docs/index.html",
			want:  "/docs%2Findex.html",
		},
		{
			// A slash is data in one parameter, not a path delimiter.
			name:  "leading slash",
			param: "/evil.com",
			want:  "/%2Fevil.com",
		},
		{
			name:  "leading slash run",
			param: "//evil.com",
			want:  "/%2F%2Fevil.com",
		},
		{
			// A backslash is encoded before a WHATWG parser can fold it.
			name:  "leading backslash",
			param: `\evil.com`,
			want:  "/%5Cevil.com",
		},
		{
			name:  "mixed slash run",
			param: `/\/evil.com`,
			want:  "/%2F%5C%2Fevil.com",
		},
		{
			// Controls are encoded before URL input preprocessing can remove them.
			name:  "tab before the slash",
			param: "\t/evil.com",
			want:  "/%09%2Fevil.com",
		},
		{
			// A scheme stays path data; its slashes are encoded.
			name:  "absolute URL is a path segment",
			param: "https://evil.com",
			want:  "/https:%2F%2Fevil.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get("/*", func(c Ctx) error {
				return c.JSON(c.Params("*"))
			}).Name("wildcard")
			c := app.AcquireCtx(&fasthttp.RequestCtx{})

			err := c.Redirect().Route("wildcard", RedirectConfig{
				Params: Map{"*": tc.param},
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, string(c.Response().Header.Peek(HeaderLocation)))

			// The same question asked the other two ways. A caller who puts
			// either answer in a Location header or an href reaches the same
			// place, so all three have to agree.
			fromRoute, err := app.GetRoute("wildcard").URL(Map{"*": tc.param})
			require.NoError(t, err)
			require.Equal(t, tc.want, fromRoute, "Route.URL")

			fromCtx, err := c.GetRouteURL("wildcard", Map{"*": tc.param})
			require.NoError(t, err)
			require.Equal(t, tc.want, fromCtx, "GetRouteURL")
		})
	}
}

// go test -run Test_Redirect_Route_WithOptionalParams
func Test_Redirect_Route_WithOptionalParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
	})
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithOptionalParamsWithoutValue
func Test_Redirect_Route_WithOptionalParamsWithoutValue(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/user/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithGreedyParameters
func Test_Redirect_Route_WithGreedyParameters(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/+", func(c Ctx) error {
		return c.JSON(c.Params("+"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"+": "test/routes",
		},
	})
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/user/test%2Froutes", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Back
func Test_Redirect_Back(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	err = c.Redirect().Back()
	require.Equal(t, 500, c.Response().StatusCode())
	require.ErrorIs(t, err, ErrRedirectBackNoFallback)
}

// go test -run Test_Redirect_Back_WithFlashMessages
func Test_Redirect_Back_WithFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	err := c.Redirect().With("success", "1").With("message", "test").Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(t, err)
	_, err = msgs.UnmarshalMsg(decoded)
	require.NoError(t, err)

	require.Len(t, msgs, 2)
	require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

// go test -run Test_Redirect_Back_WithReferer
func Test_Redirect_Back_WithReferer(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	app.Get("/back", func(c Ctx) error {
		return c.JSON("Back")
	}).Name("back")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderReferer, "/back")
	err := c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/back", c.Get(HeaderReferer))
	require.Equal(t, "/back", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Back_WithCrossOriginReferer
func Test_Redirect_Back_WithCrossOriginReferer(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Cross-origin referer should be rejected, fallback used
	c.Request().Header.Set(HeaderReferer, "https://evil.com/phishing")
	c.Request().URI().SetHost("example.com")
	err := c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	// Cross-origin referer with no fallback should return error
	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "https://evil.com/phishing")
	err = c.Redirect().Back()
	require.Equal(t, 500, c.Response().StatusCode())
	require.ErrorIs(t, err, ErrRedirectBackNoFallback)

	// Same-origin referer with host should be accepted.
	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "http://example.com/dashboard")
	c.Request().URI().SetHost("example.com")
	err = c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "http://example.com/dashboard", string(c.Response().Header.Peek(HeaderLocation)))

	// Same-origin referer with a default port should be accepted.
	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "http://example.com:80/dashboard")
	err = c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "http://example.com:80/dashboard", string(c.Response().Header.Peek(HeaderLocation)))

	// Scheme mismatch should be rejected.
	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "https://example.com/dashboard")
	err = c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	// Port mismatch should be rejected.
	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "http://example.com:8080/admin")
	err = c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	// Relative path referer should be accepted (no host in URL).
	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "/back")
	err = c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/back", string(c.Response().Header.Peek(HeaderLocation)))

	// Scheme-only referrers should be rejected.
	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "javascript:alert(1)")
	err = c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	c.Response().Reset()
	c.Request().Header.Set(HeaderReferer, "mailto:test@example.com")
	err = c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Back_RejectsEmptyAuthorityReferer
//
// Test_Redirect_Back_RejectsEmptyAuthorityReferer covers a network-path
// reference whose authority came out empty.
//
// "//?x", "//#f" and "//@" reach net/url with an empty Host and no scheme, so
// the relative-path branch called them same-origin — a different question than
// the one asked. They name no origin at all, and the Location they produced is
// one no client can resolve: a browser fails to parse "//?x" against its base
// rather than going anywhere. Fall back instead of emitting a dead redirect.
func Test_Redirect_Back_RejectsEmptyAuthorityReferer(t *testing.T) {
	t.Parallel()

	for _, referer := range []string{"//?x", "//#f", "//@"} {
		t.Run(referer, func(t *testing.T) {
			t.Parallel()

			app := New()
			app.Get("/back", func(c Ctx) error { return c.Redirect().Back("/fallback") })

			req := httptest.NewRequest(MethodGet, "/back", http.NoBody)
			req.Host = "victim.test"
			req.Header.Set(HeaderReferer, referer)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, "/fallback", resp.Header.Get(HeaderLocation))
		})
	}

	// A reference that is only slashes normalizes to a plain absolute path and
	// really is this origin, so it is still honored.
	for _, referer := range []string{"//", "///"} {
		t.Run(referer, func(t *testing.T) {
			t.Parallel()

			app := New()
			app.Get("/back", func(c Ctx) error { return c.Redirect().Back("/fallback") })

			req := httptest.NewRequest(MethodGet, "/back", http.NoBody)
			req.Host = "victim.test"
			req.Header.Set(HeaderReferer, referer)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, "/", resp.Header.Get(HeaderLocation))
		})
	}
}

func Test_Redirect_Back_RejectsBrowserNormalizedReferer(t *testing.T) {
	t.Parallel()

	// Referer values that net/url reports as ordinary relative paths but that a
	// browser resolves to another origin, because it folds backslashes onto
	// forward slashes, drops ASCII tab/newline, and treats any leading run of
	// slashes as the start of an authority.
	crossOrigin := []string{
		`/\evil.com`,
		`/\/evil.com`,
		`\\evil.com`,
		`\/evil.com`,
		"/\t/evil.com",
		"\t//evil.com",
		` //evil.com`,
		`///evil.com`,
		`/////evil.com`,
		`https:///evil.com`,
	}

	app := New()
	for _, referer := range crossOrigin {
		t.Run(referer, func(t *testing.T) {
			t.Parallel()

			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)

			c.Request().Header.Set(HeaderReferer, referer)
			c.Request().URI().SetHost("example.com")

			require.NoError(t, c.Redirect().Back("/fallback"))
			require.Equal(t, StatusSeeOther, c.Response().StatusCode())
			require.Equal(t, "/fallback", string(c.Response().Header.Peek(HeaderLocation)))
		})
	}
}

// go test -run Test_Redirect_Back_NormalizesSameOriginReferer
func Test_Redirect_Back_NormalizesSameOriginReferer(t *testing.T) {
	t.Parallel()

	// A same-origin referer is redirected to in its browser-normalized form, so
	// a client that does not fold backslashes cannot resolve a different origin
	// than the one that was validated.
	tests := []struct {
		referer  string
		expected string
	}{
		{referer: `/back`, expected: `/back`},
		{referer: `back`, expected: `back`},
		{referer: `/a\b`, expected: `/a/b`},
		{referer: `http://example.com\@evil.com`, expected: `http://example.com/@evil.com`},
		{referer: "/back\t", expected: `/back`},

		// The backslash fold belongs to the path; WHATWG treats "\\" as an
		// ordinary character in the query and fragment, so rewriting it there
		// would send the user back to a different query than they came from.
		{referer: `/search?q=a\b`, expected: `/search?q=a\b`},
		{referer: `/p?path=C:\Users\me`, expected: `/p?path=C:\Users\me`},
		{referer: `/docs#a\b`, expected: `/docs#a\b`},
		// Invalid UTF-8 must survive byte-for-byte rather than becoming U+FFFD.
		{referer: "/back?x=caf\xe9\\y", expected: "/back?x=caf\xe9\\y"},
	}

	app := New()
	for _, tc := range tests {
		t.Run(tc.referer, func(t *testing.T) {
			t.Parallel()

			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)

			c.Request().Header.Set(HeaderReferer, tc.referer)
			c.Request().URI().SetHost("example.com")

			require.NoError(t, c.Redirect().Back("/fallback"))
			require.Equal(t, StatusSeeOther, c.Response().StatusCode())
			require.Equal(t, tc.expected, string(c.Response().Header.Peek(HeaderLocation)))
		})
	}
}

// go test -run Test_Redirect_Route_WithFlashMessages
func Test_Redirect_Route_WithFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	err := c.Redirect().With("success", "1").With("message", "test").Route("user")

	require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})

	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(t, err)
	_, err = msgs.UnmarshalMsg(decoded)
	require.NoError(t, err)

	require.Len(t, msgs, 2)
	require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

// go test -run Test_Redirect_Route_WithOldInput
func Test_Redirect_Route_WithOldInput(t *testing.T) {
	t.Parallel()

	t.Run("Query", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Get("/user", func(c Ctx) error {
			return c.SendString("user")
		}).Name("user")

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

		c.Request().URI().SetQueryString("id=1&name=tom")
		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, StatusSeeOther, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
		require.NoError(t, err)
		_, err = msgs.UnmarshalMsg(decoded)
		require.NoError(t, err)

		require.Len(t, msgs, 4)
		require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "id", value: "1", level: 0, isOldInput: true})
		require.Contains(t, msgs, redirectionMsg{key: "name", value: "tom", level: 0, isOldInput: true})
	})

	t.Run("Form", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Post("/user", func(c Ctx) error {
			return c.SendString("user")
		}).Name("user")

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

		c.Request().Header.Set(HeaderContentType, MIMEApplicationForm)
		c.Request().SetBodyString("id=1&name=tom")
		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, StatusSeeOther, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
		require.NoError(t, err)
		_, err = msgs.UnmarshalMsg(decoded)
		require.NoError(t, err)

		require.Len(t, msgs, 4)
		require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "id", value: "1", level: 0, isOldInput: true})
		require.Contains(t, msgs, redirectionMsg{key: "name", value: "tom", level: 0, isOldInput: true})
	})

	t.Run("MultipartForm", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Get("/user", func(c Ctx) error {
			return c.SendString("user")
		}).Name("user")

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		require.NoError(t, writer.WriteField("id", "1"))
		require.NoError(t, writer.WriteField("name", "tom"))
		require.NoError(t, writer.Close())

		c.Request().SetBody(body.Bytes())
		c.Request().Header.Set(HeaderContentType, writer.FormDataContentType())

		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, StatusSeeOther, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
		require.NoError(t, err)
		_, err = msgs.UnmarshalMsg(decoded)
		require.NoError(t, err)

		require.Len(t, msgs, 4)
		require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "id", value: "1", level: 0, isOldInput: true})
		require.Contains(t, msgs, redirectionMsg{key: "name", value: "tom", level: 0, isOldInput: true})
	})
}

func Test_Redirect_WithInput_ReusesClearedMap(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
	defer app.ReleaseCtx(c)

	c.Request().URI().SetQueryString("first=1")
	c.Redirect().WithInput()
	require.Contains(t, c.redirect.messages, redirectionMsg{key: "first", value: "1", isOldInput: true})

	c.redirect.messages = c.redirect.messages[:0]

	c.Request().URI().SetQueryString("second=2")
	c.Redirect().WithInput()

	require.Len(t, c.redirect.messages, 1)
	require.Contains(t, c.redirect.messages, redirectionMsg{key: "second", value: "2", isOldInput: true})
	require.NotContains(t, c.redirect.messages, redirectionMsg{key: "first", value: "1", isOldInput: true})
}

// go test -run Test_Redirect_parseAndClearFlashMessages
func Test_Redirect_parseAndClearFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	msgs := redirectionMsgs{
		{
			key:   "success",
			value: "1",
		},
		{
			key:   "message",
			value: "test",
		},
		{
			key:        "name",
			value:      "tom",
			isOldInput: true,
		},
		{
			key:        "id",
			value:      "1",
			isOldInput: true,
		},
	}

	val, err := msgs.MarshalMsg(nil)
	require.NoError(t, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))

	c.Redirect().parseAndClearFlashMessages()

	require.Equal(t, FlashMessage{
		Key:   "success",
		Value: "1",
		Level: 0,
	}, c.Redirect().Message("success"))

	require.Equal(t, FlashMessage{
		Key:   "message",
		Value: "test",
		Level: 0,
	}, c.Redirect().Message("message"))

	require.Equal(t, FlashMessage{}, c.Redirect().Message("success"))
	require.Equal(t, FlashMessage{}, c.Redirect().Message("not_message"))

	require.Empty(t, c.Redirect().Messages())

	require.Equal(t, OldInputData{
		Key:   "id",
		Value: "1",
	}, c.Redirect().OldInput("id"))

	require.Equal(t, OldInputData{
		Key:   "name",
		Value: "tom",
	}, c.Redirect().OldInput("name"))

	require.Equal(t, OldInputData{}, c.Redirect().OldInput("not_name"))

	require.Equal(t, []OldInputData{
		{
			Key:   "name",
			Value: "tom",
		},
		{
			Key:   "id",
			Value: "1",
		},
	}, c.Redirect().OldInputs())

	assertFlashCookieCleared(t, string(c.Response().Header.Peek(HeaderSetCookie)))

	c.Request().Header.Set(HeaderCookie, "fiber_flash=test")

	c.Redirect().parseAndClearFlashMessages()

	require.Empty(t, c.Redirect().messages)
}

// Test_Redirect_parseAndClearFlashMessages_InvalidHex tests the case where hex decoding fails
func Test_Redirect_parseAndClearFlashMessages_InvalidHex(t *testing.T) {
	t.Parallel()

	app := New()

	// Setup request and response
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
	defer app.ReleaseCtx(c)

	// Create redirect instance
	r := AcquireRedirect()
	r.c = c

	// Set invalid hex value in flash cookie
	c.Request().Header.SetCookie(FlashCookieName, "not-a-valid-hex-string")

	// Call parseAndClearFlashMessages
	r.parseAndClearFlashMessages()

	// Verify that no flash messages are processed (should be empty)
	require.Empty(t, r.messages)

	// The cookie is expired even though hex decoding failed. A value this
	// application cannot decode is one it will never decode, so leaving it in
	// place would mean re-running hex and msgp over attacker-controlled bytes
	// on every subsequent request for as long as the browser kept sending it.
	assertFlashCookieCleared(t, string(c.Response().Header.Peek(HeaderSetCookie)))

	// Release redirect
	ReleaseRedirect(r)
}

// Test_Redirect_release_ZeroesMessages asserts that a *Redirect handed back to
// the pool does not carry the submitted form with it. WithInput copies the
// whole request body into r.messages — a rejected login puts the password the
// user just typed in there — and truncating to [:0] leaves those strings
// reachable from the backing array for the life of the process.
func Test_Redirect_release_ZeroesMessages(t *testing.T) {
	t.Parallel()

	r := AcquireRedirect()
	r.With("password", "hunter2")
	r.With("token", "s3cr3t")
	require.Len(t, r.messages, 2)

	full := r.messages[:cap(r.messages)]
	r.release()

	for i := range full {
		require.Equal(t, redirectionMsg{}, full[i], "index %d survived release", i)
	}
}

func Test_Redirect_Messages_ClearsFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
	defer app.ReleaseCtx(c)

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(t, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	require.Equal(t, []FlashMessage{
		{
			Key:   "success",
			Value: "1",
			Level: 0,
		},
		{
			Key:   "message",
			Value: "test",
			Level: 0,
		},
	}, c.Redirect().Messages())

	require.Empty(t, c.Redirect().Messages())
	require.Equal(t, FlashMessage{}, c.Redirect().Message("success"))

	require.Equal(t, []OldInputData{
		{
			Key:   "name",
			Value: "tom",
		},
		{
			Key:   "id",
			Value: "1",
		},
	}, c.Redirect().OldInputs())
}

func Test_Redirect_CompleteFlowWithFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()

	// First handler that sets flash messages and redirects
	app.Get("/source", func(c Ctx) error {
		// Redirect to the target handler
		return c.Redirect().With("string_message", "Hello, World!").
			With("number_message", "12345").
			With("bool_message", "true").
			To("/target")
	})

	// Second handler that receives and processes flash messages
	app.Get("/target", func(c Ctx) error {
		// Get all flash messages and return them as a JSON response
		return c.JSON(Map{
			"string_message": c.Redirect().Message("string_message").Value,
			"number_message": c.Redirect().Message("number_message").Value,
			"bool_message":   c.Redirect().Message("bool_message").Value,
		})
	})

	// Step 1: Make the initial request to the source route
	req := httptest.NewRequest(MethodGet, "/source", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/target", resp.Header.Get(HeaderLocation))

	// Verify and get the cookie from the response
	cookies := resp.Cookies()
	var flashCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "fiber_flash" {
			flashCookie = cookie
			break
		}
	}
	require.NotNil(t, flashCookie, "Flash cookie should be set")

	// Step 2: Make the second request to the target route with the cookie
	req = httptest.NewRequest(MethodGet, "/target", http.NoBody)
	req.Header.Set("Cookie", flashCookie.Name+"="+flashCookie.Value)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	assertFlashCookieCleared(t, resp.Header.Get(HeaderSetCookie))

	// Parse the JSON response and verify flash messages
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Verify all flash messages were received correctly
	require.Equal(t, "Hello, World!", result["string_message"])
	require.Equal(t, "12345", result["number_message"]) // JSON numbers are float64
	require.Equal(t, "true", result["bool_message"])
}

func Test_Redirect_FlashMessagesWithSpecialChars(t *testing.T) {
	t.Parallel()

	app := New()

	// Handler that sets flash messages with special characters and redirects
	app.Get("/special-source", func(c Ctx) error {
		// Create a large message to test encoding of larger data
		return c.Redirect().With("null_bytes", "Contains\x00null\x00bytes").
			With("control_chars", "Contains\r\ncontrol\tcharacters").
			With("unicode", "Unicode: 你好世界").
			With("emoji", "Emoji: 🔥🚀😊").
			To("/special-target")
	})

	// Target handler that receives the flash messages
	app.Get("/special-target", func(c Ctx) error {
		return c.JSON(Map{
			"null_bytes":    c.Redirect().Message("null_bytes").Value,
			"control_chars": c.Redirect().Message("control_chars").Value,
			"unicode":       c.Redirect().Message("unicode").Value,
			"emoji":         c.Redirect().Message("emoji").Value,
		})
	})

	// Step 1: Make the initial request
	req := httptest.NewRequest(MethodGet, "/special-source", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/special-target", resp.Header.Get(HeaderLocation))

	// Get the flash cookie
	var flashCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "fiber_flash" {
			flashCookie = cookie
			break
		}
	}
	require.NotNil(t, flashCookie, "Flash cookie should be set")

	// Step 2: Make the second request with the cookie
	req = httptest.NewRequest(MethodGet, "/special-target", http.NoBody)
	req.Header.Set("Cookie", flashCookie.Name+"="+flashCookie.Value)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	assertFlashCookieCleared(t, resp.Header.Get(HeaderSetCookie))

	// Parse and verify the response
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Verify special character handling
	require.Equal(t, "Contains\x00null\x00bytes", result["null_bytes"])
	require.Equal(t, "Contains\r\ncontrol\tcharacters", result["control_chars"])
	require.Equal(t, "Unicode: 你好世界", result["unicode"])
	require.Equal(t, "Emoji: 🔥🚀😊", result["emoji"])
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route -benchmem -count=4
func Benchmark_Redirect_Route(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()

	var err error

	for b.Loop() {
		err = c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
		})
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
	require.Equal(b, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route_WithQueries -benchmem -count=4
func Benchmark_Redirect_Route_WithQueries(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()

	var err error

	for b.Loop() {
		err = c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
			Queries: map[string]string{"a": "a", "b": "b"},
		})
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	require.NoError(b, err, "url.Parse(location)")
	require.Equal(b, "/user/fiber", location.Path)
	require.Equal(b, url.Values{"a": []string{"a"}, "b": []string{"b"}}, location.Query())
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route_WithFlashMessages -benchmem -count=4
func Benchmark_Redirect_Route_WithFlashMessages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()

	var err error

	for b.Loop() {
		err = c.Redirect().With("success", "1").With("message", "test").Route("user")
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
	require.Equal(b, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(b, err)
	_, err = msgs.UnmarshalMsg(decoded)
	require.NoError(b, err)

	require.Contains(b, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(b, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

// Test_Redirect_DisableFlashMessages pins the switch on both request handlers:
// with it set, With and WithInput set no cookie, an incoming flash cookie is
// neither read nor expired, and the accessors report nothing.
func Test_Redirect_DisableFlashMessages(t *testing.T) {
	t.Parallel()

	payload, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(t, err)
	incoming := FlashCookieName + "=" + hex.EncodeToString(payload)

	apps := []struct {
		newApp func(cfg Config) *App
		name   string
	}{
		{newApp: func(cfg Config) *App { return New(cfg) }, name: "default ctx"},
		{newApp: func(cfg Config) *App {
			return NewWithCustomCtx(func(app *App) CustomCtx {
				return &customCtx{DefaultCtx: *NewDefaultCtx(app)}
			}, cfg)
		}, name: "custom ctx"},
	}
	for _, tc := range apps {
		for _, disabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/disabled=%v", tc.name, disabled), func(t *testing.T) {
				t.Parallel()

				app := tc.newApp(Config{DisableFlashMessages: disabled})
				app.Get("/source", func(c Ctx) error {
					return c.Redirect().With("status", "ok").WithInput().To("/target")
				})
				app.Get("/target", func(c Ctx) error {
					return c.SendString(fmt.Sprintf("%d/%d", len(c.Redirect().Messages()), len(c.Redirect().OldInputs())))
				})

				resp, err := app.Test(httptest.NewRequest(MethodGet, "/source?name=john", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, StatusSeeOther, resp.StatusCode)
				if disabled {
					require.Empty(t, resp.Header.Get(HeaderSetCookie), "no flash cookie may be set")
				} else {
					require.Contains(t, resp.Header.Get(HeaderSetCookie), FlashCookieName+"=")
				}

				req := httptest.NewRequest(MethodGet, "/target", http.NoBody)
				req.Header.Set(HeaderCookie, incoming)
				resp, err = app.Test(req)
				require.NoError(t, err)
				require.Equal(t, StatusOK, resp.StatusCode)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				if disabled {
					require.Equal(t, "0/0", string(body))
					require.Empty(t, resp.Header.Get(HeaderSetCookie), "the incoming cookie must be left alone")
				} else {
					require.Equal(t, "2/2", string(body))
					assertFlashCookieCleared(t, resp.Header.Get(HeaderSetCookie))
				}
			})
		}
	}
}

// Benchmark_Redirect_FlashCookieCheck measures the per-request scan for an
// incoming flash cookie on a parsed browser-style request, which the other
// handler benchmarks miss because their hand-built requests carry no raw
// header block.
// go test -v -run=^$ -bench=Benchmark_Redirect_FlashCookieCheck -benchmem -count=4
func Benchmark_Redirect_FlashCookieCheck(b *testing.B) {
	const raw = "GET /user HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36\r\n" +
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8\r\n" +
		"Accept-Language: en-US,en;q=0.9\r\n" +
		"Accept-Encoding: gzip, deflate, br, zstd\r\n" +
		"Connection: keep-alive\r\n" +
		"Cookie: session_id=abcdef1234567890; theme=dark; _ga=GA1.2.123456789.1700000000\r\n" +
		"Cache-Control: max-age=0\r\n" +
		"Sec-Fetch-Dest: document\r\n" +
		"Sec-Fetch-Mode: navigate\r\n" +
		"Sec-Fetch-Site: none\r\n\r\n"

	for _, disabled := range []bool{false, true} {
		b.Run(fmt.Sprintf("disabled=%v", disabled), func(b *testing.B) {
			app := New(Config{DisableFlashMessages: disabled})
			app.Get("/user", func(_ Ctx) error { return nil })
			handler := app.Handler()

			ctx := &fasthttp.RequestCtx{}
			require.NoError(b, ctx.Request.Read(bufio.NewReader(strings.NewReader(raw))))

			b.ReportAllocs()
			for b.Loop() {
				handler(ctx)
			}
			require.Equal(b, StatusOK, ctx.Response.StatusCode())
		})
	}
}

var testredirectionMsgs = redirectionMsgs{
	{
		key:   "success",
		value: "1",
	},
	{
		key:   "message",
		value: "test",
	},
	{
		key:        "name",
		value:      "tom",
		isOldInput: true,
	},
	{
		key:        "id",
		value:      "1",
		isOldInput: true,
	},
}

// go test -v -run=^$ -bench=Benchmark_Redirect_parseAndClearFlashMessages -benchmem -count=4
func Benchmark_Redirect_parseAndClearFlashMessages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))

	b.ReportAllocs()

	for b.Loop() {
		c.Redirect().parseAndClearFlashMessages()
	}

	require.Equal(b, FlashMessage{
		Key:   "success",
		Value: "1",
	}, c.Redirect().Message("success"))

	require.Equal(b, FlashMessage{
		Key:   "message",
		Value: "test",
	}, c.Redirect().Message("message"))

	require.Equal(b, OldInputData{
		Key:   "id",
		Value: "1",
	}, c.Redirect().OldInput("id"))

	require.Equal(b, OldInputData{
		Key:   "name",
		Value: "tom",
	}, c.Redirect().OldInput("name"))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_processFlashMessages -benchmem -count=4
func Benchmark_Redirect_processFlashMessages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Redirect().With("success", "1").With("message", "test")

	b.ReportAllocs()

	for b.Loop() {
		c.Redirect().processFlashMessages()
	}

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(b, err)
	_, err = msgs.UnmarshalMsg(decoded)
	require.NoError(b, err)

	require.Len(b, msgs, 2)
	require.Contains(b, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(b, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Messages -benchmem -count=4
func Benchmark_Redirect_Messages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var msgs []FlashMessage

	msgTemplate := testredirectionMsgs

	b.ReportAllocs()

	for b.Loop() {
		c.flashMessages = c.flashMessages[:0]
		c.flashMessages = append(c.flashMessages, msgTemplate...)
		msgs = c.Redirect().Messages()
	}

	require.Contains(b, msgs, FlashMessage{
		Key:   "success",
		Value: "1",
		Level: 0,
	})

	require.Contains(b, msgs, FlashMessage{
		Key:   "message",
		Value: "test",
		Level: 0,
	})
}

// go test -v -run=^$ -bench=Benchmark_Redirect_OldInputs -benchmem -count=4
func Benchmark_Redirect_OldInputs(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var oldInputs []OldInputData

	b.ReportAllocs()

	for b.Loop() {
		oldInputs = c.Redirect().OldInputs()
	}

	require.Contains(b, oldInputs, OldInputData{
		Key:   "name",
		Value: "tom",
	})

	require.Contains(b, oldInputs, OldInputData{
		Key:   "id",
		Value: "1",
	})
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Message -benchmem -count=4
func Benchmark_Redirect_Message(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var msg FlashMessage

	msgTemplate := testredirectionMsgs

	b.ReportAllocs()

	for b.Loop() {
		c.flashMessages = c.flashMessages[:0]
		c.flashMessages = append(c.flashMessages, msgTemplate...)
		msg = c.Redirect().Message("message")
	}

	require.Equal(b, FlashMessage{
		Key:   "message",
		Value: "test",
		Level: 0,
	}, msg)
}

// go test -v -run=^$ -bench=Benchmark_Redirect_OldInput -benchmem -count=4
func Benchmark_Redirect_OldInput(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var input OldInputData

	b.ReportAllocs()

	for b.Loop() {
		input = c.Redirect().OldInput("name")
	}

	require.Equal(b, OldInputData{
		Key:   "name",
		Value: "tom",
	}, input)
}

// go test -run Test_Redirect_FlashCookie_Attributes
func Test_Redirect_FlashCookie_Attributes(t *testing.T) {
	t.Parallel()

	// The flash payload is hex-encoded, not encrypted, and WithInput copies the
	// whole submitted form into it — including whatever the user typed into a
	// password field. It is only ever read back on the server, so it must not be
	// reachable from document.cookie, and must not travel in the clear when the
	// request that produced it was served over TLS.
	newApp := func() *App {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0"},
			},
		})
		app.Post("/login", func(c Ctx) error {
			return c.Redirect().With("error", "bad credentials").WithInput().To("/login")
		})
		return app
	}

	post := func(t *testing.T, app *App, scheme string) string {
		t.Helper()

		req := httptest.NewRequest(MethodPost, "/login", strings.NewReader("user=alice&password=hunter2"))
		req.Header.Set(HeaderContentType, MIMEApplicationForm)
		if scheme != "" {
			req.Header.Set(HeaderXForwardedProto, scheme)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)

		for _, cookie := range resp.Header.Values(HeaderSetCookie) {
			if strings.HasPrefix(cookie, FlashCookieName+"=") {
				return cookie
			}
		}
		t.Fatalf("no %s cookie in response", FlashCookieName)
		return ""
	}

	t.Run("plain http", func(t *testing.T) {
		t.Parallel()

		cookie := post(t, newApp(), "")
		require.Contains(t, cookie, "HttpOnly")
		// Secure would make the browser drop the cookie on a plaintext request.
		require.NotContains(t, cookie, "secure")
	})

	t.Run("forwarded https", func(t *testing.T) {
		t.Parallel()

		cookie := post(t, newApp(), "https")
		require.Contains(t, cookie, "HttpOnly")
		require.Contains(t, cookie, "secure")
	})
}

// go test -run Test_Redirect_FlashMessages_NoCrossRequestLeak
func Test_Redirect_FlashMessages_NoCrossRequestLeak(t *testing.T) {
	t.Parallel()

	// The two requests below are sequential regardless, which is what makes the
	// second reuse the first's pooled ctx. Running the test alongside others
	// does not weaken that: mutation-verified with both clears reverted, this
	// still fails under the full package's parallel load.
	app := New()

	app.Get("/read", func(c Ctx) error {
		var sb strings.Builder
		for _, m := range c.Redirect().Messages() {
			sb.WriteString("MSG:" + m.Key + "=" + m.Value + ";") //nolint:errcheck // strings.Builder writes never fail
		}
		for _, in := range c.Redirect().OldInputs() {
			sb.WriteString("OLD:" + in.Key + "=" + in.Value + ";") //nolint:errcheck // strings.Builder writes never fail
		}
		return c.SendString(sb.String())
	})
	// Deliberately does not consume the flash messages, so they are still in
	// the context's slice when it is released back to the pool.
	app.Get("/ignore", func(c Ctx) error { return c.SendString("ok") })

	victim := redirectionMsgs{
		{key: "user", value: "alice", isOldInput: true},
		{key: "password", value: "hunter2", isOldInput: true},
		{key: "error", value: "bad credentials"},
	}
	raw, err := victim.MarshalMsg(nil)
	require.NoError(t, err)

	get := func(t *testing.T, path, flashCookie string) string {
		t.Helper()

		req := httptest.NewRequest(MethodGet, path, http.NoBody)
		if flashCookie != "" {
			req.Header.Set(HeaderCookie, FlashCookieName+"="+flashCookie)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(body)
	}

	// One request carries real flash data and leaves it unread.
	require.Equal(t, "ok", get(t, "/ignore", hex.EncodeToString(raw)))

	// The next request asks for N messages encoded as empty msgp maps (0x9N
	// fixarray of 0x80 fixmaps). Those decode to zero values; if the pooled
	// slice were reused without being cleared, the fields absent from each map
	// would come back holding the previous request's data instead.
	for n := 1; n <= 4; n++ {
		payload := fmt.Sprintf("9%d%s", n, strings.Repeat("80", n))

		body := get(t, "/read", payload)
		require.NotContains(t, body, "alice")
		require.NotContains(t, body, "hunter2")
		require.NotContains(t, body, "bad credentials")
		require.Equal(t, strings.Repeat("MSG:=;", n), body)
	}
}

// go test -run Test_Redirect_Back_IgnoresHeaderNameCase
//
// Ctx.Get compares the stored key byte for byte, so under
// DisableHeaderNormalizing a "referer:" from a client behind an HTTP/2 or
// HTTP/3 front end — where lower case is what the wire carries — read as no
// referer at all, and Back went to the fallback instead of back.
func Test_Redirect_Back_IgnoresHeaderNameCase(t *testing.T) {
	t.Parallel()

	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()

			for _, sent := range []string{"Referer", "referer", "REFERER"} {
				app := New(Config{DisableHeaderNormalizing: !normalize})
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				if !normalize {
					c.Request().Header.DisableNormalizing()
				}
				c.Request().Header.Set(sent, "/previous")

				require.NoError(t, c.Redirect().Back("/fallback"))
				require.Equal(t, "/previous", string(c.Response().Header.Peek(HeaderLocation)),
					"sent as %q", sent)
				app.ReleaseCtx(c)
			}
		})
	}
}

// go test -run Test_Redirect_Back_RejectsAnUnusableReferer
//
// Every referer that cannot be resolved back to this origin falls back rather
// than being echoed into Location. These are the shapes that fail before the
// same-origin comparison is even reached: one that normalizes away to nothing,
// and one the URL parser refuses.
func Test_Redirect_Back_RejectsAnUnusableReferer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		referer string
	}{
		{name: "empty", referer: ""},
		// Leading and trailing C0 controls and spaces are stripped before
		// anything else, so these name no location at all.
		{name: "only spaces", referer: " "},
		{name: "only controls", referer: "\t\n "},
		// url.Parse refuses these outright.
		{name: "bad percent escape", referer: "%zz"},
		{name: "control byte", referer: "http://a\x7fb"},
		{name: "space in the scheme", referer: "ht tp://x"},
		{name: "no scheme before the colon", referer: "://x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)

			if tc.referer != "" {
				c.Request().Header.Set(HeaderReferer, tc.referer)
			}

			require.NoError(t, c.Redirect().Back("/fallback"))
			require.Equal(t, "/fallback", string(c.Response().Header.Peek(HeaderLocation)))
		})
	}

	t.Run("no referer at all, with names left as sent", func(t *testing.T) {
		t.Parallel()

		// The walk that matches the name case-insensitively finds nothing here,
		// which is the same answer as the byte-for-byte lookup.
		app := New(Config{DisableHeaderNormalizing: true})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)

		c.Request().Header.DisableNormalizing()
		c.Request().Header.Set("X-Not-A-Referer", "/nope")

		require.NoError(t, c.Redirect().Back("/fallback"))
		require.Equal(t, "/fallback", string(c.Response().Header.Peek(HeaderLocation)))
	})

	t.Run("no referer and no fallback is an error", func(t *testing.T) {
		t.Parallel()

		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)

		require.ErrorIs(t, c.Redirect().Back(), ErrRedirectBackNoFallback)
	})
}

// Test_Redirect_parseAndClearFlashMessages_UndecodablePayload covers a cookie
// that decodes as hex but is not what this application wrote.
//
// UnmarshalMsg re-slices to the length the payload declares before it decodes
// any element, so a failure partway leaves the slice holding zero-valued
// entries. Reading those back would hand the handler empty messages nobody set,
// which is why the slice is dropped rather than kept at whatever length the
// payload claimed.
func Test_Redirect_parseAndClearFlashMessages_UndecodablePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
	}{
		// An array header claiming four elements, and then nothing.
		{name: "truncated after the count", payload: []byte{0x94}},
		// A byte message-pack never assigns.
		{name: "never a valid type", payload: []byte{0xc1, 0xc1}},
		// A long claimed length with no elements behind it, which is the shape
		// that leaves the most zero-valued entries.
		{name: "claims more than it carries", payload: []byte{0xdc, 0x01, 0x00}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
			defer app.ReleaseCtx(c)

			r := AcquireRedirect()
			r.c = c
			defer ReleaseRedirect(r)

			c.Request().Header.SetCookie(FlashCookieName, hex.EncodeToString(tc.payload))

			r.parseAndClearFlashMessages()

			require.Empty(t, r.messages)
			require.Empty(t, c.flashMessages, "a payload that failed to decode leaves nothing behind")
			assertFlashCookieCleared(t, c.GetRespHeader(HeaderSetCookie))
		})
	}
}
