package aigateway

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// maxTokenFields are the top-level request fields that cap output length
// across provider dialects; MaxTokensCap clamps whichever are present.
var maxTokenFields = []string{"max_tokens", "max_completion_tokens", "max_output_tokens"}

// errBadMaxTokens rejects a capped max-token field that is not a plain
// integer: a lenient upstream parser could still honor it, defeating the cap.
var errBadMaxTokens = errors.New("aigateway: max tokens field is not an integer")

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
			val, err := strconv.ParseInt(string(raw), 10, 64)
			if err != nil {
				return nil, errBadMaxTokens
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
