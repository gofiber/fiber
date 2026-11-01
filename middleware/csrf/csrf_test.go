package csrf

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/redact"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var trustedProxyConfig = fiber.Config{
	TrustProxy:       true,
	TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
}

func newTrustedApp() *fiber.App {
	return fiber.New(trustedProxyConfig)
}

func newTrustedRequestCtx() *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("0.0.0.0")}))

	return ctx
}

// csrfTestHandler registers the middleware configured by cfg and a POST / that
// answers 200 on app, and returns the handler of app.
func csrfTestHandler(app *fiber.App, cfg *Config) fasthttp.RequestHandler {
	app.Use(New(*cfg))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	return app.Handler()
}

// csrfTestResponseToken returns the CSRF token carried by the Set-Cookie header
// of the response of ctx, which joins every cookie the answer sets.
func csrfTestResponseToken(t *testing.T, ctx *fasthttp.RequestCtx) string {
	t.Helper()

	setCookie := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	for part := range strings.SplitSeq(setCookie, ";") {
		if name, token, found := strings.Cut(utils.TrimSpace(part), "="); found && name == ConfigDefault.CookieName {
			return token
		}
	}

	require.Failf(t, "token not found", "no %s cookie in Set-Cookie %q", ConfigDefault.CookieName, setCookie)
	return ""
}

// csrfTestToken sends the request prepared on ctx through h and returns the
// token the middleware issued in its answer.
func csrfTestToken(t *testing.T, h fasthttp.RequestHandler, ctx *fasthttp.RequestCtx) string {
	t.Helper()

	h(ctx)
	return csrfTestResponseToken(t, ctx)
}

// csrfTestPost resets ctx to a POST that carries token in the CSRF header and
// in the CSRF cookie, sends it through h and requires the status want.
func csrfTestPost(t *testing.T, h fasthttp.RequestHandler, ctx *fasthttp.RequestCtx, token string, want int) {
	t.Helper()

	csrfTestSessionPost(t, h, ctx, "", token, token, want)
}

// csrfTestSessionPost resets ctx to a POST of the session sessionID that carries
// headerToken in the CSRF header and cookieToken in the CSRF cookie, each unless
// it is empty, sends it through h and requires the status want.
func csrfTestSessionPost(t *testing.T, h fasthttp.RequestHandler, ctx *fasthttp.RequestCtx, sessionID, headerToken, cookieToken string, want int) {
	t.Helper()

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	if headerToken != "" {
		ctx.Request.Header.Set(HeaderName, headerToken)
	}
	if sessionID != "" {
		ctx.Request.Header.SetCookie("_session", sessionID)
	}
	if cookieToken != "" {
		ctx.Request.Header.SetCookie(ConfigDefault.CookieName, cookieToken)
	}
	h(ctx)
	require.Equal(t, want, ctx.Response.StatusCode())
}

// csrfTestSession registers on app the middleware configured by cfg on top of a
// session store keyed by the "_session" cookie, and a POST / that answers 200.
// It returns the handler of app, a request context that carries a saved session
// and the id of that session.
func csrfTestSession(t *testing.T, app *fiber.App, cfg *Config) (fasthttp.RequestHandler, *fasthttp.RequestCtx, string) {
	t.Helper()

	store := session.NewStore(session.Config{Extractor: extractors.FromCookie("_session")})
	ctx := &fasthttp.RequestCtx{}

	sess, err := store.Get(apptest.CtxFor(t, app, ctx))
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	id := sess.ID()
	require.NoError(t, sess.Save())
	ctx.Request.Header.SetCookie("_session", id)

	withSession := *cfg
	withSession.Session = store
	return csrfTestHandler(app, &withSession), ctx, id
}

// csrfTestCookie returns the value of the cookie name set on the response of
// ctx, requiring that it is set and not empty.
func csrfTestCookie(t *testing.T, ctx *fasthttp.RequestCtx, name string) string {
	t.Helper()

	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(name)
	require.True(t, ctx.Response.Header.Cookie(cookie), "expected the cookie %s to be set", name)
	value := string(cookie.Value())
	require.NotEmpty(t, value)
	return value
}

// csrfTestAccept registers the middleware configured by cfg, a GET / that
// answers with the token and a POST / that answers "accepted" on app, and
// returns the handler of app.
func csrfTestAccept(app *fiber.App, cfg *Config) fasthttp.RequestHandler {
	app.Use(New(*cfg))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(TokenFromContext(c)) })
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("accepted") })

	return app.Handler()
}

// csrfTestVisit sends a GET for / from example.com through the handler of an
// app set up by csrfTestAccept. It returns the token the answer carries in its
// body and the cookie that holds it, which a double-submit check needs.
func csrfTestVisit(t *testing.T, h fasthttp.RequestHandler) (token, cookie string) { //nolint:nonamedreturns // gocritic unnamedResult asks for names on two results of one type
	t.Helper()

	get := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(get)
	get.Header.SetMethod(fiber.MethodGet)
	get.SetRequestURI("/")
	get.Header.SetHost("example.com")
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(get, nil, nil)
	h(ctx)

	token = string(ctx.Response.Body())
	require.NotEmpty(t, token)
	cookie, _, _ = strings.Cut(string(ctx.Response.Header.Peek(fiber.HeaderSetCookie)), ";")
	require.NotEmpty(t, cookie)
	return token, cookie
}

// csrfTestPostWith sends a POST for / to example.com through h that carries cookie
// and, unless it is empty, token in the CSRF header, plus whatever build adds, and
// returns the status of the answer. Under raw the header names are not normalized.
//
//nolint:revive // flag-parameter: raw selects the header-name spelling the caller tests
func csrfTestPostWith(h fasthttp.RequestHandler, raw bool, cookie, token string, build func(*fasthttp.Request)) int {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	if raw {
		req.Header.DisableNormalizing()
	}
	req.Header.SetMethod(fiber.MethodPost)
	req.SetRequestURI("/")
	req.Header.SetHost("example.com")
	req.Header.Set(fiber.HeaderCookie, cookie)
	if token != "" {
		req.Header.Set(HeaderName, token)
	}
	build(req)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	h(ctx)
	return ctx.Response.StatusCode()
}

// csrfTestTrusted returns the handler of an app that trusts its proxy and guards
// POST / with the middleware configured by cfg, and a token issued to a client
// that reached it with the X-Forwarded-Proto proto.
func csrfTestTrusted(t *testing.T, cfg *Config, proto string) (h fasthttp.RequestHandler, token string) { //nolint:nonamedreturns // gocritic unnamedResult asks for names on these results
	t.Helper()

	h = csrfTestHandler(newTrustedApp(), cfg)
	ctx := newTrustedRequestCtx()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, proto)
	return h, csrfTestToken(t, h, ctx)
}

// csrfTestPostCase is a POST that carries a valid token and that the origin
// and referer tests send through an app trusting its proxy, with the status and
// body the answer must have.
type csrfTestPostCase struct {
	headers    map[string]string // other headers, such as those a proxy adds
	name       string
	scheme     string // scheme of the request URI and protocol, set together with host
	host       string // host of the request URI and of the Host header; none are set when empty
	hostHeader string // the Host header, when it is not host
	origin     string
	referer    string
	body       string // the exact body of the answer, not checked when empty
	status     int
}

// csrfTestPostCases runs every case as a parallel subtest: a POST that carries
// token in the CSRF header and cookie, shaped by the case, sent through h by a
// trusted proxy.
func csrfTestPostCases(t *testing.T, h fasthttp.RequestHandler, token string, cases []csrfTestPostCase) {
	t.Helper()

	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := newTrustedRequestCtx()
			ctx.Request.Header.SetMethod(fiber.MethodPost)
			if tc.host != "" {
				ctx.Request.URI().SetScheme(tc.scheme)
				ctx.Request.URI().SetHost(tc.host)
				ctx.Request.Header.SetProtocol(tc.scheme)
				ctx.Request.Header.SetHost(cmp.Or(tc.hostHeader, tc.host))
			}
			for name, value := range tc.headers {
				ctx.Request.Header.Set(name, value)
			}
			if tc.origin != "" {
				ctx.Request.Header.Set(fiber.HeaderOrigin, tc.origin)
			}
			if tc.referer != "" {
				ctx.Request.Header.Set(fiber.HeaderReferer, tc.referer)
			}
			ctx.Request.Header.Set(HeaderName, token)
			ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			h(ctx)

			require.Equal(t, tc.status, ctx.Response.StatusCode())
			if tc.body != "" {
				require.Equal(t, tc.body, string(ctx.Response.Body()))
			}
		})
	}
}

