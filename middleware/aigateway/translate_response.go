package aigateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/utils/v2"
)

// Stop/finish reason tables, shared by the buffered codecs and the stream
// transcoders. OpenAI does not distinguish a stop-sequence hit from a natural
// stop, so stop_sequence maps to "stop" and cannot be reconstructed on the
// way back (documented limitation).
func anthropicStopToFinish(stop string) string {
	switch stop {
	case stopMaxTokens:
		return finishLength
	case stopToolUse:
		return finishToolCalls
	}
	// end_turn, stop_sequence, refusal, pause_turn, and anything unknown.
	return finishStop
}

func finishToAnthropicStop(finish string) string {
	switch finish {
	case finishLength:
		return stopMaxTokens
	case finishToolCalls:
		return stopToolUse
	}
	// stop, content_filter, and anything unknown.
	return stopEndTurn
}

// ---- wire structs (responses) ----

type antResponse struct {
	Usage        *usageFields `json:"usage,omitempty"`
	StopSequence *string      `json:"stop_sequence"`
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Role         string       `json:"role"`
	Model        string       `json:"model"`
	StopReason   string       `json:"stop_reason,omitempty"`
	Content      []antBlock   `json:"content"`
}

type oaiChatResponse struct {
	Usage   *usageFields `json:"usage,omitempty"`
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []oaiChoice  `json:"choices"`
	Created int64        `json:"created"`
}

