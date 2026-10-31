// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
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

// redirFlashCookie returns the Cookie header that hands testredirectionMsgs to a request as its flash messages.
func redirFlashCookie(tb testing.TB) string {
	tb.Helper()

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(tb, err)
	return FlashCookieName + "=" + hex.EncodeToString(val)
}

// redirFlashCookieMsgs returns the flash messages in the cookie that c's response sets, decoded the
// way the next request decodes them.
func redirFlashCookieMsgs(tb testing.TB, c *DefaultCtx) redirectionMsgs {
	tb.Helper()

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(tb, err)
	var msgs redirectionMsgs
	_, err = msgs.UnmarshalMsg(decoded)
	require.NoError(tb, err)
	return msgs
}

// redirBenchCtx returns a context borrowed from an app with a GET route named "user" at path.
func redirBenchCtx(b *testing.B, path string) *DefaultCtx {
	b.Helper()

	app := New()
	app.Get(path, func(c Ctx) error { return c.SendString("user") }).Name("user")
	return apptest.CtxAs[*DefaultCtx](b, app)
}

// redirBenchParsedCtx is redirBenchCtx for a request that carries the flash cookie, which the context has parsed.
func redirBenchParsedCtx(b *testing.B) *DefaultCtx {
	b.Helper()

	c := redirBenchCtx(b, "/user")
	c.Request().Header.Set(HeaderCookie, redirFlashCookie(b))
	c.Redirect().parseAndClearFlashMessages()
	return c
}

// redirRouteLocation redirects a context to the route "user", registered at pattern, with config, and returns the
// Location of the 303 it answers with.
func redirRouteLocation(t *testing.T, pattern string, config ...RedirectConfig) string {
	t.Helper()

	app := New()
	app.Get(pattern, emptyHandler).Name("user")
	c := apptest.Ctx(t, app)

	require.NoError(t, c.Redirect().Route("user", config...))
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	return string(c.Response().Header.Peek(HeaderLocation))
}

func Test_Redirect_To(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.Redirect().To("http://default.com"))
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "http://default.com", string(c.Response().Header.Peek(HeaderLocation)))

	require.NoError(t, c.Redirect().Status(301).To("http://example.com"))
	require.Equal(t, 301, c.Response().StatusCode())
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// Test_Redirect_WithFlashMessages sends flash messages, and the request's input for the rows that ask for
// it, through each way of redirecting, and checks them twice: as the redirect collected them and as the
// cookie carries them to the next request.
func Test_Redirect_WithFlashMessages(t *testing.T) {
	t.Parallel()
	flash := redirectionMsgs{{key: "success", value: "1"}, {key: "message", value: "test"}}
	flashWithInput := testredirectionMsgs // the old input is the request's id=1 and name=tom
	routeWithInput := func(r *Redirect) error {
		return r.With("success", "1").With("message", "test").WithInput().Route("user")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("id", "1"))
	require.NoError(t, writer.WriteField("name", "tom"))
	require.NoError(t, writer.Close())

	tests := []struct {
		prepare  func(c *DefaultCtx)
		redirect func(r *Redirect) error
		name     string
		method   string // of the route "user"; GET when empty
		location string
		want     redirectionMsgs
	}{
		{
			// A later With replaces an earlier one for the same key.
			name: "To",
			redirect: func(r *Redirect) error {
				return r.With("success", "2").With("success", "1").With("message", "test", 2).To("http://example.com")
			},
			location: "http://example.com",
			want:     redirectionMsgs{{key: "success", value: "1"}, {key: "message", value: "test", level: 2}},
		},
		{
			name:     "Back",
			redirect: func(r *Redirect) error { return r.With("success", "1").With("message", "test").Back("/") },
			location: "/",
			want:     flash,
		},
		{
			name:     "Route",
			redirect: func(r *Redirect) error { return r.With("success", "1").With("message", "test").Route("user") },
			location: "/user",
			want:     flash,
		},
		{
			name:     "Route with old input from the query",
			prepare:  func(c *DefaultCtx) { c.Request().URI().SetQueryString("id=1&name=tom") },
			redirect: routeWithInput,
			location: "/user",
			want:     flashWithInput,
		},
		{
			name:   "Route with old input from a form",
			method: MethodPost,
			prepare: func(c *DefaultCtx) {
				c.Request().Header.Set(HeaderContentType, MIMEApplicationForm)
				c.Request().SetBodyString("id=1&name=tom")
			},
			redirect: routeWithInput,
			location: "/user",
			want:     flashWithInput,
		},
		{
			name: "Route with old input from a multipart form",
			prepare: func(c *DefaultCtx) {
				c.Request().SetBody(body.Bytes())
				c.Request().Header.Set(HeaderContentType, writer.FormDataContentType())
			},
			redirect: routeWithInput,
			location: "/user",
			want:     flashWithInput,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Add([]string{cmp.Or(tc.method, MethodGet)}, "/user", func(c Ctx) error {
				return c.SendString("user")
			}).Name("user")
			c := apptest.CtxAs[*DefaultCtx](t, app)
			if tc.prepare != nil {
				tc.prepare(c)
			}

			require.NoError(t, tc.redirect(c.Redirect()))
			require.Equal(t, StatusSeeOther, c.Response().StatusCode())
			require.Equal(t, tc.location, string(c.Response().Header.Peek(HeaderLocation)))

			// What the redirect collected, and then what the cookie carries to the next request.
			require.ElementsMatch(t, tc.want, c.redirect.messages)
			require.ElementsMatch(t, tc.want, redirFlashCookieMsgs(t, c))
		})
	}
}

