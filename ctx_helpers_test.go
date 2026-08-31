// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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

func Test_Req_RequestHelpersOnReq(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/path/here?q=1")
	fctx.Request.Header.SetMethod(MethodPost)
	fctx.Request.Header.SetHost("example.com")
	fctx.Request.Header.SetUserAgent("fiber-test")
	fctx.Request.Header.SetReferer("https://referer.example")
	fctx.Request.Header.Set(HeaderAcceptLanguage, "en-US, de")
	fctx.Request.Header.Set(HeaderAcceptEncoding, "gzip, br")
	fctx.Request.Header.Set(HeaderAccept, MIMEApplicationJSON)
	fctx.Request.Header.SetContentType(MIMEApplicationJSON + "; charset=utf-8")
	fctx.Request.SetBody([]byte(`{"a":1}`))

	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/original")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.Equal(t, "/original", c.Req().Path())
	require.Equal(t, "/rewritten", c.Req().Path("/rewritten"))
	require.Equal(t, "/rewritten", c.Path())
	require.Equal(t, "/rewritten", string(c.Request().URI().Path()))
}

func Test_Req_GetAll(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Add("X-Test", "first")
	fctx.Request.Header.Add("X-Test", "second")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

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

	_, err := app.Test(httptest.NewRequest(MethodPost, "/", strings.NewReader("0123456789")))
	require.NoError(t, err)
	require.Equal(t, 10, length)

	chunked := &fasthttp.RequestCtx{}
	chunked.Request.Header.SetContentLength(-1)
	cc := app.AcquireCtx(chunked)
	t.Cleanup(func() { app.ReleaseCtx(cc) })

	require.Equal(t, -1, cc.Req().ContentLength(), "chunked bodies report an unknown length")
	require.True(t, cc.Req().HasBody(), "an unknown length still means there is a body")

	var bodyless int
	app.Get("/", func(c Ctx) error {
		bodyless = c.Req().ContentLength()
		return nil
	})
	_, err = app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
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

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	for key, lines := range got {
		require.Nil(t, lines, key)
		require.False(t, has[key], key)
		require.Equal(t, has[key], len(lines) > 0, "GetAll and HasHeader must agree on %s", key)
	}
}

func Test_Req_ContentType(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetContentType(MIMEApplicationJSON + "; charset=utf-8")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	buffered := &fasthttp.RequestCtx{}
	buffered.Request.SetBody([]byte("buffered"))
	bc := app.AcquireCtx(buffered)
	t.Cleanup(func() { app.ReleaseCtx(bc) })
	require.Nil(t, bc.Req().BodyStream(), "a buffered body is not a stream")

	streamed := &fasthttp.RequestCtx{}
	streamed.Request.SetBodyStream(strings.NewReader("streamed"), 8)
	sc := app.AcquireCtx(streamed)
	t.Cleanup(func() { app.ReleaseCtx(sc) })

	stream := sc.Req().BodyStream()
	require.NotNil(t, stream)
	body, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.Equal(t, "streamed", string(body))
}

func Test_Req_URI(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/search?q=fiber#frag")
	fctx.Request.Header.SetHost("example.com")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	uri := c.Req().URI()
	require.NotNil(t, uri)
	require.Equal(t, "/search", string(uri.Path()))
	require.Equal(t, "q=fiber", string(uri.QueryString()))
	require.Equal(t, "frag", string(uri.Hash()))
	require.Equal(t, "example.com", string(uri.Host()))
}

func Test_Req_Origin(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Set(HeaderOrigin, "https://example.com")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })
	require.Equal(t, "https://example.com", c.Req().Origin())

	bare := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(bare) })
	require.Empty(t, bare.Req().Origin())
}