type oaiChoice struct {
	Message      *oaiRespMessage `json:"message,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
	Index        int             `json:"index"`
}

type oaiRespMessage struct {
	Content   *string       `json:"content"`
	Role      string        `json:"role"`
	ToolCalls []oaiToolCall `json:"tool_calls,omitempty"`
}

// translateResponseBody converts a successful (2xx) chat response body from
// the upstream's dialect to the client's. model is the client-requested model
// name, echoed back so SDK-side routing/accounting sees what it asked for.
func translateResponseBody(upstreamD Dialect, body []byte, model string, created int64, dec utils.JSONUnmarshal, enc utils.JSONMarshal) ([]byte, error) {
	switch upstreamD {
	case DialectAnthropic:
		return translateResponseA2O(body, model, created, dec, enc)
	case DialectOpenAI:
		return translateResponseO2A(body, model, dec, enc)
	case DialectUnspecified:
	}
	return nil, fmt.Errorf("aigateway: no response translation for %s", upstreamD)
}

// translateResponseA2O maps an Anthropic Messages response onto an OpenAI
// chat.completion object.
func translateResponseA2O(body []byte, model string, created int64, dec utils.JSONUnmarshal, enc utils.JSONMarshal) ([]byte, error) {
	var in antResponse
	if err := dec(body, &in); err != nil {
		return nil, fmt.Errorf("aigateway: decode Anthropic response: %w", err)
	}

	var text strings.Builder
	var toolCalls []oaiToolCall
	for i := range in.Content {
		b := &in.Content[i]
		switch b.Type {
		case blockText:
			_, _ = text.WriteString(b.Text) //nolint:errcheck // never errors
		case blockToolUse:
			args := "{}"
			if len(b.Input) > 0 {
				args = string(b.Input)
			}
			toolCalls = append(toolCalls, oaiToolCall{
				ID:       b.ID,
				Type:     toolTypeFunction,
				Function: oaiFunctionCall{Name: b.Name, Arguments: args},
			})
		default:
			// thinking et al.: no OpenAI equivalent, dropped.
		}
	}

	content := text.String()
	out := oaiChatResponse{
		ID:      in.ID,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []oaiChoice{{
			Index: 0,
			Message: &oaiRespMessage{
				Role:      roleAssistant,
				Content:   &content,
				ToolCalls: toolCalls,
			},
			FinishReason: anthropicStopToFinish(in.StopReason),
		}},
	}
	if in.Usage != nil {
		out.Usage = normalizeUsageFields(in.Usage)
	}
	res, err := enc(out)
	if err != nil {
		return nil, fmt.Errorf("aigateway: encode translated response: %w", err)
	}
	return res, nil
}

// translateResponseO2A maps an OpenAI chat.completion object onto an
// Anthropic Messages response.
func translateResponseO2A(body []byte, model string, dec utils.JSONUnmarshal, enc utils.JSONMarshal) ([]byte, error) {
	var in oaiChatResponse
	if err := dec(body, &in); err != nil {
		return nil, fmt.Errorf("aigateway: decode OpenAI response: %w", err)
	}
	if len(in.Choices) == 0 || in.Choices[0].Message == nil {
		return nil, errors.New("aigateway: OpenAI response carries no choices")
	}
	choice := &in.Choices[0]

	var blocks []antBlock
	if choice.Message.Content != nil && *choice.Message.Content != "" {
		blocks = append(blocks, antBlock{Type: blockText, Text: *choice.Message.Content})
	}
	for i := range choice.Message.ToolCalls {
		tc := &choice.Message.ToolCalls[i]
		input := strings.TrimSpace(tc.Function.Arguments)
		if input == "" || !json.Valid([]byte(input)) {
			input = "{}"
		}
		blocks = append(blocks, antBlock{
			Type:  blockToolUse,
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(input),
		})
	}
	if blocks == nil {
		blocks = []antBlock{}
	}

	out := antResponse{
		ID:         anthropicMessageID(in.ID),
		Type:       typeMessage,
		Role:       roleAssistant,
		Model:      model,
		Content:    blocks,
		StopReason: finishToAnthropicStop(choice.FinishReason),
	}
	// Anthropic's Message schema requires usage; emit zeros when a lenient
	// OpenAI-compatible upstream omitted it, so client SDK validation passes.
	u := &usageFields{}
	if in.Usage != nil {
		u = in.Usage
	}
	out.Usage = normalizeUsageFields(u)
	res, err := enc(out)
	if err != nil {
		return nil, fmt.Errorf("aigateway: encode translated response: %w", err)
	}
	return res, nil
}

// normalizeUsageFields fills both dialects' token field names so the
// translated body carries the pair the client's SDK reads.
func normalizeUsageFields(u *usageFields) *usageFields {
	out := *u
	out.PromptTokens = max(u.PromptTokens, u.InputTokens)
	out.InputTokens = out.PromptTokens
	out.CompletionTokens = max(u.CompletionTokens, u.OutputTokens)
	out.OutputTokens = out.CompletionTokens
	if out.TotalTokens == 0 {
		out.TotalTokens = out.PromptTokens + out.CompletionTokens
	}
	return &out
}

// anthropicMessageID shapes an id for Anthropic-dialect clients.
func anthropicMessageID(id string) string {
	if id == "" {
		return "msg_gateway"
	}
	if strings.HasPrefix(id, "msg_") {
		return id
	}
	return "msg_" + id
}

// ---- error-body translation ----

// oaiErrorEnvelope / antErrorEnvelope are the two providers' error shapes.
type oaiErrorEnvelope struct {
	Error *oaiErrorBody `json:"error"`
}

type oaiErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type antErrorEnvelope struct {
	Error *antErrorBody `json:"error"`
	Type  string        `json:"type"`
}

type antErrorBody struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// oaiErrorJSON and antErrorJSON build a client-dialect error envelope,
// defaulting an empty type to api_error, so every error producer — the
// buffered translator and both stream transcoders — shapes errors the same
// way.
func oaiErrorJSON(errType, msg string, enc utils.JSONMarshal) ([]byte, error) {
	if errType == "" {
		errType = errTypeAPI
	}
	out, err := enc(oaiErrorEnvelope{Error: &oaiErrorBody{Message: msg, Type: errType}})
	if err != nil {
		return nil, fmt.Errorf("aigateway: encode error envelope: %w", err)
	}
	return out, nil
}

func antErrorJSON(errType, msg string, enc utils.JSONMarshal) ([]byte, error) {
	if errType == "" {
		errType = errTypeAPI
	}
	out, err := enc(antErrorEnvelope{Type: evtError, Error: &antErrorBody{Type: errType, Message: msg}})
	if err != nil {
		return nil, fmt.Errorf("aigateway: encode error envelope: %w", err)
	}
	return out, nil
}

// translateErrorBody converts an upstream error body from the upstream's
// dialect to the client's. An unparseable body is synthesized into a valid
// error envelope carrying the raw text, so the client's SDK can always parse
// the failure. The HTTP status is relayed unchanged by the caller.
func translateErrorBody(upstreamD Dialect, body []byte, dec utils.JSONUnmarshal, enc utils.JSONMarshal) []byte {
	errType, msg := errTypeAPI, strings.TrimSpace(string(body))
	switch upstreamD {
	case DialectAnthropic:
		var in antErrorEnvelope
		if err := dec(body, &in); err == nil && in.Error != nil {
			errType, msg = in.Error.Type, in.Error.Message
		}
		if out, err := oaiErrorJSON(errType, msg, enc); err == nil {
			return out
		}
	case DialectOpenAI:
		var in oaiErrorEnvelope
		if err := dec(body, &in); err == nil && in.Error != nil {
			errType, msg = in.Error.Type, in.Error.Message
		}
		if out, err := antErrorJSON(errType, msg, enc); err == nil {
			return out
		}
	case DialectUnspecified:
	}
	return body
}
