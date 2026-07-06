package aigateway

import (
	"bufio"
	"bytes"
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

// ---- dialect ----

func Test_ChatDialectForPath(t *testing.T) {
	t.Parallel()

	require.Equal(t, DialectOpenAI, chatDialectForPath("/v1/chat/completions"))
	require.Equal(t, DialectAnthropic, chatDialectForPath("/v1/messages"))
	require.Equal(t, DialectUnspecified, chatDialectForPath("/v1/models"))
	require.Equal(t, DialectUnspecified, chatDialectForPath("/v1/embeddings"))

	require.False(t, needsTranslation(DialectOpenAI, DialectUnspecified))
	require.False(t, needsTranslation(DialectUnspecified, DialectAnthropic))
	require.False(t, needsTranslation(DialectOpenAI, DialectOpenAI))
	require.True(t, needsTranslation(DialectOpenAI, DialectAnthropic))
	require.True(t, needsTranslation(DialectAnthropic, DialectOpenAI))
}

func Test_AIGateway_DialectValidationAndPresets(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{Upstreams: []Upstream{{Name: "a", URL: "http://127.0.0.1:1", Key: "k", Dialect: Dialect(9)}}})
	})

	require.Equal(t, DialectOpenAI, OpenAI("k").Dialect)
	require.Equal(t, DialectAnthropic, Anthropic("k").Dialect)
	require.Equal(t, DialectOpenAI, OpenRouter("k").Dialect)
	require.Equal(t, DialectOpenAI, AzureOpenAI("https://r.openai.azure.com", "k").Dialect)
}

// ---- request codecs ----

