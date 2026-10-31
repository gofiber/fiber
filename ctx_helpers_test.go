// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Splitting a handler into func(fiber.Req) and func(fiber.Res) and passing the
// same Ctx to both is the reason the split exists, so Ctx has to satisfy each.
// A name carried by Req and Res under different signatures breaks that on
// assignment, which no test of the concrete types would catch.
var (
	_ Req = (*DefaultCtx)(nil)
	_ Res = (*DefaultCtx)(nil)
	_ Req = Ctx(nil)
	_ Res = Ctx(nil)
)

// ctxReqFor returns a context of a default app for the request that prepare builds.
func ctxReqFor(t *testing.T, prepare func(req *fasthttp.Request)) Ctx {
	t.Helper()
	fctx := &fasthttp.RequestCtx{}
	prepare(&fctx.Request)
	return apptest.CtxFor(t, New(), fctx)
}

func Test_Req_RequestHelpersOnReq(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) {
		req.SetRequestURI("/path/here?q=1")
		req.Header.SetMethod(MethodPost)
		req.Header.SetHost("example.com")
		req.Header.SetUserAgent("fiber-test")
		req.Header.SetReferer("https://referer.example")
		req.Header.Set(HeaderAcceptLanguage, "en-US, de")
		req.Header.Set(HeaderAcceptEncoding, "gzip, br")
		req.Header.Set(HeaderAccept, MIMEApplicationJSON)
		req.Header.SetContentType(MIMEApplicationJSON + "; charset=utf-8")
		req.SetBody([]byte(`{"a":1}`))
	})

	r := c.Req()

	require.Equal(t, "http://example.com/path/here?q=1", r.FullURL())
	require.Equal(t, "fiber-test", r.UserAgent())
	require.Equal(t, "https://referer.example", r.Referer())
	require.Equal(t, "en-US, de", r.AcceptLanguage())
	require.Equal(t, "gzip, br", r.AcceptEncoding())
	require.True(t, r.HasHeader(HeaderUserAgent))
	require.False(t, r.HasHeader("X-Absent"))
	require.Equal(t, MIMEApplicationJSON, r.MediaType()) //nolint:testifylint // this is comparing content-type strings, not JSON content
	require.Equal(t, "utf-8", r.Charset())
	require.True(t, r.IsJSON())
	require.False(t, r.IsForm())
	require.False(t, r.IsMultipart())
	require.True(t, r.AcceptsJSON())
	require.False(t, r.AcceptsHTML())
	require.False(t, r.AcceptsXML())
	require.False(t, r.AcceptsEventStream())

	require.Equal(t, "/path/here", r.Path())
	require.False(t, r.Secure())
	require.False(t, r.XHR())
	require.True(t, r.HasBody())
	require.False(t, r.IsWebSocket())
	require.False(t, r.IsPreflight())

	require.Equal(t, r.FullURL(), c.FullURL())
	require.Equal(t, r.UserAgent(), c.UserAgent())
	require.Equal(t, r.MediaType(), c.MediaType())
	require.Equal(t, r.Path(), c.Path())
	require.Equal(t, r.HasBody(), c.HasBody())
	require.Equal(t, r.IsPreflight(), c.IsPreflight())
}

func Test_Req_PathOverride(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) { req.SetRequestURI("/original") })

	require.Equal(t, "/original", c.Req().Path())
	require.Equal(t, "/rewritten", c.Req().Path("/rewritten"))
	require.Equal(t, "/rewritten", c.Path())
	require.Equal(t, "/rewritten", string(c.Request().URI().Path()))
}

func Test_Req_GetAll(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) {
		req.Header.Add("X-Test", "first")
		req.Header.Add("X-Test", "second")
	})

	require.Equal(t, []string{"first", "second"}, c.Req().GetAll("X-Test"))
	require.Equal(t, []string{"first", "second"}, c.Req().GetAll("x-test"))
	require.Equal(t, "first", c.Req().Get("X-Test"))
	require.Nil(t, c.Req().GetAll("X-Absent"))
}

func Test_Req_ContentLength(t *testing.T) {
	t.Parallel()
	app := New()

	var length int
	app.Post("/", func(c Ctx) error {
		length = c.Req().ContentLength()
		return nil
	})

	apptest.Do(t, app, httptest.NewRequest(MethodPost, "/", strings.NewReader("0123456789")))
	require.Equal(t, 10, length)

	cc := ctxReqFor(t, func(req *fasthttp.Request) { req.Header.SetContentLength(-1) })

	require.Equal(t, -1, cc.Req().ContentLength(), "chunked bodies report an unknown length")
	require.True(t, cc.Req().HasBody(), "an unknown length still means there is a body")

	var bodyless int
	app.Get("/", func(c Ctx) error {
		bodyless = c.Req().ContentLength()
		return nil
	})
	apptest.Do(t, app, "/")
	require.Equal(t, -2, bodyless)
}

func Test_Req_GetAll_AbsentSlottedHeaders(t *testing.T) {
	t.Parallel()
	app := New()

	var got map[string][]string
	var has map[string]bool
	app.Get("/", func(c Ctx) error {
		got = map[string][]string{}
		has = map[string]bool{}
		for _, key := range []string{HeaderContentLength, HeaderTrailer, "X-Absent"} {
			got[key] = c.Req().GetAll(key)
			has[key] = c.Req().HasHeader(key)
		}
		return nil
	})

	apptest.Do(t, app, "/")

	for key, lines := range got {
		require.Nil(t, lines, key)
		require.False(t, has[key], key)
		require.Equal(t, has[key], len(lines) > 0, "GetAll and HasHeader must agree on %s", key)
	}
}

