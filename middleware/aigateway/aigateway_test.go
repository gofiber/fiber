package aigateway

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

var testConfig = fiber.TestConfig{
	Timeout:       10 * time.Second,
	FailOnTimeout: true,
}

func startServer(t *testing.T, app *fiber.App) string {
	t.Helper()

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		if err := app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			panic(err)
		}
	}()
	t.Cleanup(func() {
		_ = app.Shutdown() //nolint:errcheck // best-effort test cleanup
	})

	return ln.Addr().String()
}

// echoUpstream starts an upstream that reports back what it received.
func echoUpstream(t *testing.T) string {
	t.Helper()

	app := fiber.New()
	app.All("/*", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"path":              c.Path(),
			"query":             string(c.RequestCtx().URI().QueryString()),
			"authorization":     c.Get(fiber.HeaderAuthorization),
			"x_api_key":         c.Get("x-api-key"),
			"api_key":           c.Get("api-key"),
			"anthropic_version": c.Get("anthropic-version"),
			"user_agent":        c.Get(fiber.HeaderUserAgent),
			"content_type":      c.Get(fiber.HeaderContentType),
			"body":              string(c.Body()),
		})
	})

	return "http://" + startServer(t, app)
}

type echoResult struct {
	Path             string `json:"path"`
	Query            string `json:"query"`
	Authorization    string `json:"authorization"`
	XAPIKey          string `json:"x_api_key"`
	APIKey           string `json:"api_key"`
	AnthropicVersion string `json:"anthropic_version"`
	UserAgent        string `json:"user_agent"`
	ContentType      string `json:"content_type"`
	Body             string `json:"body"`
}

func decodeEcho(t *testing.T, resp *http.Response) echoResult {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var res echoResult
	require.NoError(t, json.Unmarshal(body, &res), string(body))
	return res
}

func Test_AIGateway_UnifiedKeyInjection(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use("/openai", New(Config{
		PathPrefix: "/openai",
		Upstreams:  []Upstream{{Name: "test", URL: upstream, Key: "sk-server"}},
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/openai/v1/chat/completions?stream=false", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer sk-client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	// A second credential must not leak upstream.
	req.Header.Set("x-api-key", "smuggled")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	echo := decodeEcho(t, resp)
	require.Equal(t, "Bearer sk-server", echo.Authorization)
	require.Empty(t, echo.XAPIKey)
	require.Equal(t, "/v1/chat/completions", echo.Path)
	require.Equal(t, "stream=false", echo.Query)
	require.JSONEq(t, `{"model":"gpt-4o"}`, echo.Body)
	require.Equal(t, fiber.MIMEApplicationJSON, echo.ContentType)
}

func Test_AIGateway_PassthroughClientKey(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		ForwardClientKey: true,
		Upstreams:        []Upstream{{Name: "test", URL: upstream}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer sk-client")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Bearer sk-client", decodeEcho(t, resp).Authorization)
}

func Test_AIGateway_AnthropicStyleInjection(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{
			Name:    "anthropic",
			URL:     upstream,
			Auth:    AuthHeader("x-api-key"),
			Key:     "sk-ant-server",
			Headers: map[string]string{"anthropic-version": "2023-06-01"},
		}},
	}))

	// Client authenticates to the gateway with a Bearer token; the gateway
	// re-injects the upstream credential in Anthropic's header style.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-5"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer virtual-key")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	echo := decodeEcho(t, resp)
	require.Equal(t, "sk-ant-server", echo.XAPIKey)
	require.Empty(t, echo.Authorization)
	require.Equal(t, "2023-06-01", echo.AnthropicVersion)
}

func Test_AIGateway_MissingKey(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: "http://127.0.0.1:1", Key: "k"}},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody), testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "authentication_error")
}

func Test_AIGateway_AllowClientKeyMissing(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		AllowClientKeyMissing: true,
		Upstreams:             []Upstream{{Name: "test", URL: upstream, Key: "sk-server"}},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody), testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Bearer sk-server", decodeEcho(t, resp).Authorization)
}

func Test_AIGateway_KeyValidator(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk-server"}},
		KeyValidator: func(_ fiber.Ctx, key string) (bool, error) {
			return key == "virtual-good", nil
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer virtual-good")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	req = httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer virtual-bad")
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func Test_AIGateway_KeyFromContext(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	var gotKey, gotProvider, gotModel string
	app := fiber.New()
	// The outer middleware observes the context values after the gateway ran.
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		gotKey = KeyFromContext(c)
		gotProvider = ProviderFromContext(c)
		gotModel = ModelFromContext(c)
		return err
	})
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk-server"}},
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer sk-client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "sk-client", gotKey)
	require.Equal(t, "test", gotProvider)
	require.Equal(t, "gpt-4o", gotModel)
}

