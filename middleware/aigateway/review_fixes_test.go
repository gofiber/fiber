package aigateway

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/stretchr/testify/require"
)

// go test -run Test_ConfigDefault_DoesNotMutateCallerUpstreams
// configDefault normalizes upstreams in place, so it must work on its own copy
// of the slice — otherwise New() rewrites a slice the application still holds.
func Test_ConfigDefault_DoesNotMutateCallerUpstreams(t *testing.T) {
	t.Parallel()

	ups := []Upstream{{Name: "u", URL: "https://api.example.com/", Key: "k"}}
	cfg := configDefault(Config{Upstreams: ups})

	require.Equal(t, "https://api.example.com/", ups[0].URL, "caller's URL was trimmed in place")
	require.Equal(t, AuthScheme{}, ups[0].Auth, "caller's Auth was defaulted in place")
	require.Equal(t, 0, ups[0].Weight, "caller's Weight was normalized in place")

	require.Equal(t, "https://api.example.com", cfg.Upstreams[0].URL)
	require.Equal(t, AuthBearer(), cfg.Upstreams[0].Auth)
	require.Equal(t, 1, cfg.Upstreams[0].Weight)
}

// go test -run Test_HookStatus
// A *fiber.Error built as a struct literal leaves Code at zero, which fasthttp
// renders as 200 — a policy veto must never reach the client as a success.
func Test_HookStatus(t *testing.T) {
	t.Parallel()

	require.Equal(t, fiber.StatusForbidden, hookStatus(&fiber.Error{Message: "blocked"}))
	require.Equal(t, fiber.StatusForbidden, hookStatus(&fiber.Error{Code: 0, Message: "blocked"}))
	require.Equal(t, fiber.StatusForbidden, hookStatus(&fiber.Error{Code: 99, Message: "blocked"}))
	require.Equal(t, fiber.StatusForbidden, hookStatus(&fiber.Error{Code: 600, Message: "blocked"}))
	require.Equal(t, fiber.StatusTooManyRequests, hookStatus(fiber.NewError(fiber.StatusTooManyRequests, "slow down")))
	require.Equal(t, fiber.StatusForbidden, hookStatus(errBadMaxTokens))
}

// go test -run Test_MaxTokensValue
// JSON has one number type and null means "unset" in both dialects, so the cap
// must not hard-reject spellings the upstream itself accepts.
func Test_MaxTokensValue(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		raw  string
		want int64
		ok   bool
	}{
		{"128", 128, true},
		{"null", -1, true},
		{" null ", -1, true},
		{"128.0", 128, true},
		{"1e3", 1000, true},
		{"0", 0, true},
		{"128.5", 0, false},
		{`"128"`, 0, false},
		{"true", 0, false},
		{"1e400", 0, false},
	} {
		got, ok := maxTokensValue(json.RawMessage(tc.raw))
		require.Equal(t, tc.ok, ok, "raw %q", tc.raw)
		if tc.ok {
			require.Equal(t, tc.want, got, "raw %q", tc.raw)
		}
	}
}

// go test -run Test_ApplyParamPolicy_NullMaxTokens
func Test_ApplyParamPolicy_NullMaxTokens(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	cfg := configDefault(Config{
		MaxTokensCap: 512,
		Upstreams:    []Upstream{{Name: "u", URL: "https://api.example.com", Key: "k"}},
	})

	var out []byte
	var err error
	app.Get("/", func(c fiber.Ctx) error {
		out, err = applyParamPolicy(c, &cfg, []byte(`{"model":"m","max_tokens":null,"max_completion_tokens":1024.0}`))
		return nil
	})
	_, terr := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, terr)

	require.NoError(t, err)
	require.NotNil(t, out, "the over-cap float field should have been clamped")
	var obj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &obj))
	require.JSONEq(t, "null", string(obj["max_tokens"]), "null must be left alone, not rejected")
	require.JSONEq(t, "512", string(obj["max_completion_tokens"]))
}

// go test -run Test_OrderWeighted_PreservesChainOrder
// The weighted pick is rotated to the front, so the failover tail keeps chain
// order for equal weights instead of being shuffled by a swap.
func Test_OrderWeighted_PreservesChainOrder(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Strategy: StrategyWeighted,
		Upstreams: []Upstream{
			{Name: "a", Weight: 1},
			{Name: "b", Weight: 1},
			{Name: "c", Weight: 1},
			{Name: "d", Weight: 1},
		},
	}

	orig := randIntN
	defer func() { randIntN = orig }()

	for pick, want := range map[int][]int{
		0: {0, 1, 2, 3},
		1: {1, 0, 2, 3},
		2: {2, 0, 1, 3},
		3: {3, 0, 1, 2},
	} {
		randIntN = func(int) int { return pick } // r < w picks index `pick`
		idxs := []int{0, 1, 2, 3}
		orderCandidates(cfg, idxs)
		require.Equal(t, want, idxs, "pick=%d", pick)
	}
}

