package aigateway

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/utils/v2"
)

// defaultKeepaliveInterval is how often relayStream writes an SSE comment to
// the client of a transcoded stream. It must comfortably undercut common
// intermediary idle timeouts (30-60s at ALBs/nginx) while adding negligible
// traffic.
const defaultKeepaliveInterval = 15 * time.Second

// sseMaxEventBytes caps one buffered SSE event during transcoding. Unlike
// usageTail's skip-on-overflow (which only loses best-effort usage), a
// transcoder cannot drop an event — the translated stream would corrupt — so
// an oversized event aborts the stream instead.
const sseMaxEventBytes = 1 << 20 // 1 MiB

var (
	errSSEEventTooLarge = errors.New("aigateway: upstream SSE event exceeds the transcoding limit")

	// errStreamTruncated marks an upstream stream that ended without its
	// dialect's terminator; the transcoder informs the client with an error
	// event and the relay records it on the usage event.
	errStreamTruncated = errors.New("aigateway: upstream stream ended before its terminator")
)

// writeSSEKeepalive writes an SSE comment line — bytes every SSE parser
// ignores. relayStream's keepalive ticker uses it on transcoded streams so
// upstream silences (pings, thinking deltas, a slow first token) still
// produce client-side traffic and intermediary idle timeouts don't fire.
func writeSSEKeepalive(w *bufio.Writer) error {
	if _, err := w.WriteString(": keepalive\n\n"); err != nil {
		return fmt.Errorf("aigateway: write keepalive: %w", err)
	}
	return nil
}

// streamTranscoder converts an upstream SSE token stream to the client's
// dialect incrementally. feed consumes one upstream chunk (arbitrarily
// split); finish flushes terminal events on a clean EOF that lacked the
// upstream's own terminator; usage reports the token usage observed.
type streamTranscoder interface {
	feed(w *bufio.Writer, chunk []byte) error
	finish(w *bufio.Writer) error
	usage() *Usage
}

// newTranscoder builds the stream transcoder for a translated streaming
// response, or nil when the stream cannot be transcoded: a content-encoded
// stream (identity was pinned on the request, so this is a misbehaving
// upstream) or a non-SSE streaming Content-Type (NDJSON has no event framing
// to transcode). includeUsage is the client's stream_options.include_usage
// choice, recorded when the request was translated.
func newTranscoder(c fiber.Ctx, resp *client.Response, xlateFrom Dialect, model string, includeUsage bool) streamTranscoder {
	if enc := string(resp.RawResponse.Header.Peek(fiber.HeaderContentEncoding)); enc != "" && !strings.EqualFold(strings.TrimSpace(enc), "identity") {
		return nil
	}
	ct := resp.RawResponse.Header.ContentType()
	if len(ct) < len(fiber.MIMETextEventStream) ||
		!utils.EqualFold(utils.UnsafeString(ct[:len(fiber.MIMETextEventStream)]), fiber.MIMETextEventStream) {
		return nil
	}

	dec, enc := c.App().Config().JSONDecoder, c.App().Config().JSONEncoder
	switch xlateFrom {
	case DialectAnthropic:
		return newA2OTranscoder(model, time.Now().Unix(), includeUsage, dec, enc)
	case DialectOpenAI:
		return newO2ATranscoder(model, dec, enc)
	case DialectUnspecified:
	}
	return nil
}

// sseEvent is one complete upstream Server-Sent Event.
type sseEvent struct {
	name string
	data []byte
}

// sseScanner assembles complete SSE events from arbitrarily-split chunks.
// Multiple data: lines concatenate with newlines per the SSE spec; comment
// and unknown fields are ignored.
type sseScanner struct {
	carry []byte
	name  string
	data  []byte
	open  bool // an event is being accumulated
}