func Test_AIGateway_ModelAllowList(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o*", "gpt-4.1-mini"},
	}))

	send := func(body string) int {
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		return resp.StatusCode
	}

	require.Equal(t, fiber.StatusOK, send(`{"model":"gpt-4o"}`))
	require.Equal(t, fiber.StatusOK, send(`{"model":"gpt-4o-mini"}`))
	require.Equal(t, fiber.StatusOK, send(`{"model":"gpt-4.1-mini"}`))
	require.Equal(t, fiber.StatusForbidden, send(`{"model":"gpt-3.5-turbo"}`))
	// Missing or unparseable model is rejected when the allow-list is set.
	require.Equal(t, fiber.StatusForbidden, send(`{}`))
	require.Equal(t, fiber.StatusForbidden, send(`not json`))
}

func Test_AIGateway_ModelSniffWithoutAllowList(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
	}))

	// Non-JSON bodies relay untouched when no allow-list is set.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/audio/transcriptions", strings.NewReader("binary-ish"))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEOctetStream)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "binary-ish", decodeEcho(t, resp).Body)
}

func Test_AIGateway_PathAllowList(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedPaths: []string{"/v1/chat/*", "/v1/models"},
	}))

	send := func(path string) int {
		req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		return resp.StatusCode
	}

	require.Equal(t, fiber.StatusOK, send("/v1/chat/completions"))
	require.Equal(t, fiber.StatusOK, send("/v1/models"))
	require.Equal(t, fiber.StatusForbidden, send("/v1/embeddings"))
}

func Test_AIGateway_RelayFidelity(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Get("/v1/teapot", func(c fiber.Ctx) error {
		c.Set("X-Custom-Header", "custom-value")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.Status(fiber.StatusTeapot).SendString(`{"error":{"message":"teapot"}}`)
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/teapot", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)

	// Non-retryable statuses relay verbatim, headers included.
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.Equal(t, "custom-value", resp.Header.Get("X-Custom-Header"))
	require.Equal(t, fiber.MIMEApplicationJSON, resp.Header.Get(fiber.HeaderContentType))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"error":{"message":"teapot"}}`, string(body))
}

func Test_AIGateway_MaxResponseSizeBuffered(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Get("/v1/big", func(c fiber.Ctx) error {
		return c.SendString(strings.Repeat("x", 2048))
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:       []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		MaxResponseSize: 1024,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/big", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadGateway, resp.StatusCode)
}

func Test_AIGateway_UsageHookBuffered(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"id":    "cmpl-1",
			"usage": fiber.Map{"prompt_tokens": 10, "completion_tokens": 25, "total_tokens": 35},
		})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		OnUsage:   func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer sk-client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NotNil(t, got)
	require.Equal(t, "test", got.Provider)
	require.Equal(t, "gpt-4o", got.Model)
	require.Equal(t, fiber.MethodPost, got.Method)
	require.Equal(t, "/v1/chat/completions", got.Path)
	require.Equal(t, fiber.StatusOK, got.StatusCode)
	require.Equal(t, 1, got.Attempts)
	require.False(t, got.Streamed)
	require.Equal(t, "sk-client", got.ClientKey)
	require.Positive(t, got.Latency)
	require.Positive(t, got.ResponseBytes)
	require.NoError(t, got.Err)
	require.NotNil(t, got.Usage)
	require.Equal(t, 10, got.Usage.InputTokens)
	require.Equal(t, 25, got.Usage.OutputTokens)
	require.Equal(t, 35, got.Usage.TotalTokens)
}

func Test_AIGateway_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Next:      func(fiber.Ctx) bool { return true },
		Upstreams: []Upstream{{Name: "test", URL: "http://127.0.0.1:1", Key: "sk"}},
	}))
	app.Get("/local", func(c fiber.Ctx) error { return c.SendString("skipped") })

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/local", http.NoBody), testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "skipped", string(body))
}

func Benchmark_AIGateway_NonStreaming(b *testing.B) {
	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.SendString(`{"id":"cmpl-1","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	})
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	go func() {
		_ = upstreamApp.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) //nolint:errcheck // benchmark server
	}()
	defer func() { _ = upstreamApp.Shutdown() }() //nolint:errcheck // best-effort cleanup

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "bench", URL: "http://" + ln.Addr().String(), Key: "sk"}},
	}))

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		if err != nil || resp.StatusCode != fiber.StatusOK {
			b.Fatalf("unexpected result: %v %d", err, resp.StatusCode)
		}
	}
}