// Test_CSRF_StorageErrors checks that every failing storage operation reaches the
// ErrorHandler wrapped with what the middleware was doing.
func Test_CSRF_StorageErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		keyGenerator func() string
		name         string
		failing      string // the storage operation that fails on the token, one of the storagetest.Op constants
		method       string
		want         string
		stored       bool // the token is in storage
		singleUse    bool
		headerToken  bool // the request carries the token in the CSRF header
		cookieToken  bool // the request carries the token in the CSRF cookie
	}{
		{
			name: "get error", failing: storagetest.OpGet, method: fiber.MethodGet, cookieToken: true,
			want: "csrf: failed to fetch token from storage",
		},
		{
			name: "set error", failing: storagetest.OpSet, method: fiber.MethodGet,
			keyGenerator: func() string { return "token" }, want: "csrf: failed to store token in storage",
		},
		{
			name: "delete error", failing: storagetest.OpDelete, method: fiber.MethodPost, stored: true,
			singleUse: true, headerToken: true, cookieToken: true, want: "csrf: failed to delete token from storage",
		},
		// The fetch error path that runs after the double-submit cookie comparison succeeds.
		{
			name: "get error on validation", failing: storagetest.OpGet, method: fiber.MethodPost, headerToken: true,
			cookieToken: true, want: "csrf: failed to fetch token from storage",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			if tc.stored {
				storage.Data[storagePrefix+"token"] = []byte("value")
			}
			storage.SetErr(tc.failing, storagePrefix+"token", errors.New("boom"))

			var captured error
			app := fiber.New()

			app.Use(New(Config{
				Storage:        storage,
				SingleUseToken: tc.singleUse,
				KeyGenerator:   tc.keyGenerator,
				ErrorHandler: func(_ fiber.Ctx, err error) error {
					captured = err
					return fiber.ErrTeapot
				},
			}))

			app.All("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

			req := httptest.NewRequest(tc.method, "/", http.NoBody)
			if tc.headerToken {
				req.Header.Set(HeaderName, "token")
			}
			if tc.cookieToken {
				req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: "token"})
			}

			apptest.Expect(t, app, req, fiber.StatusTeapot)
			require.Error(t, captured)
			require.ErrorContains(t, captured, tc.want)
		})
	}
}

func Test_CSRF(t *testing.T) {
	t.Parallel()
	h := csrfTestHandler(fiber.New(), &Config{})
	ctx := &fasthttp.RequestCtx{}

	methods := [5]string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery}

	for _, method := range methods {
		// Generate CSRF token
		ctx.Request.Header.SetMethod(method)
		h(ctx)

		// Without CSRF cookie
		csrfTestSessionPost(t, h, ctx, "", "", "", 403)

		// Invalid CSRF token
		csrfTestSessionPost(t, h, ctx, "", "johndoe", "", 403)

		// Valid CSRF token
		ctx.Request.Header.Reset()
		ctx.Request.ResetBody()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(method)
		token := csrfTestToken(t, h, ctx)

		csrfTestPost(t, h, ctx, token, 200)
	}
}

func Test_CSRF_WithSession(t *testing.T) {
	t.Parallel()
	h, ctx, sessionID := csrfTestSession(t, fiber.New(), &Config{})

	methods := [5]string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery}

	for _, method := range methods {
		// Generate CSRF token
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		ctx.Request.Header.SetCookie("_session", sessionID)
		h(ctx)

		// Without CSRF cookie
		csrfTestSessionPost(t, h, ctx, sessionID, "", "", 403)

		// Empty/invalid CSRF token
		csrfTestSessionPost(t, h, ctx, sessionID, "johndoe", "", 403)

		// Valid CSRF token
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(method)
		ctx.Request.Header.SetCookie("_session", sessionID)
		token := csrfTestToken(t, h, ctx)

		csrfTestSessionPost(t, h, ctx, sessionID, token, token, 200)
	}
}

func Test_CSRF_WithSession_Middleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	smh, sstore := session.NewWithStore()
	app.Use(smh)
	app.Use(New(Config{Session: sstore}))

	app.Get("/", func(c fiber.Ctx) error {
		sess := session.FromContext(c)
		sess.Set("hello", "world")
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/", func(c fiber.Ctx) error {
		sess := session.FromContext(c)
		if sess.Get("hello") != "world" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token and session_id
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)

	csrfToken := csrfTestCookie(t, ctx, ConfigDefault.CookieName)
	sessionID := csrfTestCookie(t, ctx, "session_id")

	// Use the CSRF token and session_id
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, csrfToken)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, csrfToken)
	ctx.Request.Header.SetCookie("session_id", sessionID)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
}

func Test_CSRF_ExpiredToken(t *testing.T) {
	t.Parallel()
	const idleTimeout = 1 * time.Second

	h := csrfTestHandler(fiber.New(), &Config{IdleTimeout: idleTimeout})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	token := csrfTestToken(t, h, ctx)
	csrfTestPost(t, h, ctx, token, 200)

	// Wait for the token to expire on the cached clock the storage compares against
	clocktest.SleepPast(t, idleTimeout)

	// Expired CSRF token
	csrfTestPost(t, h, ctx, token, 403)
}

func Test_CSRF_ExpiredToken_WithSession(t *testing.T) {
	t.Parallel()
	h, ctx, sessionID := csrfTestSession(t, fiber.New(), &Config{IdleTimeout: 1 * time.Second})
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("_session", sessionID)
	token := csrfTestToken(t, h, ctx)
	csrfTestSessionPost(t, h, ctx, sessionID, token, token, 200)

	// Wait for the token to expire (session-backed tokens carry a real-clock expiry)
	time.Sleep(1*time.Second + 100*time.Millisecond)

	// Expired CSRF token
	csrfTestSessionPost(t, h, ctx, sessionID, token, token, 403)
}

func Test_CSRF_MultiUseToken(t *testing.T) {
	t.Parallel()
	h := csrfTestHandler(fiber.New(), &Config{Extractor: extractors.FromHeader("X-Csrf-Token")})
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	csrfTestSessionPost(t, h, ctx, "", "johndoe", "", 403)

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	token := csrfTestToken(t, h, ctx)

	csrfTestPost(t, h, ctx, token, 200)
	newToken := csrfTestResponseToken(t, ctx)

	// Check if the token is not a dummy value
	require.Equal(t, token, newToken)
}

func Test_CSRF_SingleUseToken(t *testing.T) {
	t.Parallel()
	h := csrfTestHandler(fiber.New(), &Config{SingleUseToken: true})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	token := csrfTestToken(t, h, ctx)
	csrfTestPost(t, h, ctx, token, 200)
	if newToken := csrfTestResponseToken(t, ctx); token == newToken {
		t.Error("new token should not be the same as the old token")
	}

	// Use the CSRF token again
	csrfTestPost(t, h, ctx, token, 403)
}

func Test_CSRF_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

func Test_CSRFLoggerTagRedactsToken(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New())
	app.Use(logger.New(logger.Config{Format: "${csrf-token}", Stream: &buf}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	apptest.Expect(t, app, "/", fiber.StatusOK)
	// CSRF tokens are randomly generated per request, so assert on the
	// redaction shape (4-byte prefix + Mask) rather than a fixed value.
	got := buf.String()
	require.Len(t, got, redact.PrefixLength+len(redact.Mask))
	require.True(t, strings.HasSuffix(got, redact.Mask), "expected suffix %q in %q", redact.Mask, got)
}

// Test_CSRFLogContextTagRedactsToken runs serially because it mutates
// package-global default logger output and context format.
func Test_CSRFLogContextTagRedactsToken(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, "csrf-token=${csrf-token} ")

	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Regexp(t, `\[Info\] csrf-token=.{`+strconv.Itoa(redact.PrefixLength)+`}`+regexp.QuoteMeta(redact.Mask)+` start`, buf.String())
}

func Test_CSRF_SecFetchSite(t *testing.T) {
	t.Parallel()
	errorHandler := func(c fiber.Ctx, err error) error {
		return c.Status(fiber.StatusForbidden).SendString(err.Error())
	}

	app := newTrustedApp()

	app.Use(New(Config{ErrorHandler: errorHandler}))

	app.All("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()
	ctx := newTrustedRequestCtx()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetHost("example.com")
	token := csrfTestToken(t, h, ctx)

	tests := []struct {
		name    string
		method  string // POST when empty
		site    string // the Sec-Fetch-Site header, not sent when empty
		origin  string // not sent when empty
		status  int
		https   bool
		invalid bool // the answer must say ErrFetchSiteInvalid
	}{
		{name: "same-origin allowed", site: "same-origin", origin: "http://example.com", status: 200},
		{name: "none allowed", site: "none", origin: "http://example.com", status: 200},
		{name: "cross-site with origin allowed", site: "cross-site", origin: "http://example.com", status: 200},
		{name: "same-site with origin allowed", site: "same-site", origin: "http://example.com", status: 200},
		{
			name: "cross-site with mismatched origin blocked", site: "cross-site", origin: "https://attacker.example",
			status: 403,
		},
		{name: "same-site with null origin blocked", site: "same-site", origin: "null", status: 403, https: true},
		{name: "invalid header blocked", site: "weird", origin: "http://example.com", status: 403, invalid: true},
		{name: "no header with no origin", status: 200},
		{name: "no header with matching origin", origin: "http://example.com", status: 200},
		{name: "no header with mismatched origin", origin: "https://attacker.example", status: 403},
		// Origin comparison is case-insensitive end-to-end: the raw
		// mixed-case header must be accepted on the same-origin fast path.
		{name: "no header with mixed-case matching origin", origin: "HTTP://EXAMPLE.COM", status: 200},
		{name: "no header with null origin", origin: "null", status: 403, https: true},
		// "null" detection is case-insensitive too. Over plain HTTP an
		// absent/null origin clears the error (200); a case-sensitive
		// regression would instead parse "NULL" as a URL and reject with
		// ErrOriginNoMatch (403), so the outcomes are distinguishable.
		{name: "no header with uppercase null origin over http", origin: "NULL", status: 200},
		{name: "GET allowed", method: fiber.MethodGet, site: "cross-site", status: 200},
		{name: "HEAD allowed", method: fiber.MethodHead, site: "cross-site", status: 200},
		{name: "OPTIONS allowed", method: fiber.MethodOptions, site: "cross-site", status: 200},
		{
			name: "PUT with mismatched origin blocked", method: fiber.MethodPut, site: "cross-site",
			origin: "https://attacker.example", status: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &fasthttp.RequestCtx{}
			scheme := "http"
			if tt.https {
				scheme = "https"
			}
			method := cmp.Or(tt.method, fiber.MethodPost)
			c.Request.Header.SetMethod(method)
			c.Request.URI().SetScheme(scheme)
			c.Request.URI().SetHost("example.com")
			c.Request.Header.SetHost("example.com")
			c.Request.Header.SetProtocol(scheme)
			if scheme == "https" {
				c.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
			}
			if tt.origin != "" {
				c.Request.Header.Set(fiber.HeaderOrigin, tt.origin)
			}
			if tt.site != "" {
				c.Request.Header.Set(fiber.HeaderSecFetchSite, tt.site)
			}

			safe := method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions || method == fiber.MethodTrace

			if !safe {
				c.Request.Header.Set(HeaderName, token)
				c.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			}

			h(c)
			require.Equal(t, tt.status, c.Response.StatusCode())
			if tt.invalid {
				require.Equal(t, ErrFetchSiteInvalid.Error(), string(c.Response.Body()))
			}
		})
	}
}