func Test_TranslateRequest_OpenAIToAnthropic(t *testing.T) {
	t.Parallel()

	in := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "Be terse."},
			{"role": "user", "content": [
				{"type": "text", "text": "What is in this image?"},
				{"type": "image_url", "image_url": {"url": "data:image/png;base64,aGVsbG8="}}
			]},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_1", "type": "function", "function": {"name": "look", "arguments": "{\"zoom\":2}"}}
			]},
			{"role": "tool", "tool_call_id": "call_1", "content": "a cat"},
			{"role": "user", "content": "thanks"}
		],
		"tools": [{"type": "function", "function": {"name": "look", "description": "look closer", "parameters": {"type": "object"}}}],
		"tool_choice": "required",
		"stop": ["END"],
		"temperature": 1.6,
		"max_tokens": 100000,
		"stream": true,
		"stream_options": {"include_usage": true},
		"user": "u-42",
		"seed": 7,
		"frequency_penalty": 0.5
	}`)

	body, opts, err := translateRequest(DialectOpenAI, DialectAnthropic, in, json.Unmarshal, json.Marshal, 4096)
	require.NoError(t, err)
	require.True(t, opts.stream)
	require.True(t, opts.includeUsage)

	var out antRequest
	require.NoError(t, json.Unmarshal(body, &out))
	require.Equal(t, "gpt-4o", out.Model)
	require.True(t, out.Stream)
	require.JSONEq(t, `"Be terse."`, string(out.System))
	require.Equal(t, []string{"END"}, out.StopSequences)
	require.InEpsilon(t, 1.0, *out.Temperature, 1e-9, "temperature must clamp to 1")
	require.Equal(t, 4096, out.MaxTokens, "MaxTokensCap caps the client's max_tokens")
	require.Equal(t, "u-42", out.Metadata.UserID)
	require.Len(t, out.Tools, 1)
	require.Equal(t, "look", out.Tools[0].Name)
	require.Equal(t, "any", out.ToolChoice.Type)

	// Dropped params must not appear anywhere in the translated body.
	require.NotContains(t, string(body), "seed")
	require.NotContains(t, string(body), "frequency_penalty")
	require.NotContains(t, string(body), "stream_options")

	require.Len(t, out.Messages, 4)
	// user with text + image
	var blocks []antBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &blocks))
	require.Equal(t, "text", blocks[0].Type)
	require.Equal(t, "image", blocks[1].Type)
	require.Equal(t, "base64", blocks[1].Source.Type)
	require.Equal(t, "image/png", blocks[1].Source.MediaType)
	require.Equal(t, "aGVsbG8=", blocks[1].Source.Data)
	// assistant tool call: arguments string became an input object
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &blocks))
	require.Equal(t, "tool_use", blocks[0].Type)
	require.Equal(t, "call_1", blocks[0].ID)
	require.JSONEq(t, `{"zoom":2}`, string(blocks[0].Input))
	// tool message became a user message of tool_result blocks
	require.NoError(t, json.Unmarshal(out.Messages[2].Content, &blocks))
	require.Equal(t, "user", out.Messages[2].Role)
	require.Equal(t, "tool_result", blocks[0].Type)
	require.Equal(t, "call_1", blocks[0].ToolUseID)
}

func Test_TranslateRequest_OpenAIToAnthropic_Defaults(t *testing.T) {
	t.Parallel()

	// No max_tokens: the required Anthropic field is injected.
	body, opts, err := translateRequest(DialectOpenAI, DialectAnthropic,
		[]byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
		json.Unmarshal, json.Marshal, 0)
	require.NoError(t, err)
	require.False(t, opts.stream)
	var out antRequest
	require.NoError(t, json.Unmarshal(body, &out))
	require.Equal(t, translateDefaultMaxTokens, out.MaxTokens)
}

func Test_TranslateRequest_Untranslatable(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"n>1":        `{"model":"m","n":2,"messages":[{"role":"user","content":"x"}]}`,
		"audio":      `{"model":"m","modalities":["text","audio"],"messages":[{"role":"user","content":"x"}]}`,
		"servertool": `{"model":"m","tools":[{"type":"web_search"}],"messages":[{"role":"user","content":"x"}]}`,
		"badargs":    `{"model":"m","messages":[{"role":"assistant","tool_calls":[{"id":"c","type":"function","function":{"name":"f","arguments":"not-json"}}]},{"role":"user","content":"x"}]}`,
	}
	for name, body := range cases {
		_, _, err := translateRequest(DialectOpenAI, DialectAnthropic, []byte(body), json.Unmarshal, json.Marshal, 0)
		require.ErrorIs(t, err, errUntranslatable, name)
	}

	_, _, err := translateRequest(DialectOpenAI, DialectAnthropic, nil, json.Unmarshal, json.Marshal, 0)
	require.ErrorIs(t, err, errUntranslatable, "nil body")
}

func Test_TranslateRequest_AnthropicToOpenAI(t *testing.T) {
	t.Parallel()

	in := []byte(`{
		"model": "claude-sonnet-5",
		"max_tokens": 1024,
		"system": "Be helpful.",
		"messages": [
			{"role": "user", "content": [
				{"type": "text", "text": "look at this"},
				{"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "Zm9v"}}
			]},
			{"role": "assistant", "content": [
				{"type": "text", "text": "checking"},
				{"type": "tool_use", "id": "toolu_1", "name": "look", "input": {"zoom": 3}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_1", "content": "a dog"},
				{"type": "text", "text": "and?"}
			]}
		],
		"tools": [{"name": "look", "description": "look", "input_schema": {"type": "object"}}],
		"tool_choice": {"type": "tool", "name": "look", "disable_parallel_tool_use": true},
		"stop_sequences": ["STOP"],
		"top_k": 40,
		"stream": true,
		"metadata": {"user_id": "acct-1"}
	}`)

	body, opts, err := translateRequest(DialectAnthropic, DialectOpenAI, in, json.Unmarshal, json.Marshal, 0)
	require.NoError(t, err)
	require.True(t, opts.stream)

	var out oaiChatRequest
	require.NoError(t, json.Unmarshal(body, &out))
	require.Equal(t, "claude-sonnet-5", out.Model)
	require.Equal(t, 1024, *out.MaxTokens)
	require.Equal(t, "acct-1", out.User)
	require.NotNil(t, out.StreamOptions)
	require.True(t, out.StreamOptions.IncludeUsage, "include_usage is injected for the transcoder")
	require.NotContains(t, string(body), "top_k")
	require.JSONEq(t, `["STOP"]`, string(out.Stop))
	require.False(t, *out.ParallelToolCalls)
	require.JSONEq(t, `{"type":"function","function":{"name":"look"}}`, string(out.ToolChoice))

	require.Equal(t, "system", out.Messages[0].Role)
	require.JSONEq(t, `"Be helpful."`, string(out.Messages[0].Content))

	// user w/ image → parts array with a data: URL
	var parts []oaiContentPart
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &parts))
	require.Equal(t, "image_url", parts[1].Type)
	require.Equal(t, "data:image/jpeg;base64,Zm9v", parts[1].ImageURL.URL)

	// assistant tool_use → tool_calls with a JSON-string arguments
	require.Equal(t, "assistant", out.Messages[2].Role)
	require.Len(t, out.Messages[2].ToolCalls, 1)
	require.JSONEq(t, `{"zoom":3}`, out.Messages[2].ToolCalls[0].Function.Arguments)

	// tool_result → its own role:tool message, then the trailing user text
	require.Equal(t, "tool", out.Messages[3].Role)
	require.Equal(t, "toolu_1", out.Messages[3].ToolCallID)
	require.JSONEq(t, `"a dog"`, string(out.Messages[3].Content))
	require.Equal(t, "user", out.Messages[4].Role)
}

// ---- response codecs ----

func Test_TranslateResponse_Buffered(t *testing.T) {
	t.Parallel()

	// Anthropic -> OpenAI
	ant := []byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5",
		"content":[{"type":"text","text":"Hi "},{"type":"text","text":"there"},
			{"type":"tool_use","id":"toolu_9","name":"look","input":{"a":1}}],
		"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":20}}`)
	out, err := translateResponseBody(DialectAnthropic, ant, "gpt-4o", 1700000000, json.Unmarshal, json.Marshal)
	require.NoError(t, err)
	var oai oaiChatResponse
	require.NoError(t, json.Unmarshal(out, &oai))
	require.Equal(t, "chat.completion", oai.Object)
	require.Equal(t, "gpt-4o", oai.Model, "echoes the client-requested model")
	require.Equal(t, "tool_calls", oai.Choices[0].FinishReason)
	require.Equal(t, "Hi there", *oai.Choices[0].Message.Content)
	require.JSONEq(t, `{"a":1}`, oai.Choices[0].Message.ToolCalls[0].Function.Arguments)
	require.Equal(t, 10, oai.Usage.PromptTokens)
	require.Equal(t, 20, oai.Usage.CompletionTokens)
	require.Equal(t, 30, oai.Usage.TotalTokens)

	// OpenAI -> Anthropic
	oaiBody := []byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o",
		"choices":[{"index":0,"message":{"role":"assistant","content":"Hello","tool_calls":[
			{"id":"call_2","type":"function","function":{"name":"f","arguments":"{\"b\":2}"}}]},
			"finish_reason":"length"}],
		"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`)
	out, err = translateResponseBody(DialectOpenAI, oaiBody, "claude-sonnet-5", 0, json.Unmarshal, json.Marshal)
	require.NoError(t, err)
	var ares antResponse
	require.NoError(t, json.Unmarshal(out, &ares))
	require.Equal(t, "message", ares.Type)
	require.Equal(t, "msg_chatcmpl-1", ares.ID)
	require.Equal(t, "max_tokens", ares.StopReason)
	require.Equal(t, "text", ares.Content[0].Type)
	require.Equal(t, "Hello", ares.Content[0].Text)
	require.Equal(t, "tool_use", ares.Content[1].Type)
	require.JSONEq(t, `{"b":2}`, string(ares.Content[1].Input))
	require.Equal(t, 5, ares.Usage.InputTokens)
	require.Equal(t, 7, ares.Usage.OutputTokens)
}

