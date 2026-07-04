package aigateway

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// --- Upstream.ModelMap ---

func Test_AIGateway_ModelMapRewritesModel(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{
			Name:     "azure",
			URL:      upstream,
			Key:      "sk",
			ModelMap: map[string]string{"gpt-4o": "my-deployment"},
		}},
	}))

	// The nested "model", the float, and the huge integer must survive the
	// rewrite byte-for-byte: only the top-level model may change.
	body := `{"model":"gpt-4o","temperature":0.7,"metadata":{"model":"keep-me"},"seed":9007199254740993}`
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	echoed := decodeEcho(t, resp)
	require.Contains(t, echoed.Body, `"model":"my-deployment"`)
	require.NotContains(t, echoed.Body, `"gpt-4o"`)
	require.Contains(t, echoed.Body, `{"model":"keep-me"}`)
	require.Contains(t, echoed.Body, `9007199254740993`, "int beyond float64 precision must be preserved")
	require.Contains(t, echoed.Body, `0.7`)
}

func Test_AIGateway_ModelMapUnmappedModelRelaysOriginalBytes(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{
			Name:     "azure",
			URL:      upstream,
			Key:      "sk",
			ModelMap: map[string]string{"gpt-4o": "my-deployment"},
		}},
	}))

	// No mapping for this model: the body must relay untouched, whitespace
	// and key order included.
	body := "{\n  \"model\": \"o3-mini\",\n  \"b\": 1,\n  \"a\": 2\n}"
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, body, decodeEcho(t, resp).Body)
}

func Test_AIGateway_ModelMapPerUpstreamOnFallback(t *testing.T) {
	t.Parallel()

	failing := fiber.New()
	failing.All("/*", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	})
	primary := "http://" + startServer(t, failing)
	secondary := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{
			{Name: "primary", URL: primary, Key: "sk1", ModelMap: map[string]string{"gpt-4o": "primary-name"}},
			{Name: "secondary", URL: secondary, Key: "sk2", ModelMap: map[string]string{"gpt-4o": "secondary-name"}},
		},
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, decodeEcho(t, resp).Body, `"secondary-name"`, "the serving upstream's own mapping must apply")
}

func Test_AIGateway_ModelMapEncodedBodyRelayedDecoded(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"enc":  c.Get(fiber.HeaderContentEncoding),
			"body": string(c.BodyRaw()),
		})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{
			Name:     "azure",
			URL:      upstream,
			Key:      "sk",
			ModelMap: map[string]string{"gpt-4o": "my-deployment"},
		}},
	}))

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = gz.Write([]byte(`{"model":"gpt-4o","x":1}`)) //nolint:errcheck // test setup
	require.NoError(t, gz.Close())

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", bytes.NewReader(compressed.Bytes()))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderContentEncoding, "gzip")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var echoed struct {
		Enc  string `json:"enc"`
		Body string `json:"body"`
	}
	require.NoError(t, json.Unmarshal(readBody(t, resp), &echoed))
	require.Empty(t, echoed.Enc, "rewritten body is identity-encoded, Content-Encoding must be dropped")
	require.Contains(t, echoed.Body, `"my-deployment"`)
	require.Contains(t, echoed.Body, `"x":1`)
}

func Test_AIGateway_ModelMapKeepsRequestedModelInUsage(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{
			Name:     "azure",
			URL:      upstream,
			Key:      "sk",
			ModelMap: map[string]string{"gpt-4o": "my-deployment"},
		}},
		OnUsage: func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer client")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.NotNil(t, got)
	require.Equal(t, "gpt-4o", got.Model, "usage reports the requested model, not the rewritten one")
}

// --- PolicyResolver ---

