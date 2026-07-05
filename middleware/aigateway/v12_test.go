package aigateway

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cache"
	"github.com/stretchr/testify/require"
)

// --- Load-balancing strategies ---

func Test_AIGateway_RoundRobinRotates(t *testing.T) {
	t.Parallel()

	upA := echoUpstream(t)
	upB := echoUpstream(t)

	var providers []string
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{
			{Name: "a", URL: upA, Key: "sk"},
			{Name: "b", URL: upB, Key: "sk"},
		},
		Strategy: StrategyRoundRobin,
		OnUsage:  func(e *UsageEvent) { providers = append(providers, e.Provider) },
	}))

	for range 4 {
		req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	require.Len(t, providers, 4)
	require.NotEqual(t, providers[0], providers[1], "consecutive requests must hit different upstreams")
	require.Equal(t, providers[0], providers[2])
	require.Equal(t, providers[1], providers[3])
}

func Test_OrderCandidates(t *testing.T) {
	t.Parallel()

	cfg := &Config{Upstreams: []Upstream{
		{Name: "a", Weight: 1}, {Name: "b", Weight: 5}, {Name: "c", Weight: 2},
	}}

	// Ordered: untouched.
	idxs := []int{0, 1, 2}
	orderCandidates(cfg, idxs)
	require.Equal(t, []int{0, 1, 2}, idxs)

	// Weighted with a stubbed pick: r=0 falls in upstream 0's bucket; the
	// rest sort by descending weight.
	orig := randIntN
	t.Cleanup(func() { randIntN = orig })

	cfg.Strategy = StrategyWeighted
	randIntN = func(int) int { return 0 }
	idxs = []int{0, 1, 2}
	orderCandidates(cfg, idxs)
	require.Equal(t, []int{0, 1, 2}, idxs, "pick a (w=1), then b (w=5) before c (w=2)")

	// r=1 skips a (w=1), falls into b's bucket.
	randIntN = func(int) int { return 1 }
	idxs = []int{0, 1, 2}
	orderCandidates(cfg, idxs)
	require.Equal(t, []int{1, 2, 0}, idxs, "pick b, then c (w=2) before a (w=1)")

	// r=7 lands past a (1) and b (5), in c's bucket.
	randIntN = func(int) int { return 7 }
	idxs = []int{0, 1, 2}
	orderCandidates(cfg, idxs)
	require.Equal(t, []int{2, 1, 0}, idxs, "pick c, then b before a")
}

func Test_AIGateway_ConfigStrategyValidation(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			Upstreams: []Upstream{{Name: "a", URL: "http://127.0.0.1:1", Key: "sk"}},
			Strategy:  Strategy(42),
		})
	})
}

// --- Param enforcement ---

func Test_AIGateway_MaxTokensCapClamps(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		MaxTokensCap: 1000,
	}))

	for _, field := range maxTokenFields {
		body := fmt.Sprintf(`{"model":"gpt-4o",%q:50000,"nested":{%q:99999}}`, field, field)
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		echoed := decodeEcho(t, resp).Body
		require.Contains(t, echoed, fmt.Sprintf(`%q:1000`, field), "top-level %s must be clamped", field)
		require.Contains(t, echoed, fmt.Sprintf(`{%q:99999}`, field), "nested %s must be untouched", field)
	}
}

func Test_AIGateway_MaxTokensCapUnderCapByteFidelity(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		MaxTokensCap: 1000,
	}))

	body := "{\n  \"model\": \"gpt-4o\",\n  \"max_tokens\": 500\n}"
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, body, decodeEcho(t, resp).Body, "compliant body must relay byte-for-byte")
}

func Test_AIGateway_MaxTokensCapRejects(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:    []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		MaxTokensCap: 1000,
	}))

	// Non-integer max_tokens under a cap: a lenient upstream parser could
	// still honor it, so it is rejected.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","max_tokens":"999999"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Encoded body that cannot be inspected while a cap is set: rejected.
	req = httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader("not-really-brotli"))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderContentEncoding, "br")
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_AIGateway_ParamDefaultsInjectWhenAbsent(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		ParamDefaults: map[string]any{"temperature": 0.2, "user": "gw"},
	}))

	// temperature present: kept. user absent: injected.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","temperature":0.9}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	echoed := decodeEcho(t, resp).Body
	require.Contains(t, echoed, `"temperature":0.9`)
	require.NotContains(t, echoed, `0.2`)
	require.Contains(t, echoed, `"user":"gw"`)

	// Non-JSON bodies are untouched.
	req = httptest.NewRequest(fiber.MethodPost, "/v1/audio/transcriptions", strings.NewReader("RIFFxxxx"))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "RIFFxxxx", decodeEcho(t, resp).Body)
}

