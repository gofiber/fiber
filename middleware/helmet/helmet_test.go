package helmet

import (
	"crypto/tls"
	"maps"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/stretchr/testify/require"
)

// helmetTestApp returns an app that serves "Hello, World!" at / behind New(cfg...).
func helmetTestApp(cfg ...Config) *fiber.App {
	app := fiber.New()
	app.Use(New(cfg...))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello, World!") })
	return app
}

// Test_Headers covers the response headers of the default config and of configs
// that set some or all of them.
func Test_Headers(t *testing.T) {
	t.Parallel()
	const realWorldCSP = "default-src 'self';base-uri 'self';font-src 'self' https: data:;form-action 'self';frame-ancestors 'self';img-src 'self' data:;object-src 'none';script-src 'self';script-src-attr 'none';style-src 'self' https: 'unsafe-inline';upgrade-insecure-requests"
	defaults := map[string]string{
		fiber.HeaderXXSSProtection:          "0",
		fiber.HeaderXContentTypeOptions:     "nosniff",
		fiber.HeaderXFrameOptions:           "SAMEORIGIN",
		fiber.HeaderContentSecurityPolicy:   "",
		fiber.HeaderReferrerPolicy:          "no-referrer",
		fiber.HeaderPermissionsPolicy:       "",
		"Cross-Origin-Embedder-Policy":      "require-corp",
		"Cross-Origin-Opener-Policy":        "same-origin",
		"Cross-Origin-Resource-Policy":      "same-origin",
		"Origin-Agent-Cluster":              "?1",
		"X-DNS-Prefetch-Control":            "off",
		"X-Download-Options":                "noopen",
		"X-Permitted-Cross-Domain-Policies": "none",
	}
	// The real-world config sets every header to its default value but the CSP
	// and the permissions policy.
	realWorld := maps.Clone(defaults)
	realWorld[fiber.HeaderContentSecurityPolicy] = realWorldCSP
	realWorld[fiber.HeaderPermissionsPolicy] = "geolocation=(self)"

	for _, tc := range []struct {
		want map[string]string // the value of each header; "" means it must be empty
		name string
		cfg  []Config // none: the middleware is created without a config
	}{
		{name: "default", want: defaults},
		{
			name: "custom values for all headers",
			cfg: []Config{{
				XSSProtection:             "0",
				ContentTypeNosniff:        "custom-nosniff",
				XFrameOptions:             "DENY",
				HSTSExcludeSubdomains:     true,
				ContentSecurityPolicy:     "default-src 'none'",
				CSPReportOnly:             true,
				ReferrerPolicy:            "origin",
				PermissionPolicy:          "geolocation=(self)",
				CrossOriginEmbedderPolicy: "custom-value",
				CrossOriginOpenerPolicy:   "custom-value",
				CrossOriginResourcePolicy: "custom-value",
				OriginAgentCluster:        "custom-value",
				XDNSPrefetchControl:       "custom-control",
				XDownloadOptions:          "custom-options",
				XPermittedCrossDomain:     "custom-policies",
			}},
			want: map[string]string{
				fiber.HeaderXXSSProtection:                  "0",
				fiber.HeaderXContentTypeOptions:             "custom-nosniff",
				fiber.HeaderXFrameOptions:                   "DENY",
				fiber.HeaderContentSecurityPolicyReportOnly: "default-src 'none'",
				fiber.HeaderReferrerPolicy:                  "origin",
				fiber.HeaderPermissionsPolicy:               "geolocation=(self)",
				"Cross-Origin-Embedder-Policy":              "custom-value",
				"Cross-Origin-Opener-Policy":                "custom-value",
				"Cross-Origin-Resource-Policy":              "custom-value",
				"Origin-Agent-Cluster":                      "custom-value",
				"X-DNS-Prefetch-Control":                    "custom-control",
				"X-Download-Options":                        "custom-options",
				"X-Permitted-Cross-Domain-Policies":         "custom-policies",
			},
		},
		{
			name: "real-world values for all headers",
			cfg: []Config{{
				XSSProtection:             "0",
				ContentTypeNosniff:        "nosniff",
				XFrameOptions:             "SAMEORIGIN",
				HSTSExcludeSubdomains:     false,
				ContentSecurityPolicy:     realWorldCSP,
				CSPReportOnly:             false,
				HSTSPreloadEnabled:        true,
				ReferrerPolicy:            "no-referrer",
				PermissionPolicy:          "geolocation=(self)",
				CrossOriginEmbedderPolicy: "require-corp",
				CrossOriginOpenerPolicy:   "same-origin",
				CrossOriginResourcePolicy: "same-origin",
				OriginAgentCluster:        "?1",
				XDNSPrefetchControl:       "off",
				XDownloadOptions:          "noopen",
				XPermittedCrossDomain:     "none",
			}},
			want: realWorld,
		},
		{
			name: "content security policy",
			cfg:  []Config{{ContentSecurityPolicy: "default-src 'none'"}},
			want: map[string]string{fiber.HeaderContentSecurityPolicy: "default-src 'none'"},
		},
		{
			name: "content security policy report only",
			cfg:  []Config{{ContentSecurityPolicy: "default-src 'none'", CSPReportOnly: true}},
			want: map[string]string{
				fiber.HeaderContentSecurityPolicyReportOnly: "default-src 'none'",
				fiber.HeaderContentSecurityPolicy:           "",
			},
		},
		{
			name: "permissions policy",
			cfg:  []Config{{PermissionPolicy: "microphone=()"}},
			want: map[string]string{fiber.HeaderPermissionsPolicy: "microphone=()"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Do(t, helmetTestApp(tc.cfg...), "/")
			for name, want := range tc.want {
				require.Equal(t, want, resp.Header.Get(name), name)
			}
		})
	}
}