func Test_Req_Authorization(t *testing.T) {
	t.Parallel()
	app := New()

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

			fctx := &fasthttp.RequestCtx{}
			if tc.header != "" {
				fctx.Request.Header.Set(HeaderAuthorization, tc.header)
			}
			c := app.AcquireCtx(fctx)
			t.Cleanup(func() { app.ReleaseCtx(c) })

			scheme, credentials := c.Req().Authorization()
			require.Equal(t, tc.scheme, scheme)
			require.Equal(t, tc.credentials, credentials)
			require.Equal(t, tc.bearer, c.Req().Bearer())
		})
	}
}

func Test_Req_IsSafe_IsIdempotent(t *testing.T) {
	t.Parallel()
	app := New()

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

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(tc.method)
			c := app.AcquireCtx(fctx)
			t.Cleanup(func() { app.ReleaseCtx(c) })

			require.Equal(t, tc.safe, c.Req().IsSafe())
			require.Equal(t, tc.idempotent, c.Req().IsIdempotent())
		})
	}
}

func Test_Req_CookieNames_AllCookies(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetCookie("session", "abc")
	fctx.Request.Header.SetCookie("theme", "dark")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.ElementsMatch(t, []string{"session", "theme"}, c.Req().CookieNames())
	require.Equal(t, map[string]string{"session": "abc", "theme": "dark"}, c.Req().AllCookies())

	shadowed := &fasthttp.RequestCtx{}
	shadowed.Request.Header.Set(HeaderCookie, "a=first; a=second")
	sc := app.AcquireCtx(shadowed)
	t.Cleanup(func() { app.ReleaseCtx(sc) })

	require.Equal(t, "first", sc.Req().Cookies("a"))
	require.Equal(t, "first", sc.Req().AllCookies()["a"])
	require.Equal(t, []string{"a", "a"}, sc.Req().CookieNames(), "the repetition itself stays visible")

	bare := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(bare) })
	require.Empty(t, bare.Req().CookieNames())
	require.Empty(t, bare.Req().AllCookies())
}

func Test_Req_IfNoneMatch(t *testing.T) {
	t.Parallel()
	app := New()

	tests := []struct {
		name   string
		header string
		want   []string
	}{
		{name: "absent"},
		{name: "single", header: `"abc"`, want: []string{`"abc"`}},
		{name: "list", header: `"a", W/"b"`, want: []string{`"a"`, `W/"b"`}},
		{name: "wildcard", header: "*", want: []string{"*"}},
		{name: "comma inside tag", header: `"v1,v2"`, want: []string{`"v1,v2"`}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fctx := &fasthttp.RequestCtx{}
			if tc.header != "" {
				fctx.Request.Header.Set(HeaderIfNoneMatch, tc.header)
			}
			c := app.AcquireCtx(fctx)
			t.Cleanup(func() { app.ReleaseCtx(c) })

			require.Equal(t, tc.want, c.Req().IfNoneMatch())
		})
	}
}

func Test_Req_IfNoneMatch_MultipleFieldLines(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Add(HeaderIfNoneMatch, `"a"`)
	fctx.Request.Header.Add(HeaderIfNoneMatch, `"b"`)
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.Equal(t, []string{`"a"`, `"b"`}, c.Req().IfNoneMatch())
}

func Test_Req_IfModifiedSince(t *testing.T) {
	t.Parallel()
	app := New()

	bare := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(bare) })
	_, err := bare.Req().IfModifiedSince()
	require.ErrorIs(t, err, ErrHeaderNotFound)

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	got, err := c.Req().IfModifiedSince()
	require.NoError(t, err)
	require.True(t, got.Equal(time.Date(2015, time.October, 21, 7, 28, 0, 0, time.UTC)))

	malformed := &fasthttp.RequestCtx{}
	malformed.Request.Header.Set(HeaderIfModifiedSince, "not a date")
	mc := app.AcquireCtx(malformed)
	t.Cleanup(func() { app.ReleaseCtx(mc) })

	_, err = mc.Req().IfModifiedSince()
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrHeaderNotFound, "a malformed date is not an absent header")
}