func Test_CSRF_Origin(t *testing.T) {
	t.Parallel()
	h, token := csrfTestTrusted(t, &Config{CookieSecure: true}, "http")
	viaHTTPS := map[string]string{fiber.HeaderXForwardedProto: "https"}

	csrfTestPostCases(t, h, token, []csrfTestPostCase{
		{name: "Correct Origin with port", scheme: "http", host: "example.com:8080", origin: "http://example.com:8080", status: fiber.StatusOK},
		{name: "Origin without default HTTP port against host with default port", scheme: "http", host: "example.com:80", origin: "http://example.com", status: fiber.StatusOK},
		{name: "Origin with default HTTP port against host without port", scheme: "http", host: "example.com", origin: "http://example.com:80", status: fiber.StatusOK},
		{name: "Correct Origin with wrong port", scheme: "http", host: "example.com", origin: "http://example.com:3000", status: fiber.StatusForbidden},
		{name: "Correct Origin with null", scheme: "http", host: "example.com", origin: "null", status: fiber.StatusOK},
		{
			name:       "Correct Origin with ReverseProxy",
			scheme:     "http",
			host:       "10.0.1.42.com:8080",
			hostHeader: "10.0.1.42:8080",
			origin:     "http://example.com",
			headers: map[string]string{
				fiber.HeaderXForwardedProto: "http",
				fiber.HeaderXForwardedHost:  "example.com",
				fiber.HeaderXForwardedFor:   `192.0.2.43, "[2001:db8:cafe::17]"`,
			},
			status: fiber.StatusOK,
		},
		{
			name:   "Origin without default HTTPS port against host with default port",
			scheme: "https", host: "example.com:443", origin: "https://example.com", headers: viaHTTPS,
			status: fiber.StatusOK,
		},
		{
			name:   "Origin with default HTTPS port against host without port",
			scheme: "https", host: "example.com", origin: "https://example.com:443", headers: viaHTTPS,
			status: fiber.StatusOK,
		},
		{
			name:   "Correct Origin with ReverseProxy Missing X-Forwarded-* Headers",
			scheme: "http", host: "10.0.1.42:8080", origin: "http://example.com",
			// We need to set this header to make sure c.Protocol() returns http
			headers: map[string]string{fiber.HeaderXUrlScheme: "http"},
			status:  fiber.StatusForbidden,
		},
		{
			name:    "Wrong Origin",
			origin:  "http://csrf.example.com",
			headers: map[string]string{fiber.HeaderXForwardedProto: "http", fiber.HeaderXForwardedHost: "example.com"},
			status:  fiber.StatusForbidden,
		},
	})
}

func Test_CSRF_TrustedOrigins(t *testing.T) {
	t.Parallel()
	h, token := csrfTestTrusted(t, &Config{
		CookieSecure: true,
		TrustedOrigins: []string{
			"http://safe.example.com",
			"https://safe.example.com",
			"http://*.domain-1.com",
			"https://*.domain-1.com",
		},
	}, "https")

	viaHTTPS := map[string]string{fiber.HeaderXForwardedProto: "https"}

	csrfTestPostCases(t, h, token, []csrfTestPostCase{
		{name: "Trusted Origin", scheme: "http", host: "example.com", origin: "http://safe.example.com", status: fiber.StatusOK},
		{name: "Trusted Origin Subdomain", scheme: "http", host: "domain-1.com", origin: "http://safe.domain-1.com", status: fiber.StatusOK},
		{name: "Trusted Origin deeply nested subdomain", scheme: "https", host: "a.b.c.domain-1.com", origin: "https://a.b.c.domain-1.com", status: fiber.StatusOK},
		// Origin comparisons are case-insensitive, and the fallback lowering must
		// keep matching the pre-lowered TrustedOrigins config (exact entry).
		{name: "Trusted Origin with mixed-case header", scheme: "http", host: "example.com", origin: "http://SAFE.Example.com", status: fiber.StatusOK},
		{name: "Trusted Origin with mixed-case header (wildcard subdomain entry)", scheme: "http", host: "domain-1.com", origin: "http://SAFE.Domain-1.com", status: fiber.StatusOK},
		{name: "Trusted Origin Invalid", scheme: "http", host: "domain-1.com", origin: "http://evildomain-1.com", status: fiber.StatusForbidden},
		{name: "Trusted Origin malformed subdomain", scheme: "http", host: "domain-1.com", origin: "http://evil.comdomain-1.com", status: fiber.StatusForbidden},
		{name: "Trusted Referer", scheme: "https", host: "example.com", referer: "https://safe.example.com", headers: viaHTTPS, status: fiber.StatusOK},
		// Accepted on the fold-based Match fast path, no trusted-origin fallback involved.
		{name: "same-origin Referer with mixed-case header", scheme: "https", host: "example.com", referer: "HTTPS://EXAMPLE.COM/Some/Path", headers: viaHTTPS, status: fiber.StatusOK},
		{name: "Trusted Referer Wildcard", scheme: "https", host: "domain-1.com", referer: "https://safe.domain-1.com", headers: viaHTTPS, status: fiber.StatusOK},
		{name: "Trusted Referer deeply nested subdomain", scheme: "https", host: "a.b.c.domain-1.com", referer: "https://a.b.c.domain-1.com", headers: viaHTTPS, status: fiber.StatusOK},
		// The fallback lowering must keep matching the pre-lowered TrustedOrigins config (wildcard entry).
		{name: "Trusted Referer with mixed-case header", scheme: "https", host: "domain-1.com", referer: "https://SAFE.Domain-1.com/Account/Login?Id=3", headers: viaHTTPS, status: fiber.StatusOK},
		{name: "Trusted Referer Invalid", scheme: "https", host: "api.domain-1.com", referer: "https://evildomain-1.com", headers: viaHTTPS, status: fiber.StatusForbidden},
		// The referer URL includes a path component, which must not prevent
		// matching against the trusted origin.
		{name: "Trusted Referer with path", scheme: "https", host: "example.com", referer: "https://safe.example.com/some/path?q=1", headers: viaHTTPS, status: fiber.StatusOK},
		// A wildcard subdomain referer that includes a path must still match the
		// trusted sub-origin.
		{name: "Trusted Referer Wildcard with path", scheme: "https", host: "domain-1.com", referer: "https://safe.domain-1.com/api/callback?code=abc", headers: viaHTTPS, status: fiber.StatusOK},
	})
}

func Test_CSRF_TrustedOrigins_InvalidOrigins(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		origin string
	}{
		{name: "No Scheme", origin: "localhost"},
		{name: "Wildcard", origin: "https://*"},
		{name: "Wildcard domain", origin: "https://*example.com"},
		{name: "File Scheme", origin: "file://example.com"},
		{name: "FTP Scheme", origin: "ftp://example.com"},
		{name: "Port Wildcard", origin: "http://example.com:*"},
		{name: "Multiple Wildcards", origin: "https://*.*.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() {
				app := fiber.New()
				app.Use(New(Config{CookieSecure: true, TrustedOrigins: []string{tt.origin}}))
			}, "Expected panic")
		})
	}
}

