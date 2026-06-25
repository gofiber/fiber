package csrf

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_CSRF_Security_CompareConstantTime verifies the logical behavior of the
// constant-time comparison helpers used to validate tokens. The functions must
// only report equality for byte/string-identical inputs.
func Test_CSRF_Security_CompareConstantTime(t *testing.T) {
	t.Parallel()

	t.Run("strings", func(t *testing.T) {
		t.Parallel()
		require.True(t, compareStrings("abc123", "abc123"))
		require.False(t, compareStrings("abc123", "abc124"))
		require.False(t, compareStrings("abc", "abcd"))      // different length
		require.False(t, compareStrings("", "x"))            // empty vs non-empty
		require.True(t, compareStrings("", ""))              // both empty
		require.False(t, compareStrings("Abc123", "abc123")) // case sensitive
	})

	t.Run("tokens", func(t *testing.T) {
		t.Parallel()
		require.True(t, compareTokens([]byte("raw-token"), []byte("raw-token")))
		require.False(t, compareTokens([]byte("raw-token"), []byte("raw-tokeX")))
		require.False(t, compareTokens([]byte("short"), []byte("shorter")))
		require.False(t, compareTokens([]byte(nil), []byte("x")))
		require.True(t, compareTokens([]byte(nil), []byte(nil)))
	})
}

// Test_CSRF_Security_SecFetchSite_Normalization ensures the Sec-Fetch-Site
// validation is case-insensitive and tolerant of surrounding whitespace, while
// still rejecting genuinely unknown values.
func Test_CSRF_Security_SecFetchSite_Normalization(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	cases := []struct {
		name    string
		value   string
		wantErr error
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
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)
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
		h(ctx)
		setCookie := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
		return strings.Split(strings.Split(setCookie, ";")[0], "=")[1]
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

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
	require.NotEmpty(t, cookie.Value)
}

// Test_CSRF_Security_SchemeDowngradeRejected ensures that an HTTPS request whose
// Origin downgrades to HTTP for the same host is rejected: the scheme is part of
// the origin and must match.
func Test_CSRF_Security_SchemeDowngradeRejected(t *testing.T) {
	t.Parallel()

	app := newTrustedApp()
	app.Use(New(Config{CookieSecure: true}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	// Acquire a valid token over HTTPS.
	ctx := newTrustedRequestCtx()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// HTTPS host, but the Origin claims plain HTTP for the same host.
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
}

// Test_CSRF_Security_TrustedOriginSchemeIsolation ensures that trusting an
// origin under one scheme does not implicitly trust it under another scheme.
func Test_CSRF_Security_TrustedOriginSchemeIsolation(t *testing.T) {
	t.Parallel()

	app := newTrustedApp()
	app.Use(New(Config{
		CookieSecure:   true,
		TrustedOrigins: []string{"https://trusted.example.com"},
	}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	ctx := newTrustedRequestCtx()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	post := func(origin string) int {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.URI().SetScheme("https")
		ctx.Request.URI().SetHost("example.com")
		ctx.Request.Header.SetProtocol("https")
		ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
		ctx.Request.Header.SetHost("example.com")
		ctx.Request.Header.Set(fiber.HeaderOrigin, origin)
		ctx.Request.Header.Set(HeaderName, token)
		ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
		h(ctx)
		return ctx.Response.StatusCode()
	}

	// The exact trusted origin (https) is accepted.
	require.Equal(t, fiber.StatusOK, post("https://trusted.example.com"))
	// The same host over http is NOT trusted.
	require.Equal(t, fiber.StatusForbidden, post("http://trusted.example.com"))
}