func Test_AIGateway_ParamDefaultsModelPanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			Upstreams:     []Upstream{{Name: "a", URL: "http://127.0.0.1:1", Key: "sk"}},
			ParamDefaults: map[string]any{"model": "gpt-4o"},
		})
	})
}

func Test_AIGateway_ParamPolicyThenModelMap(t *testing.T) {
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
		MaxTokensCap: 100,
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","max_tokens":5000}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	echoed := decodeEcho(t, resp).Body
	require.Contains(t, echoed, `"max_tokens":100`, "cap applies")
	require.Contains(t, echoed, `"my-deployment"`, "ModelMap applies on top of the capped body")
}

// --- OnRequest / OnResponse hooks ---

func Test_AIGateway_OnRequestMutatesBodyAndPath(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o"},
		OnRequest: func(_ fiber.Ctx, r *RelayRequest) error {
			r.Path = "/v1/chat/completions"
			r.Body = []byte(`{"model":"gpt-4o","rewritten":true}`)
			return nil
		},
	}))

	// Original body carries a disallowed model; the hook replaces it before
	// policy runs, so the request passes and the upstream sees the new body
	// on the new path.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/other", strings.NewReader(`{"model":"forbidden"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	echoed := decodeEcho(t, resp)
	require.Equal(t, "/v1/chat/completions", echoed.Path)
	require.Contains(t, echoed.Body, `"rewritten":true`)
}

func Test_AIGateway_OnRequestOutputIsPoliced(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:     []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		AllowedModels: []string{"gpt-4o"},
		OnRequest: func(_ fiber.Ctx, r *RelayRequest) error {
			r.Body = []byte(`{"model":"smuggled"}`)
			return nil
		},
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode, "the hook's output is what gets policed")
}

func Test_AIGateway_OnRequestErrorStatus(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		OnRequest: func(c fiber.Ctx, _ *RelayRequest) error {
			if c.Get("x-teapot") != "" {
				return fiber.NewError(fiber.StatusTeapot, "short and stout")
			}
			return errors.New("content policy violation")
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode, "plain errors map to 403")

	req = httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set("x-teapot", "1")
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode, "*fiber.Error chooses its status")
}

func Test_AIGateway_OnResponseMutatesBufferedResponse(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"secret": "internal", "usage": fiber.Map{"prompt_tokens": 1, "completion_tokens": 2}})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	var got *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		OnResponse: func(_ fiber.Ctx, r *RelayResponse) error {
			r.Body = []byte(`{"redacted":true}`)
			r.Status = fiber.StatusAccepted
			return nil
		},
		OnUsage: func(e *UsageEvent) { got = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusAccepted, resp.StatusCode)
	require.JSONEq(t, `{"redacted":true}`, string(readBody(t, resp)))

	require.NotNil(t, got)
	require.Equal(t, fiber.StatusOK, got.StatusCode, "usage keeps the upstream status")
	require.NotNil(t, got.Usage, "usage was parsed before the hook rewrote the body")
	require.Equal(t, int64(len(`{"redacted":true}`)), got.ResponseBytes)
}

func Test_AIGateway_OnResponseErrorAndStreamingSkip(t *testing.T) {
	t.Parallel()

	// Buffered: hook error turns the response into a 502.
	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:  []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		OnResponse: func(_ fiber.Ctx, _ *RelayResponse) error { return errors.New("nope") },
	}))
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadGateway, resp.StatusCode)

	// Streaming: the hook is never invoked; the stream relays pass-through.
	sse := sseUpstream(t, 3, time.Millisecond)
	called := false
	gw := fiber.New()
	gw.Use(New(Config{
		Upstreams:  []Upstream{{Name: "sse", URL: sse, Key: "sk"}},
		OnResponse: func(_ fiber.Ctx, _ *RelayResponse) error { called = true; return errors.New("nope") },
	}))
	gwAddr := startServer(t, gw)

	sreq, err := http.NewRequest(http.MethodPost, "http://"+gwAddr+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	require.NoError(t, err)
	sreq.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	sreq.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	sresp, err := testHTTPClient.Do(sreq)
	require.NoError(t, err)
	body, err := io.ReadAll(sresp.Body)
	require.NoError(t, err)
	require.NoError(t, sresp.Body.Close())
	require.Contains(t, string(body), "[DONE]")
	require.False(t, called, "OnResponse must not run for streaming responses")
}

// --- Quotas ---

// usageUpstream returns 200 with a fixed token usage per request.
func usageUpstream(t *testing.T, total int) string {
	t.Helper()
	app := fiber.New()
	app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"usage": fiber.Map{"prompt_tokens": total / 2, "completion_tokens": total - total/2, "total_tokens": total}})
	})
	return "http://" + startServer(t, app)
}

