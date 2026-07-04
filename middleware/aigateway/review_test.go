package aigateway

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
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

func Test_AIGateway_DoubleEncodedTraversalBlocked(t *testing.T) {
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

	// Single- and double-encoded ".." must both be rejected.
	require.Equal(t, fiber.StatusBadRequest, send("/v1/chat/%2e%2e/admin"))
	require.Equal(t, fiber.StatusBadRequest, send("/v1/chat/%252e%252e/admin"))
}

func Test_AIGateway_ModelSpoofViaContentType(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o*"},
	}))

	// A JSON body declaring a disallowed model must be blocked even when the
	// Content-Type lies (text/plain), because sniffing keys off the body shape.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-3.5-turbo"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	// A genuinely non-JSON body (audio upload) is not model-restricted.
	req = httptest.NewRequest(fiber.MethodPost, "/v1/audio/transcriptions", strings.NewReader("RIFF....binary"))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEOctetStream)
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_AIGateway_QueryCredentialStripped(t *testing.T) {
	t.Parallel()

	// Upstream echoes the query string it received.
	app := fiber.New()
	app.All("/*", func(c fiber.Ctx) error {
		return c.SendString(string(c.RequestCtx().URI().QueryString()))
	})
	upstream := "http://" + startServer(t, app)

	gw := fiber.New()
	gw.Use(New(Config{
		KeyExtractor: extractors.FromQuery("api_key"),
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk-server"}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models?api_key=client-secret&keep=1", http.NoBody)
	resp, err := gw.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	got := string(body)
	require.NotContains(t, got, "client-secret", "query credential must not be relayed upstream")
	require.Contains(t, got, "keep=1", "non-credential query params are preserved")
}

func Test_AIGateway_CookieCredentialStripped(t *testing.T) {
	t.Parallel()

	upstream := headerEchoUpstream(t, fiber.HeaderCookie)
	gw := fiber.New()
	gw.Use(New(Config{
		KeyExtractor: extractors.FromCookie("session"),
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk-server"}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderCookie, "session=client-secret; other=keep")
	resp, err := gw.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	got := string(body)
	require.NotContains(t, got, "client-secret", "cookie credential must not be relayed upstream")
	require.Contains(t, got, "other=keep", "non-credential cookies are preserved")
}

func Test_AIGateway_DecompressionBombBounded(t *testing.T) {
	t.Parallel()

	// A tiny gzip body that expands far beyond the cap must not be fully
	// decompressed; usage parsing gives up (nil) rather than allocating it all.
	var payload bytes.Buffer
	gz := gzip.NewWriter(&payload)
	zeros := make([]byte, 4<<20) // 4 MiB of zeros -> tiny gzip
	_, _ = gz.Write(zeros)       //nolint:errcheck // test setup
	require.NoError(t, gz.Close())
	bomb := payload.Bytes()
	require.Less(t, len(bomb), 64<<10, "compressed bomb should be small")

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentEncoding, "gzip")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.Send(bomb)
	})
	upstream := "http://" + startServer(t, upstreamApp)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:       []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		MaxResponseSize: 1 << 20, // 1 MiB cap; decode bounded to it
		OnUsage:         func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	// The small compressed body is under MaxResponseSize, so it relays fine;
	// only the usage-parse decompression is bounded and yields nil.
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.NotNil(t, got)
	require.Nil(t, got.Usage, "bounded decode should not parse a bomb")
}

func Test_AIGateway_ModelSpoofViaBOM(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o*"},
	}))

	// A UTF-8 BOM before the JSON must not hide the model from the allow-list.
	body := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"model":"gpt-3.5-turbo"}`)...)
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_AIGateway_BackslashTraversalBlocked(t *testing.T) {
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

	// Backslash-based traversal must be rejected (some upstreams treat \ as /).
	require.Equal(t, fiber.StatusBadRequest, send(`/v1/chat/..\..\admin`))
	require.Equal(t, fiber.StatusBadRequest, send(`/v1/chat/%2e%2e\admin`))
}

func Test_AIGateway_FormExtractorUnifiedPanics(t *testing.T) {
	t.Parallel()

	// A form/param/custom extractor in unified-key mode cannot be stripped and
	// would leak the client credential upstream, so construction must panic.
	require.Panics(t, func() {
		New(Config{
			KeyExtractor: extractors.FromForm("api_key"),
			Upstreams:    []Upstream{{Name: "test", URL: "https://api.example.com", Key: "sk"}},
		})
	})
	// In pass-through mode the client credential is meant to go upstream, so
	// the same extractor is allowed.
	require.NotPanics(t, func() {
		New(Config{
			ForwardClientKey: true,
			KeyExtractor:     extractors.FromForm("api_key"),
			Upstreams:        []Upstream{{Name: "test", URL: "https://api.example.com"}},
		})
	})
}

func Test_AIGateway_UsageParsedFromDeflate(t *testing.T) {
	t.Parallel()

	// zlib-wrapped deflate (the conventional Content-Encoding: deflate) must
	// still decode for usage parsing.
	var payload bytes.Buffer
	zw := zlib.NewWriter(&payload)
	_, _ = zw.Write([]byte(`{"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)) //nolint:errcheck // test setup
	require.NoError(t, zw.Close())
	deflated := payload.Bytes()

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentEncoding, "deflate")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.Send(deflated)
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
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.NotNil(t, got)
	require.NotNil(t, got.Usage, "zlib-wrapped deflate usage should parse")
	require.Equal(t, 3, got.Usage.InputTokens)
	require.Equal(t, 4, got.Usage.OutputTokens)
}