func Test_Req_ContentType(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) { req.Header.SetContentType(MIMEApplicationJSON + "; charset=utf-8") })

	require.Equal(t, MIMEApplicationJSONCharsetUTF8, c.Req().ContentType())                      //nolint:testifylint // this is comparing content-type headers, not JSON content
	require.Equal(t, MIMEApplicationJSON, c.Req().MediaType(), "MediaType drops the parameters") //nolint:testifylint // same
	require.Equal(t, "utf-8", c.Req().Charset())

	require.NoError(t, c.SendString("hi"))
	c.Type("html")
	require.Equal(t, c.Req().ContentType(), c.ContentType())
	require.Equal(t, MIMETextHTMLCharsetUTF8, c.Res().ContentType())
}

func Test_Req_BodyStream(t *testing.T) {
	t.Parallel()
	bc := ctxReqFor(t, func(req *fasthttp.Request) { req.SetBody([]byte("buffered")) })
	require.Nil(t, bc.Req().BodyStream(), "a buffered body is not a stream")

	sc := ctxReqFor(t, func(req *fasthttp.Request) { req.SetBodyStream(strings.NewReader("streamed"), 8) })

	stream := sc.Req().BodyStream()
	require.NotNil(t, stream)
	body, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.Equal(t, "streamed", string(body))
}

func Test_Req_URI(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) {
		req.SetRequestURI("/search?q=fiber#frag")
		req.Header.SetHost("example.com")
	})

	uri := c.Req().URI()
	require.NotNil(t, uri)
	require.Equal(t, "/search", string(uri.Path()))
	require.Equal(t, "q=fiber", string(uri.QueryString()))
	require.Equal(t, "frag", string(uri.Hash()))
	require.Equal(t, "example.com", string(uri.Host()))
}

func Test_Req_Origin(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) { req.Header.Set(HeaderOrigin, "https://example.com") })
	require.Equal(t, "https://example.com", c.Req().Origin())

	bare := apptest.Ctx(t, New())
	require.Empty(t, bare.Req().Origin())
}

func Test_Req_Authorization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		header      string
		scheme      string
		credentials string
		bearer      string
	}{
		{name: "absent"},
		{name: "bearer", header: "Bearer abc123", scheme: "Bearer", credentials: "abc123", bearer: "abc123"},
		{name: "bearer lowercase scheme", header: "bearer abc123", scheme: "bearer", credentials: "abc123", bearer: "abc123"},
		{name: "basic", header: "Basic dXNlcjpwYXNz", scheme: "Basic", credentials: "dXNlcjpwYXNz"},
		{name: "auth-param list", header: `Digest username="u", realm="r"`, scheme: "Digest", credentials: `username="u", realm="r"`},
		{name: "extra whitespace", header: "  Bearer   abc123  ", scheme: "Bearer", credentials: "abc123", bearer: "abc123"},
		{name: "tab separator", header: "Bearer\tabc123", scheme: "Bearer", credentials: "abc123", bearer: "abc123"},
		{name: "scheme only", header: "Negotiate", scheme: "Negotiate"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := ctxReqFor(t, func(req *fasthttp.Request) {
				if tc.header != "" {
					req.Header.Set(HeaderAuthorization, tc.header)
				}
			})

			scheme, credentials := c.Req().Authorization()
			require.Equal(t, tc.scheme, scheme)
			require.Equal(t, tc.credentials, credentials)
			require.Equal(t, tc.bearer, c.Req().Bearer())
		})
	}
}

func Test_Req_IsSafe_IsIdempotent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method     string
		safe       bool
		idempotent bool
	}{
		{method: MethodGet, safe: true, idempotent: true},
		{method: MethodHead, safe: true, idempotent: true},
		{method: MethodOptions, safe: true, idempotent: true},
		{method: MethodTrace, safe: true, idempotent: true},
		{method: MethodPut, safe: false, idempotent: true},
		{method: MethodDelete, safe: false, idempotent: true},
		{method: MethodPost, safe: false, idempotent: false},
		{method: MethodPatch, safe: false, idempotent: false},
	}

	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			t.Parallel()
			c := ctxReqFor(t, func(req *fasthttp.Request) { req.Header.SetMethod(tc.method) })

			require.Equal(t, tc.safe, c.Req().IsSafe())
			require.Equal(t, tc.idempotent, c.Req().IsIdempotent())
		})
	}
}

func Test_Req_CookieNames_AllCookies(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) {
		req.Header.SetCookie("session", "abc")
		req.Header.SetCookie("theme", "dark")
	})

	require.ElementsMatch(t, []string{"session", "theme"}, c.Req().CookieNames())
	require.Equal(t, map[string]string{"session": "abc", "theme": "dark"}, c.Req().AllCookies())

	sc := ctxReqFor(t, func(req *fasthttp.Request) { req.Header.Set(HeaderCookie, "a=first; a=second") })

	require.Equal(t, "first", sc.Req().Cookies("a"))
	require.Equal(t, "first", sc.Req().AllCookies()["a"])
	require.Equal(t, []string{"a", "a"}, sc.Req().CookieNames(), "the repetition itself stays visible")

	bare := apptest.Ctx(t, New())
	require.Empty(t, bare.Req().CookieNames())
	require.Empty(t, bare.Req().AllCookies())
}