func quotaReq(t *testing.T, app *fiber.App, key string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+key)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	return resp
}

func Test_AIGateway_QuotaTokensExhausted(t *testing.T) {
	t.Parallel()

	upstream := usageUpstream(t, 150)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:       []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		TokensPerWindow: 100,
	}))

	// First request: window is empty, admitted; commits 150 tokens.
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-1").StatusCode)

	// Second: 150 >= 100 → 429 with Retry-After and an OpenAI-style error.
	resp := quotaReq(t, app, "vk-1")
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
	require.Contains(t, string(readBody(t, resp)), "rate_limit_error")

	// A different key has its own window.
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-2").StatusCode)
}

func Test_AIGateway_QuotaBudgetAndPolicyOverrides(t *testing.T) {
	t.Parallel()

	upstream := usageUpstream(t, 1_000_000) // 1M tokens → $1 at the price below

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:       []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		Prices:          map[string]ModelPrice{"gpt-4o": {InputPerMTok: 1, OutputPerMTok: 1}},
		BudgetPerWindow: 0.5,
		PolicyResolver: func(_ fiber.Ctx, key string) (*KeyPolicy, error) {
			switch key {
			case "vk-vip":
				return &KeyPolicy{BudgetPerWindow: -1, TokensPerWindow: -1}, nil // exempt
			case "vk-big":
				return &KeyPolicy{BudgetPerWindow: 10}, nil // own budget
			default:
				return &KeyPolicy{}, nil // inherit the global $0.50
			}
		},
	}))

	// Default key: first spends $1, second is over the $0.50 budget.
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-default").StatusCode)
	require.Equal(t, fiber.StatusTooManyRequests, quotaReq(t, app, "vk-default").StatusCode)

	// Exempt key: never limited.
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-vip").StatusCode)
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-vip").StatusCode)

	// Bigger per-key budget: $1 spent < $10.
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-big").StatusCode)
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-big").StatusCode)
}

func Test_AIGateway_QuotaTenantSharedAcrossKeys(t *testing.T) {
	t.Parallel()

	upstream := usageUpstream(t, 80)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:       []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		TokensPerWindow: 100,
		PolicyResolver: func(_ fiber.Ctx, _ string) (*KeyPolicy, error) {
			return &KeyPolicy{Tenant: "acme"}, nil
		},
	}))

	// Two different keys, one tenant: the second request sees the first
	// key's 80 tokens... still under 100, so admitted (total 160), and the
	// third is rejected for either key.
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-a").StatusCode)
	require.Equal(t, fiber.StatusOK, quotaReq(t, app, "vk-b").StatusCode)
	require.Equal(t, fiber.StatusTooManyRequests, quotaReq(t, app, "vk-a").StatusCode)
	require.Equal(t, fiber.StatusTooManyRequests, quotaReq(t, app, "vk-b").StatusCode)
}

// errQuotaStore always fails, to prove admission fails closed.
type errQuotaStore struct{}

//nolint:gocritic // matches the QuotaStore interface
func (errQuotaStore) Peek(string, time.Duration) (int64, float64, error) {
	return 0, 0, errors.New("store down")
}

//nolint:gocritic // matches the QuotaStore interface
func (errQuotaStore) Add(string, time.Duration, int64, float64) (int64, float64, error) {
	return 0, 0, errors.New("store down")
}

func Test_AIGateway_QuotaStoreErrorFailsClosed(t *testing.T) {
	t.Parallel()

	upstream := usageUpstream(t, 1)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams:       []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		TokensPerWindow: 100,
		QuotaStore:      errQuotaStore{},
	}))

	resp := quotaReq(t, app, "vk-1")
	require.Equal(t, fiber.StatusBadGateway, resp.StatusCode)
}

