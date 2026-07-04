// Package aigateway provides a middleware that lets a Fiber app act as an AI
// gateway: clients point their native OpenAI/Anthropic/OpenRouter/... SDKs at
// the gateway's base URL and the middleware relays each request to the real
// provider, either forwarding the client's own API key or injecting a
// server-side unified key. Responses — including Server-Sent Event token
// streams — are relayed without buffering.
package aigateway

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/redact"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

// The contextKey type is unexported to prevent collisions with context keys
// defined in other packages.
type contextKey int

// The keys for the values stored in the request context.
const (
	clientKeyKey contextKey = iota
	providerKey
	modelKey
)

var registerLogContextTagsOnce sync.Once

// New creates a new middleware handler.
func New(config ...Config) fiber.Handler {
	registerLogContextTagsOnce.Do(registerLogContextTags)

	cfg := configDefault(config...)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Extract and validate the client credential.
		key, err := cfg.KeyExtractor.Extract(c)
		if err != nil {
			if !errors.Is(err, extractors.ErrNotFound) || !cfg.AllowClientKeyMissing {
				return sendError(c, fiber.StatusUnauthorized, "missing or invalid API key", "authentication_error")
			}
			key = ""
		}
		if key != "" {
			if cfg.KeyValidator != nil {
				valid, verr := cfg.KeyValidator(c, key)
				if verr != nil || !valid {
					return sendError(c, fiber.StatusUnauthorized, "invalid API key", "authentication_error")
				}
			}
			fiber.StoreInContext(c, clientKeyKey, key)
		}

		// Resolve and police the upstream path.
		strippedPath := stripPrefix(c.Path(), cfg.PathPrefix)
		if containsDotDot(strippedPath) {
			return sendError(c, fiber.StatusBadRequest, "invalid request path", "invalid_request_error")
		}
		if len(cfg.AllowedPaths) > 0 && !matchAny(cfg.AllowedPaths, strippedPath) {
			return sendError(c, fiber.StatusForbidden, "this endpoint is not allowed by the gateway", "invalid_request_error")
		}

		// Sniff the model from the JSON request body: policed when
		// AllowedModels is set, recorded in the usage event either way.
		model := sniffModel(c)
		if len(cfg.AllowedModels) > 0 && !matchAny(cfg.AllowedModels, model) {
			return sendError(c, fiber.StatusForbidden, "this model is not allowed by the gateway", "invalid_request_error")
		}
		if model != "" {
			fiber.StoreInContext(c, modelKey, model)
		}

		start := time.Now()
		ev := &UsageEvent{
			Model:        model,
			Method:       c.Method(),
			Path:         strippedPath,
			ClientKey:    key,
			RequestBytes: int64(len(c.BodyRaw())),
		}

		resp, sendErr := sendWithRetry(c, &cfg, strippedPath, key, ev)
		if resp == nil {
			ev.Err = sendErr
			fireUsage(&cfg, ev, start)
			return fiber.ErrBadGateway
		}
		fiber.StoreInContext(c, providerKey, ev.Provider)

		if isEventStream(resp) {
			ev.Streamed = true
			return relayStream(c, &cfg, resp, ev, start)
		}
		return relayBuffered(c, &cfg, resp, ev, start)
	}
}

// KeyFromContext returns the client API key from the request context.
// It accepts fiber.Ctx, *fasthttp.RequestCtx, and context.Context.
// It returns an empty string if no key was stored.
func KeyFromContext(ctx any) string {
	if key, ok := fiber.ValueFromContext[string](ctx, clientKeyKey); ok {
		return key
	}
	return ""
}

// ProviderFromContext returns the name of the upstream that served the
// request. It returns an empty string before an upstream was selected.
func ProviderFromContext(ctx any) string {
	if provider, ok := fiber.ValueFromContext[string](ctx, providerKey); ok {
		return provider
	}
	return ""
}

// ModelFromContext returns the model name sniffed from the request body.
// It returns an empty string if the body carried none.
func ModelFromContext(ctx any) string {
	if model, ok := fiber.ValueFromContext[string](ctx, modelKey); ok {
		return model
	}
	return ""
}

func registerLogContextTags() {
	logger.RegisterContextTag("ai-key", func(ctx any) string {
		return redact.Prefix(KeyFromContext(ctx))
	})
	logger.RegisterContextTag("ai-provider", ProviderFromContext)
	logger.RegisterContextTag("ai-model", ModelFromContext)
}

// sendError responds with an OpenAI-style JSON error object so native SDK
// clients can parse gateway-generated failures.
func sendError(c fiber.Ctx, status int, message, errorType string) error {
	c.Status(status)
	return c.JSON(fiber.Map{
		"error": fiber.Map{
			"message": message,
			"type":    errorType,
		},
	})
}

// stripPrefix removes the mount prefix from the request path, keeping the
// result rooted at "/".
func stripPrefix(path, prefix string) string {
	if prefix == "" || !strings.HasPrefix(path, prefix) {
		return path
	}
	stripped := path[len(prefix):]
	if stripped == "" {
		return "/"
	}
	if stripped[0] != '/' {
		// The prefix matched inside a segment (e.g. prefix /open on path
		// /openai): not a mount-point match, leave the path alone.
		return path
	}
	return stripped
}

func containsDotDot(path string) bool {
	for part := range strings.SplitSeq(path, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

// matchAny reports whether val matches one of the patterns, either exactly
// or by trailing-* wildcard. An empty val never matches.
func matchAny(patterns []string, val string) bool {
	if val == "" {
		return false
	}
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(val, p[:len(p)-1]) {
				return true
			}
			continue
		}
		if p == val {
			return true
		}
	}
	return false
}

// sniffModel best-effort decodes the "model" field from a JSON request body.
func sniffModel(c fiber.Ctx) string {
	if !c.Is("json") {
		return ""
	}
	body := c.Body()
	if len(body) == 0 {
		return ""
	}
	var m struct {
		Model string `json:"model"`
	}
	if err := c.App().Config().JSONDecoder(body, &m); err != nil {
		return ""
	}
	return m.Model
}