func Test_Req_IfNoneMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		lines []string // the If-None-Match field lines
		want  []string
	}{
		{name: "absent"},
		{name: "single", lines: []string{`"abc"`}, want: []string{`"abc"`}},
		{name: "list", lines: []string{`"a", W/"b"`}, want: []string{`"a"`, `W/"b"`}},
		{name: "wildcard", lines: []string{"*"}, want: []string{"*"}},
		{name: "comma inside tag", lines: []string{`"v1,v2"`}, want: []string{`"v1,v2"`}},
		{name: "multiple field lines", lines: []string{`"a"`, `"b"`}, want: []string{`"a"`, `"b"`}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := ctxReqFor(t, func(req *fasthttp.Request) {
				for _, line := range tc.lines {
					req.Header.Add(HeaderIfNoneMatch, line)
				}
			})

			require.Equal(t, tc.want, c.Req().IfNoneMatch())
		})
	}
}

func Test_Req_IfModifiedSince(t *testing.T) {
	t.Parallel()
	bare := apptest.Ctx(t, New())
	_, err := bare.Req().IfModifiedSince()
	require.ErrorIs(t, err, ErrHeaderNotFound)

	c := ctxReqFor(t, func(req *fasthttp.Request) {
		req.Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
	})

	got, err := c.Req().IfModifiedSince()
	require.NoError(t, err)
	require.True(t, got.Equal(time.Date(2015, time.October, 21, 7, 28, 0, 0, time.UTC)))

	mc := ctxReqFor(t, func(req *fasthttp.Request) { req.Header.Set(HeaderIfModifiedSince, "not a date") })

	_, err = mc.Req().IfModifiedSince()
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrHeaderNotFound, "a malformed date is not an absent header")
}

func Test_Res_StatusCode(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Equal(t, StatusOK, c.Res().StatusCode(), "an untouched response reports 200")
	c.Status(StatusTeapot)
	require.Equal(t, StatusTeapot, c.Res().StatusCode())
	require.Equal(t, c.Response().StatusCode(), c.Res().StatusCode())
}

func Test_Res_Body_ResetBody_Written(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Empty(t, c.Res().Body())
	require.False(t, c.Res().Written())

	c.Status(StatusCreated)
	require.False(t, c.Res().Written())

	require.NoError(t, c.SendString("hello"))
	require.Equal(t, "hello", string(c.Res().Body()))
	require.True(t, c.Res().Written())

	c.Res().ResetBody()
	require.Empty(t, c.Res().Body())
	require.False(t, c.Res().Written())
	require.Equal(t, StatusCreated, c.Res().StatusCode(), "ResetBody keeps the status")
}

func Test_Res_Del(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Set("X-Custom", "value")
	require.Equal(t, "value", c.Res().Get("X-Custom"))
	c.Res().Del("x-custom")
	require.Empty(t, c.Res().Get("X-Custom"))

	require.NotPanics(t, func() { c.Res().Del("X-Absent") })

	c.Res().Add("X-Multi", "a")
	c.Res().Add("X-Multi", "b")
	require.Len(t, c.Response().Header.PeekAll("X-Multi"), 2)
	c.Res().Del("X-Multi")
	require.Empty(t, c.Response().Header.PeekAll("X-Multi"))

	c.Cookie(&Cookie{Name: "a", Value: "1"})
	require.NotEmpty(t, c.Res().GetCookies())
	c.Res().Del(HeaderSetCookie)
	require.Empty(t, c.Res().GetCookies())
}

func Test_Res_Add(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Res().Add(HeaderWWWAuthenticate, `Basic realm="one"`)
	c.Res().Add(HeaderWWWAuthenticate, `Bearer realm="two"`)

	lines := c.Response().Header.PeekAll(HeaderWWWAuthenticate)
	require.Len(t, lines, 2, "Add keeps challenges on separate field lines")
	require.Equal(t, `Basic realm="one"`, string(lines[0]))
	require.Equal(t, `Bearer realm="two"`, string(lines[1]))

	c.Append("X-Folded", "a")
	c.Append("X-Folded", "b")
	require.Len(t, c.Response().Header.PeekAll("X-Folded"), 1)
	require.Equal(t, "a, b", c.Res().Get("X-Folded"))
}

func Test_Res_ContentType_ContentLength(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Equal(t, MIMETextPlainCharsetUTF8, c.Res().ContentType(), "an untouched response reports fasthttp's default")

	require.NoError(t, c.JSON(Map{"a": 1}))
	require.Equal(t, MIMEApplicationJSONCharsetUTF8, c.Res().ContentType()) //nolint:testifylint // this is comparing content-type headers, not JSON content

	c.Type("html")
	require.Equal(t, MIMETextHTMLCharsetUTF8, c.Res().ContentType(), "ContentType reads back what Type set")

	require.NotEmpty(t, c.Res().Body())
	require.Equal(t, 0, c.Res().ContentLength())

	c.Set(HeaderContentLength, "42")
	require.Equal(t, 42, c.Res().ContentLength())
}