func Test_TranslateErrorBody(t *testing.T) {
	t.Parallel()

	// Anthropic error -> OpenAI shape
	out := translateErrorBody(DialectAnthropic,
		[]byte(`{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`),
		json.Unmarshal, json.Marshal)
	require.JSONEq(t, `{"error":{"message":"busy","type":"overloaded_error"}}`, string(out))

	// OpenAI error -> Anthropic shape
	out = translateErrorBody(DialectOpenAI,
		[]byte(`{"error":{"message":"bad key","type":"invalid_api_key","code":"invalid_api_key"}}`),
		json.Unmarshal, json.Marshal)
	require.JSONEq(t, `{"type":"error","error":{"type":"invalid_api_key","message":"bad key"}}`, string(out))

	// Unparseable body -> synthesized envelope carrying the raw text
	out = translateErrorBody(DialectAnthropic, []byte("upstream exploded"), json.Unmarshal, json.Marshal)
	require.JSONEq(t, `{"error":{"message":"upstream exploded","type":"api_error"}}`, string(out))
}

// ---- stream transcoders ----

const antStreamFixture = "event: message_start\n" +
	"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_abc\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-5\",\"content\":[],\"usage\":{\"input_tokens\":25,\"output_tokens\":1}}}\n\n" +
	"event: content_block_start\n" +
	"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
	"event: ping\n" +
	"data: {\"type\":\"ping\"}\n\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n" +
	"event: content_block_stop\n" +
	"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
	"event: message_delta\n" +
	"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":12}}\n\n" +
	"event: message_stop\n" +
	"data: {\"type\":\"message_stop\"}\n\n"