func Test_CSRF_Referer(t *testing.T) {
	t.Parallel()
	h, token := csrfTestTrusted(t, &Config{CookieSecure: true}, "https")

	viaHTTPS := map[string]string{fiber.HeaderXForwardedProto: "https"}
	viaHTTP := map[string]string{fiber.HeaderXForwardedProto: "http"}
	viaProxy := map[string]string{
		fiber.HeaderXForwardedProto: "https",
		fiber.HeaderXForwardedHost:  "example.com",
		fiber.HeaderXForwardedFor:   `192.0.2.43, "[2001:db8:cafe::17]"`,
	}

	csrfTestPostCases(t, h, token, []csrfTestPostCase{
		{name: "Correct Referer with port", scheme: "https", host: "example.com:8443", referer: "https://example.com:8443/", headers: viaHTTPS, status: fiber.StatusOK},
		{name: "Referer without default HTTPS port against host with default port", scheme: "https", host: "example.com:443", referer: "https://example.com", headers: viaHTTPS, status: fiber.StatusOK},
		{name: "Referer with default HTTPS port against host without port", scheme: "https", host: "example.com", referer: "https://example.com:443", headers: viaHTTPS, status: fiber.StatusOK},
		{name: "Correct Referer with ReverseProxy", scheme: "https", host: "10.0.1.42.com:8443", hostHeader: "10.0.1.42:8443", referer: "https://example.com", headers: viaProxy, status: fiber.StatusOK},
		{name: "Referer without default HTTP port against host with default port", scheme: "http", host: "example.com:80", referer: "http://example.com", headers: viaHTTP, status: fiber.StatusOK},
		{name: "Referer with default HTTP port against host without port", scheme: "http", host: "example.com", referer: "http://example.com:80", headers: viaHTTP, status: fiber.StatusOK},
		{
			name:    "Correct Referer with ReverseProxy Missing X-Forwarded-* Headers",
			scheme:  "https",
			host:    "10.0.1.42:8443",
			referer: "https://example.com",
			// We need to set this header to make sure c.Protocol() returns https
			headers: map[string]string{fiber.HeaderXForwardedProto: "https", fiber.HeaderXUrlScheme: "https"},
			status:  fiber.StatusForbidden,
		},
		{
			name:    "Correct Referer with path",
			referer: "https://example.com/action/items?gogogo=true",
			headers: map[string]string{fiber.HeaderXForwardedProto: "https", fiber.HeaderXForwardedHost: "example.com"},
			status:  fiber.StatusOK,
		},
		{
			name:    "Wrong Referer",
			referer: "https://csrf.example.com",
			headers: map[string]string{fiber.HeaderXForwardedProto: "https", fiber.HeaderXForwardedHost: "example.com"},
			status:  fiber.StatusForbidden,
		},
		// An HTTPS request whose Origin downgrades to HTTP for the same host is
		// rejected: the scheme is part of the origin and must match.
		{
			name: "HTTPS request with an HTTP Origin", scheme: "https", host: "example.com",
			origin: "http://example.com", headers: viaHTTPS, status: fiber.StatusForbidden,
		},
	})
}

func Test_CSRF_DeleteToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	config := ConfigDefault
	h := csrfTestHandler(app, &config)
	ctx := &fasthttp.RequestCtx{}

	// DeleteToken after token generation and remove the cookie
	ctx.Request.Header.Reset()
	ctx.Request.ResetBody()
	ctx.Response.Reset()
	ctx.Request.Header.Set(HeaderName, "")
	handler := HandlerFromContext(app.AcquireCtx(ctx))
	if handler != nil {
		ctx.Request.Header.DelAllCookies()
		require.ErrorIs(t, handler.DeleteToken(app.AcquireCtx(ctx)), ErrTokenNotFound)
	}
	h(ctx)

	ctx.Request.Header.SetMethod(fiber.MethodGet)
	token := csrfTestToken(t, h, ctx)

	// Delete the CSRF token
	ctx.Request.Header.Reset()
	ctx.Request.ResetBody()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	handler = HandlerFromContext(app.AcquireCtx(ctx))
	if handler != nil {
		if err := handler.DeleteToken(app.AcquireCtx(ctx)); err != nil {
			t.Fatal(err)
		}
	}
	h(ctx)

	csrfTestPost(t, h, ctx, token, 403)
}

func Test_CSRF_DeleteToken_WithSession(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	h, ctx, sessionID := csrfTestSession(t, app, &Config{})

	// Generate CSRF token. The Set-Cookie header joins every cookie of the answer
	// and the session cookie comes first, so, as in the original test, "token" is
	// the session ID and not the CSRF token (csrfTestToken reads that one by its
	// name), and no handler is found in the context below, so DeleteToken is never
	// called. The scenario is kept as it was.
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("_session", sessionID)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Delete the CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	handler := HandlerFromContext(app.AcquireCtx(ctx))
	if handler != nil {
		if err := handler.DeleteToken(app.AcquireCtx(ctx)); err != nil {
			t.Fatal(err)
		}
	}
	h(ctx)

	csrfTestSessionPost(t, h, ctx, sessionID, token, token, 403)
}

func Test_CSRF_ErrorHandler_Token(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr     error
		name        string
		headerToken string
		message     string
	}{
		{name: "invalid CSRF token", headerToken: "johndoe", wantErr: ErrTokenInvalid, message: "invalid CSRF token"},
		{name: "empty CSRF token", wantErr: ErrTokenNotFound, message: "empty CSRF token"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := csrfTestHandler(fiber.New(), &Config{
				ErrorHandler: func(ctx fiber.Ctx, err error) error {
					require.Equal(t, tc.wantErr, err)
					return ctx.Status(419).Send([]byte(tc.message))
				},
			})
			ctx := &fasthttp.RequestCtx{}

			// Generate CSRF token
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			h(ctx)

			csrfTestSessionPost(t, h, ctx, "", tc.headerToken, "", 419)
			require.Equal(t, tc.message, string(ctx.Response.Body()))
		})
	}
}

func Test_CSRF_ErrorHandler_MissingReferer(t *testing.T) {
	t.Parallel()
	h, token := csrfTestTrusted(t, &Config{
		CookieSecure: true,
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			require.Equal(t, ErrRefererNotFound, err)
			return ctx.Status(419).Send([]byte("empty CSRF token"))
		},
	}, "https")

	csrfTestPostCases(t, h, token, []csrfTestPostCase{{
		name:    "no Referer over HTTPS",
		headers: map[string]string{fiber.HeaderXForwardedProto: "https", fiber.HeaderXForwardedHost: "example.com"},
		status:  419,
	}})
}

func Test_CSRF_Cookie_Injection_Exploit(t *testing.T) {
	t.Parallel()
	h := csrfTestHandler(fiber.New(), &Config{})
	ctx := &fasthttp.RequestCtx{}

	// Inject CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf_=pwned;")
	ctx.Request.SetRequestURI("/")
	token := csrfTestToken(t, h, ctx)

	// Exploit CSRF token we just injected
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf_=pwned;")
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode(), "CSRF exploit successful")
}

// Test_CSRF_UnsafeHeaderValue ensures that unsafe header values, such as those described in https://github.com/gofiber/fiber/issues/2045, are rejected and the bug remains fixed.
func Test_CSRF_UnsafeHeaderValue(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp := apptest.Expect(t, app, "/", fiber.StatusOK)

	var token string
	for _, c := range resp.Cookies() {
		if c.Name != ConfigDefault.CookieName {
			continue
		}
		token = c.Value
		break
	}

	t.Log("token", token)

	getReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	getReq.Header.Set(HeaderName, token)
	apptest.Expect(t, app, getReq, fiber.StatusOK)

	getReq = httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
	getReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	getReq.Header.Set(fiber.HeaderCacheControl, "no")
	getReq.Header.Set(HeaderName, token)
	getReq.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: token})

	apptest.Expect(t, app, getReq, fiber.StatusOK)

	getReq.Header.Set(fiber.HeaderAccept, "*/*")
	getReq.Header.Del(HeaderName)
	apptest.Expect(t, app, getReq, fiber.StatusOK)

	postReq := httptest.NewRequest(fiber.MethodPost, "/", http.NoBody)
	postReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	postReq.Header.Set(HeaderName, token)
	postReq.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: token})
	apptest.Expect(t, app, postReq, fiber.StatusOK)
}

func Benchmark_Middleware_CSRF_Check(b *testing.B) {
	app := fiber.New()

	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Test Correct Referer POST
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)

	b.ReportAllocs()

	for b.Loop() {
		h(ctx)
	}

	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}

func Benchmark_Middleware_CSRF_GenerateToken(b *testing.B) {
	app := fiber.New()

	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	b.ReportAllocs()

	for b.Loop() {
		h(ctx)
	}

	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}

func Test_CSRF_InvalidURLHeaders(t *testing.T) {
	t.Parallel()
	h, token := csrfTestTrusted(t, &Config{
		ErrorHandler: func(ctx fiber.Ctx, err error) error { return ctx.Status(419).Send([]byte(err.Error())) },
	}, "http")

	csrfTestPostCases(t, h, token, []csrfTestPostCase{
		{name: "invalid Origin", scheme: "http", host: "example.com", origin: "http://[::1]:%38%30/Invalid Origin", status: 419, body: ErrOriginInvalid.Error()},
		{
			name:    "invalid Referer",
			scheme:  "https",
			host:    "example.com",
			referer: "http://[::1]:%38%30/Invalid Referer",
			headers: map[string]string{fiber.HeaderXForwardedProto: "https"},
			status:  419,
			body:    ErrRefererInvalid.Error(),
		},
	})
}

func Test_CSRF_TokenFromContext(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.NotEmpty(t, token)
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
}