func Test_Res_GetCookie_Cookies(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	_, ok := c.Res().GetCookie("absent")
	require.False(t, ok)
	require.Empty(t, c.Res().GetCookies())

	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	c.Cookie(&Cookie{
		Name:     "session",
		Value:    "abc",
		Path:     "/scoped",
		Domain:   "example.com",
		MaxAge:   60,
		Expires:  expires,
		Secure:   true,
		HTTPOnly: true,
		SameSite: CookieSameSiteStrictMode,
	})

	got, ok := c.Res().GetCookie("session")
	require.True(t, ok)
	require.Equal(t, "session", got.Name)
	require.Equal(t, "abc", got.Value)
	require.Equal(t, "/scoped", got.Path)
	require.Equal(t, "example.com", got.Domain)
	require.Equal(t, 60, got.MaxAge)
	require.True(t, got.Secure)
	require.True(t, got.HTTPOnly)
	require.Equal(t, CookieSameSiteStrictMode, got.SameSite)
	require.False(t, got.SessionOnly)

	c.Cookie(&Cookie{Name: "flash", Value: "x", SessionOnly: true})
	flash, ok := c.Res().GetCookie("flash")
	require.True(t, ok)
	require.True(t, flash.SessionOnly)
	require.Zero(t, flash.MaxAge)
	require.True(t, flash.Expires.IsZero())

	cookies := c.Res().GetCookies()
	require.Len(t, cookies, 2)
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	require.ElementsMatch(t, []string{"session", "flash"}, names)

	got.Value = "tampered"
	again, ok := c.Res().GetCookie("session")
	require.True(t, ok)
	require.Equal(t, "abc", again.Value)
}

func Test_Res_Cookies_RepeatedNames(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Cookie(&Cookie{Name: "sid", Value: "app", Path: "/app"})
	c.Res().Add(HeaderSetCookie, "sid=admin; path=/admin")
	c.Cookie(&Cookie{Name: "other", Value: "z"})

	cookies := c.Res().GetCookies()
	require.Len(t, cookies, 3)

	type pair struct{ value, path string }
	got := make([]pair, 0, len(cookies))
	for _, cookie := range cookies {
		got = append(got, pair{cookie.Value, cookie.Path})
	}
	require.ElementsMatch(t, []pair{{"app", "/app"}, {"admin", "/admin"}, {"z", "/"}}, got)

	first, ok := c.Res().GetCookie("sid")
	require.True(t, ok)
	require.Equal(t, "app", first.Value)

	over := New()
	over.Get("/", func(c Ctx) error {
		c.Cookie(&Cookie{Name: "sid", Value: "app", Path: "/app"})
		c.Res().Add(HeaderSetCookie, "sid=admin; path=/admin")
		c.Cookie(&Cookie{Name: "other", Value: "z"})
		return nil
	})
	resp := apptest.Do(t, over, "/")
	require.Len(t, resp.Header.Values(HeaderSetCookie), 3)
}

func Test_Res_GetCookie_Deletion(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Cookie(&Cookie{Name: "session", Value: "v", MaxAge: -1})
	require.Contains(t, string(c.Response().Header.Peek(HeaderSetCookie)), "max-age=0")

	got, ok := c.Res().GetCookie("session")
	require.True(t, ok)
	require.False(t, got.SessionOnly, "an explicit max-age=0 is a deletion, not a session cookie")
	require.Negative(t, got.MaxAge)

	got.Domain = "example.com"
	c.Cookie(got)
	require.Contains(t, string(c.Response().Header.Peek(HeaderSetCookie)), "max-age=0")

	rewritten, ok := c.Res().GetCookie("session")
	require.True(t, ok)
	require.False(t, rewritten.SessionOnly)
	require.Equal(t, "example.com", rewritten.Domain)
}

func Test_Res_Body_DoesNotDrainStream(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.SendStream(strings.NewReader("streamed"), 8))
	require.True(t, c.Res().Written())
	require.Nil(t, c.Res().Body(), "a streamed body is not materialized")
	require.True(t, c.Response().IsBodyStream(), "and it is still a stream afterwards")
}

func Test_Res_Add_SpecialHeaders(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Set(HeaderContentType, MIMEApplicationJSON)
	c.Res().Add(HeaderContentType, MIMETextPlain)
	require.Equal(t, MIMETextPlain, c.Res().ContentType(), "Content-Type is replaced, not appended")
	require.Len(t, c.Response().Header.PeekAll(HeaderContentType), 1)

	c.Res().Add(HeaderLink, "</a>; rel=preload")
	c.Res().Add(HeaderLink, "</b>; rel=preload")
	require.Len(t, c.Response().Header.PeekAll(HeaderLink), 2)
}

func Test_Res_GetCookie_RoundTrip(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Cookie(&Cookie{Name: "session", Value: "plain", Path: "/app", HTTPOnly: true, SameSite: CookieSameSiteLaxMode})

	cookie, ok := c.Res().GetCookie("session")
	require.True(t, ok)
	cookie.Value = "encrypted"
	c.Cookie(cookie)

	rewritten, ok := c.Res().GetCookie("session")
	require.True(t, ok)
	require.Equal(t, "encrypted", rewritten.Value)
	require.Equal(t, "/app", rewritten.Path, "the other attributes survive the round trip")
	require.True(t, rewritten.HTTPOnly)
	require.Equal(t, CookieSameSiteLaxMode, rewritten.SameSite)
	require.Len(t, c.Res().GetCookies(), 1, "rewriting replaces the cookie rather than adding one")
}

func Test_Res_NoContent(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.JSON(Map{"a": 1}))
	require.NotEmpty(t, c.Res().Body())

	require.NoError(t, c.Res().NoContent())
	require.Equal(t, StatusNoContent, c.Res().StatusCode())
	require.Empty(t, c.Res().Body())
	require.False(t, c.Res().Written())
	require.NotEqual(t, MIMEApplicationJSONCharsetUTF8, c.Res().ContentType(),
		"the handler's Content-Type is gone; Test_Res_NoContent_OverHTTP pins that none is sent")
}