// runTranscoder feeds input to tc in chunks of n bytes and returns the output.
func runTranscoder(t *testing.T, tc streamTranscoder, input string, n int) string {
	t.Helper()
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	data := []byte(input)
	for len(data) > 0 {
		end := min(n, len(data))
		require.NoError(t, tc.feed(w, data[:end]))
		data = data[end:]
	}
	require.NoError(t, tc.finish(w))
	require.NoError(t, w.Flush())
	return buf.String()
}

func Test_A2OTranscoder(t *testing.T) {
	t.Parallel()

	dec, enc := json.Unmarshal, json.Marshal
	whole := runTranscoder(t, newA2OTranscoder("gpt-4o", 1700000000, true, dec, enc), antStreamFixture, len(antStreamFixture))

	// Identical output at every split granularity.
	for _, n := range []int{4096, 7, 1} {
		got := runTranscoder(t, newA2OTranscoder("gpt-4o", 1700000000, true, dec, enc), antStreamFixture, n)
		require.Equal(t, whole, got, "split size %d must not change the output", n)
	}
	// CRLF variant too.
	crlf := strings.ReplaceAll(antStreamFixture, "\n", "\r\n")
	require.Equal(t, whole, runTranscoder(t, newA2OTranscoder("gpt-4o", 1700000000, true, dec, enc), crlf, 5))

	require.Contains(t, whole, `"chat.completion.chunk"`)
	require.Contains(t, whole, `"id":"chatcmpl-abc"`)
	require.Contains(t, whole, `"model":"gpt-4o"`)
	require.Contains(t, whole, `"content":"Hello"`)
	require.Contains(t, whole, `"content":" world"`)
	require.Contains(t, whole, `"finish_reason":"stop"`)
	require.Contains(t, whole, `"prompt_tokens":25`)
	require.Contains(t, whole, `"completion_tokens":12`)
	require.True(t, strings.HasSuffix(whole, "data: [DONE]\n\n"))
	require.NotContains(t, whole, "ping")
	require.NotContains(t, whole, "message_start")

	// Usage reported for the quota/cost pipeline.
	tc := newA2OTranscoder("gpt-4o", 1700000000, false, dec, enc)
	out := runTranscoder(t, tc, antStreamFixture, 64)
	require.NotContains(t, out, `"usage"`, "no trailing usage chunk without include_usage")
	u := tc.usage()
	require.NotNil(t, u)
	require.Equal(t, 25, u.InputTokens)
	require.Equal(t, 12, u.OutputTokens)
}

