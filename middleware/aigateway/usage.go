package aigateway

import (
	"bytes"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
)

// Usage holds token counts parsed from a provider response. Field names are
// normalized across providers: OpenAI's prompt/completion tokens and
// Anthropic's input/output tokens map to InputTokens/OutputTokens.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// UsageEvent is passed to Config.OnUsage once per relayed request.
type UsageEvent struct {
	// Err records a relay failure: no upstream reachable, an aborted
	// stream, or a client disconnect. Nil on success.
	Err error

	// Usage holds token counts when they could be parsed from the response.
	// Nil when not parseable.
	Usage *Usage

	// quotaID is the identity the request's usage is committed against
	// (tenant or client key), or "" when quotas do not apply to it. Owned
	// memory: the commit runs on the stream writer goroutine.
	quotaID string

	// ClientKey is the raw credential the client presented. Treat it as
	// sensitive: redact before logging.
	ClientKey string

	// Provider is the Upstream.Name that served the request, or the last
	// upstream tried when all failed.
	Provider string

	// Tenant is KeyPolicy.Tenant from the resolved per-key policy.
	// Empty when no PolicyResolver is set or the policy carries no tenant.
	Tenant string

	// Model is the "model" field sniffed from the JSON request body.
	// Empty when the body had none.
	Model string

	// Method is the HTTP method of the relayed request.
	Method string

	// Path is the upstream request path (after PathPrefix stripping).
	Path string

	// SkippedUpstreams names upstreams that were skipped for this request
	// because their circuit breaker was open. Nil when none were skipped.
	SkippedUpstreams []string

	// Latency is the total relay duration including retries; for streamed
	// responses it runs until the stream ends.
	Latency time.Duration

	// RequestBytes is the size of the relayed request body.
	RequestBytes int64

	// ResponseBytes counts response body bytes. For buffered responses it is
	// the bytes sent to the client (after any translation or OnResponse
	// rewrite); for streamed responses it counts upstream bytes — under a
	// stream transcoder the client-side byte count differs, and
	// MaxResponseSize likewise bounds upstream bytes there.
	ResponseBytes int64

	// StatusCode is the upstream response status, or 0 when no upstream
	// produced a response.
	StatusCode int

	// Attempts is the number of upstream attempts performed.
	Attempts int

	// Cost is the request's price in USD, computed from Usage and
	// Config.Prices (looked up by the model the client requested). Zero when
	// usage was unparseable, no price is configured, or the model is unknown.
	Cost float64

	// Streamed reports whether the response was relayed as a stream.
	Streamed bool
}

// usageFields tolerates both OpenAI and Anthropic usage shapes.
type usageFields struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
}

func (f *usageFields) toUsage() *Usage {
	u := &Usage{
		InputTokens:  max(f.PromptTokens, f.InputTokens),
		OutputTokens: max(f.CompletionTokens, f.OutputTokens),
		TotalTokens:  f.TotalTokens,
	}
	if u.InputTokens == 0 && u.OutputTokens == 0 && u.TotalTokens == 0 {
		return nil
	}
	if u.TotalTokens == 0 {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}
	return u
}

// parseUsage extracts the top-level "usage" object from a JSON response body.
// It returns nil when the body has no usable usage information.
func parseUsage(body []byte, decoder utils.JSONUnmarshal) *Usage {
	var payload struct {
		Usage *usageFields `json:"usage"`
	}
	if err := decoder(body, &payload); err != nil || payload.Usage == nil {
		return nil
	}
	return payload.Usage.toUsage()
}

// applyCost fills ev.Cost from cfg.Prices and the parsed usage. It uses the
// model the client requested (ev.Model): billing follows what was asked for,
// and a ModelMap rewrite is an upstream naming detail.
func applyCost(cfg *Config, ev *UsageEvent) {
	if ev.Usage == nil || ev.Model == "" || len(cfg.Prices) == 0 {
		return
	}
	price, ok := lookupPrice(cfg.Prices, ev.Model)
	if !ok {
		return
	}
	const mTok = 1e6
	ev.Cost = float64(ev.Usage.InputTokens)*price.InputPerMTok/mTok +
		float64(ev.Usage.OutputTokens)*price.OutputPerMTok/mTok
}

