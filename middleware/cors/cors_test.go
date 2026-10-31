package cors

import (
	"bufio"
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/log"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_CORS_DefaultOrEmptyConfig(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string][]Config{"defaults": nil, "empty config": {{}}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(cfg...))
			h := app.Handler()

			corsTestExpectHeaders(t, corsTestRequest(h, fiber.MethodGet, "http://localhost"),
				fiber.HeaderAccessControlAllowOrigin, "*",
				fiber.HeaderAccessControlAllowCredentials, "",
				fiber.HeaderAccessControlExposeHeaders, "")
			corsTestExpectHeaders(t, corsTestPreflight(h, "http://localhost"),
				fiber.HeaderAccessControlAllowMethods, "GET, POST, HEAD, PUT, DELETE, PATCH, QUERY",
				fiber.HeaderAccessControlAllowHeaders, "",
				fiber.HeaderAccessControlMaxAge, "")
		})
	}
}

// corsTestRequest sends method from origin, followed by the header pairs kv, through h and returns the
// context that holds the answer. An empty origin sends no Origin header.
func corsTestRequest(h fasthttp.RequestHandler, method, origin string, kv ...string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(method)
	if origin != "" {
		ctx.Request.Header.Set(fiber.HeaderOrigin, origin)
	}
	for name, value := range apptest.Pairs(kv) {
		ctx.Request.Header.Set(name, value)
	}
	h(ctx)
	return ctx
}

// corsTestPreflight sends a preflight request from origin through h and returns
// the context that holds the answer.
func corsTestPreflight(h fasthttp.RequestHandler, origin string) *fasthttp.RequestCtx {
	return corsTestRequest(h, fiber.MethodOptions, origin, fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
}

// corsTestHeader returns the response header name of the answer in ctx.
func corsTestHeader(ctx *fasthttp.RequestCtx, name string) string {
	return string(ctx.Response.Header.Peek(name))
}

// corsTestExpectHeaders fails the test unless each response header named in kv has the value that follows its name,
// where "" means the header is absent.
func corsTestExpectHeaders(t *testing.T, ctx *fasthttp.RequestCtx, kv ...string) {
	t.Helper()

	for name, want := range apptest.Pairs(kv) {
		require.Equal(t, want, corsTestHeader(ctx, name), name)
	}
}

// corsTestAllowOrigin returns the Access-Control-Allow-Origin of the answer in ctx.
func corsTestAllowOrigin(ctx *fasthttp.RequestCtx) string {
	return corsTestHeader(ctx, fiber.HeaderAccessControlAllowOrigin)
}

// corsTestRaw reads the raw HTTP request raw, which keeps its header names as
// written when app does not normalize them, and sends it through app.
func corsTestRaw(t *testing.T, app *fiber.App, raw string) *fasthttp.RequestCtx {
	t.Helper()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	if app.Config().DisableHeaderNormalizing {
		req.Header.DisableNormalizing()
	}
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	fctx := &fasthttp.RequestCtx{}
	fctx.Init(req, nil, nil)
	app.Handler()(fctx)
	return fctx
}

func Test_CORS_WildcardHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{AllowMethods: []string{"*"}, AllowHeaders: []string{"*"}, ExposeHeaders: []string{"*"}}))

	corsTestExpectHeaders(t, corsTestPreflight(app.Handler(), "http://localhost"),
		fiber.HeaderAccessControlAllowOrigin, "*",
		fiber.HeaderAccessControlAllowCredentials, "",
		fiber.HeaderAccessControlAllowMethods, "*",
		fiber.HeaderAccessControlAllowHeaders, "*",
		fiber.HeaderAccessControlExposeHeaders, "*")
}

func Test_CORS_Negative_MaxAge(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{MaxAge: -1}))

	corsTestExpectHeaders(t, corsTestPreflight(app.Handler(), "http://localhost"), fiber.HeaderAccessControlMaxAge, "0")
}

func Test_CORS_MaxAge_NotSetOnSimpleRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{MaxAge: 100}))

	corsTestExpectHeaders(t, corsTestRequest(app.Handler(), fiber.MethodGet, "http://localhost"),
		fiber.HeaderAccessControlMaxAge, "")
}

func Test_CORS_AllowOrigins_Vary(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{AllowOrigins: []string{"http://localhost"}}))

	h := app.Handler()

	// Test Vary header non-Cors request
	ctx := corsTestRequest(h, fiber.MethodGet, "")
	require.Contains(t, corsTestHeader(ctx, fiber.HeaderVary), fiber.HeaderOrigin, "Vary header should be set")

	// Test Vary header Cors request
	ctx = corsTestPreflight(h, "http://localhost")
	require.Contains(t, corsTestHeader(ctx, fiber.HeaderVary), fiber.HeaderOrigin, "Vary header should be set")
}

func Test_CORS_Wildcard(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	// OPTIONS (preflight) response headers when AllowOrigins is *
	app.Use(New(Config{MaxAge: 3600, ExposeHeaders: []string{"X-Request-ID"}, AllowHeaders: []string{"Authentication"}}))
	handler := app.Handler()

	ctx := corsTestPreflight(handler, "http://localhost")
	// "*" validates that the request is not reflecting the origin in the response
	corsTestExpectHeaders(t, ctx,
		fiber.HeaderAccessControlAllowOrigin, "*",
		fiber.HeaderAccessControlAllowCredentials, "",
		fiber.HeaderAccessControlMaxAge, "3600",
		fiber.HeaderAccessControlAllowHeaders, "Authentication")
	require.Contains(t, corsTestHeader(ctx, fiber.HeaderVary), fiber.HeaderOrigin, "Vary header should be set")

	ctx = corsTestRequest(handler, fiber.MethodGet, "http://localhost")
	require.NotContains(t, corsTestHeader(ctx, fiber.HeaderVary), fiber.HeaderOrigin, "Vary header should not be set")
	corsTestExpectHeaders(t, ctx,
		fiber.HeaderAccessControlAllowCredentials, "",
		fiber.HeaderAccessControlExposeHeaders, "X-Request-ID")
}