// go test -run Test_AIGateway_EmptyKeyFromExtractorRejected
// An extractor is free to report "no credential" as ("", nil) instead of
// ErrNotFound. That must still be treated as unauthenticated: the validator,
// the policy resolver and quota admission all hang off a non-empty key, so an
// empty one would relay with the operator's upstream key, unmetered.
func Test_AIGateway_EmptyKeyFromExtractorRejected(t *testing.T) {
	t.Parallel()

	var validated, relayed atomic.Int32
	upstreamApp := fiber.New()
	upstreamApp.All("/*", func(c fiber.Ctx) error {
		relayed.Add(1)
		return c.SendString("ok")
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := gatewayApp(t, &Config{
		ForwardClientKey: true,
		KeyExtractor: extractors.FromCustom("", func(c fiber.Ctx) (string, error) {
			return c.Get("X-Secret"), nil // "" and a nil error when absent
		}),
		KeyValidator: func(fiber.Ctx, string) (bool, error) {
			validated.Add(1)
			return true, nil
		},
		Upstreams: []Upstream{{Name: "test", URL: upstream}},
	})

	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusUnauthorized, status)
	require.Contains(t, body, "authentication_error")
	require.Zero(t, relayed.Load(), "an empty credential must never reach the upstream")
	require.Zero(t, validated.Load(), "the validator never sees an empty key, so it cannot reject one")
}

// go test -run Test_AIGateway_StreamingQuotaAsksForUsage
// A quota is post-paid, so it can only charge what the upstream reports — and
// an OpenAI-dialect stream reports nothing unless asked. Without the injected
// stream_options, `"stream": true` would be a free tier.
func Test_AIGateway_StreamingQuotaAsksForUsage(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"injected", `{"model":"gpt-4o","stream":true}`, `"stream_options":{"include_usage":true}`},
		{"client choice kept", `{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":false}}`, `"include_usage":false`},
	} {
		app := gatewayApp(t, &Config{
			TokensPerWindow: 1000,
			Upstreams:       []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		})

		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(tc.body))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req, testConfig)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, tc.name)
		require.Contains(t, decodeEcho(t, resp).Body, tc.want, tc.name)
	}

	// No quota configured: nothing to meter, so the body is relayed verbatim.
	app := gatewayApp(t, &Config{Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}}})
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","stream":true}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.NotContains(t, decodeEcho(t, resp).Body, "stream_options")
}

// go test -run Test_AIGateway_OnResponseZeroStatusKeepsUpstream
// A hook that rebuilds RelayResponse without carrying Status forward leaves it
// at zero, which fasthttp renders as 200 — turning an upstream 429 into a
// success the client would never retry.
func Test_AIGateway_OnResponseZeroStatusKeepsUpstream(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.All("/*", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).SendString("slow down")
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{{Name: "test", URL: upstream, Key: "sk"}},
		Retry:     RetryConfig{Attempts: 1},
		OnResponse: func(_ fiber.Ctx, r *RelayResponse) error {
			*r = RelayResponse{Body: r.Body} // Status dropped
			return nil
		},
	})

	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusTooManyRequests, status)
	require.Equal(t, "slow down", body)
}

// go test -run Test_AIGateway_BreakerCountsRequestsNot429s
// The breaker exists to evict a broken upstream. A 429 means the opposite (it
// is up and throttling this key), and the retries of one request are one
// verdict — otherwise Retry.Attempts >= BreakerThreshold would let a single
// request open the breaker for every tenant on the mount.
func Test_AIGateway_BreakerCountsRequestsNot429s(t *testing.T) {
	t.Parallel()

	// A healthy fallback keeps every request served, so the only observable
	// signal is whether the first upstream was tried — an all-open chain is
	// tried anyway, which would make a single-upstream mount prove nothing.
	backupApp := fiber.New()
	backupApp.All("/*", func(c fiber.Ctx) error {
		return c.SendString("backup")
	})
	backup := "http://" + startServer(t, backupApp)

	var throttled atomic.Int32
	throttleApp := fiber.New()
	throttleApp.All("/*", func(c fiber.Ctx) error {
		throttled.Add(1)
		return c.SendStatus(fiber.StatusTooManyRequests)
	})
	throttling := "http://" + startServer(t, throttleApp)

	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{
			{Name: "busy", URL: throttling, Key: "sk"},
			{Name: "backup", URL: backup, Key: "sk"},
		},
		Retry:            RetryConfig{Attempts: 1},
		BreakerThreshold: 1,
		BreakerCooldown:  time.Minute,
	})
	for range 3 {
		status, body := doGet(t, app, "/v1/x")
		require.Equal(t, fiber.StatusOK, status)
		require.Equal(t, "backup", body)
	}
	require.EqualValues(t, 3, throttled.Load(), "429 is a throttled key, not a broken upstream")

	var down atomic.Int32
	downApp := fiber.New()
	downApp.All("/*", func(c fiber.Ctx) error {
		down.Add(1)
		return c.SendStatus(fiber.StatusServiceUnavailable)
	})
	broken := "http://" + startServer(t, downApp)

	app = gatewayApp(t, &Config{
		Upstreams: []Upstream{
			{Name: "down", URL: broken, Key: "sk"},
			{Name: "backup", URL: backup, Key: "sk"},
		},
		Retry:            RetryConfig{Attempts: 3, Backoff: time.Millisecond, MaxBackoff: time.Millisecond},
		BreakerThreshold: 2,
		BreakerCooldown:  time.Minute,
	})
	status, _ := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusOK, status)
	require.EqualValues(t, 3, down.Load(), "one request, three attempts")

	status, _ = doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusOK, status)
	require.EqualValues(t, 6, down.Load(),
		"the first request was one failure, not three — counting attempts would have opened the breaker already")

	status, _ = doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusOK, status)
	require.EqualValues(t, 6, down.Load(), "two failed requests open the breaker and the upstream goes untouched")
}