func Test_AIGateway_ModelSpoofViaWhitespaceBOM(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o*"},
	}))

	send := func(body []byte) int {
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		return resp.StatusCode
	}

	bom := []byte{0xEF, 0xBB, 0xBF}
	json := []byte(`{"model":"gpt-3.5-turbo"}`)
	// Whitespace-before-BOM and double-BOM must not hide the model.
	require.Equal(t, fiber.StatusForbidden, send(append([]byte("  "), append(bom, json...)...)))
	require.Equal(t, fiber.StatusForbidden, send(append(append([]byte{}, bom...), append(bom, json...)...)))
}

func Test_AIGateway_ModelCheckedThroughGzipRequest(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o*"},
	}))

	gzipBody := func(s string) []byte {
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		_, _ = gz.Write([]byte(s)) //nolint:errcheck // test setup
		require.NoError(t, gz.Close())
		return b.Bytes()
	}
	send := func(model string) int {
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", bytes.NewReader(gzipBody(`{"model":"`+model+`"}`)))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set(fiber.HeaderContentEncoding, "gzip")
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		return resp.StatusCode
	}

	// A gzipped request body must still be checked against the allow-list.
	require.Equal(t, fiber.StatusForbidden, send("gpt-3.5-turbo"))
	require.Equal(t, fiber.StatusOK, send("gpt-4o"))
}

func Test_AIGateway_GzipBombRequestBounded(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)
	app := fiber.New(fiber.Config{BodyLimit: 64 << 10})
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o*"},
	}))

	// A tiny gzip body that expands past the 1 MiB sniff cap must not be fully
	// decompressed; the model can't be determined so the request is not
	// model-restricted (and, crucially, the gateway does not OOM decoding it).
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, _ = gz.Write(make([]byte, 4<<20)) //nolint:errcheck // test setup
	require.NoError(t, gz.Close())

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", bytes.NewReader(b.Bytes()))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderContentEncoding, "gzip")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_AIGateway_EmptyKeyCustomExtractorPanics(t *testing.T) {
	t.Parallel()

	// A custom extractor (even with an empty key) is unstrippable, so unified
	// mode must reject it at construction rather than leak the credential.
	require.Panics(t, func() {
		New(Config{
			KeyExtractor: extractors.FromCustom("", func(c fiber.Ctx) (string, error) {
				return c.Get("X-Secret"), nil
			}),
			Upstreams: []Upstream{{Name: "test", URL: "https://api.example.com", Key: "sk"}},
		})
	})
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