// A cache may reuse the response stored for a request without Origin for one
// with it, unless the two carry the same CORS headers or the stored one varies on Origin.
func Test_CORS_ResponsesAreSafeToCache(t *testing.T) {
	t.Parallel()

	const origin = "http://localhost"
	corsHeaders := []string{
		fiber.HeaderAccessControlAllowOrigin,
		fiber.HeaderAccessControlExposeHeaders,
		fiber.HeaderAccessControlAllowMethods,
	}

	configs := []struct {
		name   string
		config Config
	}{
		{name: "wildcard", config: Config{ExposeHeaders: []string{"X-Request-ID"}}},
		{name: "allow list", config: Config{AllowOrigins: []string{origin}, ExposeHeaders: []string{"X-Request-ID"}}},
	}

	requests := []struct {
		name    string
		method  string
		headers [][2]string // the stored request lacks the last one
	}{
		{name: "simple request", method: fiber.MethodGet, headers: [][2]string{
			{fiber.HeaderOrigin, origin},
		}},
		{name: "preflight after OPTIONS without Origin", method: fiber.MethodOptions, headers: [][2]string{
			{fiber.HeaderAccessControlRequestMethod, fiber.MethodPut},
			{fiber.HeaderOrigin, origin},
		}},
	}

	for _, cfg := range configs {
		for _, req := range requests {
			t.Run(cfg.name+"/"+req.name, func(t *testing.T) {
				t.Parallel()

				app := fiber.New()
				app.Use(New(cfg.config))
				app.All("/", func(c fiber.Ctx) error {
					return c.SendString("ok")
				})
				handler := app.Handler()

				do := func(headers [][2]string) *fasthttp.RequestCtx {
					ctx := &fasthttp.RequestCtx{}
					ctx.Request.SetRequestURI("/")
					ctx.Request.Header.SetMethod(req.method)
					for _, h := range headers {
						ctx.Request.Header.Set(h[0], h[1])
					}
					handler(ctx)
					return ctx
				}

				last := len(req.headers) - 1
				stored := do(req.headers[:last])
				later := do(req.headers)

				require.NotEmpty(t, string(later.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				if slices.Contains(strings.Split(string(stored.Response.Header.Peek(fiber.HeaderVary)), ", "), req.headers[last][0]) {
					return
				}
				for _, name := range corsHeaders {
					require.Equal(t, string(later.Response.Header.Peek(name)), string(stored.Response.Header.Peek(name)), name)
				}
			})
		}
	}
}

func Test_CORS_Origin_AllowCredentials(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		AllowOrigins:     []string{"http://localhost"},
		AllowCredentials: true,
		MaxAge:           3600,
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowHeaders:     []string{"Authentication"},
	}))
	handler := app.Handler()

	corsTestExpectHeaders(t, corsTestPreflight(handler, "http://localhost"),
		fiber.HeaderAccessControlAllowOrigin, "http://localhost",
		fiber.HeaderAccessControlAllowCredentials, "true",
		fiber.HeaderAccessControlMaxAge, "3600",
		fiber.HeaderAccessControlAllowHeaders, "Authentication")
	corsTestExpectHeaders(t, corsTestRequest(handler, fiber.MethodGet, "http://localhost"),
		fiber.HeaderAccessControlAllowCredentials, "true",
		fiber.HeaderAccessControlExposeHeaders, "X-Request-ID")
}

// Test for fiber-ghsa-fmg4-x8pw-hjhg
func Test_CORS_Wildcard_AllowCredentials_Panic(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() {
		fiber.New().Use(New(Config{AllowOrigins: []string{"*"}, AllowCredentials: true}))
	}, "Expected a panic when AllowOrigins is '*' and AllowCredentials is true")
}

// Test that a warning is logged when AllowOrigins allows all origins and
// AllowOriginsFunc is also provided.
func Test_CORS_Warn_AllowAllOrigins_WithFunc(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	fiber.New().Use(New(Config{AllowOrigins: []string{"*"}, AllowOriginsFunc: func(string) bool { return true }}))

	require.Contains(t, buf.String(), "AllowOriginsFunc' will not be used")
}

func Test_CORS_Invalid_Origins_Panic(t *testing.T) {
	t.Parallel()
	invalidOrigins := []string{
		"localhost",
		"http://foo.[a-z]*.example.com",
		"http://*",
		"https://*",
		"http://*.com*",
		"invalid url",
		"*",
		"http://origin.com,invalid url",
		// add more invalid origins as needed
	}

	for _, origin := range invalidOrigins {
		t.Run(origin, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() {
				fiber.New().Use(New(Config{AllowOrigins: []string{origin}, AllowCredentials: true}))
			}, "Expected a panic for invalid origin: %s", origin)
		})
	}
}

func Test_CORS_DisableValueRedaction(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "[CORS] Invalid origin format in configuration: [redacted]", func() {
		New(Config{AllowOrigins: []string{"http://"}, DisableValueRedaction: false})
	})

	require.PanicsWithValue(t, "[CORS] Invalid origin format in configuration: http://", func() {
		New(Config{AllowOrigins: []string{"http://"}, DisableValueRedaction: true})
	})
}