func Test_Res_StatusCode(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.Equal(t, StatusOK, c.Res().StatusCode(), "an untouched response reports 200")
	c.Status(StatusTeapot)
	require.Equal(t, StatusTeapot, c.Res().StatusCode())
	require.Equal(t, c.Response().StatusCode(), c.Res().StatusCode())
}

func Test_Res_Body_ResetBody_Written(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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

func Test_Res_Written_BodyStream(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.NoError(t, c.SendStream(strings.NewReader("streamed"), 8))
	require.True(t, c.Res().Written())
	require.True(t, c.Response().IsBodyStream())
}

func Test_Res_Del(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	resp, err := over.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Len(t, resp.Header.Values(HeaderSetCookie), 3)
}

func Test_Res_GetCookie_Deletion(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.NoError(t, c.SendStream(strings.NewReader("streamed"), 8))
	require.True(t, c.Res().Written())
	require.Nil(t, c.Res().Body(), "a streamed body is not materialized")
	require.True(t, c.Response().IsBodyStream(), "and it is still a stream afterwards")
}

func Test_Res_Add_SpecialHeaders(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app.Delete("/item", func(c Ctx) error {
		return c.Res().NoContent()
	})

	resp, err := app.Test(httptest.NewRequest(MethodDelete, "/item", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNoContent, resp.StatusCode)
	require.Empty(t, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Empty(t, body)
}

func Test_Res_NoContent_DiffersFromSendStatus(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/sendstatus", func(c Ctx) error {
		c.Type("json")
		return c.SendStatus(StatusNoContent)
	})
	app.Get("/nocontent", func(c Ctx) error {
		c.Type("json")
		return c.Res().NoContent()
	})

	sent, err := app.Test(httptest.NewRequest(MethodGet, "/sendstatus", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNoContent, sent.StatusCode)
	require.NotEmpty(t, sent.Header.Get(HeaderContentType))

	none, err := app.Test(httptest.NewRequest(MethodGet, "/nocontent", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNoContent, none.StatusCode)
	require.Empty(t, none.Header.Get(HeaderContentType))
}

func Test_Ctx_BodyContentLengthCookiesPreferRequest(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetBody([]byte("request body"))
	fctx.Request.Header.SetContentLength(len("request body"))
	fctx.Request.Header.SetCookie("who", "client")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

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

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)

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

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	require.NotNil(t, local)
	require.NotNil(t, remote)
	require.NotEmpty(t, remote.Network())
}

func Test_Ctx_Hijack(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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

	_, err := app.Test(httptest.NewRequest(MethodGet, "/home", http.NoBody))
	require.NoError(t, err)

	require.Equal(t, "home", handlerName)
	require.Empty(t, mwName, "middleware reports its own unnamed route, not the endpoint's")
}

func Test_Ctx_RouteName_Unmatched(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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

	_, err := app.Test(httptest.NewRequest(MethodGet, "/chain", http.NoBody))
	require.NoError(t, err)

	require.False(t, inGlobalMW)
	require.False(t, inRouteMW, "a route handler with another after it is not final")
	require.True(t, inHandler)

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })
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
	_, err := terminal.Test(httptest.NewRequest(MethodGet, "/anything", http.NoBody))
	require.NoError(t, err)
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
	_, err = overlapping.Test(httptest.NewRequest(MethodGet, "/specific", http.NoBody))
	require.NoError(t, err)
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

	_, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", http.NoBody))
	require.NoError(t, err)
	_, err = app.Test(httptest.NewRequest(MethodGet, "/top", http.NoBody))
	require.NoError(t, err)

	var standalone string
	micro.Get("/solo", func(c Ctx) error {
		standalone = c.MountPath()
		return nil
	})
	_, err = micro.Test(httptest.NewRequest(MethodGet, "/solo", http.NoBody))
	require.NoError(t, err)
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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app.Get("/", func(c Ctx) error {
		return c.Error(StatusBadRequest, "id must be numeric")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "id must be numeric", string(body))
}

func Test_Req_HeaderFieldWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.Set("origin", "https://example.com")
	fctx.Request.Header.Set("authorization", "Bearer abc123")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.Equal(t, "https://example.com", c.Get(HeaderOrigin))
	require.Equal(t, "https://example.com", c.Req().Origin())

	scheme, credentials := c.Req().Authorization()
	require.Equal(t, "Bearer", scheme)
	require.Equal(t, "abc123", credentials)
	require.Equal(t, "abc123", c.Req().Bearer())
}