func Test_Next(t *testing.T) {
	t.Parallel()
	app := helmetTestApp(Config{
		Next:           func(ctx fiber.Ctx) bool { return ctx.Path() == "/next" },
		ReferrerPolicy: "no-referrer",
	})
	app.Get("/next", func(c fiber.Ctx) error { return c.SendString("Skipped!") })

	resp := apptest.Do(t, app, "/")
	require.Equal(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))

	resp = apptest.Do(t, app, "/next")
	require.Empty(t, resp.Header.Get(fiber.HeaderReferrerPolicy))
}

func Test_HSTSHeaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		scheme   string
		expected string
		config   Config
	}{
		{"TLS request with max age sets header", "https", "max-age=60; includeSubDomains", Config{HSTSMaxAge: 60}},
		{
			name: "TLS request with excluded subdomains sets bare max-age", scheme: "https", expected: "max-age=60",
			config: Config{HSTSMaxAge: 60, HSTSExcludeSubdomains: true},
		},
		{
			name: "TLS request with preload includes subdomains", scheme: "https",
			expected: "max-age=31536000; includeSubDomains; preload",
			config:   Config{HSTSMaxAge: 31536000, HSTSPreloadEnabled: true},
		},
		{name: "TLS request with zero max age omits header", config: Config{HSTSMaxAge: 0}, scheme: "https"},
		{name: "HTTP request with max age omits header", config: Config{HSTSMaxAge: 60}, scheme: "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, hstsHeaderForRequest(t, &tt.config, tt.scheme))
		})
	}
}

func Test_HSTSHeadersPanicsOnNegativeMaxAge(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "helmet: HSTSMaxAge must be greater than or equal to 0", func() {
		New(Config{HSTSMaxAge: -1})
	})
}

func Test_HSTSExcludeSubdomainsAndPreload(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "helmet: HSTSPreloadEnabled requires HSTSExcludeSubdomains to be false", func() {
		New(Config{HSTSMaxAge: 31536000, HSTSExcludeSubdomains: true, HSTSPreloadEnabled: true})
	})
}

func hstsHeaderForRequest(t *testing.T, config *Config, scheme string) string {
	t.Helper()

	app := helmetTestApp(*config)

	if scheme == "http" {
		resp := apptest.Do(t, app, "/")
		header := resp.Header.Get(fiber.HeaderStrictTransportSecurity)
		require.NoError(t, resp.Body.Close())
		return header
	}

	require.Equal(t, "https", scheme)

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	tlsListener := tls.NewListener(ln, serverTLSConf)
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listener(tlsListener, fiber.ListenConfig{DisableStartupMessage: true}) }()

	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: clientTLSConf}}

	var resp *http.Response
	for deadline := time.Now().Add(time.Second); ; {
		resp, err = client.Get("https://" + ln.Addr().String() + "/")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			require.NoError(t, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	header := resp.Header.Get(fiber.HeaderStrictTransportSecurity)
	require.NoError(t, resp.Body.Close())
	client.CloseIdleConnections()

	require.NoError(t, app.Shutdown())
	require.NoError(t, apptest.Recv(t, errCh, time.Second, "timed out waiting for TLS test server shutdown"))

	return header
}
