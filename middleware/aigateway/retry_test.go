package aigateway

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_IsRetryableStatus(t *testing.T) {
	t.Parallel()

	for _, status := range []int{429, 500, 502, 503, 504} {
		require.True(t, isRetryableStatus(status), status)
	}
	for _, status := range []int{200, 201, 400, 401, 403, 404, 418, 422} {
		require.False(t, isRetryableStatus(status), status)
	}
}

func gatewayApp(t *testing.T, cfg *Config) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Use(New(*cfg))
	return app
}

func doGet(t *testing.T, app *fiber.App, path string) (status int, body string) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming the status and body results for readability
	t.Helper()

	req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func Test_AIGateway_RetrySameUpstream(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	upstreamApp := fiber.New()
	upstreamApp.Get("/v1/flaky", func(c fiber.Ctx) error {
		if calls.Add(1) == 1 {
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		return c.SendString("ok")
	})
	upstream := "http://" + startServer(t, upstreamApp)

	var got *UsageEvent
	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{{Name: "flaky", URL: upstream, Key: "sk"}},
		Retry:     RetryConfig{Attempts: 2, Backoff: 10 * time.Millisecond, MaxBackoff: 50 * time.Millisecond},
		OnUsage:   func(e *UsageEvent) { got = e },
	})

	status, body := doGet(t, app, "/v1/flaky")
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, "ok", body)
	require.EqualValues(t, 2, calls.Load())
	require.NotNil(t, got)
	require.Equal(t, 2, got.Attempts)
}

func Test_AIGateway_FallbackToSecondary(t *testing.T) {
	t.Parallel()

	primaryApp := fiber.New()
	primaryApp.Get("/v1/x", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	})
	primary := "http://" + startServer(t, primaryApp)

	secondaryApp := fiber.New()
	secondaryApp.Get("/v1/x", func(c fiber.Ctx) error {
		return c.SendString("from-secondary")
	})
	secondary := "http://" + startServer(t, secondaryApp)

	var got *UsageEvent
	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{
			{Name: "primary", URL: primary, Key: "sk1"},
			{Name: "secondary", URL: secondary, Key: "sk2"},
		},
		OnUsage: func(e *UsageEvent) { got = e },
	})

	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, "from-secondary", body)
	require.NotNil(t, got)
	require.Equal(t, "secondary", got.Provider)
	require.Equal(t, 2, got.Attempts)
}

func Test_AIGateway_DialErrorFailover(t *testing.T) {
	t.Parallel()

	// Reserve a port and close it so the primary dial fails.
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	deadAddr := ln.Addr().String()
	require.NoError(t, ln.Close())

	secondaryApp := fiber.New()
	secondaryApp.Get("/v1/x", func(c fiber.Ctx) error {
		return c.SendString("alive")
	})
	secondary := "http://" + startServer(t, secondaryApp)

	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{
			{Name: "dead", URL: "http://" + deadAddr, Key: "sk1"},
			{Name: "alive", URL: secondary, Key: "sk2"},
		},
	})

	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, "alive", body)
}

func Test_AIGateway_ExhaustionRelaysLastResponse(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Get("/v1/x", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusServiceUnavailable).SendString(`{"error":{"message":"overloaded"}}`)
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{{Name: "only", URL: upstream, Key: "sk"}},
	})

	// The provider's own error relays verbatim once all attempts fail.
	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusServiceUnavailable, status)
	require.JSONEq(t, `{"error":{"message":"overloaded"}}`, body)
}

func Test_AIGateway_AllUpstreamsUnreachable(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	deadAddr := ln.Addr().String()
	require.NoError(t, ln.Close())

	var got *UsageEvent
	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{{Name: "dead", URL: "http://" + deadAddr, Key: "sk"}},
		OnUsage:   func(e *UsageEvent) { got = e },
	})

	status, _ := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusBadGateway, status)
	require.NotNil(t, got)
	require.Error(t, got.Err)
	require.Zero(t, got.StatusCode)
}

func Test_AIGateway_BadRequestNotRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	upstreamApp := fiber.New()
	upstreamApp.Get("/v1/x", func(c fiber.Ctx) error {
		calls.Add(1)
		return c.Status(fiber.StatusBadRequest).SendString("bad request")
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		Retry:     RetryConfig{Attempts: 3, Backoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond},
	})

	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusBadRequest, status)
	require.Equal(t, "bad request", body)
	require.EqualValues(t, 1, calls.Load())
}

func Test_AIGateway_RetryAfterAboveCapSkipsWait(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	upstreamApp := fiber.New()
	upstreamApp.Get("/v1/x", func(c fiber.Ctx) error {
		if calls.Add(1) == 1 {
			c.Set(fiber.HeaderRetryAfter, "30") // way above MaxBackoff
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		return c.SendString("ok")
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		Retry:     RetryConfig{Attempts: 2, Backoff: 10 * time.Millisecond, MaxBackoff: 100 * time.Millisecond},
	})

	start := time.Now()
	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, "ok", body)
	require.EqualValues(t, 2, calls.Load())
	// A 30s Retry-After above the cap must not be honored.
	require.Less(t, time.Since(start), 5*time.Second)
}

func Test_RetryAfterParsing(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Get("/seconds", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderRetryAfter, "7")
		return c.SendStatus(fiber.StatusTooManyRequests)
	})
	upstreamApp.Get("/date", func(c fiber.Ctx) error {
		// HTTP-date is RFC1123 with a GMT zone (net/http.TimeFormat).
		c.Set(fiber.HeaderRetryAfter, time.Now().Add(9*time.Second).UTC().Format(http.TimeFormat))
		return c.SendStatus(fiber.StatusTooManyRequests)
	})
	upstreamApp.Get("/garbage", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderRetryAfter, "soon")
		return c.SendStatus(fiber.StatusTooManyRequests)
	})
	upstreamApp.Get("/none", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	})
	upstream := "http://" + startServer(t, upstreamApp)

	cfg := configDefault(Config{
		Upstreams: []Upstream{{Name: "t", URL: upstream, Key: "sk"}},
	})

	fetch := func(path string) (time.Duration, bool) {
		req := cfg.Client.R()
		req.SetURL(upstream + path)
		resp, err := req.Send()
		require.NoError(t, err)
		defer resp.Close()
		// Drain the streamed body so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.BodyStream()) //nolint:errcheck // drain only
		return retryAfter(resp)
	}

	d, ok := fetch("/seconds")
	require.True(t, ok)
	require.Equal(t, 7*time.Second, d)

	d, ok = fetch("/date")
	require.True(t, ok)
	require.InDelta(t, (9 * time.Second).Seconds(), d.Seconds(), 2)

	_, ok = fetch("/garbage")
	require.False(t, ok)

	_, ok = fetch("/none")
	require.False(t, ok)
}