func Test_Res_NoContent_OverHTTP(t *testing.T) {
	t.Parallel()
	app := New()
	app.Delete("/item", func(c Ctx) error { return c.Res().NoContent() })
	app.Get("/sendstatus", func(c Ctx) error {
		c.Type("json")
		return c.SendStatus(StatusNoContent)
	})
	app.Get("/nocontent", func(c Ctx) error {
		c.Type("json")
		return c.Res().NoContent()
	})

	for _, tc := range []struct {
		name            string
		method          string
		path            string
		wantContentType bool // SendStatus keeps the Content-Type the handler set, NoContent drops it
	}{
		{name: "NoContent", method: MethodDelete, path: "/item"},
		{name: "NoContent after Type", method: MethodGet, path: "/nocontent"},
		{name: "SendStatus after Type", method: MethodGet, path: "/sendstatus", wantContentType: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.ExpectBody(t, app, apptest.Req(tc.method, tc.path), StatusNoContent, "")
			if tc.wantContentType {
				require.NotEmpty(t, resp.Header.Get(HeaderContentType))
			} else {
				require.Empty(t, resp.Header.Get(HeaderContentType))
			}
		})
	}
}

func Test_Ctx_BodyContentLengthCookiesPreferRequest(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) {
		req.SetBody([]byte("request body"))
		req.Header.SetContentLength(len("request body"))
		req.Header.SetCookie("who", "client")
	})

	require.NoError(t, c.SendString("response body"))
	c.Cookie(&Cookie{Name: "who", Value: "server"})

	require.Equal(t, "request body", string(c.Body()))
	require.Equal(t, "request body", string(c.Req().Body()))
	require.Equal(t, "response body", string(c.Res().Body()))

	require.Equal(t, len("request body"), c.ContentLength())
	require.Equal(t, len("request body"), c.Req().ContentLength())

	require.Equal(t, "client", c.Cookies("who"))
	require.Equal(t, "client", c.Req().Cookies("who"))
	require.Equal(t, "fallback", c.Cookies("absent", "fallback"))

	serverCookies := c.Res().GetCookies()
	require.Len(t, serverCookies, 1)
	require.Equal(t, "server", serverCookies[0].Value)

	require.Equal(t, c.Req().ContentType(), c.ContentType())
}

func Test_Ctx_ID_StartTime_Elapsed(t *testing.T) {
	t.Parallel()
	app := New()

	var (
		id      uint64
		stable  bool
		start   time.Time
		elapsed time.Duration
	)
	app.Get("/", func(c Ctx) error {
		id = c.ID()
		stable = c.ID() == id
		start = c.StartTime()
		elapsed = c.Elapsed()
		return nil
	})

	apptest.Do(t, app, "/")

	require.NotZero(t, id)
	require.True(t, stable, "ID is stable within a request")
	require.False(t, start.IsZero())
	require.GreaterOrEqual(t, elapsed, time.Duration(0))
	require.Less(t, elapsed, time.Minute, "Elapsed measures from StartTime, not from the epoch")
}

func Test_Ctx_LocalAddr_RemoteAddr(t *testing.T) {
	t.Parallel()
	app := New()

	var local, remote net.Addr
	app.Get("/", func(c Ctx) error {
		local = c.LocalAddr()
		remote = c.RemoteAddr()
		return nil
	})

	apptest.Do(t, app, "/")

	require.NotNil(t, local)
	require.NotNil(t, remote)
	require.NotEmpty(t, remote.Network())
}

func Test_Ctx_Hijack(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.False(t, c.Hijacked())
	c.Hijack(func(net.Conn) {})
	require.True(t, c.Hijacked())
}

func Test_Ctx_RouteName(t *testing.T) {
	t.Parallel()
	app := New()

	var mwName, handlerName string
	app.Use(func(c Ctx) error {
		mwName = c.RouteName()
		return c.Next()
	})
	app.Get("/home", func(c Ctx) error {
		handlerName = c.RouteName()
		return nil
	}).Name("home")

	apptest.Do(t, app, "/home")

	require.Equal(t, "home", handlerName)
	require.Empty(t, mwName, "middleware reports its own unnamed route, not the endpoint's")
}

func Test_Ctx_RouteName_Unmatched(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NotPanics(t, func() { require.Empty(t, c.RouteName()) })
}

func Test_Ctx_IsFinal(t *testing.T) {
	t.Parallel()
	app := New()

	var (
		inGlobalMW bool
		inRouteMW  bool
		inHandler  bool
	)
	app.Use(func(c Ctx) error {
		inGlobalMW = c.IsFinal()
		return c.Next()
	})
	app.Get("/chain", func(c Ctx) error {
		inRouteMW = c.IsFinal()
		return c.Next()
	}, func(c Ctx) error {
		inHandler = c.IsFinal()
		return nil
	})

	apptest.Do(t, app, "/chain")

	require.False(t, inGlobalMW)
	require.False(t, inRouteMW, "a route handler with another after it is not final")
	require.True(t, inHandler)

	c := apptest.Ctx(t, app)
	require.False(t, c.IsFinal())
	require.False(t, c.IsMiddleware())
}