func Test_Req_ConditionalHeadersWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.Set("if-none-match", `"a", W/"b"`)
	fctx.Request.Header.Set("if-modified-since", "Wed, 21 Oct 2015 07:28:00 GMT")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.Equal(t, []string{`"a"`, `W/"b"`}, c.Req().IfNoneMatch())

	since, err := c.Req().IfModifiedSince()
	require.NoError(t, err)
	require.True(t, since.Equal(time.Date(2015, time.October, 21, 7, 28, 0, 0, time.UTC)))
}

func Test_Req_IsPreflightWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.SetMethod(MethodOptions)
	fctx.Request.Header.Set("origin", "https://example.com")
	fctx.Request.Header.Set("access-control-request-method", MethodGet)
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.True(t, c.Req().IsPreflight())
}

func Test_Res_Del_CaseInsensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Response.Header.DisableNormalizing()
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	c.Response().Header.Set("x-token", "secret")
	require.Equal(t, "secret", string(c.Response().Header.Peek("x-token")))

	c.Res().Del(HeaderXRequestID)
	c.Res().Del("X-Token")
	require.Empty(t, string(c.Response().Header.Peek("x-token")), "a differently cased field is still removed")
}

func Test_Res_Cookies_UnparsableEntry(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.Set("x-test", "yes")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
		app.Get("/bare"+strconv.Itoa(status), func(c Ctx) error {
			return c.SendStatus(status)
		})
	}
	app.Get("/nocontent", func(c Ctx) error {
		c.Set(HeaderContentLength, "42")
		return c.Res().NoContent()
	})

	get := func(path string) []string {
		resp, err := app.Test(httptest.NewRequest(MethodGet, path, http.NoBody))
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Empty(t, body, path)
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
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.Set("accept", "application/json")
	fctx.Request.Header.Set("accept-charset", "utf-8")
	fctx.Request.Header.Set("accept-encoding", "gzip")
	fctx.Request.Header.Set("accept-language", "en-GB")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

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
	app := New(Config{DisableHeaderNormalizing: true})

	newCtx := func(t *testing.T, set func(h *fasthttp.RequestHeader)) Ctx {
		t.Helper()
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.DisableNormalizing()
		fctx.Request.Header.SetMethod(MethodGet)
		set(&fctx.Request.Header)
		c := app.AcquireCtx(fctx)
		t.Cleanup(func() { app.ReleaseCtx(c) })
		return c
	}

	c := newCtx(t, func(h *fasthttp.RequestHeader) {
		h.Set("if-none-match", `"abc"`)
	})
	c.Res().Set(HeaderETag, `"abc"`)
	require.True(t, c.Req().Fresh())
	require.Equal(t, []string{`"abc"`}, c.Req().IfNoneMatch())

	noCache := newCtx(t, func(h *fasthttp.RequestHeader) {
		h.Set("if-none-match", `"abc"`)
		h.Set("cache-control", "no-cache")
	})
	noCache.Res().Set(HeaderETag, `"abc"`)
	require.False(t, noCache.Req().Fresh())

	since := newCtx(t, func(h *fasthttp.RequestHeader) {
		h.Set("if-modified-since", "Wed, 21 Oct 2015 07:28:00 GMT")
	})
	since.Res().Set(HeaderLastModified, "Tue, 20 Oct 2015 07:28:00 GMT")
	require.True(t, since.Req().Fresh())
}

