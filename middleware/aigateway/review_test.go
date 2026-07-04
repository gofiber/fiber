package aigateway

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/stretchr/testify/require"
)

// headerEchoUpstream echoes one named request header back in the body.
func headerEchoUpstream(t *testing.T, header string) string {
	t.Helper()

	app := fiber.New()
	app.All("/*", func(c fiber.Ctx) error {
		return c.SendString(c.Get(header))
	})
	return "http://" + startServer(t, app)
}

func Test_AIGateway_CustomExtractorHeaderStripped(t *testing.T) {
	t.Parallel()

	// A custom extractor reads the client credential from X-Custom-Token; it
	// must be stripped before relaying so the gateway virtual key never leaks
	// upstream in unified-key mode.
	upstream := headerEchoUpstream(t, "X-Custom-Token")
	app := fiber.New()
	app.Use(New(Config{
		KeyExtractor: extractors.FromHeader("X-Custom-Token"),
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk-server"}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set("X-Custom-Token", "client-virtual-key")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Empty(t, string(body), "custom credential header must not reach the upstream")
}

func Test_AIGateway_CustomAuthHeaderSmugglingBlocked(t *testing.T) {
	t.Parallel()

	// The upstream authenticates via x-goog-api-key. A client sending that
	// header must not smuggle its own value past the injected server key.
	upstream := headerEchoUpstream(t, "x-goog-api-key")
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{
			Name: "gemini",
			URL:  upstream,
			Auth: AuthHeader("x-goog-api-key"),
			Key:  "server-goog-key",
		}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer client")
	req.Header.Set("x-goog-api-key", "smuggled")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "server-goog-key", string(body))
}

func Test_AIGateway_EncodedTraversalBlocked(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedPaths: []string{"/v1/chat/*"},
	}))

	send := func(path string) int {
		req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		return resp.StatusCode
	}

	require.Equal(t, fiber.StatusOK, send("/v1/chat/completions"))
	// Encoded ".." must not slip past the allow-list.
	require.Equal(t, fiber.StatusBadRequest, send("/v1/chat/%2e%2e/%2e%2e/v1/admin"))
}

func Test_AIGateway_CaseInsensitivePrefix(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New() // default CaseSensitive: false
	app.Use("/openai", New(Config{
		PathPrefix: "/openai",
		Upstreams:  []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/OpenAI/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	// The differently-cased prefix is still stripped before relaying.
	require.Equal(t, "/v1/models", decodeEcho(t, resp).Path)
}

func Test_AIGateway_UsageParsedFromGzip(t *testing.T) {
	t.Parallel()

	var payload bytes.Buffer
	gz := gzip.NewWriter(&payload)
	_, _ = gz.Write([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":25,"total_tokens":35}}`)) //nolint:errcheck // test setup
	require.NoError(t, gz.Close())
	gzipped := payload.Bytes()

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentEncoding, "gzip")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.Send(gzipped)
	})
	upstream := "http://" + startServer(t, upstreamApp)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		OnUsage:   func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// The client still receives the gzip bytes unchanged.
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))

	require.NotNil(t, got)
	require.NotNil(t, got.Usage, "usage should be parsed from the gzip-decoded body")
	require.Equal(t, 10, got.Usage.InputTokens)
	require.Equal(t, 25, got.Usage.OutputTokens)
}

func Test_AIGateway_LoggerTagsRegistered(t *testing.T) {
	t.Parallel()

	// A logger referencing the ai-* tags must compile without panicking even
	// if it is constructed before any aigateway.New() runs; the tags are
	// pre-registered as stubs.
	require.NotPanics(t, func() {
		h := logger.New(logger.Config{
			Format: "${" + fiberlog.TagAIProvider + "} ${" + fiberlog.TagAIModel + "} ${" + fiberlog.TagAIKey + "}\n",
			Stream: io.Discard,
		})
		require.NotNil(t, h)
	})
}
