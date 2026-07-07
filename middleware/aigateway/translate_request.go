package aigateway

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/utils/v2"
)

// errUntranslatable marks a request whose semantics cannot be expressed in
// the upstream's dialect (e.g. n>1, audio modalities, server tools). It does
// not abort the fallback chain — a same-dialect upstream can still serve the
// request verbatim — but when no upstream could take it at all, the gateway
// answers 400 rather than 502.
var errUntranslatable = errors.New("aigateway: request not translatable")

func untranslatable(reason string) error {
	return fmt.Errorf("%w: %s", errUntranslatable, reason)
}

// translateDefaultMaxTokens is injected as the Anthropic max_tokens (which
// the Messages API requires) when an OpenAI-dialect client did not set any
// max-token field. MaxTokensCap, when configured, caps it further.
const translateDefaultMaxTokens = 4096

// streamOpts records the streaming intent of a translated request: whether
// the client asked for a stream, and — for OpenAI-dialect clients — whether
// they opted into the trailing usage chunk (stream_options.include_usage),
// which controls what the stream transcoder emits.
type streamOpts struct {
	stream       bool
	includeUsage bool
}

// ---- OpenAI wire structs (requests) ----

type oaiChatRequest struct {
	Temperature         *float64          `json:"temperature,omitempty"`
	TopP                *float64          `json:"top_p,omitempty"`
	MaxTokens           *int              `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int              `json:"max_completion_tokens,omitempty"`
	N                   *int              `json:"n,omitempty"`
	ParallelToolCalls   *bool             `json:"parallel_tool_calls,omitempty"`
	StreamOptions       *oaiStreamOptions `json:"stream_options,omitempty"`
	Audio               json.RawMessage   `json:"audio,omitempty"`
	ToolChoice          json.RawMessage   `json:"tool_choice,omitempty"`
	Stop                json.RawMessage   `json:"stop,omitempty"`
	Model               string            `json:"model"`
	User                string            `json:"user,omitempty"`
	Messages            []oaiMessage      `json:"messages"`
	Tools               []oaiTool         `json:"tools,omitempty"`
	Modalities          []string          `json:"modalities,omitempty"`
	Stream              bool              `json:"stream,omitempty"`
}

type oaiMessage struct {
	Content    json.RawMessage `json:"content,omitempty"` // string or []oaiContentPart
	Role       string          `json:"role"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []oaiToolCall   `json:"tool_calls,omitempty"`
}

type oaiContentPart struct {
	ImageURL *oaiImageURL `json:"image_url,omitempty"`
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
}

type oaiImageURL struct {
	URL string `json:"url"`
}

type oaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiFunctionCall `json:"function"`
}

type oaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaiTool struct {
	Function *oaiFuncDef `json:"function,omitempty"`
	Type     string      `json:"type"`
}

type oaiFuncDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type oaiStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ---- Anthropic wire structs (requests) ----

type antRequest struct {
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	ToolChoice    *antToolChoice  `json:"tool_choice,omitempty"`
	Metadata      *antMetadata    `json:"metadata,omitempty"`
	System        json.RawMessage `json:"system,omitempty"` // string or []antBlock
	Thinking      json.RawMessage `json:"thinking,omitempty"`
	Model         string          `json:"model"`
	Messages      []antMessage    `json:"messages"`
	Tools         []antTool       `json:"tools,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Stream        bool            `json:"stream,omitempty"`
}

type antMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []antBlock
}

type antBlock struct {
	Source    *antImageSource `json:"source,omitempty"`
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`          // tool_use
	Name      string          `json:"name,omitempty"`        // tool_use
	ToolUseID string          `json:"tool_use_id,omitempty"` // tool_result
	Input     json.RawMessage `json:"input,omitempty"`       // tool_use
	Content   json.RawMessage `json:"content,omitempty"`     // tool_result: string or []antBlock
}