// feed appends a chunk and invokes fn for every event completed by it. It
// errors when a single event outgrows sseMaxEventBytes.
func (s *sseScanner) feed(chunk []byte, fn func(ev *sseEvent) error) error {
	if len(s.carry)+len(chunk) > sseMaxEventBytes && !bytes.Contains(chunk, []byte{'\n'}) {
		return errSSEEventTooLarge
	}
	buf := append(s.carry, chunk...) //nolint:gocritic // carry is rebuilt below
	for {
		nl := bytes.IndexByte(buf, '\n')
		if nl == -1 {
			break
		}
		line := bytes.TrimSuffix(buf[:nl], []byte{'\r'})
		buf = buf[nl+1:]
		if err := s.line(line, fn); err != nil {
			return err
		}
	}
	if len(buf) > sseMaxEventBytes {
		return errSSEEventTooLarge
	}
	s.carry = append(s.carry[:0], buf...)
	return nil
}

func (s *sseScanner) line(line []byte, fn func(ev *sseEvent) error) error {
	if len(line) == 0 {
		// Blank line: event boundary.
		if !s.open {
			return nil
		}
		ev := sseEvent{name: s.name, data: s.data}
		s.name, s.data, s.open = "", nil, false
		return fn(&ev)
	}
	if line[0] == ':' {
		return nil // comment
	}
	field, value, _ := bytes.Cut(line, []byte{':'})
	value = bytes.TrimPrefix(value, []byte{' '})
	switch string(field) {
	case "event":
		s.name = string(value)
		s.open = true
	case "data":
		if len(s.data) > 0 {
			s.data = append(s.data, '\n')
		}
		if len(s.data)+len(value) > sseMaxEventBytes {
			return errSSEEventTooLarge
		}
		s.data = append(s.data, value...)
		s.open = true
	default:
		// id/retry/unknown fields: not needed for transcoding.
	}
	return nil
}