func Test_CORS_AllowOriginScheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reqOrigin         string
		pattern           []string
		shouldAllowOrigin bool
	}{
		{pattern: []string{"http://example.com"}, reqOrigin: "http://example.com", shouldAllowOrigin: true},
		{pattern: []string{"HTTP://EXAMPLE.COM"}, reqOrigin: "http://example.com", shouldAllowOrigin: true},
		// the origin is answered in the case it was sent in
		{pattern: []string{"http://example.com"}, reqOrigin: "HTTP://EXAMPLE.COM", shouldAllowOrigin: true},
		// a configured origin is trimmed, lower-cased and stripped of its trailing slash for the exact lookup
		{pattern: []string{" HTTP://EXAMPLE.COM/ "}, reqOrigin: "http://example.com", shouldAllowOrigin: true},
		{pattern: []string{"https://example.com"}, reqOrigin: "https://example.com", shouldAllowOrigin: true},
		{pattern: []string{"http://example.com"}, reqOrigin: "https://example.com", shouldAllowOrigin: false},
		{pattern: []string{"http://*.example.com"}, reqOrigin: "http://aaa.example.com", shouldAllowOrigin: true},
		{pattern: []string{"http://*.example.com"}, reqOrigin: "http://bbb.aaa.example.com", shouldAllowOrigin: true},
		{pattern: []string{"http://*.aaa.example.com"}, reqOrigin: "http://bbb.aaa.example.com", shouldAllowOrigin: true},
		{pattern: []string{"http://*.example.com:8080"}, reqOrigin: "http://aaa.example.com:8080", shouldAllowOrigin: true},
		{pattern: []string{"http://*.example.com"}, reqOrigin: "http://1.2.aaa.example.com", shouldAllowOrigin: true},
		{pattern: []string{"http://example.com"}, reqOrigin: "http://gofiber.com", shouldAllowOrigin: false},
		{pattern: []string{"http://*.aaa.example.com"}, reqOrigin: "http://ccc.bbb.example.com", shouldAllowOrigin: false},
		{
			pattern:           []string{"http://*.example.com"},
			reqOrigin:         "http://1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.example.com",
			shouldAllowOrigin: true,
		},
		{pattern: []string{"http://example.com"}, reqOrigin: "http://ccc.bbb.example.com", shouldAllowOrigin: false},
		{pattern: []string{"https://--aaa.bbb.com"}, reqOrigin: "https://prod-preview--aaa.bbb.com", shouldAllowOrigin: false},
		{pattern: []string{"http://*.example.com"}, reqOrigin: "http://ccc.bbb.example.com", shouldAllowOrigin: true},
		{pattern: []string{"http://domain-1.com", "http://example.com"}, reqOrigin: "http://example.com", shouldAllowOrigin: true},
		{pattern: []string{"http://domain-1.com", "http://example.com"}, reqOrigin: "http://domain-2.com", shouldAllowOrigin: false},
		{pattern: []string{"http://domain-1.com", "http://example.com"}, reqOrigin: "http://domain-1.com", shouldAllowOrigin: true},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.pattern, ", ")+" with Origin "+tt.reqOrigin, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use("/", New(Config{AllowOrigins: tt.pattern}))

			got := corsTestAllowOrigin(corsTestPreflight(app.Handler(), tt.reqOrigin))
			if tt.shouldAllowOrigin {
				require.Equal(t, tt.reqOrigin, got)
			} else {
				require.Empty(t, got)
			}
		})
	}
}

func Test_CORS_AllowOriginHeader_NoMatch(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use("/", New(Config{AllowOrigins: []string{"http://example-1.com", "https://example-1.com"}}))

	ctx := corsTestRequest(app.Handler(), fiber.MethodOptions, "http://google.com")

	var headerExists bool
	for key := range ctx.Response.Header.All() {
		if string(key) == fiber.HeaderAccessControlAllowOrigin {
			headerExists = true
		}
	}
	require.False(t, headerExists, "Access-Control-Allow-Origin header should not be set")
}

func Test_CORS_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

