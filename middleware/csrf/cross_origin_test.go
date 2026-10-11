package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type crossOriginRequest struct {
	method       string
	host         string
	secFetchSite []string
	origin       []string
}

// crossOriginAllowed reports the verdict and the error handed to ErrorHandler.
func crossOriginAllowed(t *testing.T, cfg CrossOriginConfig, r *crossOriginRequest) (bool, error) {
	t.Helper()

	var captured error
	cfg.ErrorHandler = func(_ fiber.Ctx, err error) error {
		captured = err
		return fiber.ErrForbidden
	}

	app := fiber.New()
	app.Use(NewCrossOriginProtection(cfg))
	app.All("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	host := r.host
	if host == "" {
		host = "example.com"
	}
	req := httptest.NewRequest(r.method, "https://"+host+"/", http.NoBody)
	for _, value := range r.secFetchSite {
		req.Header.Add(fiber.HeaderSecFetchSite, value)
	}
	for _, value := range r.origin {
		req.Header.Add(fiber.HeaderOrigin, value)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	if resp.StatusCode == fiber.StatusNoContent {
		require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie))
		return true, captured
	}
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	return false, captured
}

func Test_CSRF_CrossOriginProtection(t *testing.T) {
	t.Parallel()

	trusted := []string{"https://trusted.example", "https://*.example.net"}

	tests := []struct {
		name           string
		method         string
		secFetchSite   []string
		origin         []string
		trustedOrigins []string
		bypass         bool
		allowed        bool
	}{
		{name: "safe get", method: fiber.MethodGet, secFetchSite: []string{"cross-site"}, allowed: true},
		{name: "safe head", method: fiber.MethodHead, secFetchSite: []string{"cross-site"}, allowed: true},
		{name: "safe options", method: fiber.MethodOptions, secFetchSite: []string{"cross-site"}, allowed: true},
		{name: "same origin fetch metadata", method: fiber.MethodPost, secFetchSite: []string{"same-origin"}, allowed: true},
		{name: "browser navigation", method: fiber.MethodPost, secFetchSite: []string{"none"}, allowed: true},
		{name: "non browser request", method: fiber.MethodPost, allowed: true},
		{name: "matching origin host", method: fiber.MethodPost, origin: []string{"https://example.com"}, allowed: true},
		{name: "matching origin host over http", method: fiber.MethodPost, origin: []string{"http://example.com"}, allowed: true},
		{name: "cross site fetch metadata", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}},
		{name: "same site fetch metadata", method: fiber.MethodPost, secFetchSite: []string{"same-site"}},
		{name: "mixed case fetch metadata", method: fiber.MethodPost, secFetchSite: []string{"Same-Origin"}},
		{name: "malformed fetch metadata", method: fiber.MethodPost, secFetchSite: []string{"invalid"}},
		{name: "cross site overrides matching origin host", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://example.com"}},
		{name: "trace is unsafe", method: fiber.MethodTrace, secFetchSite: []string{"cross-site"}},
		{name: "query is unsafe", method: fiber.MethodQuery, secFetchSite: []string{"cross-site"}},
		{name: "mismatched origin host", method: fiber.MethodPost, origin: []string{"https://evil.example"}},
		{name: "origin host case mismatch", method: fiber.MethodPost, origin: []string{"https://EXAMPLE.com"}},
		{name: "origin default port is not folded", method: fiber.MethodPost, origin: []string{"https://example.com:443"}},
		{name: "malformed origin", method: fiber.MethodPost, origin: []string{"://invalid"}},
		{name: "null origin", method: fiber.MethodPost, origin: []string{"null"}},
		{name: "trusted origin", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://trusted.example"}, trustedOrigins: trusted, allowed: true},
		{name: "trusted origin without fetch metadata", method: fiber.MethodPost, origin: []string{"https://trusted.example"}, trustedOrigins: trusted, allowed: true},
		{name: "trusted origin exempts malformed fetch metadata", method: fiber.MethodPost, secFetchSite: []string{"invalid"}, origin: []string{"https://trusted.example"}, trustedOrigins: trusted, allowed: true},
		{name: "trusted origin entry is normalized", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://trusted.example"}, trustedOrigins: []string{" https://Trusted.Example/ "}, allowed: true},
		{name: "trusted origin match is exact", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://TRUSTED.example"}, trustedOrigins: trusted},
		{name: "untrusted origin", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://evil.example"}, trustedOrigins: trusted},
		{name: "cross site without origin", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, trustedOrigins: trusted},
		{name: "trusted subdomain", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://api.example.net"}, trustedOrigins: trusted, allowed: true},
		{name: "trusted subdomain match is exact", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://API.EXAMPLE.net"}, trustedOrigins: trusted},
		{name: "wildcard does not cover the apex", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://example.net"}, trustedOrigins: trusted},
		{name: "wildcard covers nested subdomains", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://a.b.example.net"}, trustedOrigins: trusted, allowed: true},
		{name: "wildcard needs a label boundary", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://evilexample.net"}, trustedOrigins: trusted},
		{name: "repeated fetch metadata", method: fiber.MethodPost, secFetchSite: []string{"same-origin", "cross-site"}},
		{name: "repeated fetch metadata reversed", method: fiber.MethodPost, secFetchSite: []string{"cross-site", "same-origin"}},
		{name: "repeated origin", method: fiber.MethodPost, origin: []string{"https://example.com", "https://evil.example"}},
		{name: "same origin fetch metadata does not read origin", method: fiber.MethodPost, secFetchSite: []string{"same-origin"}, origin: []string{"https://example.com", "https://evil.example"}, allowed: true},
		{name: "repeated trusted origin", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, origin: []string{"https://trusted.example", "https://evil.example"}, trustedOrigins: trusted},
		{name: "next bypass", method: fiber.MethodPost, secFetchSite: []string{"cross-site"}, bypass: true, allowed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			allowed, captured := crossOriginAllowed(t, CrossOriginConfig{
				TrustedOrigins: test.trustedOrigins,
				Next:           func(fiber.Ctx) bool { return test.bypass },
			}, &crossOriginRequest{method: test.method, secFetchSite: test.secFetchSite, origin: test.origin})

			require.Equal(t, test.allowed, allowed)
			if test.allowed {
				require.NoError(t, captured)
				return
			}
			require.ErrorIs(t, captured, ErrCrossOriginRequest)
		})
	}
}

func Test_CSRF_CrossOriginProtection_MatchesNetHTTPCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		method        string
		host          string
		secFetchSite  string
		origin        string
		trustedOrigin string
	}{
		{name: "safe method", method: fiber.MethodGet, host: "example.com", secFetchSite: "cross-site"},
		{name: "trace", method: fiber.MethodTrace, host: "example.com", secFetchSite: "cross-site"},
		{name: "query", method: fiber.MethodQuery, host: "example.com", secFetchSite: "cross-site"},
		{name: "same origin fetch metadata", method: fiber.MethodPost, host: "example.com", secFetchSite: "same-origin"},
		{name: "same origin fetch metadata with mismatched origin", method: fiber.MethodPost, host: "example.com", secFetchSite: "same-origin", origin: "https://evil.example"},
		{name: "browser navigation", method: fiber.MethodPost, host: "example.com", secFetchSite: "none"},
		{name: "no browser headers", method: fiber.MethodPost, host: "example.com"},
		{name: "matching origin host", method: fiber.MethodPost, host: "example.com", origin: "https://example.com"},
		{name: "matching origin host over http", method: fiber.MethodPost, host: "example.com", origin: "http://example.com"},
		{name: "matching origin host with port", method: fiber.MethodPost, host: "example.com:8443", origin: "https://example.com:8443"},
		{name: "origin default port is not folded", method: fiber.MethodPost, host: "example.com", origin: "https://example.com:443"},
		{name: "cross site", method: fiber.MethodPost, host: "example.com", secFetchSite: "cross-site"},
		{name: "same site", method: fiber.MethodPost, host: "example.com", secFetchSite: "same-site"},
		{name: "mixed case fetch metadata", method: fiber.MethodPost, host: "example.com", secFetchSite: "Same-Origin"},
		{name: "malformed fetch metadata", method: fiber.MethodPost, host: "example.com", secFetchSite: "invalid"},
		{name: "cross site with matching host", method: fiber.MethodPost, host: "example.com", secFetchSite: "cross-site", origin: "https://example.com"},
		{name: "mismatched origin host", method: fiber.MethodPost, host: "example.com", origin: "https://evil.example"},
		{name: "origin host case mismatch", method: fiber.MethodPost, host: "example.com", origin: "https://EXAMPLE.com"},
		{name: "host header case mismatch", method: fiber.MethodPost, host: "EXAMPLE.com", origin: "https://example.com"},
		{name: "malformed origin", method: fiber.MethodPost, host: "example.com", origin: "://invalid"},
		{name: "null origin", method: fiber.MethodPost, host: "example.com", origin: "null"},
		{name: "trusted origin", method: fiber.MethodPost, host: "example.com", secFetchSite: "cross-site", origin: "https://trusted.example", trustedOrigin: "https://trusted.example"},
		{name: "trusted origin without fetch metadata", method: fiber.MethodPost, host: "example.com", origin: "https://trusted.example", trustedOrigin: "https://trusted.example"},
		{name: "trusted origin exempts malformed fetch metadata", method: fiber.MethodPost, host: "example.com", secFetchSite: "invalid", origin: "https://trusted.example", trustedOrigin: "https://trusted.example"},
		{name: "trusted origin exact mismatch", method: fiber.MethodPost, host: "example.com", secFetchSite: "cross-site", origin: "https://TRUSTED.example", trustedOrigin: "https://trusted.example"},
		{name: "untrusted origin", method: fiber.MethodPost, host: "example.com", secFetchSite: "cross-site", origin: "https://evil.example", trustedOrigin: "https://trusted.example"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			protection := http.NewCrossOriginProtection()
			var cfg CrossOriginConfig
			if test.trustedOrigin != "" {
				require.NoError(t, protection.AddTrustedOrigin(test.trustedOrigin))
				cfg.TrustedOrigins = []string{test.trustedOrigin}
			}
			stdlibAllowed := protection.Check(&http.Request{
				Method: test.method,
				Host:   test.host,
				Header: http.Header{"Sec-Fetch-Site": {test.secFetchSite}, "Origin": {test.origin}},
			}) == nil

			request := &crossOriginRequest{method: test.method, host: test.host}
			if test.secFetchSite != "" {
				request.secFetchSite = []string{test.secFetchSite}
			}
			if test.origin != "" {
				request.origin = []string{test.origin}
			}
			fiberAllowed, captured := crossOriginAllowed(t, cfg, request)

			require.Equal(t, stdlibAllowed, fiberAllowed)
			if !fiberAllowed {
				require.ErrorIs(t, captured, ErrCrossOriginRequest)
			}
		})
	}
}

// net/http allows this request: "null" parses to an empty host, which equals an empty Host.
func Test_CSRF_CrossOriginProtection_EmptyHost(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(NewCrossOriginProtection())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "null")
	app.Handler()(ctx)

	require.Empty(t, ctx.Request.Header.Host())
	require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
}

func Test_CSRF_CrossOriginProtection_InvalidTrustedOrigin(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"trusted.example", "https://trusted.example/path", "capacitor://localhost"} {
		t.Run(origin, func(t *testing.T) {
			t.Parallel()

			require.PanicsWithValue(t, "[CSRF] Invalid origin format in configuration:"+redactedKey, func() {
				NewCrossOriginProtection(CrossOriginConfig{TrustedOrigins: []string{origin}})
			})
		})
	}
}

// A header the lookup missed would read as absent, which this check allows.
func Test_CSRF_CrossOriginProtection_DisableHeaderNormalizing(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(NewCrossOriginProtection())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	h := app.Handler()

	tests := []struct{ header, value string }{
		{header: "sec-fetch-site", value: "cross-site"},
		{header: "origin", value: "https://evil.example"},
	}

	for _, test := range tests {
		t.Run(test.header, func(t *testing.T) {
			t.Parallel()

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.DisableNormalizing()
			ctx.Request.Header.SetMethod(fiber.MethodPost)
			ctx.Request.SetRequestURI("/")
			ctx.Request.Header.SetHost("example.com")
			ctx.Request.Header.Set(test.header, test.value)
			h(ctx)

			require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Middleware_CSRF_CrossOriginProtection -benchmem -count=4
func Benchmark_Middleware_CSRF_CrossOriginProtection(b *testing.B) {
	app := fiber.New()

	app.Use(NewCrossOriginProtection())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderSecFetchSite, "same-origin")

	b.ReportAllocs()

	for b.Loop() {
		h(ctx)
	}

	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}