const oaiStreamFixture = `data: {"id":"chatcmpl-9x","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}` + "\n\n" +
	`data: {"id":"chatcmpl-9x","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}` + "\n\n" +
	`data: {"id":"chatcmpl-9x","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}` + "\n\n" +
	`data: {"id":"chatcmpl-9x","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Paris\"}"}}]},"finish_reason":null}]}` + "\n\n" +
	`data: {"id":"chatcmpl-9x","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n" +
	`data: {"id":"chatcmpl-9x","choices":[],"usage":{"prompt_tokens":30,"completion_tokens":15,"total_tokens":45}}` + "\n\n" +
	"data: [DONE]\n\n"

func Test_O2ATranscoder(t *testing.T) {
	t.Parallel()

	dec, enc := json.Unmarshal, json.Marshal
	whole := runTranscoder(t, newO2ATranscoder("claude-sonnet-5", dec, enc), oaiStreamFixture, len(oaiStreamFixture))
	for _, n := range []int{4096, 7, 1} {
		got := runTranscoder(t, newO2ATranscoder("claude-sonnet-5", dec, enc), oaiStreamFixture, n)
		require.Equal(t, whole, got, "split size %d must not change the output", n)
	}

	// Assert the exact event sequence by re-scanning the output.
	var events []string
	var scan sseScanner
	require.NoError(t, scan.feed([]byte(whole), func(ev *sseEvent) error {
		events = append(events, ev.name)
		return nil
	}))
	require.Equal(t, []string{
		"message_start",
		"content_block_start", "content_block_delta", "content_block_stop", // text "Hi"
		"content_block_start", "content_block_delta", "content_block_stop", // tool call
		"message_delta", "message_stop",
	}, events)

	require.Contains(t, whole, `"id":"msg_9x"`)
	require.Contains(t, whole, `"model":"claude-sonnet-5"`)
	require.Contains(t, whole, `"text_delta"`)
	require.Contains(t, whole, `"partial_json":"{\"city\":\"Paris\"}"`)
	require.Contains(t, whole, `"stop_reason":"tool_use"`)
	require.Contains(t, whole, `"input_tokens":30`)
	require.Contains(t, whole, `"output_tokens":15`)

	tc := newO2ATranscoder("claude-sonnet-5", dec, enc)
	_ = runTranscoder(t, tc, oaiStreamFixture, 32)
	u := tc.usage()
	require.NotNil(t, u)
	require.Equal(t, 45, u.TotalTokens)
}

func Test_O2ATranscoder_EOFWithoutDone(t *testing.T) {
	t.Parallel()

	// A stream that ends without [DONE] must still terminate with
	// message_delta + message_stop so Anthropic SDKs don't hang.
	partial := `data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"hey"},"finish_reason":null}]}` + "\n\n"
	out := runTranscoder(t, newO2ATranscoder("m", json.Unmarshal, json.Marshal), partial, 16)
	require.Contains(t, out, "event: message_delta\n")
	require.Contains(t, out, "event: message_stop\n")
}

func Test_SSEScannerOversizedEvent(t *testing.T) {
	t.Parallel()

	tc := newA2OTranscoder("m", 0, false, json.Unmarshal, json.Marshal)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	huge := append([]byte("data: "), bytes.Repeat([]byte("x"), sseMaxEventBytes+2)...)
	require.ErrorIs(t, tc.feed(w, huge), errSSEEventTooLarge)
}

// ---- integration ----

// fakeAnthropicUpstream records what it receives and answers a fixed
// Messages response.
func fakeAnthropicUpstream(t *testing.T, got map[string]string) string {
	t.Helper()
	app := fiber.New()
	app.All("/*", func(c fiber.Ctx) error {
		got["path"] = c.Path()
		got["body"] = string(c.BodyRaw())
		got["x-api-key"] = c.Get("x-api-key")
		got["anthropic-version"] = c.Get(headerAnthropicVersion)
		got["accept-encoding"] = c.Get(fiber.HeaderAcceptEncoding)
		return c.JSON(fiber.Map{
			"id": "msg_up", "type": "message", "role": "assistant", "model": "claude-sonnet-5",
			"content":     []fiber.Map{{"type": "text", "text": "bonjour"}},
			"stop_reason": "end_turn",
			"usage":       fiber.Map{"input_tokens": 9, "output_tokens": 4},
		})
	})
	return "http://" + startServer(t, app)
}

func Test_AIGateway_TranslateOpenAIClientToAnthropicUpstream(t *testing.T) {
	t.Parallel()

	got := map[string]string{}
	upstream := fakeAnthropicUpstream(t, got)

	var ev *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "claude", URL: upstream, Key: "sk-ant", Auth: AuthHeader("x-api-key"), Dialect: DialectAnthropic}},
		OnUsage:   func(e *UsageEvent) { ev = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"salut"}]}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer client-key")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Upstream saw a translated Messages request.
	require.Equal(t, anthropicChatPath, got["path"])
	require.Equal(t, "sk-ant", got["x-api-key"])
	require.Equal(t, defaultAnthropicVersion, got["anthropic-version"], "mandatory version header is filled")
	require.Equal(t, "identity", got["accept-encoding"])
	require.Contains(t, got["body"], `"max_tokens"`)
	require.NotContains(t, got["body"], `"messages":[{"role":"user","content":"salut"}]`, "body must be re-shaped")

	// Client got an OpenAI chat.completion.
	var out oaiChatResponse
	require.NoError(t, json.Unmarshal(readBody(t, resp), &out))
	require.Equal(t, "chat.completion", out.Object)
	require.Equal(t, "gpt-4o", out.Model)
	require.Equal(t, "bonjour", *out.Choices[0].Message.Content)
	require.Equal(t, "stop", out.Choices[0].FinishReason)
	require.Equal(t, 9, out.Usage.PromptTokens)

	// Usage flowed into the event (parsed from the upstream-dialect body).
	require.NotNil(t, ev)
	require.NotNil(t, ev.Usage)
	require.Equal(t, 13, ev.Usage.TotalTokens)
	require.Equal(t, "gpt-4o", ev.Model)
}

func Test_AIGateway_TranslateAnthropicClientToOpenAIUpstream(t *testing.T) {
	t.Parallel()

	var gotPath, gotBody string
	upstreamApp := fiber.New()
	upstreamApp.All("/*", func(c fiber.Ctx) error {
		gotPath = c.Path()
		gotBody = string(c.BodyRaw())
		return c.JSON(fiber.Map{
			"id": "chatcmpl-7", "object": "chat.completion", "created": 1, "model": "gpt-4o",
			"choices": []fiber.Map{{"index": 0, "message": fiber.Map{"role": "assistant", "content": "hello"}, "finish_reason": "stop"}},
			"usage":   fiber.Map{"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5},
		})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "oai", URL: upstream, Key: "sk-oai", Dialect: DialectOpenAI}},
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"claude-sonnet-5","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", "client-key")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.Equal(t, openAIChatPath, gotPath)
	require.Contains(t, gotBody, `"max_tokens":50`)

	var out antResponse
	require.NoError(t, json.Unmarshal(readBody(t, resp), &out))
	require.Equal(t, "message", out.Type)
	require.Equal(t, "claude-sonnet-5", out.Model)
	require.Equal(t, "hello", out.Content[0].Text)
	require.Equal(t, "end_turn", out.StopReason)
	require.Equal(t, 3, out.Usage.InputTokens)
}

func Test_AIGateway_TranslateStreamingBothDirections(t *testing.T) {
	t.Parallel()

	// OpenAI client <- Anthropic SSE upstream.
	antApp := fiber.New()
	antApp.Post(anthropicChatPath, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/event-stream")
		return c.SendStreamWriter(func(w *bufio.Writer) {
			_, _ = w.WriteString(antStreamFixture) //nolint:errcheck // test upstream
			_ = w.Flush()                          //nolint:errcheck // test upstream
		})
	})
	antURL := "http://" + startServer(t, antApp)

	usageCh := make(chan *UsageEvent, 1)
	gw := fiber.New()
	gw.Use(New(Config{
		Upstreams: []Upstream{{Name: "claude", URL: antURL, Key: "sk", Auth: AuthHeader("x-api-key"), Dialect: DialectAnthropic}},
		OnUsage:   func(e *UsageEvent) { usageCh <- e },
	}))
	gwAddr := startServer(t, gw)

	req, err := http.NewRequest(http.MethodPost, "http://"+gwAddr+"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`))
	require.NoError(t, err)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := string(body)
	require.Contains(t, out, `"chat.completion.chunk"`)
	require.Contains(t, out, `"content":"Hello"`)
	require.Contains(t, out, `"finish_reason":"stop"`)
	require.Contains(t, out, `"total_tokens":37`)
	require.True(t, strings.HasSuffix(out, "data: [DONE]\n\n"))
	require.NotContains(t, out, "message_start")

	select {
	case ev := <-usageCh:
		require.True(t, ev.Streamed)
		require.NotNil(t, ev.Usage, "transcoder usage reaches the event")
		require.Equal(t, 37, ev.Usage.TotalTokens)
	case <-time.After(10 * time.Second):
		t.Fatal("usage hook did not fire")
	}

	// Anthropic client <- OpenAI SSE upstream.
	var gotUpBody string
	oaiApp := fiber.New()
	oaiApp.Post(openAIChatPath, func(c fiber.Ctx) error {
		gotUpBody = string(c.BodyRaw())
		c.Set(fiber.HeaderContentType, "text/event-stream")
		return c.SendStreamWriter(func(w *bufio.Writer) {
			_, _ = w.WriteString(oaiStreamFixture) //nolint:errcheck // test upstream
			_ = w.Flush()                          //nolint:errcheck // test upstream
		})
	})
	oaiURL := "http://" + startServer(t, oaiApp)

	gw2 := fiber.New()
	gw2.Use(New(Config{
		Upstreams: []Upstream{{Name: "oai", URL: oaiURL, Key: "sk", Dialect: DialectOpenAI}},
	}))
	gw2Addr := startServer(t, gw2)

	req2, err := http.NewRequest(http.MethodPost, "http://"+gw2Addr+"/v1/messages",
		strings.NewReader(`{"model":"claude-sonnet-5","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	require.NoError(t, err)
	req2.Header.Set("x-api-key", "k")
	req2.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp2, err := testHTTPClient.Do(req2)
	require.NoError(t, err)
	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	require.NoError(t, resp2.Body.Close())

	out2 := string(body2)
	require.Contains(t, gotUpBody, `"include_usage":true`, "usage chunk is requested on the client's behalf")
	require.Contains(t, out2, "event: message_start\n")
	require.Contains(t, out2, `"partial_json"`)
	require.Contains(t, out2, `"stop_reason":"tool_use"`)
	require.Contains(t, out2, "event: message_stop\n")
	require.NotContains(t, out2, "[DONE]")
}

func Test_AIGateway_GatewayErrorsAreDialectShaped(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "x", URL: "http://127.0.0.1:1", Key: "sk"}},
	}))

	// Anthropic-dialect path: Anthropic error envelope.
	req := httptest.NewRequest(fiber.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	require.Contains(t, string(readBody(t, resp)), `"type":"error"`)

	// OpenAI-dialect path: OpenAI error shape.
	req = httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err = app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	body := string(readBody(t, resp))
	require.Contains(t, body, `"error"`)
	require.NotContains(t, body, `"type":"error"`)
}

func Test_AIGateway_UntranslatableFallsBackOr400(t *testing.T) {
	t.Parallel()

	got := map[string]string{}
	antURL := fakeAnthropicUpstream(t, got)
	echo := echoUpstream(t)

	// Only a cross-dialect upstream: n>1 cannot be expressed -> 400.
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "claude", URL: antURL, Key: "sk", Dialect: DialectAnthropic}},
	}))
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","n":2,"messages":[{"role":"user","content":"x"}]}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// With a same-dialect fallback the request is served verbatim.
	app2 := fiber.New()
	app2.Use(New(Config{
		Upstreams: []Upstream{
			{Name: "claude", URL: antURL, Key: "sk", Dialect: DialectAnthropic},
			{Name: "oai", URL: echo, Key: "sk2", Dialect: DialectOpenAI},
		},
	}))
	req = httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","n":2,"messages":[{"role":"user","content":"x"}]}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err = app2.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, decodeEcho(t, resp).Body, `"n":2`, "same-dialect fallback relays the original bytes")
}

func Test_AIGateway_TranslateWithModelMap(t *testing.T) {
	t.Parallel()

	got := map[string]string{}
	antURL := fakeAnthropicUpstream(t, got)

	var ev *UsageEvent
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{
			Name: "claude", URL: antURL, Key: "sk", Dialect: DialectAnthropic,
			ModelMap: map[string]string{"gpt-4o": "claude-sonnet-5"},
		}},
		OnUsage: func(e *UsageEvent) { ev = e },
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"x"}]}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.Contains(t, got["body"], `"model":"claude-sonnet-5"`, "ModelMap applies to the translated body")
	require.NotNil(t, ev)
	require.Equal(t, "gpt-4o", ev.Model, "usage reports the client-requested model")
}

func Test_AIGateway_TranslateUpstreamErrorBody(t *testing.T) {
	t.Parallel()

	upstreamApp := fiber.New()
	upstreamApp.Post(anthropicChatPath, func(c fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"type":  "error",
			"error": fiber.Map{"type": "invalid_request_error", "message": "bad request"},
		})
	})
	upstream := "http://" + startServer(t, upstreamApp)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "claude", URL: upstream, Key: "sk", Dialect: DialectAnthropic}},
	}))

	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"x"}]}`))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "the upstream status relays unchanged")
	require.JSONEq(t, `{"error":{"message":"bad request","type":"invalid_request_error"}}`, string(readBody(t, resp)))
}

func Test_AIGateway_NonChatPathsPassThroughWithDialects(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "claude", URL: upstream, Key: "sk", Dialect: DialectAnthropic}},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/models", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "/v1/models", decodeEcho(t, resp).Path, "non-chat paths relay untranslated")
}

func Test_AIGateway_ChatPassThroughSameDialect(t *testing.T) {
	t.Parallel()

	upstream := echoUpstream(t)

	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "oai", URL: upstream, Key: "sk", Dialect: DialectOpenAI}},
	}))

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"seed":7}`
	req := httptest.NewRequest(fiber.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, testConfig)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, body, decodeEcho(t, resp).Body, "same-dialect chat relays byte-for-byte")
}