func Test_CORS_Headers_BasedOnRequestType(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Use(func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	handler := app.Handler()

	// QUERY (RFC 10008) is not CORS-safelisted (the safelisted set is GET/HEAD/POST per the Fetch spec), so a
	// CORS-aware user agent must preflight it: its preflight is answered with QUERY in Access-Control-Allow-Methods,
	// and the actual request gets a simple response (Allow-Origin set, no Allow-Methods), i.e. QUERY is gated by the
	// preflight and not treated as server-side safelisted.
	const allMethods = "GET, POST, HEAD, PUT, DELETE, PATCH, QUERY"
	methods := []string{
		fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodDelete,
		fiber.MethodPatch, fiber.MethodHead, fiber.MethodQuery,
	}
	for _, tc := range []struct {
		name        string
		origin      string // none when empty
		reqHeaders  string // the Access-Control-Request-Headers sent, none when empty
		wantOrigin  string
		wantMethods string
		wantHeaders string
		status      int
		preflight   bool // an OPTIONS whose Access-Control-Request-Method is the method under test
	}{
		{name: "Without origin", status: fiber.StatusOK, wantOrigin: "*"}, // the wildcard is sent on every response
		{
			name: "Preflight request with origin and Access-Control-Request-Method headers", origin: "http://example.com",
			preflight: true, status: fiber.StatusNoContent, wantOrigin: "*", wantMethods: allMethods,
		},
		{name: "Non-preflight request with origin", origin: "http://example.com", status: fiber.StatusOK, wantOrigin: "*"},
		{
			name: "Preflight with Access-Control-Request-Headers", origin: "http://example.com", preflight: true,
			reqHeaders: "X-Custom-Header", status: fiber.StatusNoContent, wantOrigin: "*", wantMethods: allMethods,
			wantHeaders: "X-Custom-Header",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, method := range methods {
				reqMethod, kv := method, []string(nil)
				if tc.preflight {
					reqMethod, kv = fiber.MethodOptions, []string{fiber.HeaderAccessControlRequestMethod, method}
				}
				if tc.reqHeaders != "" {
					kv = append(kv, fiber.HeaderAccessControlRequestHeaders, tc.reqHeaders)
				}

				ctx := corsTestRequest(handler, reqMethod, tc.origin, kv...)
				require.Equal(t, tc.status, ctx.Response.StatusCode(), method)
				require.Equal(t, tc.wantOrigin, corsTestAllowOrigin(ctx), method)
				require.Equal(t, tc.wantMethods, corsTestHeader(ctx, fiber.HeaderAccessControlAllowMethods), method)
				require.Equal(t, tc.wantHeaders, corsTestHeader(ctx, fiber.HeaderAccessControlAllowHeaders), method)
			}
		})
	}
}

func Test_CORS_AllowOriginsFunc(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use("/", New(Config{AllowOriginsFunc: func(origin string) bool { return strings.Contains(origin, "example-2") }}))
	handler := app.Handler()

	// Allow-Origin header should be empty because http://google.com does not satisfy 'strings.Contains(origin, "example-2")'
	// and AllowOrigins has not been set
	require.Empty(t, corsTestAllowOrigin(corsTestRequest(handler, fiber.MethodOptions, "http://google.com")))

	require.Equal(t, "http://example-2.com", corsTestAllowOrigin(corsTestPreflight(handler, "http://example-2.com")))
}

func Test_CORS_AllowOriginsFuncRejectsNonSerializedOrigins(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		Name               string
		Origin             string
		Method             string
		SetPreflightMethod bool
		ExpectAllowed      bool
	}{
		{Name: "UserInfoPresent", Origin: "http://user:pass@example.com", Method: fiber.MethodGet},
		{Name: "PathPresent", Origin: "http://example.com/path", Method: fiber.MethodOptions, SetPreflightMethod: true},
		{Name: "QueryPresent", Origin: "http://example.com?query=1", Method: fiber.MethodOptions, SetPreflightMethod: true},
		{Name: "FragmentPresent", Origin: "http://example.com#section", Method: fiber.MethodGet},
		{Name: "WildcardHost", Origin: "http://*.example.com", Method: fiber.MethodGet},
		{Name: "StandaloneWildcard", Origin: "*", Method: fiber.MethodGet},
		{Name: "NullOriginUppercase", Origin: "NULL", Method: fiber.MethodGet},
		{Name: "NullOriginMixedCase", Origin: "Null", Method: fiber.MethodGet},
		{Name: "NullOriginLowercase", Origin: "null", Method: fiber.MethodGet, ExpectAllowed: true},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use("/", New(Config{AllowOriginsFunc: func(string) bool { return true }}))
			app.All("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

			var kv []string
			if tc.SetPreflightMethod {
				kv = []string{fiber.HeaderAccessControlRequestMethod, fiber.MethodGet}
			}
			ctx := corsTestRequest(app.Handler(), tc.Method, tc.Origin, kv...)

			if tc.ExpectAllowed {
				require.Equal(t, "null", corsTestAllowOrigin(ctx))
			} else {
				require.Empty(t, corsTestAllowOrigin(ctx))
			}

			if tc.Method == fiber.MethodOptions {
				require.Equal(t, fiber.StatusNoContent, ctx.Response.StatusCode())
			} else {
				require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
			}
		})
	}
}

