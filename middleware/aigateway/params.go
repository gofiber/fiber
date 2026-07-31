package aigateway

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// maxTokenFields are the top-level request fields that cap output length
// across provider dialects; MaxTokensCap clamps whichever are present.
var maxTokenFields = []string{"max_tokens", "max_completion_tokens", "max_output_tokens"}

// errBadMaxTokens rejects a capped max-token field that is not an integral
// JSON number or null: a lenient upstream parser could still honor it,
// defeating the cap.
var errBadMaxTokens = errors.New("aigateway: max tokens field is not an integer")

// maxTokensValue reads a max-token field's raw JSON as an integer. It returns
// -1 for JSON null, which both dialects treat as "unset". Float and exponent
// spellings of an integer (100.0, 1e3) are accepted because JSON has a single
// number type and SDKs emit them; a fractional value is not, since clamping it
// would silently change a value the upstream would have rejected.
func maxTokensValue(raw json.RawMessage) (int64, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "null" {
		return -1, true
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) ||
		f > float64(math.MaxInt64) || f < float64(math.MinInt64) {
		return 0, false
	}
	return int64(f), true
}

// applyParamPolicy applies ParamDefaults (inject when absent) and
// MaxTokensCap (clamp when present and above the cap) to a decoded JSON
// object body. It returns the re-encoded body when something changed, or nil
// when the body is already compliant — the caller then relays the original
// bytes untouched. Like rewriteForUpstream, only the top level is decoded, so
// every other value survives byte-for-byte.
func applyParamPolicy(c fiber.Ctx, cfg *Config, jsonBody []byte) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := c.App().Config().JSONDecoder(jsonBody, &obj); err != nil {
		return nil, err
	}

	changed := false
	for k, v := range cfg.rawParamDefaults {
		if _, ok := obj[k]; !ok {
			obj[k] = v
			changed = true
		}
	}

	if cfg.MaxTokensCap > 0 {
		for _, field := range maxTokenFields {
			raw, ok := obj[field]
			if !ok {
				continue
			}
			val, ok := maxTokensValue(raw)
			if !ok {
				return nil, errBadMaxTokens
			}
			if val < 0 {
				// JSON null: both dialects read it as "unset", so there is
				// nothing to clamp and nothing to reject.
				continue
			}
			if val > int64(cfg.MaxTokensCap) {
				obj[field] = json.RawMessage(strconv.Itoa(cfg.MaxTokensCap))
				changed = true
			}
		}
	}

	if !changed {
		return nil, nil
	}
	out, err := c.App().Config().JSONEncoder(obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// requestStreamUsage opts a streaming OpenAI-dialect request into the trailing
// usage chunk (stream_options.include_usage) so post-paid quota metering has
// something to commit. Without it an OpenAI-compatible upstream reports no
// usage for a stream at all, and `"stream": true` would silently bypass
// TokensPerWindow/BudgetPerWindow. It returns nil when the body is not a
// stream request or already made the choice itself — the client's own
// stream_options is never overridden.
func requestStreamUsage(c fiber.Ctx, jsonBody []byte) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := c.App().Config().JSONDecoder(jsonBody, &obj); err != nil {
		return nil, err
	}
	var streaming bool
	if raw, ok := obj["stream"]; !ok {
		return nil, nil
	} else if err := c.App().Config().JSONDecoder(raw, &streaming); err != nil || !streaming {
		return nil, nil //nolint:nilerr // a non-bool "stream" is the upstream's problem, not ours
	}
	if _, ok := obj["stream_options"]; ok {
		return nil, nil
	}
	obj["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	out, err := c.App().Config().JSONEncoder(obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}