func Test_Ctx_IsFinal_ScopedToTheRoute(t *testing.T) {
	t.Parallel()
	terminal := New()
	var inUse bool
	terminal.Use(func(c Ctx) error {
		inUse = c.IsFinal()
		return c.SendString("done")
	})
	apptest.Do(t, terminal, "/anything")
	require.False(t, inUse, "a Use route is middleware by registration, whatever its position")

	overlapping := New()
	var inSpecific, inCatchAll bool
	overlapping.Get("/specific", func(c Ctx) error {
		inSpecific = c.IsFinal()
		return c.Next()
	})
	overlapping.Get("/*", func(_ Ctx) error {
		inCatchAll = true
		return nil
	})
	apptest.Do(t, overlapping, "/specific")
	require.True(t, inSpecific, "it is the last handler of its own route")
	require.True(t, inCatchAll, "but another route still matched after it")
}

func Test_Ctx_MountPath(t *testing.T) {
	t.Parallel()
	micro := New()
	var mounted, mountedPath string
	micro.Get("/doe", func(c Ctx) error {
		mounted = c.MountPath()
		mountedPath = strings.Clone(c.Path())
		return nil
	})

	app := New()
	var top string
	app.Get("/top", func(c Ctx) error {
		top = c.MountPath()
		return nil
	})
	app.Use("/john", micro)

	apptest.Do(t, app, "/john/doe")
	apptest.Do(t, app, "/top")

	var standalone string
	micro.Get("/solo", func(c Ctx) error {
		standalone = c.MountPath()
		return nil
	})
	apptest.Do(t, micro, "/solo")
	require.Empty(t, standalone)
	require.Equal(t, "/john", micro.MountPath(), "App.MountPath still reports the mount")

	require.Equal(t, "/john", mounted)
	require.Equal(t, "/john/doe", mountedPath,
		"Fiber bakes the prefix into the cloned route, so Path is not relative to the mount")
	require.True(t, strings.HasPrefix(mountedPath, mounted), "the served path lives under MountPath")
	require.Empty(t, top, "the top-level app is not mounted under anything")
}

func Test_Ctx_Error(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	err := c.Error(StatusTeapot)
	var fiberErr *Error
	require.ErrorAs(t, err, &fiberErr)
	require.Equal(t, StatusTeapot, fiberErr.Code)
	require.Equal(t, utils.StatusMessage(StatusTeapot), fiberErr.Message)

	err = c.Error(StatusBadRequest, "id must be numeric")
	require.ErrorAs(t, err, &fiberErr)
	require.Equal(t, StatusBadRequest, fiberErr.Code)
	require.Equal(t, "id must be numeric", fiberErr.Message)

	require.Equal(t, StatusOK, c.Res().StatusCode(), "Error builds an error, it does not write the response")
	require.False(t, c.Res().Written())
}

func Test_Ctx_Error_ReachesErrorHandler(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.Error(StatusBadRequest, "id must be numeric") })

	apptest.ExpectBody(t, app, "/", StatusBadRequest, "id must be numeric")
}

// rootMiscARawHeaderCtx returns a context for a request that keeps the header
// names exactly as prepare sets them, on an app with DisableHeaderNormalizing.
// That is the setup in which a lookup has to match a name whatever its case.
func rootMiscARawHeaderCtx(t *testing.T, prepare func(req *fasthttp.Request)) Ctx {
	t.Helper()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	prepare(&fctx.Request)

	return apptest.CtxFor(t, New(Config{DisableHeaderNormalizing: true}), fctx)
}

func Test_Req_HeaderFieldWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.Set("origin", "https://example.com")
		req.Header.Set("authorization", "Bearer abc123")
	})

	require.Equal(t, "https://example.com", c.Get(HeaderOrigin))
	require.Equal(t, "https://example.com", c.Req().Origin())

	scheme, credentials := c.Req().Authorization()
	require.Equal(t, "Bearer", scheme)
	require.Equal(t, "abc123", credentials)
	require.Equal(t, "abc123", c.Req().Bearer())
}

func Test_Req_ConditionalHeadersWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.Set("if-none-match", `"a", W/"b"`)
		req.Header.Set("if-modified-since", "Wed, 21 Oct 2015 07:28:00 GMT")
	})

	require.Equal(t, []string{`"a"`, `W/"b"`}, c.Req().IfNoneMatch())

	since, err := c.Req().IfModifiedSince()
	require.NoError(t, err)
	require.True(t, since.Equal(time.Date(2015, time.October, 21, 7, 28, 0, 0, time.UTC)))
}

func Test_Req_IsPreflightWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.SetMethod(MethodOptions)
		req.Header.Set("origin", "https://example.com")
		req.Header.Set("access-control-request-method", MethodGet)
	})

	require.True(t, c.Req().IsPreflight())
}

func Test_Res_Del_CaseInsensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Response.Header.DisableNormalizing()
	c := apptest.CtxFor(t, app, fctx)

	c.Response().Header.Set("x-token", "secret")
	require.Equal(t, "secret", string(c.Response().Header.Peek("x-token")))

	c.Res().Del(HeaderXRequestID)
	c.Res().Del("X-Token")
	require.Empty(t, string(c.Response().Header.Peek("x-token")), "a differently cased field is still removed")
}

func Test_Res_Cookies_UnparsableEntry(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Cookie(&Cookie{Name: "good", Value: "1"})
	c.Res().Add(HeaderSetCookie, "")

	cookies := c.Res().GetCookies()
	require.Len(t, cookies, 1, "the unparsable entry is skipped")
	require.Equal(t, "good", cookies[0].Name)

	_, ok := c.Res().GetCookie("")
	require.False(t, ok, "GetCookie reports failure rather than a half-parsed cookie")

	good, ok := c.Res().GetCookie("good")
	require.True(t, ok)
	require.Equal(t, "1", good.Value)
}