func Test_CSRF_FromContextMethods(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{PassLocalsToContext: true})

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.NotEmpty(t, token)

		handler := HandlerFromContext(c)
		require.NotNil(t, handler)

		customCtx, ok := c.(fiber.CustomCtx)
		require.True(t, ok)
		require.Equal(t, token, TokenFromContext(customCtx))
		require.Equal(t, handler, HandlerFromContext(customCtx))
		require.Equal(t, token, TokenFromContext(c.RequestCtx()))
		require.Equal(t, token, TokenFromContext(c.Context()))
		require.Equal(t, handler, HandlerFromContext(c.RequestCtx()))
		require.Equal(t, handler, HandlerFromContext(c.Context()))

		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
}

func Test_CSRF_FromContextMethods_Invalid(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.Empty(t, token)

		handler := HandlerFromContext(c)
		require.Nil(t, handler)

		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
}

func Test_deleteTokenFromStorage(t *testing.T) {
	t.Parallel()
	ctx := apptest.Ctx(t, fiber.New())

	token := "token123"
	dummy := []byte("dummy")

	store := session.NewStore()
	sm := newSessionManager(store)
	stm := newStorageManager(nil, true)

	sm.setRaw(ctx, token, dummy, time.Minute)
	cfg := Config{Session: store}
	require.NoError(t, deleteTokenFromStorage(ctx, token, &cfg, sm, stm))
	raw := sm.getRaw(ctx, token, dummy)
	require.Nil(t, raw)

	sm2 := newSessionManager(nil)
	stm2 := newStorageManager(nil, true)

	require.NoError(t, stm2.setRaw(context.Background(), token, dummy, time.Minute))
	cfg = Config{}
	require.NoError(t, deleteTokenFromStorage(ctx, token, &cfg, sm2, stm2))
	raw, err := stm2.getRaw(context.Background(), token)
	require.NoError(t, err)
	require.Nil(t, raw)
}

func Test_storageManager_logKey(t *testing.T) {
	t.Parallel()
	redacted := newStorageManager(nil, true)
	require.Equal(t, redact.Placeholder, redacted.logKey("secret"))

	plain := newStorageManager(nil, false)
	require.Equal(t, "secret", plain.logKey("secret"))
}

func Test_CSRF_All_Extractors(t *testing.T) {
	t.Parallel()
	// bodyExtractor reads the token from a body of the form "_csrf=<token>".
	bodyExtractor := extractors.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			body := string(c.Body())
			selectors := strings.Split(body, "=")

			if len(selectors) != 2 || selectors[1] == "" {
				return "", extractors.ErrNotFound
			}
			return selectors[1], nil
		},
		Source: extractors.SourceCustom,
		Key:    "_csrf",
	}
	emptyExtractor := extractors.Extractor{
		Extract: func(_ fiber.Ctx) (string, error) { return "", nil },
		Source:  extractors.SourceCustom,
		Key:     "_csrf",
	}
	customHeader := extractors.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			token := c.Get("X-Custom-CSRF")
			if token == "" {
				return "", extractors.ErrNotFound
			}
			return token, nil
		},
		Source: extractors.SourceCustom,
		Key:    "X-Custom-CSRF",
	}
	header := extractors.FromHeader("X-Csrf-Token")
	form := extractors.FromForm("_csrf")
	query := extractors.FromQuery("csrf_token")
	queryCSRF := extractors.FromQuery("_csrf")
	// Chain: try the header first, fall back to the form
	chain := extractors.Chain(header, form)
	// A chain of a single extractor behaves like that extractor
	single := extractors.Chain(header)

	// postWithHeader sends the token in the named header and in the cookie.
	postWithHeader := func(header string) func(ctx *fasthttp.RequestCtx, token string) {
		return func(ctx *fasthttp.RequestCtx, token string) {
			ctx.Request.Header.SetMethod(fiber.MethodPost)
			ctx.Request.Header.Set(header, token)
			ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
		}
	}
	withHeader := postWithHeader("X-Csrf-Token")
	// postWithCookie sends the token in the cookie only.
	postWithCookie := func(ctx *fasthttp.RequestCtx, token string) {
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	}
	// postRoot sends the token in the cookie only, to "/".
	postRoot := func(ctx *fasthttp.RequestCtx, token string) {
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.SetRequestURI("/")
		ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	}
	// postQuery sends the token in the query parameter param and in the cookie.
	postQuery := func(param string) func(ctx *fasthttp.RequestCtx, token string) {
		return func(ctx *fasthttp.RequestCtx, token string) {
			ctx.Request.Header.SetMethod(fiber.MethodPost)
			ctx.Request.SetRequestURI("/?" + param + "=" + token)
			ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
		}
	}
	// postBody sends the token in a body of the given type and in the cookie.
	postBody := func(contentType string) func(ctx *fasthttp.RequestCtx, token string) {
		return func(ctx *fasthttp.RequestCtx, token string) {
			ctx.Request.Header.SetMethod(fiber.MethodPost)
			ctx.Request.Header.Set(fiber.HeaderContentType, contentType)
			ctx.Request.SetBodyString("_csrf=" + token)
			ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
		}
	}
	// postEmptyForm sends an empty form, with neither token nor cookie.
	postEmptyForm := func(ctx *fasthttp.RequestCtx, _ string) {
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	}

	testCases := []struct {
		setup        func(ctx *fasthttp.RequestCtx, token string)
		errorHandler fiber.ErrorHandler
		name         string
		body         string // the exact body of the answer, not checked when empty
		extractor    extractors.Extractor
		status       int
	}{
		{name: "FromHeader", extractor: header, setup: withHeader, status: 200},
		{name: "FromHeader_Missing", extractor: header, setup: postWithCookie, status: 403},
		{name: "FromForm", extractor: form, setup: postBody(fiber.MIMEApplicationForm), status: 200},
		{
			name:      "FromForm_Missing",
			extractor: form,
			setup: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			status: 403,
		},
		{name: "FromForm_NoCookieNoToken", extractor: form, setup: postEmptyForm, status: 403},
		{name: "FromQuery", extractor: query, setup: postQuery("csrf_token"), status: 200},
		{name: "FromQuery_Missing", extractor: query, setup: postRoot, status: 403},
		{
			name:      "FromQuery_InvalidToken",
			extractor: queryCSRF,
			setup: func(ctx *fasthttp.RequestCtx, _ string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/?_csrf=" + utils.UUIDv4())
			},
			status: 403,
		},
		{name: "FromQuery_ValidToken", extractor: queryCSRF, setup: postQuery("_csrf"), status: 200, body: "OK"},
		{
			name:      "FromCustom_NoToken",
			extractor: bodyExtractor,
			setup: func(ctx *fasthttp.RequestCtx, _ string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
			},
			status: 403,
		},
		{name: "FromCustom", extractor: bodyExtractor, setup: postBody(fiber.MIMETextPlain), status: 200},
		{
			name:         "FromCustom_EmptyString",
			extractor:    emptyExtractor,
			errorHandler: func(c fiber.Ctx, err error) error { return c.Status(403).SendString(err.Error()) },
			setup:        postBody(fiber.MIMETextPlain),
			status:       403,
			body:         ErrTokenNotFound.Error(),
		},
		{name: "FromCustomHeader", extractor: customHeader, setup: postWithHeader("X-Custom-CSRF"), status: 200},
		{name: "FromCustomHeader_Missing", extractor: customHeader, setup: postWithCookie, status: 403},
		// No param is provided: "/" is requested instead of "/:csrf", and the CSRF
		// middleware on the catch-all route answers 403.
		{name: "FromParam_Missing", extractor: extractors.FromParam("csrf"), setup: postRoot, status: 403},
		// Chain: the header is tried first, the form is the fallback
		{name: "Chain_TokenInHeader", extractor: chain, setup: withHeader, status: 200},
		{name: "Chain_TokenInForm", extractor: chain, setup: postBody(fiber.MIMEApplicationForm), status: 200},
		{
			name:      "Chain_HeaderTakesPrecedenceOverForm",
			extractor: chain,
			setup: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
				ctx.Request.Header.Set("X-Csrf-Token", token)
				ctx.Request.SetBodyString("_csrf=wrong_token")
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			status: 200,
		},
		{name: "Chain_NoToken", extractor: chain, setup: postEmptyForm, status: 403},
		{
			name:      "Chain_WrongTokenInBothLocations",
			extractor: chain,
			setup: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
				ctx.Request.Header.Set("X-Csrf-Token", "wrong_token")
				ctx.Request.SetBodyString("_csrf=also_wrong")
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			status: 403,
		},
		// An empty chain never finds a token
		{name: "Chain_Empty", extractor: extractors.Chain(), setup: withHeader, status: 403},
		{name: "Chain_SingleExtractor_TokenInHeader", extractor: single, setup: withHeader, status: 200},
		{name: "Chain_SingleExtractor_NoToken", extractor: single, setup: postWithCookie, status: 403},
	}

	for i := range testCases {
		tc := &testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := csrfTestHandler(fiber.New(), &Config{Extractor: tc.extractor, ErrorHandler: tc.errorHandler})
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.SetRequestURI("/")
			token := csrfTestToken(t, h, ctx)

			ctx.Request.Reset()
			ctx.Response.Reset()
			tc.setup(ctx, token)
			h(ctx)
			require.Equal(t, tc.status, ctx.Response.StatusCode(),
				"Test case %s failed: expected %d, got %d", tc.name, tc.status, ctx.Response.StatusCode())
			if tc.body != "" {
				require.Equal(t, tc.body, string(ctx.Response.Body()))
			}
		})
	}
}