func Test_Req_HeaderFieldSkipsEmptyFirstLine(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Add("X-Thing", "")
	fctx.Request.Header.Add("X-Thing", "v2")
	fctx.Request.Header.Add(HeaderAuthorization, "")
	fctx.Request.Header.Add(HeaderAuthorization, "Bearer tok")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.True(t, c.Req().HasHeader("X-Thing"))
	require.Equal(t, []string{"v2"}, c.Req().GetAll("X-Thing"))
	require.Equal(t, "v2", c.Req().Get("X-Thing"))
	require.Equal(t, "tok", c.Req().Bearer())
}

func Test_Req_GetWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.Set("x-api-key", "k1")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.True(t, c.Req().HasHeader("X-Api-Key"))
	require.Equal(t, "k1", c.Req().Get("X-Api-Key"))
	require.Equal(t, "k1", GetReqHeader[string](c, "X-Api-Key"))
}

func Test_Req_BodyContentEncodingWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.SetMethod(MethodPost)
	fctx.Request.Header.Set("content-encoding", "gzip")
	fctx.Request.SetBody(buf.Bytes())
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.Equal(t, []byte("hello world"), c.Req().Body())
}

func Test_Req_UpgradePredicatesWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.Set("connection", "Upgrade")
	fctx.Request.Header.Set("upgrade", "websocket")
	fctx.Request.Header.Set("x-requested-with", "XMLHttpRequest")
	fctx.Request.Header.Set("range", "bytes=0-5")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.True(t, c.Req().IsWebSocket())
	require.True(t, c.Req().XHR())

	rg, err := c.Req().Range(100)
	require.NoError(t, err)
	require.Equal(t, "bytes", rg.Type)
	require.Equal(t, []RangeSet{{Start: 0, End: 5}}, rg.Ranges)
}

func Test_Res_FormatWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.DisableNormalizing()
	fctx.Request.Header.Set("accept", "text/html")
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	err := c.Res().Format(
		ResFmt{MediaType: "application/json", Handler: func(cc Ctx) error { return cc.SendString("json") }},
		ResFmt{MediaType: "text/html", Handler: func(cc Ctx) error { return cc.SendString("html") }},
	)
	require.NoError(t, err)
	require.Equal(t, "html", string(c.Response().Body()))
}

func Test_Res_DelKeyCaseWithoutNormalizing(t *testing.T) {
	t.Parallel()
	app := New(Config{DisableHeaderNormalizing: true})

	fctx := &fasthttp.RequestCtx{}
	fctx.Response.Header.DisableNormalizing()
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	c.Res().Set("X-Token", "secret")
	c.Res().Del("x-token")
	require.Empty(t, string(c.Response().Header.Peek("X-Token")))
}

