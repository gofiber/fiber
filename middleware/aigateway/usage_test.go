package aigateway

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ParseUsage_OpenAI(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"cmpl-1","usage":{"prompt_tokens":10,"completion_tokens":25,"total_tokens":35}}`)
	u := parseUsage(body, json.Unmarshal)
	require.NotNil(t, u)
	require.Equal(t, 10, u.InputTokens)
	require.Equal(t, 25, u.OutputTokens)
	require.Equal(t, 35, u.TotalTokens)
}

func Test_ParseUsage_Anthropic(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"msg-1","usage":{"input_tokens":12,"output_tokens":34}}`)
	u := parseUsage(body, json.Unmarshal)
	require.NotNil(t, u)
	require.Equal(t, 12, u.InputTokens)
	require.Equal(t, 34, u.OutputTokens)
	require.Equal(t, 46, u.TotalTokens)
}

func Test_ParseUsage_Unparseable(t *testing.T) {
	t.Parallel()

	require.Nil(t, parseUsage([]byte(`{"id":"cmpl-1"}`), json.Unmarshal))
	require.Nil(t, parseUsage([]byte(`{"usage":null}`), json.Unmarshal))
	require.Nil(t, parseUsage([]byte(`{"usage":{}}`), json.Unmarshal))
	require.Nil(t, parseUsage([]byte(`not json`), json.Unmarshal))
	require.Nil(t, parseUsage(nil, json.Unmarshal))
}

func Test_UsageTail_OpenAIFinalChunk(t *testing.T) {
	t.Parallel()

	tail := &usageTail{}
	// OpenAI with stream_options.include_usage: intermediate chunks carry
	// "usage":null, the final pre-[DONE] chunk carries the real counts.
	tail.observe([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}],\"usage\":null}\n\n"))
	tail.observe([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"!\"}}],\"usage\":null}\n\n"))
	tail.observe([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":2,\"total_tokens\":11}}\n\n"))
	tail.observe([]byte("data: [DONE]\n\n"))

	u := tail.usage(json.Unmarshal)
	require.NotNil(t, u)
	require.Equal(t, 9, u.InputTokens)
	require.Equal(t, 2, u.OutputTokens)
	require.Equal(t, 11, u.TotalTokens)
}

func Test_UsageTail_AnthropicStartAndDelta(t *testing.T) {
	t.Parallel()

	tail := &usageTail{}
	// Anthropic: input tokens nest in message_start, output tokens sit at
	// the top level of the final message_delta.
	tail.observe([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-1\",\"usage\":{\"input_tokens\":25,\"output_tokens\":1}}}\n\n"))
	tail.observe([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"Hello\"}}\n\n"))
	tail.observe([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":17}}\n\n"))

	u := tail.usage(json.Unmarshal)
	require.NotNil(t, u)
	require.Equal(t, 25, u.InputTokens)
	require.Equal(t, 17, u.OutputTokens)
	require.Equal(t, 42, u.TotalTokens)
}

func Test_UsageTail_SplitAcrossChunks(t *testing.T) {
	t.Parallel()

	tail := &usageTail{}
	line := "data: {\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":7,\"total_tokens\":12}}\n"
	// Feed the line byte by byte to exercise the carry buffer.
	for i := range len(line) {
		tail.observe([]byte{line[i]})
	}

	u := tail.usage(json.Unmarshal)
	require.NotNil(t, u)
	require.Equal(t, 5, u.InputTokens)
	require.Equal(t, 7, u.OutputTokens)
}

func Test_UsageTail_NoUsage(t *testing.T) {
	t.Parallel()

	tail := &usageTail{}
	tail.observe([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
	tail.observe([]byte("data: [DONE]\n\n"))
	require.Nil(t, tail.usage(json.Unmarshal))
}

func Test_UsageTail_OversizedLineDiscarded(t *testing.T) {
	t.Parallel()

	tail := &usageTail{}
	// A line exceeding the cap is discarded without growing memory, and
	// scanning resumes on the next line.
	huge := make([]byte, usageTailMaxLine+10)
	for i := range huge {
		huge[i] = 'a'
	}
	tail.observe(huge)
	tail.observe([]byte("...tail of huge line\n"))
	tail.observe([]byte("data: {\"usage\":{\"total_tokens\":3,\"prompt_tokens\":1,\"completion_tokens\":2}}\n"))

	u := tail.usage(json.Unmarshal)
	require.NotNil(t, u)
	require.Equal(t, 3, u.TotalTokens)
}

func Test_UsageTail_IgnoresNonDataLines(t *testing.T) {
	t.Parallel()

	tail := &usageTail{}
	tail.observe([]byte("event: {\"usage\":{\"total_tokens\":9}}\n"))
	tail.observe([]byte(": comment mentioning \"usage\"\n"))
	tail.observe([]byte("data: [DONE] \"usage\"\n"))
	require.Nil(t, tail.usage(json.Unmarshal))
}