func Test_Redirect_Route_Location(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		want    string
		config  []RedirectConfig
	}{
		{
			name:    "param",
			pattern: "/user/:name",
			config:  []RedirectConfig{{Params: Map{"name": "fiber"}}},
			want:    "/user/fiber",
		},
		{
			name:    "optional param",
			pattern: "/user/:name?",
			config:  []RedirectConfig{{Params: Map{"name": "fiber"}}},
			want:    "/user/fiber",
		},
		{name: "optional param without a value", pattern: "/user/:name?", want: "/user/"},
		{
			name:    "greedy param",
			pattern: "/user/+",
			config:  []RedirectConfig{{Params: Map{"+": "test/routes"}}},
			want:    "/user/test/routes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, redirRouteLocation(t, tc.pattern, tc.config...))
		})
	}
}

func Test_Redirect_Route_WithQueries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		queries map[string]string
		want    url.Values
		name    string
	}{
		{
			name:    "nested keys",
			queries: map[string]string{"data[0][name]": "john", "data[0][age]": "10", "test": "doe"},
			want:    url.Values{"data[0][name]": []string{"john"}, "data[0][age]": []string{"10"}, "test": []string{"doe"}},
		},
		{
			// The value must survive as a single query parameter, not be split into
			// `q=a` and `b=c`.
			name:    "special characters in a value",
			queries: map[string]string{"q": "a&b=c"},
			want:    url.Values{"q": []string{"a&b=c"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			locationHeader := redirRouteLocation(t, "/user/:name", RedirectConfig{Params: Map{"name": "fiber"}, Queries: tc.queries})

			// analysis of query parameters with url parsing, since a map pass is always randomly ordered
			location, err := url.Parse(locationHeader)
			require.NoError(t, err, "url.Parse(location)")
			require.Equal(t, "/user/fiber", location.Path)
			require.Equal(t, tc.want, location.Query())
		})
	}
}

func Test_Redirect_Route_ParamCannotMoveTheQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		param string
		want  string
	}{
		{name: "plain", param: "fiber", want: "/user/fiber?q=1"},
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
		{name: "param opens both", param: "a?b=2#c", want: "/user/a%3Fb=2%23c?q=1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, redirRouteLocation(t, "/user/:name", RedirectConfig{
				Params:  Map{"name": tc.param},
				Queries: map[string]string{"q": "1"},
			}))
		})
	}
}

