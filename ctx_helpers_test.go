// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------
// Req
// -----------------------------------------------------------------------------

// Test_Req_RequestHelpersOnReq pins the helpers to Req rather than Ctx. They
// were first written with a *DefaultCtx receiver, which kept them off the Req
// interface: c.UserAgent() compiled but c.Req().UserAgent() did not. Every call
// here goes through Req, so that regression cannot come back unnoticed.
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

	// Ctx keeps promoting all of them, so no caller loses an existing spelling.
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
	// Field names are case-insensitive.
	require.Equal(t, []string{"first", "second"}, c.Req().GetAll("x-test"))
	// Get still answers with the first line only.
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
		// etagc permits "," inside the quoted opaque-tag, so this is one tag.
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

// -----------------------------------------------------------------------------
// Res
// -----------------------------------------------------------------------------

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

	// A status alone is not a body write.
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
	// Written must answer without draining the stream, so the body still goes
	// out as a stream afterwards.
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

	// Deleting an absent header is a no-op.
	require.NotPanics(t, func() { c.Res().Del("X-Absent") })

	// Every field line goes, not just the first.
	c.Res().Add("X-Multi", "a")
	c.Res().Add("X-Multi", "b")
	require.Len(t, c.Response().Header.PeekAll("X-Multi"), 2)
	c.Res().Del("X-Multi")
	require.Empty(t, c.Response().Header.PeekAll("X-Multi"))

	// Del(Set-Cookie) withdraws the cookies this response was going to set.
	c.Cookie(&Cookie{Name: "a", Value: "1"})
	require.NotEmpty(t, c.Res().Cookies())
	c.Res().Del(HeaderSetCookie)
	require.Empty(t, c.Res().Cookies())
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

	// Append folds into one line instead, which is the difference between them.
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

	// Content-Length reflects the header, which fasthttp only fills in as it
	// serializes, so it stays 0 here even though the body is not empty.
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
	require.Empty(t, c.Res().Cookies())

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

	// A session cookie carries neither Max-Age nor Expires, and reads back as one.
	c.Cookie(&Cookie{Name: "flash", Value: "x", SessionOnly: true})
	flash, ok := c.Res().GetCookie("flash")
	require.True(t, ok)
	require.True(t, flash.SessionOnly)
	require.Zero(t, flash.MaxAge)
	require.True(t, flash.Expires.IsZero())

	cookies := c.Res().Cookies()
	require.Len(t, cookies, 2)
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	require.ElementsMatch(t, []string{"session", "flash"}, names)

	// The returned struct is a copy: editing it leaves the response alone.
	got.Value = "tampered"
	again, ok := c.Res().GetCookie("session")
	require.True(t, ok)
	require.Equal(t, "abc", again.Value)
}

// Test_Res_GetCookie_RoundTrip pins the read-modify-write path GetCookie exists
// for: middleware that rewrites a cookie an earlier handler set.
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
	require.Len(t, c.Res().Cookies(), 1, "rewriting replaces the cookie rather than adding one")
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

// -----------------------------------------------------------------------------
// Ctx
// -----------------------------------------------------------------------------

// Test_Ctx_BodyContentLengthCookiesPreferRequest pins the three names Req and
// Res both define. Ctx has to resolve them itself or the embedded selectors are
// ambiguous and the package stops compiling; the request wins, as it does for
// Get, and the response stays reachable through Res().
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

	serverCookies := c.Res().Cookies()
	require.Len(t, serverCookies, 1)
	require.Equal(t, "server", serverCookies[0].Value)
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

	// No route matched at all: there is no final handler to be.
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })
	require.False(t, c.IsFinal())
	require.False(t, c.IsMiddleware())
}

func Test_Ctx_MountPath(t *testing.T) {
	t.Parallel()

	micro := New()
	var mounted, mountedPath string
	micro.Get("/doe", func(c Ctx) error {
		mounted = c.MountPath()
		// Path aliases the pooled request buffer, which the next request
		// overwrites; clone it to read it after the handler returns.
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

// Test_Ctx_Copy is the point of Copy: the snapshot still reads the request the
// handler saw after the ctx has gone back to the pool and other requests have
// reused its buffers.
func Test_Ctx_Copy(t *testing.T) {
	t.Parallel()
	app := New()

	var snapshot Ctx
	app.Get("/user/:id", func(c Ctx) error {
		if snapshot != nil {
			// The churn requests below match this same route; only the first
			// one is the request the snapshot has to keep.
			return c.SendString("ok")
		}
		c.Locals("who", "alice")
		snapshot = c.Copy()
		return c.SendString("ok")
	})

	_, err := app.Test(httptest.NewRequest(MethodGet, "/user/42?q=first", http.NoBody))
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	// Drive more requests through the pooled ctx so its buffers are certainly
	// rewritten under the snapshot.
	for range 5 {
		_, err := app.Test(httptest.NewRequest(MethodGet, "/user/other?q=second", http.NoBody))
		require.NoError(t, err)
	}

	require.Equal(t, "42", snapshot.Params("id"))
	require.Equal(t, "/user/42", snapshot.Path())
	require.Equal(t, "first", snapshot.Query("q"))
	require.Equal(t, MethodGet, snapshot.Method())
	require.Equal(t, "alice", snapshot.Locals("who"))
	require.Equal(t, "/user/:id", snapshot.Route().Path)
}

func Test_Ctx_Copy_DetachedResponse(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/original")
	fctx.Request.SetBody([]byte("body"))
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	require.NoError(t, c.SendString("original"))
	snapshot := c.Copy()

	// Writing to the snapshot leaves the live response alone.
	require.NoError(t, snapshot.SendString("detached"))
	snapshot.Status(StatusTeapot)
	require.Equal(t, "detached", string(snapshot.Res().Body()))
	require.Equal(t, "original", string(c.Res().Body()))
	require.Equal(t, StatusOK, c.Res().StatusCode())

	// The request is a deep copy, so mutating the live one does not reach it.
	c.Request().SetBody([]byte("rewritten"))
	require.Equal(t, "body", string(snapshot.Body()))
	require.Equal(t, "rewritten", string(c.Body()))
}

func Test_Ctx_Copy_BodyStreamNotCopied(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetBodyStream(strings.NewReader("streamed"), 8)
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	snapshot := c.Copy()
	require.Nil(t, snapshot.Req().BodyStream(), "an unread stream does not survive the copy")
	require.Empty(t, snapshot.Body())
	// The original still has it, so a handler can read it after copying.
	require.NotNil(t, c.Req().BodyStream())
}

func Test_Ctx_Copy_IsNotPooled(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/keep")
	c := app.AcquireCtx(fctx)

	snapshot := c.Copy()
	app.ReleaseCtx(c)

	// Releasing the original must not have handed the snapshot back to the pool.
	require.Equal(t, "/keep", snapshot.Path())
	require.NotSame(t, c.RequestCtx(), snapshot.RequestCtx())
}