func Test_Res_GetCookie_NoAttributes(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Res().Add(HeaderSetCookie, "plain=value")

	got, ok := c.Res().GetCookie("plain")
	require.True(t, ok)
	require.Equal(t, "value", got.Value)
	require.Empty(t, got.Path)
	require.Zero(t, got.MaxAge)
	require.True(t, got.SessionOnly, "no Max-Age and no Expires is a session cookie")
}

func Test_Ctx_MountPath_NoApp(t *testing.T) {
	t.Parallel()
	var c DefaultCtx
	require.NotPanics(t, func() { require.Empty(t, c.MountPath()) })
}

func Test_Req_HasHeaderWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) { req.Header.Set("x-test", "yes") })

	require.True(t, c.Req().HasHeader("X-Test"))
	require.Equal(t, []string{"yes"}, c.Req().GetAll("X-Test"))
	require.True(t, c.Req().HasHeader("x-test"))
	require.False(t, c.Req().HasHeader("X-Absent"))
}

func Test_Res_SendStatus_DropsDeclaredContentLength(t *testing.T) {
	t.Parallel()
	app := New()
	for _, status := range []int{StatusNoContent, StatusResetContent, StatusNotModified} {
		app.Get("/set"+strconv.Itoa(status), func(c Ctx) error {
			c.Set(HeaderContentLength, "42")
			return c.SendStatus(status)
		})
		app.Get("/bare"+strconv.Itoa(status), func(c Ctx) error { return c.SendStatus(status) })
	}
	app.Get("/nocontent", func(c Ctx) error {
		c.Set(HeaderContentLength, "42")
		return c.Res().NoContent()
	})

	get := func(path string) []string {
		resp := apptest.Do(t, app, path)
		require.Empty(t, apptest.BodyBytes(t, resp), path)
		require.NotContains(t, resp.Header.Values(HeaderContentLength), "42", path)
		return resp.Header.Values(HeaderContentLength)
	}

	for _, status := range []int{StatusNoContent, StatusResetContent, StatusNotModified} {
		require.Equal(t, get("/bare"+strconv.Itoa(status)), get("/set"+strconv.Itoa(status)), status)
	}

	require.Empty(t, get("/nocontent"))
	require.Equal(t, []string{"0"}, get("/bare205"))
}

func Test_Req_AcceptsWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.Set("accept", "application/json")
		req.Header.Set("accept-charset", "utf-8")
		req.Header.Set("accept-encoding", "gzip")
		req.Header.Set("accept-language", "en-GB")
	})

	r := c.Req()
	require.Equal(t, "gzip", r.AcceptEncoding())
	require.Equal(t, "en-GB", r.AcceptLanguage())

	require.Equal(t, "application/json", r.Accepts("application/json", "text/html"))
	require.Empty(t, r.Accepts("text/html"))
	require.Equal(t, "utf-8", r.AcceptsCharsets("utf-8", "iso-8859-1"))
	require.Empty(t, r.AcceptsCharsets("iso-8859-1"))
	require.Equal(t, "gzip", r.AcceptsEncodings("gzip", "br"))
	require.Empty(t, r.AcceptsEncodings("br"))
	require.Equal(t, "en-GB", r.AcceptsLanguages("en-GB", "de"))
	require.Empty(t, r.AcceptsLanguages("de"))
}

func Test_Req_FreshWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.SetMethod(MethodGet)
		req.Header.Set("if-none-match", `"abc"`)
	})
	c.Res().Set(HeaderETag, `"abc"`)
	require.True(t, c.Req().Fresh())
	require.Equal(t, []string{`"abc"`}, c.Req().IfNoneMatch())

	noCache := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.SetMethod(MethodGet)
		req.Header.Set("if-none-match", `"abc"`)
		req.Header.Set("cache-control", "no-cache")
	})
	noCache.Res().Set(HeaderETag, `"abc"`)
	require.False(t, noCache.Req().Fresh())

	since := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.SetMethod(MethodGet)
		req.Header.Set("if-modified-since", "Wed, 21 Oct 2015 07:28:00 GMT")
	})
	since.Res().Set(HeaderLastModified, "Tue, 20 Oct 2015 07:28:00 GMT")
	require.True(t, since.Req().Fresh())
}

func Test_Req_HeaderFieldSkipsEmptyFirstLine(t *testing.T) {
	t.Parallel()
	c := ctxReqFor(t, func(req *fasthttp.Request) {
		req.Header.Add("X-Thing", "")
		req.Header.Add("X-Thing", "v2")
		req.Header.Add(HeaderAuthorization, "")
		req.Header.Add(HeaderAuthorization, "Bearer tok")
	})

	require.True(t, c.Req().HasHeader("X-Thing"))
	require.Equal(t, []string{"v2"}, c.Req().GetAll("X-Thing"))
	require.Equal(t, "v2", c.Req().Get("X-Thing"))
	require.Equal(t, "tok", c.Req().Bearer())
}

func Test_Req_GetWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) { req.Header.Set("x-api-key", "k1") })

	require.True(t, c.Req().HasHeader("X-Api-Key"))
	require.Equal(t, "k1", c.Req().Get("X-Api-Key"))
	require.Equal(t, "k1", GetReqHeader[string](c, "X-Api-Key"))
}

func Test_Req_BodyContentEncodingWithoutNormalizing(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.SetMethod(MethodPost)
		req.Header.Set("content-encoding", "gzip")
		req.SetBody(buf.Bytes())
	})

	require.Equal(t, []byte("hello world"), c.Req().Body())
}