func Test_CSRF_Param_Extractor(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		setupRequest func(ctx *fasthttp.RequestCtx, token string)
		name         string
		expectBody   string
		expectStatus int
	}{
		{
			name: "FromParam_Valid",
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/" + token)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 200,
			expectBody:   "OK",
		},
		{
			name: "FromParam_Invalid",
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/wrong_token")
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 403,
		},
		{
			name: "FromParam_NoCookie",
			setupRequest: func(ctx *fasthttp.RequestCtx, _ string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/" + utils.UUIDv4())
			},
			expectStatus: 403,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()

			// Only use param-based routing for param extractor tests
			csrfGroup := app.Group("/:csrf", New(Config{Extractor: extractors.FromParam("csrf")}))
			csrfGroup.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

			h := app.Handler()
			ctx := &fasthttp.RequestCtx{}

			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.SetRequestURI("/" + utils.UUIDv4())
			token := csrfTestToken(t, h, ctx)

			ctx.Request.Reset()
			ctx.Response.Reset()
			tc.setupRequest(ctx, token)
			h(ctx)
			require.Equal(t, tc.expectStatus, ctx.Response.StatusCode(),
				"Test case %s failed: expected %d, got %d", tc.name, tc.expectStatus, ctx.Response.StatusCode())
			if tc.expectBody != "" {
				require.Equal(t, tc.expectBody, string(ctx.Response.Body()))
			}
		})
	}
}

func Test_CSRF_DisableValueRedaction_TrustedOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		origin string
	}{
		{name: "origin", origin: "http://"},
		// The same path for the wildcard subdomain branch.
		{name: "wildcard", origin: "http://*."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, "[CSRF] Invalid origin format in configuration:"+tc.origin, func() {
				New(Config{TrustedOrigins: []string{tc.origin}, DisableValueRedaction: true})
			})
		})
	}
}

func Test_CSRF_Extractor_NonNotFoundError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("extractor boom")

	var captured error
	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.Extractor{
			Extract: func(fiber.Ctx) (string, error) { return "", sentinel },
			Source:  extractors.SourceCustom,
			Key:     "_csrf",
		},
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return fiber.ErrTeapot
		},
	}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	apptest.Expect(t, app, apptest.Req(fiber.MethodPost, "/"), fiber.StatusTeapot)
	require.ErrorIs(t, captured, sentinel)
}

// A shared Storage holds more than tokens, such as a limiter's counters. The
// client names the token, so it must not reach those.
func Test_CSRF_TokenCannotNameForeignStorageEntry(t *testing.T) {
	t.Parallel()
	const foreignKey = "198.51.100.9"
	foreignValue := []byte("entry of another middleware")

	for _, singleUse := range []bool{false, true} {
		t.Run(fmt.Sprintf("SingleUseToken=%t", singleUse), func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			storage.Data[foreignKey] = foreignValue

			app := fiber.New()
			app.Use(New(Config{Storage: storage, SingleUseToken: singleUse}))
			app.All("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
			app.Get("/logout", func(c fiber.Ctx) error { return HandlerFromContext(c).DeleteToken(c) })

			for _, target := range []string{"/", "/logout"} {
				req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
				req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: foreignKey})
				resp := apptest.Expect(t, app, req, fiber.StatusOK)
				require.NotContains(t, resp.Header.Get(fiber.HeaderSetCookie), foreignKey)
				require.Equal(t, foreignValue, storage.Data[foreignKey])
			}

			req := httptest.NewRequest(fiber.MethodPost, "/", http.NoBody)
			req.Header.Set(HeaderName, foreignKey)
			req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: foreignKey})
			apptest.Expect(t, app, req, fiber.StatusForbidden)
			require.Equal(t, foreignValue, storage.Data[foreignKey])
		})
	}
}

// keyKeepingStorage records every key it is given as is, beside a copy made at
// the same moment.
type keyKeepingStorage struct {
	*storagetest.Memory
	kept   []string
	copied []string
}

func (s *keyKeepingStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.kept = append(s.kept, key)
	s.copied = append(s.copied, strings.Clone(key))
	return s.Memory.SetWithContext(ctx, key, val, exp)
}

// The key handed to the Storage must not change when the connection's next
// request reuses the request buffers.
func Test_CSRF_Storage_KeyOutlivesRequest(t *testing.T) {
	t.Parallel()
	storage := &keyKeepingStorage{Memory: storagetest.New()}
	app := fiber.New()
	app.Use(New(Config{Storage: storage}))
	app.All("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	h := app.Handler()

	// One RequestCtx for every request, as the server keeps one per connection.
	fctx := &fasthttp.RequestCtx{}
	send := func(method, token string) {
		fctx.Request.Reset()
		fctx.Response.Reset()
		fctx.Request.Header.SetMethod(method)
		fctx.Request.SetRequestURI("/")
		if token != "" {
			fctx.Request.Header.Set(fiber.HeaderCookie, ConfigDefault.CookieName+"="+token)
			fctx.Request.Header.Set(HeaderName, token)
		}
		h(fctx)
		require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	}

	send(fiber.MethodGet, "")
	require.Len(t, storage.copied, 1)
	token, ok := strings.CutPrefix(storage.copied[0], storagePrefix)
	require.True(t, ok)

	send(fiber.MethodGet, token)
	send(fiber.MethodPost, token)
	send(fiber.MethodGet, strings.Repeat("A", len(token)))

	require.Len(t, storage.kept, 4)
	require.Equal(t, storage.copied, storage.kept)
}

// Test_CSRF_DeleteToken_Errors covers the errors DeleteToken returns, which reach
// the ErrorHandler too: the early return when no CSRF cookie is present on the
// request, and the storage delete failure, where the cookie is present but
// removing the token from storage returns an error.
func Test_CSRF_DeleteToken_Errors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantIs  error // what both errors must match, unless nil
		name    string
		wantMsg string // what both errors must contain, unless empty
		failing bool   // the request carries a cookie for a token that storage fails to delete
	}{
		{name: "no cookie", wantIs: ErrTokenNotFound},
		{name: "storage error", failing: true, wantMsg: "csrf: failed to delete token from storage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var captured error
			cfg := Config{ErrorHandler: func(_ fiber.Ctx, err error) error {
				captured = err
				return err
			}}
			req := apptest.Req(fiber.MethodGet, "/")
			if tc.failing {
				storage := storagetest.New()
				storage.Data[storagePrefix+"token"] = []byte("value")
				storage.SetErr(storagetest.OpDelete, storagePrefix+"token", errors.New("boom"))
				cfg.Storage = storage
				req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: "token"})
			}
			app := fiber.New()
			app.Use(New(cfg))

			var deleteErr error
			app.Get("/", func(c fiber.Ctx) error {
				handler := HandlerFromContext(c)
				require.NotNil(t, handler)
				deleteErr = handler.DeleteToken(c)
				return c.SendStatus(fiber.StatusOK)
			})

			apptest.Expect(t, app, req, fiber.StatusOK)
			if tc.wantIs != nil {
				require.ErrorIs(t, deleteErr, tc.wantIs)
				require.ErrorIs(t, captured, tc.wantIs)
				return
			}
			require.ErrorContains(t, deleteErr, tc.wantMsg)
			require.ErrorContains(t, captured, tc.wantMsg)
		})
	}
}