func Test_Req_Res_AccessorsImmutable(t *testing.T) {
	t.Parallel()

	scrubHeader := func(field string) func(c Ctx) {
		return func(c Ctx) {
			raw := c.Request().Header.Peek(field)
			for i := range raw {
				raw[i] = 'X'
			}
		}
	}

	tests := []struct {
		setup func(c Ctx)
		read  func(c Ctx) string
		scrub func(c Ctx)
		name  string
		want  string
		// alwaysCopies marks an accessor that builds its own string whatever
		// Immutable says, so the buffer it was read from cannot be observed.
		alwaysCopies bool
	}{
		{
			name:  "UserAgent",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderUserAgent, "agent-one") },
			read:  func(c Ctx) string { return c.UserAgent() },
			scrub: scrubHeader(HeaderUserAgent),
			want:  "agent-one",
		},
		{
			name:  "Referer",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderReferer, "https://ref.example/a") },
			read:  func(c Ctx) string { return c.Referer() },
			scrub: scrubHeader(HeaderReferer),
			want:  "https://ref.example/a",
		},
		{
			name:  "Origin",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderOrigin, "https://origin.example") },
			read:  func(c Ctx) string { return c.Origin() },
			scrub: scrubHeader(HeaderOrigin),
			want:  "https://origin.example",
		},
		{
			name:  "AcceptLanguage",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderAcceptLanguage, "en-GB,en;q=0.8") },
			read:  func(c Ctx) string { return c.AcceptLanguage() },
			scrub: scrubHeader(HeaderAcceptLanguage),
			want:  "en-GB,en;q=0.8",
		},
		{
			name:  "AcceptEncoding",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderAcceptEncoding, "gzip, br") },
			read:  func(c Ctx) string { return c.AcceptEncoding() },
			scrub: scrubHeader(HeaderAcceptEncoding),
			want:  "gzip, br",
		},
		{
			name:  "ContentType",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderContentType, "text/plain; charset=iso-8859-1") },
			read:  func(c Ctx) string { return c.ContentType() },
			scrub: scrubHeader(HeaderContentType),
			want:  "text/plain; charset=iso-8859-1",
		},
		{
			name:  "MediaType",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderContentType, "text/plain; charset=iso-8859-1") },
			read:  func(c Ctx) string { return c.MediaType() },
			scrub: scrubHeader(HeaderContentType),
			want:  "text/plain",
		},
		{
			name:  "Charset",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderContentType, "text/plain; charset=iso-8859-1") },
			read:  func(c Ctx) string { return c.Charset() },
			scrub: scrubHeader(HeaderContentType),
			want:  "iso-8859-1",
		},
		{
			name:  "Bearer",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderAuthorization, "Bearer tok-123") },
			read:  func(c Ctx) string { return c.Bearer() },
			scrub: scrubHeader(HeaderAuthorization),
			want:  "tok-123",
		},
		{
			name:  "Authorization",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderAuthorization, "Basic Y3JlZA==") },
			read: func(c Ctx) string {
				_, credentials := c.Authorization()
				return credentials
			},
			scrub: scrubHeader(HeaderAuthorization),
			want:  "Y3JlZA==",
		},
		{
			name:  "GetAll",
			setup: func(c Ctx) { c.Request().Header.Set("X-Multi", "one") },
			read:  func(c Ctx) string { return strings.Join(c.GetAll("X-Multi"), "|") },
			scrub: scrubHeader("X-Multi"),
			want:  "one",
		},
		{
			name:  "IfNoneMatch",
			setup: func(c Ctx) { c.Request().Header.Set(HeaderIfNoneMatch, `"v1", "v2"`) },
			read:  func(c Ctx) string { return c.IfNoneMatch()[0] },
			scrub: scrubHeader(HeaderIfNoneMatch),
			want:  `"v1"`,
		},
		{
			name:         "AllCookies",
			setup:        func(c Ctx) { c.Request().Header.Set(HeaderCookie, "sid=abc") },
			read:         func(c Ctx) string { return c.AllCookies()["sid"] },
			scrub:        scrubHeader(HeaderCookie),
			want:         "abc",
			alwaysCopies: true,
		},
		{
			name:  "Res.ContentType",
			setup: func(c Ctx) { c.Response().Header.Set(HeaderContentType, "application/xml") },
			read:  func(c Ctx) string { return c.Res().ContentType() },
			scrub: func(c Ctx) {
				raw := c.Response().Header.Peek(HeaderContentType)
				for i := range raw {
					raw[i] = 'X'
				}
			},
			want: "application/xml",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := New(Config{Immutable: true})
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			tc.setup(c)

			got := tc.read(c)
			require.Equal(t, tc.want, got)

			tc.scrub(c)
			require.Equal(t, tc.want, got, "Immutable must hand back a copy, not a view of the header buffer")
		})

		if tc.alwaysCopies {
			continue
		}

		t.Run(tc.name+"/scrubReachesTheBuffer", func(t *testing.T) {
			t.Parallel()

			app := New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			tc.setup(c)

			got := tc.read(c)
			require.Equal(t, tc.want, got)

			tc.scrub(c)
			require.NotEqual(t, tc.want, got, "without Immutable the value views the header buffer, so this guards the test above against a scrub that reaches nothing")
		})
	}
}