// go test -run Test_AIGateway_UsageProviderIsTheRelayedUpstream
// On exhaustion the relayed response can come from an earlier upstream than
// the last one attempted; UsageEvent.Provider must name the one whose status
// and body the client actually received.
func Test_AIGateway_UsageProviderIsTheRelayedUpstream(t *testing.T) {
	t.Parallel()

	failingApp := fiber.New()
	failingApp.All("/*", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusServiceUnavailable).SendString("primary is down")
	})
	primary := "http://" + startServer(t, failingApp)

	// Reserve a port and close it: the second upstream produces a dial error,
	// never a response, so the primary's 503 is what gets relayed.
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	dead := "http://" + ln.Addr().String()
	require.NoError(t, ln.Close())

	var ev *UsageEvent
	app := gatewayApp(t, &Config{
		Upstreams: []Upstream{
			{Name: "primary", URL: primary, Key: "sk"},
			{Name: "secondary", URL: dead, Key: "sk"},
		},
		Retry:   RetryConfig{Attempts: 1},
		OnUsage: func(e *UsageEvent) { ev = e },
	})

	status, body := doGet(t, app, "/v1/x")
	require.Equal(t, fiber.StatusServiceUnavailable, status)
	require.Equal(t, "primary is down", body)
	require.NotNil(t, ev)
	require.Equal(t, "primary", ev.Provider, "Provider must name the upstream that produced the relayed response")
}

// go test -run Test_CacheKeyGenerator_PartitionsByAccept
// Installing a KeyGenerator replaces the cache middleware's default, which is
// what keeps a gzip entry from being replayed to a client that asked for
// identity — so the generator has to carry that partitioning itself.
func Test_CacheKeyGenerator_PartitionsByAccept(t *testing.T) {
	t.Parallel()

	gen := CacheKeyGenerator()
	keyFor := func(t *testing.T, accept, encoding string) string {
		t.Helper()
		app := fiber.New()
		var key string
		app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
			key = gen(c)
			return nil
		})
		req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m"}`))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
		if accept != "" {
			req.Header.Set(fiber.HeaderAccept, accept)
		}
		if encoding != "" {
			req.Header.Set(fiber.HeaderAcceptEncoding, encoding)
		}
		_, err := app.Test(req)
		require.NoError(t, err)
		return key
	}

	base := keyFor(t, "application/json", "identity")
	require.Equal(t, base, keyFor(t, "application/json", "identity"), "the key must stay stable")
	require.NotEqual(t, base, keyFor(t, "application/json", "gzip"))
	require.NotEqual(t, base, keyFor(t, "text/event-stream", "identity"))
}