func Test_CORS_AllowOriginsAndAllowOriginsFunc_AllUseCases(t *testing.T) {
	t.Parallel()
	allowAll := func(string) bool { return true }
	allowNone := func(string) bool { return false }
	// AllowOrigins set to a subdomain pattern, surrounded by blanks
	subdomain := Config{AllowOrigins: []string{"  http://*.example.com  "}}
	originsAndFunc := Config{
		AllowOrigins:     []string{"http://example-1.com"},
		AllowOriginsFunc: func(origin string) bool { return strings.Contains(origin, "example-2") },
	}

	testCases := []struct {
		Name           string
		RequestOrigin  string
		ResponseOrigin string
		Config         Config
	}{
		{Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/OriginAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com"}}, RequestOrigin: "http://aaa.com", ResponseOrigin: "http://aaa.com"},
		{Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/MultipleOrigins/OriginAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com", "http://bbb.com"}}, RequestOrigin: "http://bbb.com", ResponseOrigin: "http://bbb.com"},
		{Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/MultipleOrigins/OriginNotAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com", "http://bbb.com"}}, RequestOrigin: "http://ccc.com", ResponseOrigin: ""},
		{Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/MultipleOrigins/Whitespace/OriginAllowed", Config: Config{AllowOrigins: []string{" http://aaa.com ", " http://bbb.com "}}, RequestOrigin: "http://aaa.com", ResponseOrigin: "http://aaa.com"},
		{Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/OriginNotAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com"}}, RequestOrigin: "http://bbb.com", ResponseOrigin: ""},
		{Name: "AllowOriginsDefined/AllowOriginsFuncReturnsTrue/OriginAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com"}, AllowOriginsFunc: allowAll}, RequestOrigin: "http://aaa.com", ResponseOrigin: "http://aaa.com"},
		{Name: "AllowOriginsDefined/AllowOriginsFuncReturnsTrue/OriginNotAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com"}, AllowOriginsFunc: allowAll}, RequestOrigin: "http://bbb.com", ResponseOrigin: "http://bbb.com"},
		{Name: "AllowOriginsDefined/AllowOriginsFuncReturnsFalse/OriginAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com"}, AllowOriginsFunc: allowNone}, RequestOrigin: "http://aaa.com", ResponseOrigin: "http://aaa.com"},
		{Name: "AllowOriginsDefined/AllowOriginsFuncReturnsFalse/OriginNotAllowed", Config: Config{AllowOrigins: []string{"http://aaa.com"}, AllowOriginsFunc: allowNone}, RequestOrigin: "http://bbb.com", ResponseOrigin: ""},
		{Name: "AllowOriginsEmpty/AllowOriginsFuncUndefined/OriginAllowed", Config: Config{}, RequestOrigin: "http://aaa.com", ResponseOrigin: "*"},
		{Name: "AllowOriginsEmpty/AllowOriginsFuncReturnsTrue/OriginAllowed", Config: Config{AllowOriginsFunc: allowAll}, RequestOrigin: "http://aaa.com", ResponseOrigin: "http://aaa.com"},
		{Name: "AllowOriginsEmpty/AllowOriginsFuncReturnsFalse/OriginNotAllowed", Config: Config{AllowOriginsFunc: allowNone}, RequestOrigin: "http://aaa.com", ResponseOrigin: ""},
		// Allow-Origin header should be "" because http://google.com does not satisfy http://*.example.com
		{Name: "Subdomain/disallowed origin", Config: subdomain, RequestOrigin: "http://google.com", ResponseOrigin: ""},
		{Name: "Subdomain/domain only (disallowed)", Config: subdomain, RequestOrigin: "http://example.com", ResponseOrigin: ""},
		{Name: "Subdomain/malformed subdomain (disallowed)", Config: subdomain, RequestOrigin: "http://evil.comexample.com", ResponseOrigin: ""},
		{Name: "Subdomain/allowed origin", Config: subdomain, RequestOrigin: "http://test.example.com", ResponseOrigin: "http://test.example.com"},
		// Allow-Origin header should be "" because http://google.com does not satisfy http://example-1.com or 'strings.Contains(origin, "example-2")'
		{Name: "OriginsAndFunc/disallowed origin", Config: originsAndFunc, RequestOrigin: "http://google.com", ResponseOrigin: ""},
		{Name: "OriginsAndFunc/allowed origin", Config: originsAndFunc, RequestOrigin: "http://example-1.com", ResponseOrigin: "http://example-1.com"},
		{Name: "OriginsAndFunc/allowed origin by the func", Config: originsAndFunc, RequestOrigin: "http://example-2.com", ResponseOrigin: "http://example-2.com"},
	}

	for i := range testCases {
		tc := &testCases[i]
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use("/", New(tc.Config))

			require.Equal(t, tc.ResponseOrigin, corsTestAllowOrigin(corsTestPreflight(app.Handler(), tc.RequestOrigin)))
		})
	}
}

// The fix for issue #2422
func Test_CORS_AllowCredentials(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		Name                string
		RequestOrigin       string
		ResponseOrigin      string
		ResponseCredentials string
		Config              Config
	}{
		{
			Name:          "AllowOriginsFuncDefined",
			Config:        Config{AllowCredentials: true, AllowOriginsFunc: func(_ string) bool { return true }},
			RequestOrigin: "http://aaa.com",
			// The AllowOriginsFunc config was defined, should use the real origin of the function
			ResponseOrigin:      "http://aaa.com",
			ResponseCredentials: "true",
		},
		{
			Name:           "fiber-ghsa-fmg4-x8pw-hjhg-wildcard-credentials",
			Config:         Config{AllowCredentials: true, AllowOriginsFunc: func(_ string) bool { return true }},
			RequestOrigin:  "*",
			ResponseOrigin: "",
			// Middleware will validate that wildcard won't set credentials to true and reject non-serialized origins
			ResponseCredentials: "",
		},
		{
			Name: "AllowOriginsFuncNotDefined",
			Config: Config{
				// Setting this to true will cause the middleware to panic since default AllowOrigins is "*"
				AllowCredentials: false,
			},
			RequestOrigin: "http://aaa.com",
			// None of the AllowOrigins or AllowOriginsFunc config was defined, should use the default origin of "*"
			// which will cause the CORS error in the client:
			// The value of the 'Access-Control-Allow-Origin' header in the response must not be the wildcard '*'
			// when the request's credentials mode is 'include'.
			ResponseOrigin:      "*",
			ResponseCredentials: "",
		},
		{
			Name:                "AllowOriginsDefined",
			Config:              Config{AllowCredentials: true, AllowOrigins: []string{"http://aaa.com"}},
			RequestOrigin:       "http://aaa.com",
			ResponseOrigin:      "http://aaa.com",
			ResponseCredentials: "true",
		},
		{
			Name:                "AllowOriginsDefined/UnallowedOrigin",
			Config:              Config{AllowCredentials: true, AllowOrigins: []string{"http://aaa.com"}},
			RequestOrigin:       "http://bbb.com",
			ResponseOrigin:      "",
			ResponseCredentials: "",
		},
	}

	for i := range testCases {
		tc := &testCases[i]
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use("/", New(tc.Config))

			ctx := corsTestPreflight(app.Handler(), tc.RequestOrigin)

			corsTestExpectHeaders(t, ctx,
				fiber.HeaderAccessControlAllowCredentials, tc.ResponseCredentials,
				fiber.HeaderAccessControlAllowOrigin, tc.ResponseOrigin)
		})
	}
}