func Test_Req_UpgradePredicatesWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) {
		req.Header.Set("connection", "Upgrade")
		req.Header.Set("upgrade", "websocket")
		req.Header.Set("x-requested-with", "XMLHttpRequest")
		req.Header.Set("range", "bytes=0-5")
	})

	require.True(t, c.Req().IsWebSocket())
	require.True(t, c.Req().XHR())

	rg, err := c.Req().Range(100)
	require.NoError(t, err)
	require.Equal(t, "bytes", rg.Type)
	require.Equal(t, []RangeSet{{Start: 0, End: 5}}, rg.Ranges)
}

func Test_Res_FormatWithoutNormalizing(t *testing.T) {
	t.Parallel()
	c := rootMiscARawHeaderCtx(t, func(req *fasthttp.Request) { req.Header.Set("accept", "text/html") })

	require.NoError(t, c.Res().Format(
		ResFmt{MediaType: "application/json", Handler: func(cc Ctx) error { return cc.SendString("json") }},
		ResFmt{MediaType: "text/html", Handler: func(cc Ctx) error { return cc.SendString("html") }},
	))
	require.Equal(t, "html", string(c.Response().Body()))
}

func Test_Res_DelKeyCaseWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Response.Header.DisableNormalizing()
	c := apptest.CtxFor(t, app, fctx)

	c.Res().Set("X-Token", "secret")
	c.Res().Del("x-token")
	require.Empty(t, string(c.Response().Header.Peek("X-Token")))
}

func Test_Req_Res_AccessorsImmutable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		read   func(c Ctx) string
		name   string
		header string // the header that holds value
		value  string // what the accessor reads from header
		want   string
		// response selects the response header instead of the request header.
		response bool
		// alwaysCopies marks an accessor that builds its own string whatever
		// Immutable says, so the buffer it was read from cannot be observed.
		alwaysCopies bool
	}{
		{name: "UserAgent", header: HeaderUserAgent, value: "agent-one", want: "agent-one", read: Ctx.UserAgent},
		{
			name: "Referer", header: HeaderReferer, value: "https://ref.example/a", want: "https://ref.example/a",
			read: Ctx.Referer,
		},
		{
			name: "Origin", header: HeaderOrigin, value: "https://origin.example", want: "https://origin.example",
			read: Ctx.Origin,
		},
		{
			name: "AcceptLanguage", header: HeaderAcceptLanguage, value: "en-GB,en;q=0.8", want: "en-GB,en;q=0.8",
			read: Ctx.AcceptLanguage,
		},
		{
			name: "AcceptEncoding", header: HeaderAcceptEncoding, value: "gzip, br", want: "gzip, br",
			read: Ctx.AcceptEncoding,
		},
		{
			name: "ContentType", header: HeaderContentType, value: "text/plain; charset=iso-8859-1", want: "text/plain; charset=iso-8859-1",
			read: Ctx.ContentType,
		},
		{
			name: "MediaType", header: HeaderContentType, value: "text/plain; charset=iso-8859-1", want: "text/plain",
			read: Ctx.MediaType,
		},
		{
			name: "Charset", header: HeaderContentType, value: "text/plain; charset=iso-8859-1", want: "iso-8859-1",
			read: Ctx.Charset,
		},
		{name: "Bearer", header: HeaderAuthorization, value: "Bearer tok-123", want: "tok-123", read: Ctx.Bearer},
		{
			name: "Authorization", header: HeaderAuthorization, value: "Basic Y3JlZA==", want: "Y3JlZA==",
			read: func(c Ctx) string {
				_, credentials := c.Authorization()
				return credentials
			},
		},
		{
			name: "GetAll", header: "X-Multi", value: "one", want: "one",
			read: func(c Ctx) string { return strings.Join(c.GetAll("X-Multi"), "|") },
		},
		{
			name: "IfNoneMatch", header: HeaderIfNoneMatch, value: `"v1", "v2"`, want: `"v1"`,
			read: func(c Ctx) string { return c.IfNoneMatch()[0] },
		},
		{
			name: "AllCookies", header: HeaderCookie, value: "sid=abc", want: "abc", alwaysCopies: true,
			read: func(c Ctx) string { return c.AllCookies()["sid"] },
		},
		{
			name: "Res.ContentType", header: HeaderContentType, value: "application/xml", want: "application/xml", response: true,
			read: func(c Ctx) string { return c.Res().ContentType() },
		},
	}

	for _, tc := range tests {
		// header returns the header block that holds the row's value.
		header := func(c Ctx) interface {
			Set(key, value string)
			Peek(key string) []byte
		} {
			if tc.response {
				return &c.Response().Header
			}
			return &c.Request().Header
		}
		// scrub overwrites the stored value in place.
		scrub := func(c Ctx) {
			raw := header(c).Peek(tc.header)
			for i := range raw {
				raw[i] = 'X'
			}
		}

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(Config{Immutable: true}))
			header(c).Set(tc.header, tc.value)

			got := tc.read(c)
			require.Equal(t, tc.want, got)

			scrub(c)
			require.Equal(t, tc.want, got, "Immutable must hand back a copy, not a view of the header buffer")
		})

		if tc.alwaysCopies {
			continue
		}

		t.Run(tc.name+"/scrubReachesTheBuffer", func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			header(c).Set(tc.header, tc.value)

			got := tc.read(c)
			require.Equal(t, tc.want, got)

			scrub(c)
			require.NotEqual(t, tc.want, got, "without Immutable the value views the header buffer, so this guards the test above against a scrub that reaches nothing")
		})
	}
}