// go test -run Test_O2ATranscoder_FinishReasonWithoutDone
// llama.cpp's server, Ollama's OpenAI shim and several vLLM/Bedrock proxies
// close the stream after the finish_reason chunk without sending [DONE]. The
// message IS complete, so it must terminate normally rather than be reported
// to the client as a truncation.
func Test_O2ATranscoder_FinishReasonWithoutDone(t *testing.T) {
	t.Parallel()

	in := `data: {"id":"c1","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}` + "\n\n" +
		`data: {"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"
	out, ferr := runTranscoder(t, newO2ATranscoder("m", json.Unmarshal, json.Marshal), in, len(in))
	require.NoError(t, ferr, "a finish_reason is a complete message, [DONE] or not")
	require.Contains(t, out, `"stop_reason":"end_turn"`)
	require.Contains(t, out, "event: message_stop")
	require.NotContains(t, out, "event: error")

	// A stream that stops before any finish_reason is still a truncation.
	partial := `data: {"id":"c1","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}` + "\n\n"
	out, ferr = runTranscoder(t, newO2ATranscoder("m", json.Unmarshal, json.Marshal), partial, len(partial))
	require.ErrorIs(t, ferr, errStreamTruncated)
	require.Contains(t, out, "event: error")
}

// go test -run Test_O2ATranscoder_ToolBlockCarriesRequiredKeys
// Anthropic's ToolUseBlock schema requires id, name and input; a strict SDK
// rejects a content_block_start that omits any of them, and an empty input
// must be {} rather than dropped.
func Test_O2ATranscoder_ToolBlockCarriesRequiredKeys(t *testing.T) {
	t.Parallel()

	out, ferr := runTranscoder(t, newO2ATranscoder("m", json.Unmarshal, json.Marshal), oaiStreamFixture, 64)
	require.NoError(t, ferr)
	require.Contains(t, out, `{"type":"tool_use","id":"call_1","name":"get_weather","input":{}}`)
}

// go test -run Test_AIGateway_TranscodedStreamAbortTellsClient
// A gateway-side abort (idle timeout here) must terminate the translated
// stream in the client's dialect: an OpenAI SDK reading a dropped connection
// mid-message either hangs or raises a bare transport error.
func Test_AIGateway_TranscodedStreamAbortTellsClient(t *testing.T) {
	t.Parallel()

	const idle = 200 * time.Millisecond

	upstreamApp := fiber.New()
	upstreamApp.Post(anthropicChatPath, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/event-stream")
		return c.SendStreamWriter(func(w *bufio.Writer) {
			_, _ = w.WriteString("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\"}}\n\n") //nolint:errcheck // test upstream
			_ = w.Flush()                                                                                                       //nolint:errcheck // test upstream
			// Go quiet well past the gateway's idle timeout, but bounded: an
			// upstream blocked forever would hold Shutdown at test cleanup.
			time.Sleep(20 * idle)
		})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	usageCh := make(chan *UsageEvent, 1)
	gw := fiber.New()
	gw.Use(New(Config{
		Upstreams:         []Upstream{{Name: "claude", URL: upstream, Key: "sk", Dialect: DialectAnthropic}},
		StreamIdleTimeout: idle,
		OnUsage:           func(e *UsageEvent) { usageCh <- e },
	}))
	gwAddr := startServer(t, gw)

	req, err := http.NewRequest(http.MethodPost, "http://"+gwAddr+"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"x"}]}`))
	require.NoError(t, err)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Contains(t, string(body), `"error"`, "the client is told why the stream ended")
	require.True(t, strings.HasSuffix(string(body), "data: [DONE]\n\n"),
		"an OpenAI client needs the terminator to stop reading")

	select {
	case ev := <-usageCh:
		require.Error(t, ev.Err, "the abort reason, not the abort notice, is what the usage event reports")
	case <-time.After(10 * time.Second):
		t.Fatal("usage hook did not fire")
	}
}

// go test -run Test_TranslateRequestO2A_ToolSchemaAndParallelUse
// "parameters" is optional in OpenAI's tool shape but "input_schema" is
// required by the Messages API, and disable_parallel_tool_use only exists on
// Anthropic's auto/any/tool choices — both mistakes are a flat 400 upstream.
func Test_TranslateRequestO2A_ToolSchemaAndParallelUse(t *testing.T) {
	t.Parallel()

	body, _, err := translateRequestO2A([]byte(`{
		"model":"gpt-4o","max_tokens":16,"messages":[{"role":"user","content":"x"}],
		"parallel_tool_calls":false,
		"tools":[{"type":"function","function":{"name":"now"}}]
	}`), json.Unmarshal, json.Marshal, 0)
	require.NoError(t, err)
	require.Contains(t, string(body), `"input_schema":{"type":"object"}`)
	require.Contains(t, string(body), `"disable_parallel_tool_use":true`)

	// No tools declared: Anthropic rejects a synthesized tool_choice outright.
	body, _, err = translateRequestO2A([]byte(`{
		"model":"gpt-4o","max_tokens":16,"messages":[{"role":"user","content":"x"}],
		"parallel_tool_calls":false
	}`), json.Unmarshal, json.Marshal, 0)
	require.NoError(t, err)
	require.NotContains(t, string(body), "tool_choice")

	// tool_choice "none" has no disable_parallel_tool_use field at all.
	body, _, err = translateRequestO2A([]byte(`{
		"model":"gpt-4o","max_tokens":16,"messages":[{"role":"user","content":"x"}],
		"parallel_tool_calls":false,"tool_choice":"none",
		"tools":[{"type":"function","function":{"name":"now","parameters":{"type":"object"}}}]
	}`), json.Unmarshal, json.Marshal, 0)
	require.NoError(t, err)
	require.NotContains(t, string(body), "disable_parallel_tool_use")
}