// The Enhancement for issue #2804
func Test_CORS_AllowPrivateNetwork(t *testing.T) {
	t.Parallel()
	enabled := Config{AllowPrivateNetwork: true}
	tests := []struct {
		name      string
		method    string
		sent      string // the Access-Control-Request-Private-Network the client sends, none when empty
		want      string // the Access-Control-Allow-Private-Network of the answer
		config    Config
		preflight bool // the request carries Access-Control-Request-Method
	}{
		{name: "enabled preflight", config: enabled, method: fiber.MethodOptions, preflight: true, sent: "true", want: "true"},
		// only a preflight request is answered with the header
		{name: "enabled non-preflight GET", config: enabled, method: fiber.MethodGet, sent: "true"},
		{name: "enabled non-preflight OPTIONS", config: enabled, method: fiber.MethodOptions, sent: "true"},
		{name: "disabled by default", method: fiber.MethodOptions, preflight: true},
		{name: "disabled and the client sends the header", method: fiber.MethodOptions, preflight: true, sent: "true"},
		{name: "enabled and the client does not send the header", config: enabled, method: fiber.MethodOptions, preflight: true},
		{name: "enabled and the client sends false", config: enabled, method: fiber.MethodOptions, preflight: true, sent: "false"},
	}

	for i := range tests {
		tc := &tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tc.config))

			var kv []string
			if tc.preflight {
				kv = append(kv, fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
			}
			if tc.sent != "" {
				kv = append(kv, "Access-Control-Request-Private-Network", tc.sent)
			}
			ctx := corsTestRequest(app.Handler(), tc.method, "https://example.com", kv...)

			require.Equal(t, tc.want, corsTestHeader(ctx, "Access-Control-Allow-Private-Network"))
		})
	}
}

// corsBenchHandler returns the handler of an app that answers 200 behind the middleware, configured for
// origins (every origin when nil) and credentials, allowing four methods and three headers for 600s.
func corsBenchHandler(origins []string, credentials bool) fasthttp.RequestHandler {
	app := fiber.New()
	app.Use(New(Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodDelete},
		AllowHeaders:     []string{fiber.HeaderOrigin, fiber.HeaderContentType, fiber.HeaderAccept},
		AllowCredentials: credentials,
		MaxAge:           600,
	}))
	app.Use(func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	return app.Handler()
}

// corsBenchCtx returns a context holding a request from origin that names three request headers: a GET,
// or for OPTIONS the preflight of a POST.
func corsBenchCtx(origin, method string) *fasthttp.RequestCtx {
	req := &fasthttp.Request{}
	req.Header.SetMethod(method)
	req.SetRequestURI("/")
	req.Header.Set(fiber.HeaderOrigin, origin)
	if method == fiber.MethodOptions {
		req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodPost)
	}
	req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	return ctx
}

// corsBench measures h serving the request corsBenchCtx(origin, method) builds, over and over.
func corsBench(b *testing.B, h fasthttp.RequestHandler, origin, method string) {
	b.Helper()

	ctx := corsBenchCtx(origin, method)

	b.ReportAllocs()

	for b.Loop() {
		h(ctx)
	}
}

// corsBenchParallel is corsBench on every goroutine, each with a context of its own.
func corsBenchParallel(b *testing.B, h fasthttp.RequestHandler, origin, method string) {
	b.Helper()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := corsBenchCtx(origin, method)

		for pb.Next() {
			h(ctx)
		}
	})
}

func Benchmark_CORS_NewHandler(b *testing.B) {
	corsBench(b, corsBenchHandler([]string{"http://localhost", "http://example.com"}, true), "http://localhost", fiber.MethodGet)
}

func Benchmark_CORS_NewHandler_Parallel(b *testing.B) {
	h := corsBenchHandler([]string{"http://localhost", "http://example.com"}, true)
	corsBenchParallel(b, h, "http://localhost", fiber.MethodGet)
}

func Benchmark_CORS_NewHandlerSingleOrigin(b *testing.B) {
	corsBench(b, corsBenchHandler([]string{"http://example.com"}, true), "http://example.com", fiber.MethodGet)
}

func Benchmark_CORS_NewHandlerSingleOrigin_Parallel(b *testing.B) {
	corsBenchParallel(b, corsBenchHandler([]string{"http://example.com"}, true), "http://example.com", fiber.MethodGet)
}

func Benchmark_CORS_NewHandlerWildcard(b *testing.B) {
	corsBench(b, corsBenchHandler(nil, false), "http://example.com", fiber.MethodGet)
}

func Benchmark_CORS_NewHandlerWildcard_Parallel(b *testing.B) {
	corsBenchParallel(b, corsBenchHandler(nil, false), "http://example.com", fiber.MethodGet)
}

func Benchmark_CORS_NewHandlerPreflight(b *testing.B) {
	h := corsBenchHandler([]string{"http://localhost", "http://example.com"}, true)
	corsBench(b, h, "http://example.com", fiber.MethodOptions)
}