func Test_AIGateway_QuotaStreamingCommit(t *testing.T) {
	t.Parallel()

	// sseUpstream's final usage chunk reports total_tokens: 12.
	sse := sseUpstream(t, 2, time.Millisecond)

	usageCh := make(chan *UsageEvent, 1)
	gw := fiber.New()
	gw.Use(New(Config{
		Upstreams:       []Upstream{{Name: "sse", URL: sse, Key: "sk"}},
		TokensPerWindow: 10,
		OnUsage:         func(e *UsageEvent) { usageCh <- e },
	}))
	gwAddr := startServer(t, gw)

	sreq, err := http.NewRequest(http.MethodPost, "http://"+gwAddr+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":true}`))
	require.NoError(t, err)
	sreq.Header.Set(fiber.HeaderAuthorization, "Bearer vk-s")
	sreq.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	sresp, err := testHTTPClient.Do(sreq)
	require.NoError(t, err)
	_, err = io.ReadAll(sresp.Body)
	require.NoError(t, err)
	require.NoError(t, sresp.Body.Close())

	// Wait for the stream's usage hook: the quota commit happens just before
	// it on the same goroutine.
	select {
	case ev := <-usageCh:
		require.NotNil(t, ev.Usage)
		require.Equal(t, 12, ev.Usage.TotalTokens)
	case <-time.After(10 * time.Second):
		t.Fatal("usage hook did not fire")
	}

	// The committed 12 tokens exceed the 10-token window: next request 429.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer vk-s")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := gw.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

func Test_MemoryQuotaStore(t *testing.T) {
	t.Parallel()

	s := newMemoryQuotaStore()
	window := time.Hour

	tokens, cost, err := s.Peek("a", window)
	require.NoError(t, err)
	require.Zero(t, tokens)
	require.Zero(t, cost)

	tokens, cost, err = s.Add("a", window, 10, 0.5)
	require.NoError(t, err)
	require.Equal(t, int64(10), tokens)
	require.InEpsilon(t, 0.5, cost, 1e-12)

	tokens, _, err = s.Add("a", window, 5, 0)
	require.NoError(t, err)
	require.Equal(t, int64(15), tokens)

	// Concurrency: hammer one identity, expect an exact total.
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_, _, _ = s.Add("conc", window, 1, 0.01) //nolint:errcheck // total asserted via Peek below
		})
	}
	wg.Wait()
	tokens, cost, err = s.Peek("conc", window)
	require.NoError(t, err)
	require.Equal(t, int64(50), tokens)
	require.InEpsilon(t, 0.5, cost, 1e-9)
}

// --- Cache middleware integration ---

func Test_AIGateway_CacheKeyGeneratorVariance(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	keyGen := CacheKeyGenerator()
	var keys []string
	app.Post("/*", func(c fiber.Ctx) error {
		keys = append(keys, keyGen(c))
		return c.SendString("ok")
	})

	send := func(path, body, auth string) {
		req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader(body))
		if auth != "" {
			req.Header.Set(fiber.HeaderAuthorization, "Bearer "+auth)
		}
		_, err := app.Test(req, testConfig)
		require.NoError(t, err)
	}

	send("/v1/chat", `{"a":1}`, "k1")
	send("/v1/chat", `{"a":1}`, "k1")     // identical
	send("/v1/chat", `{"a":2}`, "k1")     // different body
	send("/v1/other", `{"a":1}`, "k1")    // different path
	send("/v1/chat?x=1", `{"a":1}`, "k1") // different query
	send("/v1/chat", `{"a":1}`, "k2")     // different credential

	require.Len(t, keys, 6)
	require.Equal(t, keys[0], keys[1], "identical requests share a key")
	unique := map[string]struct{}{}
	for _, k := range keys {
		unique[k] = struct{}{}
	}
	require.Len(t, unique, 5, "body, path, query, and credential must all partition the key")
}

func Test_AIGateway_CacheRecipeEndToEnd(t *testing.T) {
	t.Parallel()

	var upstreamHits int
	upstreamApp := fiber.New()
	upstreamApp.Post("/v1/embeddings", func(c fiber.Ctx) error {
		upstreamHits++
		return c.JSON(fiber.Map{"data": "embedding", "usage": fiber.Map{"prompt_tokens": 5, "total_tokens": 5}})
	})
	upstreamApp.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		upstreamHits++
		c.Set(fiber.HeaderContentType, "text/event-stream")
		return c.SendStreamWriter(func(w *bufio.Writer) {
			fmt.Fprint(w, "data: {}\n\ndata: [DONE]\n\n")
			_ = w.Flush() //nolint:errcheck // test upstream
		})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := fiber.New()
	// The documented recipe: cache in front of the gateway, POST enabled,
	// body+credential keys, streaming responses never stored. Clients use
	// x-api-key (an Authorization header suppresses heuristic caching).
	app.Use(cache.New(cache.Config{
		Methods:      []string{fiber.MethodPost},
		KeyGenerator: CacheKeyGenerator(),
		Next:         CacheSkipStreaming(),
	}))
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
	}))

	embed := func(key string) *http.Response {
		req := httptest.NewRequest(fiber.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"m","input":"hi"}`))
		req.Header.Set("x-api-key", key)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		return resp
	}

	embed("k1")
	require.Equal(t, 1, upstreamHits)
	embed("k1") // identical: served from cache
	require.Equal(t, 1, upstreamHits, "identical request must be a cache hit")
	embed("k2") // different credential: no cross-key hit
	require.Equal(t, 2, upstreamHits)

	// Streaming completions are never cached.
	for range 2 {
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","stream":true}`))
		req.Header.Set("x-api-key", "k1")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}
	require.Equal(t, 4, upstreamHits, "streaming requests must never be served from cache")
}