type antImageSource struct {
	Type      string `json:"type"` // "base64" or "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type antTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"` // set on server tools (untranslatable)
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type antToolChoice struct {
	DisableParallelToolUse *bool  `json:"disable_parallel_tool_use,omitempty"`
	Type                   string `json:"type"`
	Name                   string `json:"name,omitempty"`
}

type antMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// translateRequest converts a chat request body from the client's dialect to
// the upstream's. It reports the request's streaming intent alongside the
// translated body.
func translateRequest(clientD, upstreamD Dialect, jsonBody []byte, dec utils.JSONUnmarshal, enc utils.JSONMarshal, maxTokensCap int) ([]byte, streamOpts, error) {
	switch {
	case jsonBody == nil:
		return nil, streamOpts{}, untranslatable("request body is not a JSON object")
	case clientD == DialectOpenAI && upstreamD == DialectAnthropic:
		return translateRequestO2A(jsonBody, dec, enc, maxTokensCap)
	case clientD == DialectAnthropic && upstreamD == DialectOpenAI:
		return translateRequestA2O(jsonBody, dec, enc)
	default:
		return nil, streamOpts{}, fmt.Errorf("aigateway: no translation from %s to %s", clientD, upstreamD)
	}
}

// decodeStringOrRaw decodes a JSON value that is either a plain string or
// something else; ok reports the string case.
func decodeStringOrRaw(raw json.RawMessage, dec utils.JSONUnmarshal) (string, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed[0] != '"' {
		return "", false
	}
	var s string
	if err := dec(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// ---- OpenAI -> Anthropic ----

func translateRequestO2A(jsonBody []byte, dec utils.JSONUnmarshal, enc utils.JSONMarshal, maxTokensCap int) ([]byte, streamOpts, error) {
	var in oaiChatRequest
	if err := dec(jsonBody, &in); err != nil {
		return nil, streamOpts{}, untranslatable("cannot decode OpenAI chat request: " + err.Error())
	}
	if in.N != nil && *in.N > 1 {
		return nil, streamOpts{}, untranslatable("n > 1 has no Anthropic equivalent")
	}
	if len(in.Audio) > 0 {
		return nil, streamOpts{}, untranslatable("audio output has no Anthropic equivalent")
	}
	for _, m := range in.Modalities {
		if !strings.EqualFold(m, "text") {
			return nil, streamOpts{}, untranslatable("modality " + m + " has no Anthropic equivalent")
		}
	}

	out := antRequest{
		Model:  in.Model,
		Stream: in.Stream,
		TopP:   in.TopP,
	}

	// Temperature: OpenAI allows 0-2, Anthropic 0-1 — clamp.
	if in.Temperature != nil {
		t := min(*in.Temperature, 1)
		out.Temperature = &t
	}

	// max_tokens is required by the Messages API: prefer the client's value,
	// fall back to a default, and respect MaxTokensCap either way.
	switch {
	case in.MaxCompletionTokens != nil:
		out.MaxTokens = *in.MaxCompletionTokens
	case in.MaxTokens != nil:
		out.MaxTokens = *in.MaxTokens
	default:
		out.MaxTokens = translateDefaultMaxTokens
	}
	if maxTokensCap > 0 && out.MaxTokens > maxTokensCap {
		out.MaxTokens = maxTokensCap
	}

	if len(in.Stop) > 0 {
		if s, ok := decodeStringOrRaw(in.Stop, dec); ok {
			out.StopSequences = []string{s}
		} else {
			var seqs []string
			if err := dec(in.Stop, &seqs); err != nil {
				return nil, streamOpts{}, untranslatable("cannot decode stop: " + err.Error())
			}
			out.StopSequences = seqs
		}
	}

	if in.User != "" {
		out.Metadata = &antMetadata{UserID: in.User}
	}

	// Messages: system/developer roles feed the system param; tool results
	// group into user messages of tool_result blocks; everything else maps
	// onto content blocks.
	var system []string
	var toolResults []antBlock
	flushToolResults := func() error {
		if len(toolResults) == 0 {
			return nil
		}
		raw, err := enc(toolResults)
		if err != nil {
			return untranslatable("cannot encode tool results: " + err.Error())
		}
		out.Messages = append(out.Messages, antMessage{Role: roleUser, Content: raw})
		toolResults = nil
		return nil
	}

	for i := range in.Messages {
		m := &in.Messages[i]
		switch m.Role {
		case "system", "developer":
			if err := flushToolResults(); err != nil {
				return nil, streamOpts{}, err
			}
			if s, ok := decodeStringOrRaw(m.Content, dec); ok {
				system = append(system, s)
			} else if txt, err := oaiPartsText(m.Content, dec); err == nil {
				system = append(system, txt)
			} else {
				return nil, streamOpts{}, untranslatable("cannot decode system message content")
			}
		case "tool":
			block := antBlock{Type: blockToolResult, ToolUseID: m.ToolCallID}
			if s, ok := decodeStringOrRaw(m.Content, dec); ok {
				block.Content = mustJSONString(s, enc)
			} else if txt, err := oaiPartsText(m.Content, dec); err == nil {
				block.Content = mustJSONString(txt, enc)
			} else {
				return nil, streamOpts{}, untranslatable("cannot decode tool message content")
			}
			toolResults = append(toolResults, block)
		case roleUser, roleAssistant:
			if err := flushToolResults(); err != nil {
				return nil, streamOpts{}, err
			}
			blocks, err := oaiMessageToBlocks(m, dec)
			if err != nil {
				return nil, streamOpts{}, err
			}
			if len(blocks) == 0 {
				continue
			}
			raw, eerr := enc(blocks)
			if eerr != nil {
				return nil, streamOpts{}, untranslatable("cannot encode content blocks: " + eerr.Error())
			}
			out.Messages = append(out.Messages, antMessage{Role: m.Role, Content: raw})
		default:
			return nil, streamOpts{}, untranslatable("message role " + m.Role + " has no Anthropic equivalent")
		}
	}
	if err := flushToolResults(); err != nil {
		return nil, streamOpts{}, err
	}
	if len(system) > 0 {
		out.System = mustJSONString(strings.Join(system, "\n\n"), enc)
	}
	if len(out.Messages) == 0 {
		return nil, streamOpts{}, untranslatable("no user or assistant messages")
	}

	// Tools and tool choice.
	for i := range in.Tools {
		t := &in.Tools[i]
		if t.Type != toolTypeFunction || t.Function == nil {
			return nil, streamOpts{}, untranslatable("tool type " + t.Type + " has no Anthropic equivalent")
		}
		out.Tools = append(out.Tools, antTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	if len(in.ToolChoice) > 0 {
		tc, err := oaiToolChoiceToAnthropic(in.ToolChoice, dec)
		if err != nil {
			return nil, streamOpts{}, err
		}
		out.ToolChoice = tc
	}
	if in.ParallelToolCalls != nil && !*in.ParallelToolCalls {
		if out.ToolChoice == nil {
			out.ToolChoice = &antToolChoice{Type: choiceAuto}
		}
		disable := true
		out.ToolChoice.DisableParallelToolUse = &disable
	}

	body, err := enc(out)
	if err != nil {
		return nil, streamOpts{}, fmt.Errorf("aigateway: encode translated request: %w", err)
	}
	opts := streamOpts{stream: in.Stream, includeUsage: in.StreamOptions != nil && in.StreamOptions.IncludeUsage}
	return body, opts, nil
}

// oaiMessageToBlocks maps one OpenAI user/assistant message onto Anthropic
// content blocks: text and image parts, plus tool_use blocks for assistant
// tool calls.
func oaiMessageToBlocks(m *oaiMessage, dec utils.JSONUnmarshal) ([]antBlock, error) {
	var blocks []antBlock
	if len(m.Content) > 0 {
		if s, ok := decodeStringOrRaw(m.Content, dec); ok {
			if s != "" {
				blocks = append(blocks, antBlock{Type: blockText, Text: s})
			}
		} else {
			var parts []oaiContentPart
			if err := dec(m.Content, &parts); err != nil {
				return nil, untranslatable("cannot decode message content: " + err.Error())
			}
			for i := range parts {
				block, err := oaiPartToBlock(&parts[i])
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, block)
			}
		}
	}
	for i := range m.ToolCalls {
		tc := &m.ToolCalls[i]
		if tc.Type != "" && tc.Type != toolTypeFunction {
			return nil, untranslatable("tool call type " + tc.Type + " has no Anthropic equivalent")
		}
		input := strings.TrimSpace(tc.Function.Arguments)
		if input == "" {
			input = "{}"
		}
		if !json.Valid([]byte(input)) {
			return nil, untranslatable("tool call arguments are not valid JSON")
		}
		blocks = append(blocks, antBlock{
			Type:  blockToolUse,
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(input),
		})
	}
	return blocks, nil
}

func oaiPartToBlock(p *oaiContentPart) (antBlock, error) {
	switch p.Type {
	case "text":
		return antBlock{Type: blockText, Text: p.Text}, nil
	case "image_url":
		if p.ImageURL == nil {
			return antBlock{}, untranslatable("image_url part without image_url")
		}
		src, err := imageURLToSource(p.ImageURL.URL)
		if err != nil {
			return antBlock{}, err
		}
		return antBlock{Type: "image", Source: src}, nil
	default:
		return antBlock{}, untranslatable("content part type " + p.Type + " has no Anthropic equivalent")
	}
}

// imageURLToSource maps an OpenAI image URL — a data: URL or a fetchable
// https URL — onto an Anthropic image source.
func imageURLToSource(url string) (*antImageSource, error) {
	if data, ok := strings.CutPrefix(url, "data:"); ok {
		mediaType, b64, found := strings.Cut(data, ";base64,")
		if !found {
			return nil, untranslatable("image data: URL is not base64-encoded")
		}
		if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
			return nil, untranslatable("image data: URL carries invalid base64")
		}
		return &antImageSource{Type: "base64", MediaType: mediaType, Data: b64}, nil
	}
	return &antImageSource{Type: "url", URL: url}, nil
}

// oaiPartsText decodes an OpenAI parts array and concatenates its text parts.
func oaiPartsText(raw json.RawMessage, dec utils.JSONUnmarshal) (string, error) {
	var parts []oaiContentPart
	if err := dec(raw, &parts); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := range parts {
		if parts[i].Type == blockText {
			_, _ = sb.WriteString(parts[i].Text) //nolint:errcheck // never errors
		}
	}
	return sb.String(), nil
}

func oaiToolChoiceToAnthropic(raw json.RawMessage, dec utils.JSONUnmarshal) (*antToolChoice, error) {
	if s, ok := decodeStringOrRaw(raw, dec); ok {
		switch s {
		case choiceAuto:
			return &antToolChoice{Type: choiceAuto}, nil
		case choiceNone:
			return &antToolChoice{Type: choiceNone}, nil
		case choiceRequired:
			return &antToolChoice{Type: choiceAny}, nil
		default:
			return nil, untranslatable("tool_choice " + s + " has no Anthropic equivalent")
		}
	}
	var obj struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
		Type string `json:"type"`
	}
	if err := dec(raw, &obj); err != nil || obj.Type != toolTypeFunction || obj.Function.Name == "" {
		return nil, untranslatable("cannot decode tool_choice")
	}
	return &antToolChoice{Type: choiceTool, Name: obj.Function.Name}, nil
}

// mustJSONString encodes a Go string as a JSON string. Encoding a string
// cannot fail; the fallback quotes it defensively.
func mustJSONString(s string, enc utils.JSONMarshal) json.RawMessage {
	raw, err := enc(s)
	if err != nil {
		return json.RawMessage(fmt.Sprintf("%q", s))
	}
	return raw
}

// ---- Anthropic -> OpenAI ----

func translateRequestA2O(jsonBody []byte, dec utils.JSONUnmarshal, enc utils.JSONMarshal) ([]byte, streamOpts, error) {
	var in antRequest
	if err := dec(jsonBody, &in); err != nil {
		return nil, streamOpts{}, untranslatable("cannot decode Anthropic messages request: " + err.Error())
	}

	out := oaiChatRequest{
		Model:       in.Model,
		Stream:      in.Stream,
		Temperature: in.Temperature,
		TopP:        in.TopP,
	}
	if in.MaxTokens > 0 {
		mt := in.MaxTokens
		out.MaxTokens = &mt
	}
	if len(in.StopSequences) > 0 {
		raw, err := enc(in.StopSequences)
		if err != nil {
			return nil, streamOpts{}, untranslatable("cannot encode stop_sequences: " + err.Error())
		}
		out.Stop = raw
	}
	if in.Metadata != nil && in.Metadata.UserID != "" {
		out.User = in.Metadata.UserID
	}
	// top_k and thinking have no OpenAI equivalent: dropped (documented).

	// System prompt becomes the leading system message.
	if len(in.System) > 0 {
		var sys string
		if s, ok := decodeStringOrRaw(in.System, dec); ok {
			sys = s
		} else {
			var blocks []antBlock
			if err := dec(in.System, &blocks); err != nil {
				return nil, streamOpts{}, untranslatable("cannot decode system blocks")
			}
			var sb strings.Builder
			for i := range blocks {
				if blocks[i].Type == blockText {
					_, _ = sb.WriteString(blocks[i].Text) //nolint:errcheck // never errors
				}
			}
			sys = sb.String()
		}
		if sys != "" {
			out.Messages = append(out.Messages, oaiMessage{Role: "system", Content: mustJSONString(sys, enc)})
		}
	}

	for i := range in.Messages {
		msgs, err := antMessageToOpenAI(&in.Messages[i], dec, enc)
		if err != nil {
			return nil, streamOpts{}, err
		}
		out.Messages = append(out.Messages, msgs...)
	}
	if len(out.Messages) == 0 {
		return nil, streamOpts{}, untranslatable("no messages")
	}

	for i := range in.Tools {
		t := &in.Tools[i]
		if t.Type != "" && t.Type != "custom" {
			return nil, streamOpts{}, untranslatable("server tool " + t.Type + " has no OpenAI equivalent")
		}
		schema := t.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		out.Tools = append(out.Tools, oaiTool{
			Type:     toolTypeFunction,
			Function: &oaiFuncDef{Name: t.Name, Description: t.Description, Parameters: schema},
		})
	}
	if in.ToolChoice != nil {
		raw, err := anthropicToolChoiceToOpenAI(in.ToolChoice, enc)
		if err != nil {
			return nil, streamOpts{}, err
		}
		out.ToolChoice = raw
		if in.ToolChoice.DisableParallelToolUse != nil && *in.ToolChoice.DisableParallelToolUse {
			par := false
			out.ParallelToolCalls = &par
		}
	}

	// The transcoder needs the final usage chunk to fill Anthropic's
	// message_delta usage, so opt into it on the client's behalf.
	if in.Stream {
		out.StreamOptions = &oaiStreamOptions{IncludeUsage: true}
	}

	body, err := enc(out)
	if err != nil {
		return nil, streamOpts{}, fmt.Errorf("aigateway: encode translated request: %w", err)
	}
	return body, streamOpts{stream: in.Stream, includeUsage: true}, nil
}

// antMessageToOpenAI maps one Anthropic message onto one or more OpenAI
// messages: tool_result blocks become their own role:tool messages, the
// remaining content becomes a user/assistant message (with tool_use blocks
// turning into assistant tool_calls).
func antMessageToOpenAI(m *antMessage, dec utils.JSONUnmarshal, enc utils.JSONMarshal) ([]oaiMessage, error) {
	if s, ok := decodeStringOrRaw(m.Content, dec); ok {
		return []oaiMessage{{Role: m.Role, Content: mustJSONString(s, enc)}}, nil
	}

	var blocks []antBlock
	if err := dec(m.Content, &blocks); err != nil {
		return nil, untranslatable("cannot decode message content blocks: " + err.Error())
	}

	var msgs []oaiMessage
	var parts []oaiContentPart
	var toolCalls []oaiToolCall
	var textOnly strings.Builder
	hasNonText := false

	for i := range blocks {
		b := &blocks[i]
		switch b.Type {
		case blockText:
			parts = append(parts, oaiContentPart{Type: blockText, Text: b.Text})
			_, _ = textOnly.WriteString(b.Text) //nolint:errcheck // never errors
		case "image":
			url, err := sourceToImageURL(b.Source)
			if err != nil {
				return nil, err
			}
			parts = append(parts, oaiContentPart{Type: "image_url", ImageURL: &oaiImageURL{URL: url}})
			hasNonText = true
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
		case blockToolResult:
			// Its own role:tool message; nested images and the is_error flag
			// are dropped (documented) — OpenAI tool messages have neither.
			text, err := toolResultText(b, dec)
			if err != nil {
				return nil, err
			}
			msgs = append(msgs, oaiMessage{
				Role:       choiceTool,
				ToolCallID: b.ToolUseID,
				Content:    mustJSONString(text, enc),
			})
		case "thinking", "redacted_thinking":
			// No OpenAI equivalent: dropped (documented).
		default:
			return nil, untranslatable("content block type " + b.Type + " has no OpenAI equivalent")
		}
	}

	if len(parts) > 0 || len(toolCalls) > 0 {
		msg := oaiMessage{Role: m.Role, ToolCalls: toolCalls}
		switch {
		case len(parts) == 0:
			// tool_calls-only assistant message.
		case m.Role == roleAssistant || !hasNonText:
			// OpenAI assistant messages carry string content; user text-only
			// content also collapses to a string for fidelity.
			msg.Content = mustJSONString(textOnly.String(), enc)
		default:
			raw, err := enc(parts)
			if err != nil {
				return nil, untranslatable("cannot encode content parts: " + err.Error())
			}
			msg.Content = raw
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

func sourceToImageURL(src *antImageSource) (string, error) {
	if src == nil {
		return "", untranslatable("image block without source")
	}
	switch src.Type {
	case "base64":
		return "data:" + src.MediaType + ";base64," + src.Data, nil
	case "url":
		return src.URL, nil
	default:
		return "", untranslatable("image source type " + src.Type + " has no OpenAI equivalent")
	}
}

// toolResultText flattens a tool_result block's content (string or blocks)
// to text.
func toolResultText(b *antBlock, dec utils.JSONUnmarshal) (string, error) {
	if len(b.Content) == 0 {
		return "", nil
	}
	if s, ok := decodeStringOrRaw(b.Content, dec); ok {
		return s, nil
	}
	var blocks []antBlock
	if err := dec(b.Content, &blocks); err != nil {
		return "", untranslatable("cannot decode tool_result content")
	}
	var sb strings.Builder
	for i := range blocks {
		if blocks[i].Type == blockText {
			_, _ = sb.WriteString(blocks[i].Text) //nolint:errcheck // never errors
		}
	}
	return sb.String(), nil
}

// oaiToolChoiceFunc / oaiToolChoiceName encode OpenAI's named tool_choice.
type oaiToolChoiceFunc struct {
	Type     string            `json:"type"`
	Function oaiToolChoiceName `json:"function"`
}

type oaiToolChoiceName struct {
	Name string `json:"name"`
}

func anthropicToolChoiceToOpenAI(tc *antToolChoice, enc utils.JSONMarshal) (json.RawMessage, error) {
	switch tc.Type {
	case choiceAuto:
		return json.RawMessage(`"auto"`), nil
	case choiceNone:
		return json.RawMessage(`"none"`), nil
	case choiceAny:
		return json.RawMessage(`"required"`), nil
	case choiceTool:
		obj := oaiToolChoiceFunc{Type: toolTypeFunction, Function: oaiToolChoiceName{Name: tc.Name}}
		raw, err := enc(obj)
		if err != nil {
			return nil, untranslatable("cannot encode tool_choice")
		}
		return raw, nil
	default:
		return nil, untranslatable("tool_choice type " + tc.Type + " has no OpenAI equivalent")
	}
}
