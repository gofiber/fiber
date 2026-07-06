package aigateway

// Dialect identifies the wire API a peer speaks. The gateway translates a
// chat request when the client's dialect (detected from the endpoint path)
// and the serving upstream's declared Dialect are both known and differ.
type Dialect int

const (
	// DialectUnspecified relays requests and responses byte-for-byte; no
	// translation ever engages. This is the zero value and the default for
	// hand-built Upstreams.
	DialectUnspecified Dialect = iota

	// DialectOpenAI is the OpenAI Chat Completions API
	// (POST /v1/chat/completions).
	DialectOpenAI

	// DialectAnthropic is the Anthropic Messages API (POST /v1/messages).
	DialectAnthropic
)

// String returns the dialect's name for logs and panics.
func (d Dialect) String() string {
	switch d {
	case DialectUnspecified:
		return "unspecified"
	case DialectOpenAI:
		return "openai"
	case DialectAnthropic:
		return "anthropic"
	default:
		return "invalid"
	}
}

// Chat endpoint paths, the only paths translation applies to.
const (
	openAIChatPath    = "/v1/chat/completions"
	anthropicChatPath = "/v1/messages"
)

// Shared wire strings used across the codecs and transcoders.
const (
	roleAssistant    = "assistant"
	roleUser         = "user"
	blockText        = "text"
	blockToolUse     = "tool_use"
	blockToolResult  = "tool_result"
	toolTypeFunction = "function"
	choiceAuto       = "auto"
	choiceNone       = "none"
	choiceAny        = "any"
	choiceTool       = "tool"
	choiceRequired   = "required"
	typeMessage      = "message"
	stopEndTurn      = "end_turn"
	stopMaxTokens    = "max_tokens"
	stopToolUse      = "tool_use"
	finishStop       = "stop"
	finishLength     = "length"
	finishToolCalls  = "tool_calls"
	errTypeAPI       = "api_error"
)

// Anthropic stream event names.
const (
	evtMessageStart      = "message_start"
	evtMessageDelta      = "message_delta"
	evtMessageStop       = "message_stop"
	evtContentBlockStart = "content_block_start"
	evtContentBlockDelta = "content_block_delta"
	evtContentBlockStop  = "content_block_stop"
	evtError             = "error"
)

// headerAnthropicVersion is Anthropic's mandatory API-version header;
// defaultAnthropicVersion is filled on translated requests when neither the
// client nor Upstream.Headers set one.
const (
	headerAnthropicVersion  = "anthropic-version"
	defaultAnthropicVersion = "2023-06-01"
)

// chatDialectForPath returns the dialect a client speaks based on the chat
// endpoint it called, or DialectUnspecified for any non-chat path (which is
// always relayed untranslated). path must already be prefix-stripped and
// percent-decoded.
func chatDialectForPath(path string) Dialect {
	switch path {
	case openAIChatPath:
		return DialectOpenAI
	case anthropicChatPath:
		return DialectAnthropic
	default:
		return DialectUnspecified
	}
}

// chatPathForDialect returns the chat endpoint path of a dialect.
func chatPathForDialect(d Dialect) string {
	switch d {
	case DialectOpenAI:
		return openAIChatPath
	case DialectAnthropic:
		return anthropicChatPath
	case DialectUnspecified:
	}
	return ""
}

// needsTranslation reports whether a request in the client dialect must be
// translated for an upstream: both dialects known, and different.
func needsTranslation(clientD, upstreamD Dialect) bool {
	return clientD != DialectUnspecified && upstreamD != DialectUnspecified && clientD != upstreamD
}