func Benchmark_CORS_NewHandlerPreflight_Parallel(b *testing.B) {
	h := corsBenchHandler([]string{"http://localhost", "http://example.com"}, true)
	corsBenchParallel(b, h, "http://example.com", fiber.MethodOptions)
}

func Benchmark_CORS_NewHandlerPreflightSingleOrigin(b *testing.B) {
	corsBench(b, corsBenchHandler([]string{"http://example.com"}, true), "http://example.com", fiber.MethodOptions)
}

func Benchmark_CORS_NewHandlerPreflightSingleOrigin_Parallel(b *testing.B) {
	corsBenchParallel(b, corsBenchHandler([]string{"http://example.com"}, true), "http://example.com", fiber.MethodOptions)
}

func Benchmark_CORS_NewHandlerPreflightWildcard(b *testing.B) {
	corsBench(b, corsBenchHandler(nil, false), "http://example.com", fiber.MethodOptions)
}

func Benchmark_CORS_NewHandlerPreflightWildcard_Parallel(b *testing.B) {
	corsBenchParallel(b, corsBenchHandler(nil, false), "http://example.com", fiber.MethodOptions)
}

// Test_CORS_setSimpleHeaders_NilConfig ensures the nil-config guard returns
// without panicking or mutating the response headers.
func Test_CORS_setSimpleHeaders_NilConfig(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, fiber.New())

	require.NotPanics(t, func() { setSimpleHeaders(c, "https://example.com", nil, nil) })
	require.Empty(t, string(c.Response().Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

// Test_CORS_setSimpleHeaders_WildcardWithCredentials verifies that when
// AllowCredentials is true and the resolved origin is "*", the middleware logs
// a warning and still reflects the value (the configuration is considered
// invalid, but the header is set to surface the misconfiguration).
func Test_CORS_setSimpleHeaders_WildcardWithCredentials(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	c := apptest.Ctx(t, fiber.New())

	setSimpleHeaders(c, "*", &Config{AllowCredentials: true}, nil)

	require.Equal(t, "*", string(c.Response().Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
	require.Empty(t, string(c.Response().Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Contains(t, buf.String(), "'AllowCredentials' is true, but 'AllowOrigins' cannot be set to '*'")
}

// Test_CORS_setPreflightHeaders_NilConfig ensures the preflight helper tolerates
// a nil config (delegating to setSimpleHeaders and skipping MaxAge handling).
func Test_CORS_setPreflightHeaders_NilConfig(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, fiber.New())

	require.NotPanics(t, func() { setPreflightHeaders(c, "https://example.com", "600", nil, nil) })
	require.Empty(t, string(c.Response().Header.Peek(fiber.HeaderAccessControlMaxAge)))
}

// Test_CORS_Security_CredentialsWithSubdomainWildcard ensures that when
// credentials are allowed together with a wildcard-subdomain origin pattern, a
// matching origin is reflected verbatim (never "*") alongside the credentials
// header, while a non-matching origin receives neither.
func Test_CORS_Security_CredentialsWithSubdomainWildcard(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{AllowCredentials: true, AllowOrigins: []string{"https://*.example.com"}}))

	handler := app.Handler()

	cases := []struct {
		name          string
		origin        string
		expectOrigin  string
		expectCredHdr string
	}{
		{name: "matching subdomain reflected with credentials", origin: "https://api.example.com", expectOrigin: "https://api.example.com", expectCredHdr: "true"},
		{name: "non-matching origin gets nothing", origin: "https://attacker.evil.com"},
		{name: "apex domain does not match wildcard", origin: "https://example.com"},
		{name: "scheme downgrade does not match", origin: "http://api.example.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := corsTestPreflight(handler, tc.origin)

			gotOrigin := corsTestAllowOrigin(ctx)
			gotCred := corsTestHeader(ctx, fiber.HeaderAccessControlAllowCredentials)
			require.Equal(t, tc.expectOrigin, gotOrigin)
			require.Equal(t, tc.expectCredHdr, gotCred)
			require.NotEqual(t, "*", gotOrigin, "credentialed responses must never use the wildcard")
		})
	}
}

// Test_CORS_Security_CredentialsSimpleRequest ensures the credentialed-origin
// reflection also applies to non-preflight (simple) requests, not just OPTIONS
// preflights.
func Test_CORS_Security_CredentialsSimpleRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{AllowCredentials: true, AllowOrigins: []string{"https://trusted.example.com"}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	// Allowed origin is reflected with credentials on a simple GET.
	resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/",
		apptest.WithHeader(fiber.HeaderOrigin, "https://trusted.example.com")))
	require.Equal(t, "https://trusted.example.com", resp.Header.Get(fiber.HeaderAccessControlAllowOrigin))
	require.Equal(t, "true", resp.Header.Get(fiber.HeaderAccessControlAllowCredentials))

	// Disallowed origin is reflected nowhere.
	resp = apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderOrigin, "https://evil.example.org")))
	require.Empty(t, resp.Header.Get(fiber.HeaderAccessControlAllowOrigin))
	require.Empty(t, resp.Header.Get(fiber.HeaderAccessControlAllowCredentials))
}

// Test_CORS_Security_NoOriginReflectionForDisallowed ensures a disallowed origin
// is never reflected back even when many origins are configured — the response
// must not echo an arbitrary attacker-controlled Origin.
func Test_CORS_Security_NoOriginReflectionForDisallowed(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{AllowOrigins: []string{"https://a.example.com", "https://b.example.com"}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/",
		apptest.WithHeader(fiber.HeaderOrigin, "https://attacker.example.net")))

	got := resp.Header.Get(fiber.HeaderAccessControlAllowOrigin)
	require.Empty(t, got)
	require.NotEqual(t, "https://attacker.example.net", got)
}