// writeSSE writes one client-dialect SSE event. Anthropic SDKs require the
// event: field; OpenAI's protocol is data:-only.
func writeSSE(w *bufio.Writer, eventName string, data []byte) error {
	if eventName != "" {
		if _, err := w.WriteString("event: " + eventName + "\n"); err != nil {
			return fmt.Errorf("aigateway: write translated event: %w", err)
		}
	}
	if _, err := w.WriteString("data: "); err != nil {
		return fmt.Errorf("aigateway: write translated event: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("aigateway: write translated event: %w", err)
	}
	if _, err := w.WriteString("\n\n"); err != nil {
		return fmt.Errorf("aigateway: write translated event: %w", err)
	}
	return nil
}

// Typed payloads for emitted Anthropic events (avoids per-event map literals).
type antEventPayload struct {
	Message      *antStartMessage `json:"message,omitempty"`
	ContentBlock *antStartBlock   `json:"content_block,omitempty"`
	Delta        any              `json:"delta,omitempty"` // *antEventDelta or *antMsgDelta
	Usage        *antEventUsage   `json:"usage,omitempty"`
	Index        *int             `json:"index,omitempty"`
	Type         string           `json:"type"`
}

// antMsgDelta is the terminal message_delta payload. Like antStartMessage,
// stop_sequence is required-nullable in Anthropic's schema, so the key is
// always emitted (as null) for strict client SDK validation.
type antMsgDelta struct {
	StopSequence *string `json:"stop_sequence"`
	StopReason   string  `json:"stop_reason"`
}

// antStartMessage is the message_start payload. StopReason/StopSequence stay
// nil on purpose and deliberately lack omitempty: Anthropic's schema requires
// the keys present (as null) and strict client SDKs validate that — do not
// "clean up" the always-nil pointers.
type antStartMessage struct {
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        *antEventUsage `json:"usage"`
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	Content      []antBlock     `json:"content"`
}

type antStartBlock struct {
	Input map[string]any `json:"input,omitempty"`
	Text  *string        `json:"text,omitempty"`
	Type  string         `json:"type"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
}

type antEventDelta struct {
	Text        *string `json:"text,omitempty"`
	PartialJSON *string `json:"partial_json,omitempty"`
	Type        string  `json:"type,omitempty"`
}

type antEventUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ---- Anthropic upstream -> OpenAI client ----

// Anthropic stream event payloads (the subset the transcoder reads).
type antStreamEvent struct {
	Message *struct {
		Usage *usageFields `json:"usage"`
		ID    string       `json:"id"`
	} `json:"message,omitempty"`
	ContentBlock *antBlock `json:"content_block,omitempty"`
	Delta        *struct {
		StopReason  *string `json:"stop_reason,omitempty"`
		Type        string  `json:"type"`
		Text        string  `json:"text,omitempty"`
		PartialJSON string  `json:"partial_json,omitempty"`
	} `json:"delta,omitempty"`
	Usage *usageFields  `json:"usage,omitempty"`
	Error *antErrorBody `json:"error,omitempty"`
	Type  string        `json:"type"`
	Index int           `json:"index"`
}

// OpenAI stream chunk shapes (emitted).
type oaiStreamChunk struct {
	Usage   *usageFields     `json:"usage,omitempty"`
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Model   string           `json:"model"`
	Choices []oaiChunkChoice `json:"choices"`
	Created int64            `json:"created"`
}

type oaiChunkChoice struct {
	Delta        *oaiChunkDelta `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
	Index        int            `json:"index"`
}

type oaiChunkDelta struct {
	Content   *string             `json:"content,omitempty"`
	Role      string              `json:"role,omitempty"`
	ToolCalls []oaiChunkToolDelta `json:"tool_calls,omitempty"`
}

type oaiChunkToolDelta struct {
	Function *oaiChunkFuncDelta `json:"function,omitempty"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Index    int                `json:"index"`
}

type oaiChunkFuncDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

// a2oTranscoder translates an Anthropic Messages event stream into OpenAI
// chat.completion.chunk events.
type a2oTranscoder struct {
	toolIdxByBlk map[int]int
	enc          utils.JSONMarshal
	dec          utils.JSONUnmarshal
	finishReason string
	id           string
	model        string
	scan         sseScanner
	created      int64
	inputTokens  int
	outputTokens int
	nextToolIdx  int
	sawUsage     bool
	includeUsage bool
	done         bool
}

func newA2OTranscoder(model string, created int64, includeUsage bool, dec utils.JSONUnmarshal, enc utils.JSONMarshal) *a2oTranscoder {
	return &a2oTranscoder{
		model:        model,
		created:      created,
		includeUsage: includeUsage,
		dec:          dec,
		enc:          enc,
		toolIdxByBlk: make(map[int]int),
		id:           "chatcmpl-gateway",
	}
}

func (t *a2oTranscoder) feed(w *bufio.Writer, chunk []byte) error {
	if t.done {
		return nil
	}
	return t.scan.feed(chunk, func(ev *sseEvent) error { return t.event(w, ev) })
}

func (t *a2oTranscoder) usage() *Usage {
	if !t.sawUsage {
		return nil
	}
	return (&usageFields{InputTokens: t.inputTokens, OutputTokens: t.outputTokens}).toUsage()
}

// emitErrorAndDone terminates the OpenAI stream with an explicit error
// object followed by [DONE] — the shared epilogue of upstream error events
// and truncation.
func (t *a2oTranscoder) emitErrorAndDone(w *bufio.Writer, errType, msg string) error {
	if data, err := oaiErrorJSON(errType, msg, t.enc); err == nil {
		if werr := writeSSE(w, "", data); werr != nil {
			return werr
		}
	}
	return writeSSE(w, "", []byte("[DONE]"))
}

// finish handles a clean upstream EOF without message_stop: the client is
// told about the truncation with an explicit error object before [DONE], and
// the relay records errStreamTruncated on the usage event.
func (t *a2oTranscoder) finish(w *bufio.Writer) error {
	if t.done {
		return nil
	}
	t.done = true
	if err := t.emitErrorAndDone(w, "", errStreamTruncated.Error()); err != nil {
		return err
	}
	return errStreamTruncated
}

func (t *a2oTranscoder) emit(w *bufio.Writer, delta *oaiChunkDelta, finish *string) error {
	chunk := oaiStreamChunk{
		ID:      t.id,
		Object:  "chat.completion.chunk",
		Created: t.created,
		Model:   t.model,
		Choices: []oaiChunkChoice{{Index: 0, Delta: delta, FinishReason: finish}},
	}
	data, err := t.enc(chunk)
	if err != nil {
		return err
	}
	return writeSSE(w, "", data)
}

func (t *a2oTranscoder) event(w *bufio.Writer, ev *sseEvent) error {
	if t.done || len(ev.data) == 0 {
		return nil
	}
	var e antStreamEvent
	if err := t.dec(ev.data, &e); err != nil {
		return fmt.Errorf("aigateway: undecodable upstream stream event: %w", err)
	}
	kind := e.Type
	if kind == "" {
		kind = ev.name
	}

	switch kind {
	case evtMessageStart:
		if e.Message != nil {
			if e.Message.ID != "" {
				t.id = "chatcmpl-" + strings.TrimPrefix(e.Message.ID, "msg_")
			}
			if e.Message.Usage != nil {
				t.inputTokens = max(e.Message.Usage.InputTokens, e.Message.Usage.PromptTokens)
				t.sawUsage = true
			}
		}
		empty := ""
		return t.emit(w, &oaiChunkDelta{Role: roleAssistant, Content: &empty}, nil)

	case evtContentBlockStart:
		if e.ContentBlock == nil || e.ContentBlock.Type != blockToolUse {
			return nil // text blocks open implicitly; thinking is dropped
		}
		idx := t.nextToolIdx
		t.nextToolIdx++
		t.toolIdxByBlk[e.Index] = idx
		return t.emit(w, &oaiChunkDelta{ToolCalls: []oaiChunkToolDelta{{
			Index:    idx,
			ID:       e.ContentBlock.ID,
			Type:     toolTypeFunction,
			Function: &oaiChunkFuncDelta{Name: e.ContentBlock.Name, Arguments: ""},
		}}}, nil)

	case evtContentBlockDelta:
		if e.Delta == nil {
			return nil
		}
		switch e.Delta.Type {
		case "text_delta":
			text := e.Delta.Text
			return t.emit(w, &oaiChunkDelta{Content: &text}, nil)
		case "input_json_delta":
			idx, ok := t.toolIdxByBlk[e.Index]
			if !ok {
				return nil
			}
			return t.emit(w, &oaiChunkDelta{ToolCalls: []oaiChunkToolDelta{{
				Index:    idx,
				Function: &oaiChunkFuncDelta{Arguments: e.Delta.PartialJSON},
			}}}, nil)
		default:
			return nil // thinking/signature deltas: dropped
		}

	case evtMessageDelta:
		if e.Usage != nil {
			t.outputTokens = max(e.Usage.OutputTokens, e.Usage.CompletionTokens)
			if in := max(e.Usage.InputTokens, e.Usage.PromptTokens); in > 0 {
				t.inputTokens = in
			}
			t.sawUsage = true
		}
		if e.Delta != nil && e.Delta.StopReason != nil {
			t.finishReason = anthropicStopToFinish(*e.Delta.StopReason)
			finish := t.finishReason
			return t.emit(w, &oaiChunkDelta{}, &finish)
		}
		return nil

	case evtMessageStop:
		t.done = true
		if t.includeUsage && t.sawUsage {
			chunk := oaiStreamChunk{
				ID:      t.id,
				Object:  "chat.completion.chunk",
				Created: t.created,
				Model:   t.model,
				Choices: []oaiChunkChoice{},
				Usage:   normalizeUsageFields(&usageFields{InputTokens: t.inputTokens, OutputTokens: t.outputTokens}),
			}
			data, err := t.enc(chunk)
			if err != nil {
				return err
			}
			if werr := writeSSE(w, "", data); werr != nil {
				return werr
			}
		}
		return writeSSE(w, "", []byte("[DONE]"))

	case evtError:
		t.done = true
		msg, errType := "upstream error", errTypeAPI
		if e.Error != nil {
			msg, errType = e.Error.Message, e.Error.Type
		}
		return t.emitErrorAndDone(w, errType, msg)

	default:
		// ping, content_block_stop, and future event types translate to
		// nothing. relayStream's keepalive ticker keeps the client
		// connection warm through the resulting silences.
		return nil
	}
}

// ---- OpenAI upstream -> Anthropic client ----

const (
	openBlockNone = iota
	openBlockText
	openBlockTool
)

// o2aTranscoder translates an OpenAI chat.completion.chunk stream into
// Anthropic Messages events.
type o2aTranscoder struct {
	dec          utils.JSONUnmarshal
	enc          utils.JSONMarshal
	model        string
	msgID        string
	finishReason string
	curToolID    string
	scan         sseScanner
	inputTokens  int
	outputTokens int
	blockIdx     int
	curToolIdx   int
	openBlock    int
	started      bool
	sawUsage     bool
	done         bool
}

func newO2ATranscoder(model string, dec utils.JSONUnmarshal, enc utils.JSONMarshal) *o2aTranscoder {
	return &o2aTranscoder{model: model, dec: dec, enc: enc, msgID: "msg_gateway", curToolIdx: -1}
}

func (t *o2aTranscoder) feed(w *bufio.Writer, chunk []byte) error {
	if t.done {
		return nil
	}
	return t.scan.feed(chunk, func(ev *sseEvent) error { return t.event(w, ev) })
}

func (t *o2aTranscoder) usage() *Usage {
	if !t.sawUsage {
		return nil
	}
	return (&usageFields{InputTokens: t.inputTokens, OutputTokens: t.outputTokens}).toUsage()
}

// emitError closes any open block and emits an Anthropic error event — the
// shared epilogue of upstream error chunks and truncation. An error event is
// valid at any point in the stream (the real API emits e.g. overloaded_error
// even before message_start) and client SDKs raise on it.
func (t *o2aTranscoder) emitError(w *bufio.Writer, errType, msg string) error {
	if err := t.closeBlock(w); err != nil {
		return err
	}
	data, err := antErrorJSON(errType, msg, t.enc)
	if err != nil {
		return err
	}
	return writeSSE(w, evtError, data)
}

// finish handles a clean upstream EOF without [DONE]: rather than fabricating
// a completed message (which would present a truncated answer as final), the
// client gets an Anthropic error event, and the relay records
// errStreamTruncated on the usage event.
func (t *o2aTranscoder) finish(w *bufio.Writer) error {
	if t.done {
		return nil
	}
	t.done = true
	if err := t.emitError(w, "", errStreamTruncated.Error()); err != nil {
		return err
	}
	return errStreamTruncated
}

func (t *o2aTranscoder) emitEvent(w *bufio.Writer, name string, payload any) error {
	data, err := t.enc(payload)
	if err != nil {
		return err
	}
	return writeSSE(w, name, data)
}

func (t *o2aTranscoder) start(w *bufio.Writer, chunkID string) error {
	t.started = true
	if chunkID != "" {
		t.msgID = anthropicMessageID(strings.TrimPrefix(chunkID, "chatcmpl-"))
	}
	// input_tokens is unknown until the final usage chunk (documented
	// limitation): report zeros here and truth in message_delta.
	return t.emitEvent(w, evtMessageStart, &antEventPayload{
		Type: evtMessageStart,
		Message: &antStartMessage{
			ID:      t.msgID,
			Type:    typeMessage,
			Role:    roleAssistant,
			Model:   t.model,
			Content: []antBlock{},
			Usage:   &antEventUsage{},
		},
	})
}

func (t *o2aTranscoder) closeBlock(w *bufio.Writer) error {
	if t.openBlock == openBlockNone {
		return nil
	}
	idx := t.blockIdx
	err := t.emitEvent(w, evtContentBlockStop, &antEventPayload{Type: evtContentBlockStop, Index: &idx})
	t.openBlock = openBlockNone
	t.curToolIdx = -1
	t.curToolID = ""
	t.blockIdx++
	return err
}

func (t *o2aTranscoder) terminate(w *bufio.Writer) error {
	t.done = true
	if err := t.closeBlock(w); err != nil {
		return err
	}
	stop := t.finishReason
	if stop == "" {
		stop = stopEndTurn
	} else {
		stop = finishToAnthropicStop(stop)
	}
	if err := t.emitEvent(w, evtMessageDelta, &antEventPayload{
		Type:  evtMessageDelta,
		Delta: &antMsgDelta{StopReason: stop},
		Usage: &antEventUsage{InputTokens: t.inputTokens, OutputTokens: t.outputTokens},
	}); err != nil {
		return err
	}
	return t.emitEvent(w, evtMessageStop, &antEventPayload{Type: evtMessageStop})
}

func (t *o2aTranscoder) event(w *bufio.Writer, ev *sseEvent) error {
	if t.done || len(ev.data) == 0 {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(ev.data), []byte("[DONE]")) {
		if !t.started {
			if err := t.start(w, ""); err != nil {
				return err
			}
		}
		return t.terminate(w)
	}

	var chunk struct {
		Error   *oaiErrorBody `json:"error"`
		Usage   *usageFields  `json:"usage"`
		ID      string        `json:"id"`
		Choices []struct {
			Delta        *oaiChunkDelta `json:"delta"`
			FinishReason *string        `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := t.dec(ev.data, &chunk); err != nil {
		return fmt.Errorf("aigateway: undecodable upstream stream chunk: %w", err)
	}

	if chunk.Error != nil {
		if !t.started {
			if err := t.start(w, chunk.ID); err != nil {
				return err
			}
		}
		t.done = true
		return t.emitError(w, chunk.Error.Type, chunk.Error.Message)
	}

	if !t.started {
		if err := t.start(w, chunk.ID); err != nil {
			return err
		}
	}

	if chunk.Usage != nil {
		t.inputTokens = max(chunk.Usage.PromptTokens, chunk.Usage.InputTokens)
		t.outputTokens = max(chunk.Usage.CompletionTokens, chunk.Usage.OutputTokens)
		t.sawUsage = true
	}
	if len(chunk.Choices) == 0 {
		return nil // usage-only chunk: nothing translates
	}
	choice := &chunk.Choices[0]
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		t.finishReason = *choice.FinishReason
	}
	if choice.Delta == nil {
		return nil
	}

	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		if t.openBlock != openBlockText {
			if err := t.closeBlock(w); err != nil {
				return err
			}
			idx, empty := t.blockIdx, ""
			if err := t.emitEvent(w, evtContentBlockStart, &antEventPayload{
				Type:         evtContentBlockStart,
				Index:        &idx,
				ContentBlock: &antStartBlock{Type: blockText, Text: &empty},
			}); err != nil {
				return err
			}
			t.openBlock = openBlockText
		}
		idx := t.blockIdx
		if err := t.emitEvent(w, evtContentBlockDelta, &antEventPayload{
			Type:  evtContentBlockDelta,
			Index: &idx,
			Delta: &antEventDelta{Type: "text_delta", Text: choice.Delta.Content},
		}); err != nil {
			return err
		}
	}

	for i := range choice.Delta.ToolCalls {
		td := &choice.Delta.ToolCalls[i]
		// A new tool call is signaled by a changed index — or, for upstreams
		// that omit indexes, by a DIFFERENT tool-call id at the same index.
		// An id arriving on a later delta of the same call (open block still
		// has no id) is adopted below, not treated as a new call.
		if t.openBlock != openBlockTool || td.Index != t.curToolIdx ||
			(td.ID != "" && t.curToolID != "" && td.ID != t.curToolID) {
			if err := t.closeBlock(w); err != nil {
				return err
			}
			name, id := "", ""
			if td.Function != nil {
				name = td.Function.Name
			}
			id = td.ID
			idx := t.blockIdx
			if err := t.emitEvent(w, evtContentBlockStart, &antEventPayload{
				Type:  evtContentBlockStart,
				Index: &idx,
				ContentBlock: &antStartBlock{
					Type: blockToolUse, ID: id, Name: name, Input: map[string]any{},
				},
			}); err != nil {
				return err
			}
			t.openBlock = openBlockTool
			t.curToolIdx = td.Index
			t.curToolID = td.ID
		} else if t.curToolID == "" && td.ID != "" {
			t.curToolID = td.ID
		}
		if td.Function != nil && td.Function.Arguments != "" {
			idx := t.blockIdx
			if err := t.emitEvent(w, evtContentBlockDelta, &antEventPayload{
				Type:  evtContentBlockDelta,
				Index: &idx,
				Delta: &antEventDelta{Type: "input_json_delta", PartialJSON: &td.Function.Arguments},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