// Test_CSRF_DeleteToken_WithSessionMiddleware exercises DeleteToken while the
// session is loaded into the context by the session middleware, covering the
// in-context branch of the session manager's delRaw.
func Test_CSRF_DeleteToken_WithSessionMiddleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	smh, sstore := session.NewWithStore()
	app.Use(smh)
	app.Use(New(Config{Session: sstore}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Post("/delete", func(c fiber.Ctx) error {
		handler := HandlerFromContext(c)
		require.NotNil(t, handler)
		if err := handler.DeleteToken(c); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate the CSRF token and the session id.
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)

	csrfToken := csrfTestCookie(t, ctx, ConfigDefault.CookieName)
	sessionID := csrfTestCookie(t, ctx, "session_id")

	// Delete the token with the session loaded in the context.
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/delete")
	ctx.Request.Header.Set(HeaderName, csrfToken)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, csrfToken)
	ctx.Request.Header.SetCookie("session_id", sessionID)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
}

func Test_StorageManager_Memory_UnexpectedType(t *testing.T) {
	t.Parallel()
	m := newStorageManager(nil, false)
	m.memory.Set("key", "not-bytes", 0)

	raw, err := m.getRaw(context.Background(), "key")
	require.Nil(t, raw)
	require.ErrorContains(t, err, "unexpected value type")
}

func Test_StorageManager_Storage_Success(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	m := newStorageManager(storage, false)

	require.NoError(t, m.setRaw(context.Background(), "key", []byte("value"), time.Minute))

	raw, err := m.getRaw(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, []byte("value"), raw)

	require.NoError(t, m.delRaw(context.Background(), "key"))

	raw, err = m.getRaw(context.Background(), "key")
	require.NoError(t, err)
	require.Nil(t, raw)
}

// newFlakySessionManager returns a session manager on a store whose storage
// fails the operations ops names: storagetest.OpGet to load sessions,
// storagetest.OpSet to save them.
func newFlakySessionManager(ops ...string) *sessionManager {
	storage := storagetest.New()
	for _, op := range ops {
		storage.SetErr(op, "*", errors.New(op+" failed"))
	}
	return newSessionManager(session.NewStore(session.Config{
		Storage:   storage,
		Extractor: extractors.FromCookie("session_id"),
	}))
}

// newSessionCtx builds a fiber.Ctx carrying the given session_id cookie. The
// session middleware is intentionally not run, so the session manager falls back
// to loading the session from the store (the else branch).
func newSessionCtx(t *testing.T, sessionID string) fiber.Ctx {
	t.Helper()

	reqCtx := &fasthttp.RequestCtx{}
	if sessionID != "" {
		reqCtx.Request.Header.SetCookie("session_id", sessionID)
	}
	return apptest.CtxFor(t, fiber.New(), reqCtx)
}

// Test_SessionManager_StoreError covers the error branch of each operation when
// loading the session from the store fails.
func Test_SessionManager_StoreError(t *testing.T) {
	t.Parallel()
	t.Run("GetRaw", func(t *testing.T) {
		t.Parallel()
		m, c := newFlakySessionManager(storagetest.OpGet), newSessionCtx(t, "abc")
		require.Nil(t, m.getRaw(c, "key", dummyValue))
	})

	t.Run("SetRaw", func(t *testing.T) {
		t.Parallel()
		m, c := newFlakySessionManager(storagetest.OpGet), newSessionCtx(t, "abc")
		require.NotPanics(t, func() { m.setRaw(c, "key", dummyValue, time.Minute) })
	})

	t.Run("DelRaw", func(t *testing.T) {
		t.Parallel()
		m, c := newFlakySessionManager(storagetest.OpGet), newSessionCtx(t, "abc")
		require.NotPanics(t, func() { m.delRaw(c) })
	})
}

// Test_SessionManager_SaveError covers the save-failure warning branch of setRaw
// and delRaw, where the store loads successfully but persisting it fails. It and
// its subtests run serially because they replace the output of the package-global
// logger.
func Test_SessionManager_SaveError(t *testing.T) {
	for _, tc := range []struct {
		op   func(m *sessionManager, c fiber.Ctx)
		name string
	}{
		{name: "SetRaw", op: func(m *sessionManager, c fiber.Ctx) { m.setRaw(c, "key", dummyValue, time.Minute) }},
		{name: "DelRaw", op: func(m *sessionManager, c fiber.Ctx) { m.delRaw(c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			fiberlog.SetOutput(&buf)
			t.Cleanup(func() { fiberlog.SetOutput(os.Stderr) })

			tc.op(newFlakySessionManager(storagetest.OpSet), newSessionCtx(t, ""))
			require.Contains(t, buf.String(), "failed to save session")
		})
	}
}

// Test_CSRF_Security_SecFetchSite_Normalization ensures the Sec-Fetch-Site
// validation is case-insensitive and tolerant of surrounding whitespace, while
// still rejecting genuinely unknown values.
func Test_CSRF_Security_SecFetchSite_Normalization(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "empty header is ignored", value: "", wantErr: nil},
		{name: "same-origin", value: "same-origin", wantErr: nil},
		{name: "mixed case same-origin", value: "Same-Origin", wantErr: nil},
		{name: "upper case cross-site", value: "CROSS-SITE", wantErr: nil},
		{name: "leading/trailing spaces", value: "  same-site  ", wantErr: nil},
		{name: "none", value: "none", wantErr: nil},
		{name: "unknown value rejected", value: "totally-bogus", wantErr: ErrFetchSiteInvalid},
		{name: "embedded space rejected", value: "same origin", wantErr: ErrFetchSiteInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			if tc.value != "" {
				c.Request().Header.Set(fiber.HeaderSecFetchSite, tc.value)
			}
			require.Equal(t, tc.wantErr, validateSecFetchSite(c))
		})
	}
}

// Test_CSRF_Security_DoubleSubmitMismatch ensures that a request presenting a
// valid token in the configured extractor location but a different (also valid)
// token in the cookie is rejected with ErrTokenInvalid. This is the core
// double-submit-cookie protection.
func Test_CSRF_Security_DoubleSubmitMismatch(t *testing.T) {
	t.Parallel()
	var captured error
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.SendStatus(fiber.StatusForbidden)
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	// Generate two distinct, individually valid tokens.
	genToken := func() string {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		return csrfTestToken(t, h, ctx)
	}
	tokenA := genToken()
	tokenB := genToken()
	require.NotEqual(t, tokenA, tokenB)

	// Submit tokenA in the header but tokenB in the cookie.
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, tokenA)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, tokenB)
	h(ctx)

	require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
	require.ErrorIs(t, captured, ErrTokenInvalid)
}

// Test_CSRF_Security_ForgedTokenNotInStorage ensures that a token which is
// consistent across the header and cookie (passing the double-submit check) but
// absent from storage is rejected and the stale cookie is expired.
func Test_CSRF_Security_ForgedTokenNotInStorage(t *testing.T) {
	t.Parallel()
	var captured error
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.SendStatus(fiber.StatusForbidden)
		},
	}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	const forged = "this-token-was-never-issued"
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, forged)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, forged)
	h(ctx)

	require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
	require.ErrorIs(t, captured, ErrTokenNotFound)

	// The stale cookie must be expired by the middleware.
	expired := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(expired)
	expired.SetKey(ConfigDefault.CookieName)
	require.True(t, ctx.Response.Header.Cookie(expired), "expected the cookie to be reset")
	require.Empty(t, string(expired.Value()), "expected the cookie value to be cleared")
}

// Test_CSRF_Security_CookieAttributes verifies that the security-relevant cookie
// attributes from the configuration are reflected on the Set-Cookie response
// header. Missing HttpOnly/Secure/SameSite flags would weaken the protection.
func Test_CSRF_Security_CookieAttributes(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		CookieName:     "__Host-csrf",
		CookiePath:     "/",
		CookieSecure:   true,
		CookieHTTPOnly: true,
		CookieSameSite: "Strict",
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp := apptest.Do(t, app, "/")

	var cookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "__Host-csrf" {
			cookie = ck
			break
		}
	}
	require.NotNil(t, cookie, "CSRF cookie should be set")
	require.True(t, cookie.HttpOnly, "cookie must be HttpOnly")
	require.True(t, cookie.Secure, "cookie must be Secure")
	require.Equal(t, http.SameSiteStrictMode, cookie.SameSite, "cookie must be SameSite=Strict")
	require.Equal(t, "/", cookie.Path)
	// A "__Host-" prefixed cookie must stay host-only: setCSRFCookie copies
	// cfg.CookieDomain onto the response cookie, so an empty Domain guards
	// against a regression that would start scoping the cookie to a domain.
	require.Empty(t, cookie.Domain, "__Host- cookie must not set a Domain")
	require.NotEmpty(t, cookie.Value)
}

// Test_CSRF_Security_TrustedOriginSchemeIsolation ensures that trusting an
// origin under one scheme does not implicitly trust it under another scheme.
func Test_CSRF_Security_TrustedOriginSchemeIsolation(t *testing.T) {
	t.Parallel()
	h, token := csrfTestTrusted(t, &Config{
		CookieSecure:   true,
		TrustedOrigins: []string{"https://trusted.example.com"},
	}, "https")

	viaHTTPS := map[string]string{fiber.HeaderXForwardedProto: "https"}

	csrfTestPostCases(t, h, token, []csrfTestPostCase{
		{name: "the exact trusted origin (https) is accepted", scheme: "https", host: "example.com", origin: "https://trusted.example.com", headers: viaHTTPS, status: fiber.StatusOK},
		{name: "the same host over http is NOT trusted", scheme: "https", host: "example.com", origin: "http://trusted.example.com", headers: viaHTTPS, status: fiber.StatusForbidden},
	})
}

// Test_CSRF_OriginCheckIgnoresHeaderNameCase pins that the origin check tells a
// cross-site POST from a same-site one whatever case the field names arrive in.
//
// Ctx.Get compares the stored key byte for byte, so under
// DisableHeaderNormalizing every read came back empty — and an absent Origin is
// not a failure here: on a plaintext connection the check is skipped for it. A
// cross-site request that spelled the header in lower case, which is what
// HTTP/2 and HTTP/3 put on the wire, was therefore accepted where the canonical
// spelling was refused.
func Test_CSRF_OriginCheckIgnoresHeaderNameCase(t *testing.T) {
	t.Parallel()
	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()
			h := csrfTestAccept(fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize}), &Config{})

			// A legitimate visit, for the cookie and token a double-submit
			// check needs. Without them the token check refuses everything and
			// the origin check is never reached.
			token, cookie := csrfTestVisit(t, h)

			post := func(originHeader, origin string) int {
				return csrfTestPostWith(h, !normalize, cookie, token, func(req *fasthttp.Request) {
					req.Header.Set(originHeader, origin)
				})
			}

			// Same-origin passes under either spelling, so the assertions below
			// cannot pass by refusing everything.
			require.Equal(t, fiber.StatusOK, post(fiber.HeaderOrigin, "http://example.com"))
			require.Equal(t, fiber.StatusOK, post("origin", "http://example.com"))

			require.Equal(t, fiber.StatusForbidden, post(fiber.HeaderOrigin, "http://evil.com"))
			require.Equal(t, fiber.StatusForbidden, post("origin", "http://evil.com"),
				"a cross-site origin must be refused whatever case its name arrives in")
		})
	}
}