func Test_AIGateway_PolicyResolverRejectsNilPolicy(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		PolicyResolver: func(_ fiber.Ctx, key string) (*KeyPolicy, error) {
			if key == "vk-good" {
				return &KeyPolicy{}, nil
			}
			return nil, nil //nolint:nilnil // nil policy means "unknown key" by contract
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer vk-bad")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	req = httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer vk-good")
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_AIGateway_PolicyModelsTightenGlobalList(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-*"},
		PolicyResolver: func(_ fiber.Ctx, _ string) (*KeyPolicy, error) {
			return &KeyPolicy{AllowedModels: []string{"gpt-4o"}}, nil
		},
	}))

	send := func(model string) int {
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"`+model+`"}`))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer vk")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		return resp.StatusCode
	}

	require.Equal(t, fiber.StatusOK, send("gpt-4o"))
	// Passes the global wildcard but not the per-key list.
	require.Equal(t, fiber.StatusForbidden, send("gpt-4o-mini"))
	// Fails the global list before the per-key list is consulted.
	require.Equal(t, fiber.StatusForbidden, send("o3"))
}

func Test_AIGateway_PolicyModelListAloneFailsClosedOnEncodedBody(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		PolicyResolver: func(_ fiber.Ctx, _ string) (*KeyPolicy, error) {
			return &KeyPolicy{AllowedModels: []string{"gpt-4o"}}, nil
		},
	}))

	// An undecodable "encoded" body: with only the per-key model list set,
	// the unverifiable-body rejection must still apply.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader("not-gzip"))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer vk")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderContentEncoding, "br")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_AIGateway_PolicyPathsAndTenant(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		PolicyResolver: func(_ fiber.Ctx, _ string) (*KeyPolicy, error) {
			return &KeyPolicy{Tenant: "acme", AllowedPaths: []string{"/v1/chat/*"}}, nil
		},
		OnUsage: func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer vk")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	req = httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer vk")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.NotNil(t, got)
	require.Equal(t, "acme", got.Tenant)
}

// --- Cost calculation ---

func Test_AIGateway_CostFromPrices(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"usage": fiber.Map{"prompt_tokens": 1000, "completion_tokens": 2000, "total_tokens": 3000},
		})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		Prices: map[string]ModelPrice{
			"gpt-4o": {InputPerMTok: 2.5, OutputPerMTok: 10},
		},
		OnUsage: func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NotNil(t, got)
	require.NotNil(t, got.Usage)
	// 1000 in @ $2.5/M + 2000 out @ $10/M
	require.InEpsilon(t, 1000*2.5/1e6+2000*10.0/1e6, got.Cost, 1e-12)
}

func Test_AIGateway_CostZeroWithoutPriceEntry(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"usage": fiber.Map{"prompt_tokens": 10, "completion_tokens": 5}})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		Prices:    map[string]ModelPrice{"claude-*": {InputPerMTok: 3, OutputPerMTok: 15}},
		OnUsage:   func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	_, err := app.Test(req, testConfig)
	require.NoError(t, err)

	require.NotNil(t, got)
	require.Zero(t, got.Cost)
}

func Test_LookupPrice(t *testing.T) {
	t.Parallel()

	prices := map[string]ModelPrice{
		"gpt-4o":  {InputPerMTok: 1, OutputPerMTok: 1},
		"gpt-4o*": {InputPerMTok: 2, OutputPerMTok: 2},
		"gpt-*":   {InputPerMTok: 3, OutputPerMTok: 3},
	}

	p, ok := lookupPrice(prices, "gpt-4o")
	require.True(t, ok)
	require.InDelta(t, 1.0, p.InputPerMTok, 0, "exact match wins over wildcards")

	p, ok = lookupPrice(prices, "gpt-4o-mini")
	require.True(t, ok)
	require.InDelta(t, 2.0, p.InputPerMTok, 0, "longest wildcard wins")

	p, ok = lookupPrice(prices, "gpt-3.5-turbo")
	require.True(t, ok)
	require.InDelta(t, 3.0, p.InputPerMTok, 0)

	_, ok = lookupPrice(prices, "claude-sonnet-5")
	require.False(t, ok)
}

// --- Circuit breaker ---

func Test_AIGateway_BreakerSkipsOpenUpstream(t *testing.T) {
	t.Parallel()

	var primaryHits int
	failingApp := fiber.New()
	failingApp.All("/*", func(c fiber.Ctx) error {
		primaryHits++
		return c.SendStatus(fiber.StatusServiceUnavailable)
	})
	primary := "http://" + startServer(t, failingApp)
	secondary := echoUpstream(t)

	var events []*UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{
			{Name: "primary", URL: primary, Key: "sk1"},
			{Name: "secondary", URL: secondary, Key: "sk2"},
		},
		BreakerThreshold: 1,
		BreakerCooldown:  time.Minute,
		OnUsage:          func(e *UsageEvent) { events = append(events, e) },
	}))

	send := func() {
		req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	// First request: primary fails (opening its breaker), secondary serves.
	send()
	require.Equal(t, 1, primaryHits)

	// Second request: primary's breaker is open, so it is skipped entirely.
	send()
	require.Equal(t, 1, primaryHits, "open upstream must not be tried during cooldown")
	require.Len(t, events, 2)
	require.Nil(t, events[0].SkippedUpstreams)
	require.Equal(t, []string{"primary"}, events[1].SkippedUpstreams)
	require.Equal(t, 1, events[1].Attempts, "only the secondary was attempted")
}

func Test_AIGateway_BreakerAllOpenStillTries(t *testing.T) {
	t.Parallel()

	var hits int
	failingApp := fiber.New()
	failingApp.All("/*", func(c fiber.Ctx) error {
		hits++
		return c.SendStatus(fiber.StatusServiceUnavailable)
	})
	upstream := "http://" + startServer(t, failingApp)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:        []Upstream{{Name: "only", URL: upstream, Key: "sk"}},
		BreakerThreshold: 1,
		BreakerCooldown:  time.Minute,
	}))

	send := func() {
		req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		// Exhaustion relays the upstream's 503 verbatim.
		require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	}

	send()
	require.Equal(t, 1, hits)
	// The only upstream's breaker is open, but an all-open chain is tried
	// anyway instead of failing without an attempt.
	send()
	require.Equal(t, 2, hits)
}

func Test_UpstreamBreaker(t *testing.T) {
	t.Parallel()

	b := &upstreamBreaker{}
	now := time.Now()

	require.False(t, b.open(now))

	// Below threshold: stays closed.
	b.recordFailure(3, time.Minute)
	b.recordFailure(3, time.Minute)
	require.False(t, b.open(now))

	// Threshold reached: opens for the cooldown.
	b.recordFailure(3, time.Minute)
	require.True(t, b.open(now))
	require.False(t, b.open(now.Add(2*time.Minute)), "expired deadline means half-open (probe allowed)")

	// A single failure while half-open reopens immediately (count is still
	// past the threshold).
	b.recordFailure(3, time.Minute)
	require.True(t, b.open(now))

	// Success closes it and resets the count.
	b.recordSuccess()
	require.False(t, b.open(now))
	b.recordFailure(3, time.Minute)
	require.False(t, b.open(now), "count restarts after success")
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close() //nolint:errcheck // test helper
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}