func Test_CORS_OriginWithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(New(Config{AllowOrigins: []string{"http://example.com"}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	do := func(header string) *fasthttp.RequestCtx {
		return corsTestRaw(t, app, "GET / HTTP/1.1\r\nHost: example.com\r\n"+header+"\r\n\r\n")
	}

	require.Equal(t, "http://example.com", corsTestAllowOrigin(do("origin: http://example.com")))
	require.Equal(t, "http://example.com", corsTestAllowOrigin(do("Origin: http://example.com")))
	require.Empty(t, corsTestAllowOrigin(do("origin: http://evil.example")))
}

func Test_CORS_PreflightWithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(New(Config{
		AllowOrigins: []string{"http://example.com"},
		AllowMethods: []string{fiber.MethodGet, fiber.MethodPut},
	}))
	app.Put("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	raw := "OPTIONS / HTTP/1.1\r\nHost: example.com\r\n" +
		"origin: http://example.com\r\n" +
		"access-control-request-method: PUT\r\n" +
		"access-control-request-headers: X-Custom\r\n\r\n"

	fctx := corsTestRaw(t, app, raw)

	require.Equal(t, fiber.StatusNoContent, fctx.Response.StatusCode())
	require.Equal(t, "http://example.com", corsTestAllowOrigin(fctx))
	require.Contains(t, corsTestHeader(fctx, fiber.HeaderAccessControlAllowMethods), fiber.MethodPut)
	require.Equal(t, "X-Custom", corsTestHeader(fctx, fiber.HeaderAccessControlAllowHeaders))
}

func Test_CORS_PreflightSplitRequestHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		AllowOrigins: []string{"http://example.com"},
		AllowMethods: []string{fiber.MethodGet, fiber.MethodPut},
	}))
	app.Put("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	raw := "OPTIONS / HTTP/1.1\r\nHost: example.com\r\n" +
		"Origin: http://example.com\r\n" +
		"Access-Control-Request-Method: PUT\r\n" +
		"Access-Control-Request-Headers: X-One\r\n" +
		"Access-Control-Request-Headers: X-Two\r\n\r\n"

	fctx := corsTestRaw(t, app, raw)

	require.Equal(t, fiber.StatusNoContent, fctx.Response.StatusCode())
	require.Equal(t, "X-One, X-Two", corsTestHeader(fctx, fiber.HeaderAccessControlAllowHeaders))
}

// Test_CORS_ConfiguredEmptyAllowHeaders pins the difference between an
// unconfigured AllowHeaders and one configured with nothing usable in it. An
// absent list means "echo whatever the request asked for"; a configured list
// that happens to join to the empty string must not be mistaken for one, or a
// malformed or environment-derived entry would silently authorize every header
// the request names.
func Test_CORS_ConfiguredEmptyAllowHeaders(t *testing.T) {
	t.Parallel()
	preflight := func(t *testing.T, cfg Config) string {
		t.Helper()

		app := fiber.New()
		app.Use(New(cfg))
		ctx := corsTestRequest(app.Handler(), fiber.MethodOptions, "http://localhost",
			fiber.HeaderAccessControlRequestMethod, fiber.MethodGet,
			fiber.HeaderAccessControlRequestHeaders, "X-Requested-Header")
		return corsTestHeader(ctx, fiber.HeaderAccessControlAllowHeaders)
	}

	t.Run("unconfigured echoes the request", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "X-Requested-Header", preflight(t, Config{}))
	})

	t.Run("configured empty authorizes nothing", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, preflight(t, Config{AllowHeaders: []string{""}}),
			"a configured list must not fall back to the requested headers")
	})
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

// Test_CORS_VaryDoesNotShareFieldsWithCustomCtx pins that the package-level
// field lists survive a custom context. They are shared by every request the
// middleware serves, so a Vary implementation that writes to its argument
// would otherwise corrupt the field names of every later response and race
// with the requests running alongside it.
func Test_CORS_VaryDoesNotShareFieldsWithCustomCtx(t *testing.T) {
	t.Parallel()
	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &varyMutatingCtx{DefaultCtx: *fiber.NewDefaultCtx(app)}
	})
	app.Use(New(Config{AllowPrivateNetwork: true}))
	handler := app.Handler()

	// One request down each branch that emits a Vary header.
	corsTestRequest(handler, fiber.MethodGet, "http://localhost")
	corsTestRequest(handler, fiber.MethodOptions, "") // no Origin: not a preflight
	corsTestPreflight(handler, "http://localhost")
	corsTestRequest(handler, fiber.MethodOptions, "http://localhost",
		fiber.HeaderAccessControlRequestMethod, fiber.MethodGet, fiber.HeaderAccessControlRequestPrivateNetwork, "true")

	require.Equal(t, []string{fiber.HeaderOrigin}, varyOrigin)
	require.Equal(t, []string{
		fiber.HeaderAccessControlRequestMethod,
		fiber.HeaderAccessControlRequestHeaders,
		fiber.HeaderOrigin,
	}, varyPreflight)
	require.Equal(t, []string{
		fiber.HeaderAccessControlRequestMethod,
		fiber.HeaderAccessControlRequestHeaders,
		fiber.HeaderAccessControlRequestPrivateNetwork,
		fiber.HeaderOrigin,
	}, varyPreflightPrivate)
}