// lookupPrice resolves a model's price: an exact entry wins, otherwise the
// longest matching trailing-* wildcard entry (longest = most specific, and
// deterministic regardless of map iteration order).
func lookupPrice(prices map[string]ModelPrice, model string) (ModelPrice, bool) {
	if p, ok := prices[model]; ok {
		return p, true
	}
	var best ModelPrice
	bestLen := -1
	for pattern, p := range prices {
		if !strings.HasSuffix(pattern, "*") {
			continue
		}
		prefix := pattern[:len(pattern)-1]
		if strings.HasPrefix(model, prefix) && len(prefix) > bestLen {
			best = p
			bestLen = len(prefix)
		}
	}
	return best, bestLen >= 0
}

const (
	// usageTailMaxLine caps the size of a buffered SSE line so a
	// misbehaving upstream cannot grow gateway memory unboundedly.
	usageTailMaxLine = 64 * 1024
	ssePrefix        = "data:"
)

var usageNeedle = []byte(`"usage"`)

// usageTail scans a relayed SSE byte stream for "data:" lines that mention
// usage, keeping the first and last candidates. OpenAI reports usage in the
// final chunk before [DONE] (earlier chunks carry "usage":null); Anthropic
// reports input tokens in message_start and output tokens in the final
// message_delta. Merging first and last covers both without buffering the
// stream.
type usageTail struct {
	first   []byte
	last    []byte
	carry   []byte
	skipped bool // current line exceeded usageTailMaxLine and is discarded
}

// observe consumes the next relayed chunk. It only copies lines that mention
// usage; everything else is scanned in place.
func (t *usageTail) observe(chunk []byte) {
	for len(chunk) > 0 {
		nl := bytes.IndexByte(chunk, '\n')
		if nl == -1 {
			if t.skipped {
				return
			}
			if len(t.carry)+len(chunk) > usageTailMaxLine {
				t.carry = t.carry[:0]
				t.skipped = true
				return
			}
			t.carry = append(t.carry, chunk...)
			return
		}
		line := chunk[:nl]
		chunk = chunk[nl+1:]
		if t.skipped {
			t.skipped = false
			continue
		}
		if len(t.carry) > 0 {
			line = append(t.carry, line...)
			t.carry = nil
		}
		t.observeLine(line)
	}
}

func (t *usageTail) observeLine(line []byte) {
	line = bytes.TrimRight(line, "\r")
	if !bytes.HasPrefix(line, []byte(ssePrefix)) || !bytes.Contains(line, usageNeedle) {
		return
	}
	payload := bytes.TrimSpace(line[len(ssePrefix):])
	if len(payload) == 0 || payload[0] != '{' {
		return
	}
	if t.first == nil {
		t.first = append([]byte(nil), payload...)
		return
	}
	t.last = append(t.last[:0], payload...)
}

// usage merges the retained candidates into a Usage, or returns nil when
// none decoded to usable token counts.
func (t *usageTail) usage(decoder utils.JSONUnmarshal) *Usage {
	var merged usageFields
	found := false
	for _, payload := range [][]byte{t.first, t.last} {
		if payload == nil {
			continue
		}
		// Usage sits at the top level in OpenAI chunks and Anthropic
		// message_delta events, but nests under "message" in Anthropic
		// message_start events.
		var chunk struct {
			Usage   *usageFields `json:"usage"`
			Message struct {
				Usage *usageFields `json:"usage"`
			} `json:"message"`
		}
		if err := decoder(payload, &chunk); err != nil {
			continue
		}
		for _, f := range []*usageFields{chunk.Usage, chunk.Message.Usage} {
			if f == nil {
				continue
			}
			merged.PromptTokens = max(merged.PromptTokens, f.PromptTokens)
			merged.CompletionTokens = max(merged.CompletionTokens, f.CompletionTokens)
			merged.TotalTokens = max(merged.TotalTokens, f.TotalTokens)
			merged.InputTokens = max(merged.InputTokens, f.InputTokens)
			merged.OutputTokens = max(merged.OutputTokens, f.OutputTokens)
			found = true
		}
	}
	if !found {
		return nil
	}
	return merged.toUsage()
}