func Test_Redirect_Route_ParamCannotLeaveTheOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		param string
		want  string
	}{
		{name: "embedded slash", param: "docs/index.html", want: "/docs/index.html"},
		{name: "query delimiter", param: "a?b=2", want: "/a%3Fb=2"},
		{name: "fragment delimiter", param: "a#b", want: "/a%23b"},
		{name: "percent", param: "100%", want: "/100%25"},
		{
			// A leading slash stays on-origin after URL normalization.
			name:  "leading slash",
			param: "/evil.com",
			want:  "/evil.com",
		},
		{name: "leading slash run", param: "//evil.com", want: "/evil.com"},
		{
			// A backslash is encoded before a WHATWG parser can fold it.
			name:  "leading backslash",
			param: `\evil.com`,
			want:  "/%5Cevil.com",
		},
		{name: "mixed slash run", param: `/\/evil.com`, want: "/%5C/evil.com"},
		{
			// Controls are encoded before URL input preprocessing can remove them.
			name:  "tab before the slash",
			param: "\t/evil.com",
			want:  "/%09/evil.com",
		},
		{
			// A scheme stays path data.
			name:  "absolute URL is a path segment",
			param: "https://evil.com",
			want:  "/https://evil.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get("/*", func(c Ctx) error { return c.JSON(c.Params("*")) }).Name("wildcard")
			c := apptest.Ctx(t, app)

			require.NoError(t, c.Redirect().Route("wildcard", RedirectConfig{Params: Map{"*": tc.param}}))
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

func Test_Redirect_Back(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.JSON("Home") }).Name("home")
	c := apptest.Ctx(t, app)

	require.NoError(t, c.Redirect().Back("/"))
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	err := c.Redirect().Back()
	require.Equal(t, 500, c.Response().StatusCode())
	require.ErrorIs(t, err, ErrRedirectBackNoFallback)
}

func Test_Redirect_Back_WithReferer(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.JSON("Home") }).Name("home")
	app.Get("/back", func(c Ctx) error { return c.JSON("Back") }).Name("back")
	c := apptest.Ctx(t, app)

	c.Request().Header.Set(HeaderReferer, "/back")
	require.NoError(t, c.Redirect().Back("/"))
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/back", c.Get(HeaderReferer))
	require.Equal(t, "/back", string(c.Response().Header.Peek(HeaderLocation)))
}

// rootMiscBExpectBackTo asks a context for a request to example.com that came from referer to redirect back,
// with fallback as the fallback, and expects a 303 to want.
func rootMiscBExpectBackTo(t *testing.T, referer, fallback, want string) {
	t.Helper()

	c := apptest.Ctx(t, New())
	c.Request().Header.Set(HeaderReferer, referer)
	c.Request().URI().SetHost("example.com")

	require.NoError(t, c.Redirect().Back(fallback))
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, want, string(c.Response().Header.Peek(HeaderLocation)))
}

func Test_Redirect_Back_WithCrossOriginReferer(t *testing.T) {
	t.Parallel()

	// The request is to example.com over plain HTTP, and the fallback is "/".
	tests := []struct {
		name    string
		referer string
		want    string
	}{
		{name: "cross-origin referer is rejected", referer: "https://evil.com/phishing", want: "/"},
		{name: "same-origin referer with host is accepted", referer: "http://example.com/dashboard", want: "http://example.com/dashboard"},
		{name: "same-origin referer with a default port is accepted", referer: "http://example.com:80/dashboard", want: "http://example.com:80/dashboard"},
		{name: "scheme mismatch is rejected", referer: "https://example.com/dashboard", want: "/"},
		{name: "port mismatch is rejected", referer: "http://example.com:8080/admin", want: "/"},
		{name: "relative path referer is accepted (no host in URL)", referer: "/back", want: "/back"},
		// Scheme-only referrers are rejected.
		{name: "javascript scheme-only referer is rejected", referer: "javascript:alert(1)", want: "/"},
		{name: "mailto scheme-only referer is rejected", referer: "mailto:test@example.com", want: "/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rootMiscBExpectBackTo(t, tc.referer, "/", tc.want)
		})
	}
}

// Test_Redirect_Back_NoFallback pins that Back without a fallback is an error when there is no referer to go back
// to: none was sent, or the one that was sent is cross-origin.
func Test_Redirect_Back_NoFallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		referer string
	}{
		{name: "no referer at all"},
		{name: "cross-origin referer", referer: "https://evil.com/phishing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			if tc.referer != "" {
				c.Request().Header.Set(HeaderReferer, tc.referer)
				c.Request().URI().SetHost("example.com")
			}

			err := c.Redirect().Back()
			require.Equal(t, 500, c.Response().StatusCode())
			require.ErrorIs(t, err, ErrRedirectBackNoFallback)
		})
	}
}

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
	tests := []struct {
		referer string
		want    string
	}{
		{referer: "//?x", want: "/fallback"},
		{referer: "//#f", want: "/fallback"},
		{referer: "//@", want: "/fallback"},
		// A reference that is only slashes normalizes to a plain absolute path and
		// really is this origin, so it is still honored.
		{referer: "//", want: "/"},
		{referer: "///", want: "/"},
	}

	for _, tc := range tests {
		t.Run(tc.referer, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get("/back", func(c Ctx) error { return c.Redirect().Back("/fallback") })

			req := apptest.Req(MethodGet, "/back", apptest.WithHost("victim.test"), apptest.WithHeader(HeaderReferer, tc.referer))
			resp := apptest.Do(t, app, req)
			require.Equal(t, tc.want, resp.Header.Get(HeaderLocation))
		})
	}
}

// Test_Redirect_Back_BrowserNormalizedReferer pins that Back judges and
// returns a referer the way a browser resolves it.
func Test_Redirect_Back_BrowserNormalizedReferer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		referer  string
		expected string
	}{
		// Referer values that net/url reports as ordinary relative paths but that a
		// browser resolves to another origin, because it folds backslashes onto
		// forward slashes, drops ASCII tab/newline, and treats any leading run of
		// slashes as the start of an authority.
		{referer: `/\evil.com`, expected: "/fallback"},
		{referer: `/\/evil.com`, expected: "/fallback"},
		{referer: `\\evil.com`, expected: "/fallback"},
		{referer: `\/evil.com`, expected: "/fallback"},
		{referer: "/\t/evil.com", expected: "/fallback"},
		{referer: "\t//evil.com", expected: "/fallback"},
		{referer: ` //evil.com`, expected: "/fallback"},
		{referer: `///evil.com`, expected: "/fallback"},
		{referer: `/////evil.com`, expected: "/fallback"},
		{referer: `https:///evil.com`, expected: "/fallback"},

		// A same-origin referer is redirected to in its browser-normalized form, so
		// a client that does not fold backslashes cannot resolve a different origin
		// than the one that was validated.
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

	for _, tc := range tests {
		t.Run(tc.referer, func(t *testing.T) {
			t.Parallel()
			rootMiscBExpectBackTo(t, tc.referer, "/fallback", tc.expected)
		})
	}
}

func Test_Redirect_WithInput_ReusesClearedMap(t *testing.T) {
	t.Parallel()
	c := apptest.CtxAs[*DefaultCtx](t, New())

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

func Test_Redirect_parseAndClearFlashMessages(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user", func(c Ctx) error { return c.SendString("user") }).Name("user")

	c := apptest.CtxAs[*DefaultCtx](t, app)

	c.Request().Header.Set(HeaderCookie, redirFlashCookie(t))
	c.Redirect().parseAndClearFlashMessages()

	require.Equal(t, FlashMessage{Key: "success", Value: "1", Level: 0}, c.Redirect().Message("success"))
	require.Equal(t, FlashMessage{Key: "message", Value: "test", Level: 0}, c.Redirect().Message("message"))
	require.Equal(t, FlashMessage{}, c.Redirect().Message("success"))
	require.Equal(t, FlashMessage{}, c.Redirect().Message("not_message"))
	require.Empty(t, c.Redirect().Messages())

	require.Equal(t, OldInputData{Key: "id", Value: "1"}, c.Redirect().OldInput("id"))
	require.Equal(t, OldInputData{Key: "name", Value: "tom"}, c.Redirect().OldInput("name"))
	require.Equal(t, OldInputData{}, c.Redirect().OldInput("not_name"))
	require.Equal(t, []OldInputData{{Key: "name", Value: "tom"}, {Key: "id", Value: "1"}}, c.Redirect().OldInputs())

	assertFlashCookieCleared(t, string(c.Response().Header.Peek(HeaderSetCookie)))

	c.Request().Header.Set(HeaderCookie, "fiber_flash=test")
	c.Redirect().parseAndClearFlashMessages()
	require.Empty(t, c.Redirect().messages)
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

func Test_ReleaseRedirect_DropsOversizedMessages(t *testing.T) {
	t.Parallel()

	r := AcquireRedirect()
	for i := range maxPoolableMessages + 1 {
		r.With(strconv.Itoa(i), "v")
	}
	ReleaseRedirect(r)

	// Dropped rather than cleared and pooled, so the array is left as it was.
	require.Len(t, r.messages, maxPoolableMessages+1)
}

// Not parallel: r is read after the pool took it, and Go pauses the parallel
// tests, which could be handed it meanwhile, while a sequential one runs.
func Test_ReleaseRedirect_PoolsUpToTheLimit(t *testing.T) {
	r := AcquireRedirect()
	for i := range maxPoolableMessages {
		r.With(strconv.Itoa(i), "v")
	}
	require.Greater(t, cap(r.messages), maxPoolableMessages, "append grew the array past the limit")
	ReleaseRedirect(r)

	require.Empty(t, r.messages, "cleared and pooled, not dropped")
}

func Test_Redirect_Messages_ClearsFlashMessages(t *testing.T) {
	t.Parallel()
	c := apptest.CtxAs[*DefaultCtx](t, New())
	c.Request().Header.Set(HeaderCookie, redirFlashCookie(t))
	c.Redirect().parseAndClearFlashMessages()

	require.Equal(t, []FlashMessage{
		{Key: "success", Value: "1", Level: 0},
		{Key: "message", Value: "test", Level: 0},
	}, c.Redirect().Messages())

	require.Empty(t, c.Redirect().Messages())
	require.Equal(t, FlashMessage{}, c.Redirect().Message("success"))

	require.Equal(t, []OldInputData{{Key: "name", Value: "tom"}, {Key: "id", Value: "1"}}, c.Redirect().OldInputs())
}

func Test_Redirect_CompleteFlowWithFlashMessages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		flash  map[string]string
		name   string
		source string
		target string
	}{
		{
			name:   "plain values",
			source: "/source",
			target: "/target",
			flash: map[string]string{
				"string_message": "Hello, World!",
				"number_message": "12345",
				"bool_message":   "true",
			},
		},
		{
			// Values that need encoding to travel in a cookie.
			name:   "special characters",
			source: "/special-source",
			target: "/special-target",
			flash: map[string]string{
				"null_bytes":    "Contains\x00null\x00bytes",
				"control_chars": "Contains\r\ncontrol\tcharacters",
				"unicode":       "Unicode: 你好世界",
				"emoji":         "Emoji: 🔥🚀😊",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get(tc.source, func(c Ctx) error {
				r := c.Redirect()
				for key, value := range tc.flash {
					r.With(key, value)
				}
				return r.To(tc.target)
			})
			app.Get(tc.target, func(c Ctx) error {
				received := make(map[string]string, len(tc.flash))
				for key := range tc.flash {
					received[key] = c.Redirect().Message(key).Value
				}
				return c.JSON(received)
			})

			resp := apptest.Expect(t, app, tc.source, StatusSeeOther)
			require.Equal(t, tc.target, resp.Header.Get(HeaderLocation))

			var flashCookie *http.Cookie
			for _, cookie := range resp.Cookies() {
				if cookie.Name == FlashCookieName {
					flashCookie = cookie
					break
				}
			}
			require.NotNil(t, flashCookie, "Flash cookie should be set")

			req := apptest.Req(MethodGet, tc.target, apptest.WithHeader(HeaderCookie, flashCookie.Name+"="+flashCookie.Value))
			resp = apptest.Expect(t, app, req, StatusOK)
			assertFlashCookieCleared(t, resp.Header.Get(HeaderSetCookie))

			var received map[string]string
			require.NoError(t, json.Unmarshal(apptest.BodyBytes(t, resp), &received))
			require.Equal(t, tc.flash, received)
		})
	}
}

func Benchmark_Redirect_Route(b *testing.B) {
	c := redirBenchCtx(b, "/user/:name")
	b.ReportAllocs()
	var err error
	for b.Loop() {
		err = c.Redirect().Route("user", RedirectConfig{Params: Map{"name": "fiber"}})
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
	require.Equal(b, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

func Benchmark_Redirect_Route_WithQueries(b *testing.B) {
	c := redirBenchCtx(b, "/user/:name")
	b.ReportAllocs()
	var err error
	for b.Loop() {
		err = c.Redirect().Route("user", RedirectConfig{
			Params:  Map{"name": "fiber"},
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

func Benchmark_Redirect_Route_WithFlashMessages(b *testing.B) {
	c := redirBenchCtx(b, "/user")
	b.ReportAllocs()
	var err error
	for b.Loop() {
		err = c.Redirect().With("success", "1").With("message", "test").Route("user")
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
	require.Equal(b, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	msgs := redirFlashCookieMsgs(b, c)
	require.Contains(b, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(b, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

// Test_Redirect_DisableFlashMessages pins the switch on both request handlers:
// with it set, With and WithInput set no cookie, an incoming flash cookie is
// neither read nor expired, and the accessors report nothing.
func Test_Redirect_DisableFlashMessages(t *testing.T) {
	t.Parallel()
	incoming := redirFlashCookie(t)

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

				resp := apptest.Expect(t, app, "/source?name=john", StatusSeeOther)
				if disabled {
					require.Empty(t, resp.Header.Get(HeaderSetCookie), "no flash cookie may be set")
				} else {
					require.Contains(t, resp.Header.Get(HeaderSetCookie), FlashCookieName+"=")
				}

				resp = apptest.Expect(t, app, apptest.Req(MethodGet, "/target", apptest.WithHeader(HeaderCookie, incoming)), StatusOK)
				body := apptest.Body(t, resp)
				if disabled {
					require.Equal(t, "0/0", body)
					require.Empty(t, resp.Header.Get(HeaderSetCookie), "the incoming cookie must be left alone")
				} else {
					require.Equal(t, "2/2", body)
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
	{key: "success", value: "1"},
	{key: "message", value: "test"},
	{key: "name", value: "tom", isOldInput: true},
	{key: "id", value: "1", isOldInput: true},
}

func Benchmark_Redirect_parseAndClearFlashMessages(b *testing.B) {
	c := redirBenchCtx(b, "/user")
	c.Request().Header.Set(HeaderCookie, redirFlashCookie(b))

	b.ReportAllocs()
	for b.Loop() {
		c.Redirect().parseAndClearFlashMessages()
	}

	require.Equal(b, FlashMessage{Key: "success", Value: "1"}, c.Redirect().Message("success"))
	require.Equal(b, FlashMessage{Key: "message", Value: "test"}, c.Redirect().Message("message"))
	require.Equal(b, OldInputData{Key: "id", Value: "1"}, c.Redirect().OldInput("id"))
	require.Equal(b, OldInputData{Key: "name", Value: "tom"}, c.Redirect().OldInput("name"))
}

func Benchmark_Redirect_processFlashMessages(b *testing.B) {
	c := redirBenchCtx(b, "/user")
	c.Redirect().With("success", "1").With("message", "test")

	b.ReportAllocs()
	for b.Loop() {
		c.Redirect().processFlashMessages()
	}

	msgs := redirFlashCookieMsgs(b, c)
	require.Len(b, msgs, 2)
	require.Contains(b, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(b, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

func Benchmark_Redirect_Messages(b *testing.B) {
	c := redirBenchParsedCtx(b)

	var msgs []FlashMessage
	msgTemplate := testredirectionMsgs

	b.ReportAllocs()
	for b.Loop() {
		c.flashMessages = c.flashMessages[:0]
		c.flashMessages = append(c.flashMessages, msgTemplate...)
		msgs = c.Redirect().Messages()
	}

	require.Contains(b, msgs, FlashMessage{Key: "success", Value: "1", Level: 0})
	require.Contains(b, msgs, FlashMessage{Key: "message", Value: "test", Level: 0})
}

func Benchmark_Redirect_OldInputs(b *testing.B) {
	c := redirBenchParsedCtx(b)

	var oldInputs []OldInputData

	b.ReportAllocs()
	for b.Loop() {
		oldInputs = c.Redirect().OldInputs()
	}

	require.Contains(b, oldInputs, OldInputData{Key: "name", Value: "tom"})
	require.Contains(b, oldInputs, OldInputData{Key: "id", Value: "1"})
}

func Benchmark_Redirect_Message(b *testing.B) {
	c := redirBenchParsedCtx(b)

	var msg FlashMessage
	msgTemplate := testredirectionMsgs

	b.ReportAllocs()
	for b.Loop() {
		c.flashMessages = c.flashMessages[:0]
		c.flashMessages = append(c.flashMessages, msgTemplate...)
		msg = c.Redirect().Message("message")
	}

	require.Equal(b, FlashMessage{Key: "message", Value: "test", Level: 0}, msg)
}

func Benchmark_Redirect_OldInput(b *testing.B) {
	c := redirBenchParsedCtx(b)

	var input OldInputData

	b.ReportAllocs()
	for b.Loop() {
		input = c.Redirect().OldInput("name")
	}

	require.Equal(b, OldInputData{Key: "name", Value: "tom"}, input)
}

func Test_Redirect_FlashCookie_Attributes(t *testing.T) {
	t.Parallel()

	// The flash payload is hex-encoded, not encrypted, and WithInput copies the
	// whole submitted form into it — including whatever the user typed into a
	// password field. It is only ever read back on the server, so it must not be
	// reachable from document.cookie, and must not travel in the clear when the
	// request that produced it was served over TLS.
	newApp := func() *App {
		app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0"}}})
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
		resp := apptest.Do(t, app, req)

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
		return apptest.Body(t, apptest.Do(t, app, req))
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
				require.Equal(t, "/previous", string(c.Response().Header.Peek(HeaderLocation)), "sent as %q", sent)
				app.ReleaseCtx(c)
			}
		})
	}
}

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
			c := apptest.Ctx(t, New())

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
		c := apptest.Ctx(t, New(Config{DisableHeaderNormalizing: true}))

		c.Request().Header.DisableNormalizing()
		c.Request().Header.Set("X-Not-A-Referer", "/nope")

		require.NoError(t, c.Redirect().Back("/fallback"))
		require.Equal(t, "/fallback", string(c.Response().Header.Peek(HeaderLocation)))
	})
}

// Test_Redirect_parseAndClearFlashMessages_UndecodablePayload covers a cookie
// that is not what this application wrote: one that is not even hex, and ones
// that decode as hex but not as a list of messages.
//
// UnmarshalMsg re-slices to the length the payload declares before it decodes
// any element, so a failure partway leaves the slice holding zero-valued
// entries. Reading those back would hand the handler empty messages nobody set,
// which is why the slice is dropped rather than kept at whatever length the
// payload claimed.
func Test_Redirect_parseAndClearFlashMessages_UndecodablePayload(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		cookie string
	}{
		// The cookie is expired even though hex decoding failed. A value this
		// application cannot decode is one it will never decode, so leaving it in
		// place would mean re-running hex and msgp over attacker-controlled bytes
		// on every subsequent request for as long as the browser kept sending it.
		{name: "not hex", cookie: "not-a-valid-hex-string"},
		// An array header claiming four elements, and then nothing.
		{name: "truncated after the count", cookie: hex.EncodeToString([]byte{0x94})},
		// A byte message-pack never assigns.
		{name: "never a valid type", cookie: hex.EncodeToString([]byte{0xc1, 0xc1})},
		// A long claimed length with no elements behind it, which is the shape
		// that leaves the most zero-valued entries.
		{name: "claims more than it carries", cookie: hex.EncodeToString([]byte{0xdc, 0x01, 0x00})},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.CtxAs[*DefaultCtx](t, New())

			r := AcquireRedirect()
			r.c = c
			defer ReleaseRedirect(r)

			c.Request().Header.SetCookie(FlashCookieName, tc.cookie)

			r.parseAndClearFlashMessages()

			require.Empty(t, r.messages)
			require.Empty(t, c.flashMessages, "a payload that failed to decode leaves nothing behind")
			assertFlashCookieCleared(t, c.GetRespHeader(HeaderSetCookie))
		})
	}
}

func Test_Redirect_Messages_ProgrammaticCookieHeader(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.SendString(c.Redirect().Message("success").Value + "," + c.Redirect().OldInput("id").Value)
	})

	msgs := redirectionMsgs{{key: "success", value: "1"}, {key: "id", value: "42", isOldInput: true}}
	val, err := msgs.MarshalMsg(nil)
	require.NoError(t, err)

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/")
	// Set, not parsed from the wire: RawHeaders stays empty.
	fctx.Request.Header.Set(HeaderCookie, FlashCookieName+"="+hex.EncodeToString(val))
	require.Empty(t, fctx.Request.Header.RawHeaders())

	app.Handler()(fctx)

	require.Equal(t, StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "1,42", string(fctx.Response.Body()))

	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(FlashCookieName)
	require.True(t, fctx.Response.Header.Cookie(cookie), "the consumed flash cookie must be expired on the client")
	require.Empty(t, cookie.Value())
	require.True(t, cookie.Expire().Before(time.Now()), "expiry must lie in the past")
}