// Test_CSRF_RepeatedOriginIsRefused pins what a message carrying two Origin
// lines gets.
//
// Ctx.Get answers with the first line stored under the exact key, empty or not,
// and an empty answer is indistinguishable from an absent header — which the
// origin check treats as no failure at all, skipping it on a plaintext request.
// A message carrying "Origin:" ahead of its real Origin therefore turned the
// check off, under both spellings and whether or not the names are normalized.
//
// Origin is a single-value field, so a second line is malformed however it is
// spelled and whatever it holds. It is refused rather than resolved: reading
// past the empty line would let whoever wrote the other one pick which is read.
// That is why the same-origin case below is refused too — the check cannot know
// which line the request meant.
func Test_CSRF_RepeatedOriginIsRefused(t *testing.T) {
	t.Parallel()
	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()
			h := csrfTestAccept(fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize}), &Config{})
			token, cookie := csrfTestVisit(t, h)

			post := func(build func(*fasthttp.Request)) int {
				return csrfTestPostWith(h, !normalize, cookie, token, build)
			}

			single := func(name, origin string) int {
				return post(func(req *fasthttp.Request) { req.Header.Set(name, origin) })
			}

			// shadowed sends an empty Origin line ahead of a second one carrying
			// origin, which is what a header-writing intermediary in front of the
			// application can produce. Set then Add stores the two lines fasthttp
			// parses such a message into.
			shadowed := func(secondName, origin string) int {
				return post(func(req *fasthttp.Request) {
					req.Header.Set(fiber.HeaderOrigin, "")
					req.Header.Add(secondName, origin)
				})
			}

			// A single Origin line still passes, so the assertions below cannot
			// pass by refusing everything.
			require.Equal(t, fiber.StatusOK, single(fiber.HeaderOrigin, "http://example.com"))
			require.Equal(t, fiber.StatusOK, single("origin", "http://example.com"))

			require.Equal(t, fiber.StatusForbidden, shadowed(fiber.HeaderOrigin, "http://evil.com"),
				"an empty first line must not hide the origin beside it")
			require.Equal(t, fiber.StatusForbidden, shadowed("origin", "http://evil.com"),
				"an empty first line must not hide the origin beside it")

			require.Equal(t, fiber.StatusForbidden, shadowed(fiber.HeaderOrigin, "http://example.com"),
				"a repeated Origin is refused rather than resolved, same-origin included")
			require.Equal(t, fiber.StatusForbidden, shadowed("origin", "http://example.com"))
		})
	}
}

// Test_CSRF_RepeatedSecFetchSiteAndRefererAreRefused covers the other two
// single-value fields the check reads, for the same reason as Origin: a second
// line makes the field unreadable, and neither may be treated as absent — an
// absent Sec-Fetch-Site is skipped, and an absent Referer on HTTPS is what the
// Referer check exists to refuse.
func Test_CSRF_RepeatedSecFetchSiteAndRefererAreRefused(t *testing.T) {
	t.Parallel()
	// Trusted-proxy app: Ctx.Scheme reads X-Forwarded-Proto only from a trusted
	// peer, and the Referer check runs only on HTTPS.
	h := csrfTestAccept(newTrustedApp(), &Config{})
	token, cookie := csrfTestVisit(t, h)

	post := func(build func(*fasthttp.RequestCtx)) int {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.SetRequestURI("/")
		ctx.Request.URI().SetScheme("https")
		ctx.Request.URI().SetHost("example.com")
		ctx.Request.Header.SetProtocol("https")
		ctx.Request.Header.SetHost("example.com")
		// Ctx.Scheme reads this rather than the URI, and the Referer check runs
		// only on HTTPS.
		ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
		ctx.Request.Header.Set(fiber.HeaderCookie, cookie)
		ctx.Request.Header.Set(HeaderName, token)
		build(ctx)
		h(ctx)
		return ctx.Response.StatusCode()
	}

	// A same-origin request with one of each still passes, so the refusals
	// below cannot pass by refusing everything.
	require.Equal(t, fiber.StatusOK, post(func(ctx *fasthttp.RequestCtx) {
		ctx.Request.Header.Set(fiber.HeaderSecFetchSite, "same-origin")
		ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com")
	}))
	require.Equal(t, fiber.StatusOK, post(func(ctx *fasthttp.RequestCtx) {
		ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com/page")
	}), "no Origin on HTTPS falls through to the Referer check, which this passes")

	require.Equal(t, fiber.StatusForbidden, post(func(ctx *fasthttp.RequestCtx) {
		ctx.Request.Header.Set(fiber.HeaderSecFetchSite, "same-origin")
		ctx.Request.Header.Add(fiber.HeaderSecFetchSite, "same-origin")
		ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com")
	}), "a repeated Sec-Fetch-Site must not be skipped as an absent one is")

	require.Equal(t, fiber.StatusForbidden, post(func(ctx *fasthttp.RequestCtx) {
		// No Origin, so the check falls through to Referer on this HTTPS request.
		ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com/page")
		ctx.Request.Header.Add(fiber.HeaderReferer, "https://evil.example/page")
	}), "a repeated Referer is refused rather than resolved to one of the lines")
}

// Test_CSRF_TrustedOriginIsAllowedWhenCrossSite pins that Sec-Fetch-Site does
// not decide cross-site on its own.
//
// "cross-site" is exactly what a browser sends for a legitimate request to an
// origin the application trusts, so rejecting it in the header check would make
// TrustedOrigins unusable; the decision belongs to the origin check, which
// knows which origins those are.
func Test_CSRF_TrustedOriginIsAllowedWhenCrossSite(t *testing.T) {
	t.Parallel()
	h := csrfTestAccept(fiber.New(), &Config{TrustedOrigins: []string{"http://partner.com"}})
	token, cookie := csrfTestVisit(t, h)

	post := func(origin, secFetchSite string) int {
		return csrfTestPostWith(h, false, cookie, token, func(req *fasthttp.Request) {
			req.Header.Set(fiber.HeaderOrigin, origin)
			if secFetchSite != "" {
				req.Header.Set(fiber.HeaderSecFetchSite, secFetchSite)
			}
		})
	}

	require.Equal(t, fiber.StatusOK, post("http://partner.com", "cross-site"),
		"a trusted origin is allowed even though the browser calls it cross-site")
	require.Equal(t, fiber.StatusOK, post("http://example.com", "same-origin"))

	// The origin check is still what refuses an untrusted one.
	require.Equal(t, fiber.StatusForbidden, post("http://evil.com", "cross-site"))
	// And a value no browser produces is refused by the header check itself.
	require.Equal(t, fiber.StatusForbidden, post("http://partner.com", "not-a-real-value"))
}

// Test_CSRF_CrossSiteWithoutOriginOverPlaintext pins what happens when the
// browser says the request is cross-site but sends no Origin, on a plaintext
// connection.
//
// Sec-Fetch-Site does not decide cross-site on its own, and an absent Origin is
// not a failure over plaintext — the check is skipped for it, because there is
// no Referer fallback to fall back to. What is left holding the request is the
// double-submit token, so that is what this pins: with a valid pair it is
// accepted, and without one it is not.
func Test_CSRF_CrossSiteWithoutOriginOverPlaintext(t *testing.T) {
	t.Parallel()
	h := csrfTestAccept(fiber.New(), &Config{})
	token, cookie := csrfTestVisit(t, h)

	// No Origin, and no Referer either.
	crossSite := func(req *fasthttp.Request) { req.Header.Set(fiber.HeaderSecFetchSite, "cross-site") }

	require.Equal(t, fiber.StatusOK, csrfTestPostWith(h, false, cookie, token, crossSite),
		"over plaintext with no Origin, the token is what the request rests on")
	require.Equal(t, fiber.StatusForbidden, csrfTestPostWith(h, false, cookie, "", crossSite),
		"and without it nothing else lets the request through")
}

// varyMutatingCtx is a custom context whose Vary rewrites the field list it is
// handed. Nothing in Ctx's contract forbids that, so the middleware must not
// hand it a slice that outlives the request.
type varyMutatingCtx struct {
	fiber.DefaultCtx
}

// Vary overwrites every field it is handed before forwarding the call — the
// most a custom Vary can do to a slice its caller still holds.
func (c *varyMutatingCtx) Vary(fields ...string) {
	for i := range fields {
		fields[i] = "X-Mutated"
	}
	c.DefaultCtx.Vary(fields...)
}

// Test_CSRF_VaryDoesNotShareFieldsWithCustomCtx pins that varyCookie survives a
// custom context. It is shared by every response that mints a token, so a Vary
// implementation writing to its argument would otherwise corrupt the field name
// for every later response and race with the requests running alongside it.
func Test_CSRF_VaryDoesNotShareFieldsWithCustomCtx(t *testing.T) {
	t.Parallel()
	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &varyMutatingCtx{DefaultCtx: *fiber.NewDefaultCtx(app)}
	})
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	app.Handler()(ctx)

	require.Contains(t, string(ctx.Response.Header.Peek(fiber.HeaderVary)), "X-Mutated",
		"the custom Vary must still be the one that ran")
	require.Equal(t, []string{fiber.HeaderCookie}, varyCookie)
}
