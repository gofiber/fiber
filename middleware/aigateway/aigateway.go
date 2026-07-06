// Package aigateway provides a middleware that lets a Fiber app act as an AI
// gateway: clients point their native OpenAI/Anthropic/OpenRouter/... SDKs at
// the gateway's base URL and the middleware relays each request to the real
// provider, either forwarding the client's own API key or injecting a
// server-side unified key. Responses — including Server-Sent Event token
// streams — are relayed without buffering.
package aigateway

import (
	"bytes"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/redact"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/utils/v2"
)

// The contextKey type is unexported to prevent collisions with context keys
// defined in other packages.
type contextKey int

// The keys for the values stored in the request context.
const (
	clientKeyKey contextKey = iota
	providerKey
	modelKey
	dialectKey
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

		// Detect the client's wire dialect from the chat endpoint path before
		// anything can fail, so every gateway-generated error — including the
		// auth errors below — is shaped for the caller's SDK.
		strippedPath := stripPrefix(c.Path(), cfg.PathPrefix)
		clientDialect := chatDialectForPath(decodePath(strippedPath))
		fiber.StoreInContext(c, dialectKey, clientDialect)

		// Extract and validate the client credential.
		key, err := cfg.KeyExtractor.Extract(c)
		if err != nil {
			if !errors.Is(err, extractors.ErrNotFound) || !cfg.AllowClientKeyMissing {
				return sendError(c, fiber.StatusUnauthorized, "missing or invalid API key", "authentication_error")
			}
			key = ""
		}
		var policy *KeyPolicy
		if key != "" {
			if cfg.KeyValidator != nil {
				valid, verr := cfg.KeyValidator(c, key)
				if verr != nil || !valid {
					return sendError(c, fiber.StatusUnauthorized, "invalid API key", "authentication_error")
				}
			}
			if cfg.PolicyResolver != nil {
				p, perr := cfg.PolicyResolver(c, key)
				if perr != nil || p == nil {
					return sendError(c, fiber.StatusUnauthorized, "invalid API key", "authentication_error")
				}
				policy = p
			}
			fiber.StoreInContext(c, clientKeyKey, key)
		}

		// The OnRequest hook runs after authentication (so KeyFromContext
		// works inside it) and before every policy check, so the path and
		// body it produces are exactly what gets policed and relayed — a
		// hook cannot bypass the allow-lists.
		if cfg.OnRequest != nil {
			r := &RelayRequest{Path: strippedPath}
			if herr := cfg.OnRequest(c, r); herr != nil {
				return sendError(c, hookStatus(herr), herr.Error(), "invalid_request_error")
			}
			if r.Path != strippedPath {
				// The hook rewrote the path: the client dialect follows it.
				strippedPath = r.Path
				clientDialect = chatDialectForPath(decodePath(strippedPath))
				fiber.StoreInContext(c, dialectKey, clientDialect)
			}
			if r.Body != nil {
				// The replacement body is identity-encoded: drop any stale
				// Content-Encoding so the model sniff and the relay treat it
				// as plain bytes.
				c.Request().SetBodyRaw(r.Body)
				c.Request().Header.Del(fiber.HeaderContentEncoding)
			}
		}

		// Resolve and police the upstream path. Policy checks run on the
		// percent-decoded path so encoded traversal (e.g. %2e%2e) cannot slip
		// past the allow-list; the original path is what gets relayed.
		decodedPath := decodePath(strippedPath)
		if containsDotDot(decodedPath) {
			return sendError(c, fiber.StatusBadRequest, "invalid request path", "invalid_request_error")
		}
		if len(cfg.AllowedPaths) > 0 && !matchAny(cfg.AllowedPaths, decodedPath) {
			return sendError(c, fiber.StatusForbidden, "this endpoint is not allowed by the gateway", "invalid_request_error")
		}
		if policy != nil && len(policy.AllowedPaths) > 0 && !matchAny(policy.AllowedPaths, decodedPath) {
			return sendError(c, fiber.StatusForbidden, "this endpoint is not allowed for this API key", "invalid_request_error")
		}

		// Sniff the model from the JSON request body. The allow-lists only
		// restrict requests that actually declare a model, so non-model
		// endpoints (GET /v1/models, multipart audio uploads) are not blocked;
		// pair AllowedModels with AllowedPaths to bound endpoints. A request
		// whose model the gateway cannot verify (a content-encoded body it can't
		// decode) is rejected when any model policy — global or per-key — is
		// set, so an encoded body cannot smuggle a disallowed model past the
		// check.
		model, verifiable, jsonBody := sniffModel(c)
		if len(cfg.AllowedModels) > 0 || (policy != nil && len(policy.AllowedModels) > 0) {
			if !verifiable {
				return sendError(c, fiber.StatusForbidden, "the gateway could not verify the model of an encoded request body", "invalid_request_error")
			}
			if model != "" {
				if len(cfg.AllowedModels) > 0 && !matchAny(cfg.AllowedModels, model) {
					return sendError(c, fiber.StatusForbidden, "this model is not allowed by the gateway", "invalid_request_error")
				}
				if policy != nil && len(policy.AllowedModels) > 0 && !matchAny(policy.AllowedModels, model) {
					return sendError(c, fiber.StatusForbidden, "this model is not allowed for this API key", "invalid_request_error")
				}
			}
		}
		if model != "" {
			fiber.StoreInContext(c, modelKey, model)
		}

		// Enforce the request parameter policy (defaults injected when
		// absent, max-token fields clamped to the cap) on JSON bodies. Like
		// the model check, the cap is not bypassable: an encoded body that
		// cannot be inspected is rejected while a cap is set.
		if cfg.MaxTokensCap > 0 || len(cfg.rawParamDefaults) > 0 {
			if cfg.MaxTokensCap > 0 && !verifiable {
				return sendError(c, fiber.StatusForbidden, "the gateway could not verify the parameters of an encoded request body", "invalid_request_error")
			}
			if jsonBody != nil {
				newBody, perr := applyParamPolicy(c, &cfg, jsonBody)
				if perr != nil {
					return sendError(c, fiber.StatusBadRequest, "invalid request parameters", "invalid_request_error")
				}
				if newBody != nil {
					// Same write-back contract as the OnRequest hook: the
					// re-encoded body is identity-encoded and becomes what
					// ModelMap and the relay see.
					c.Request().SetBodyRaw(newBody)
					c.Request().Header.Del(fiber.HeaderContentEncoding)
					jsonBody = newBody
				}
			}
		}

		// Quota admission (post-paid): reject when the identity's totals for
		// the current window already reached its limits; the actual usage of
		// this request is committed after the response in fireUsage. The
		// identity is the tenant when the policy names one, else the key.
		var quotaID string
		if cfg.QuotaStore != nil && key != "" {
			quotaID = key
			if policy != nil && policy.Tenant != "" {
				quotaID = policy.Tenant
			}
			limitTokens, limitBudget := effectiveQuota(&cfg, policy)
			if limitTokens > 0 || limitBudget > 0 {
				usedTokens, usedCost, qerr := cfg.QuotaStore.Peek(quotaID, cfg.QuotaWindow)
				if qerr != nil {
					// Fail closed: an unreachable store must not turn limits off.
					return sendError(c, fiber.StatusBadGateway, "quota store unavailable", "api_error")
				}
				if (limitTokens > 0 && usedTokens >= limitTokens) || (limitBudget > 0 && usedCost >= limitBudget) {
					c.Set(fiber.HeaderRetryAfter, strconv.Itoa(quotaRetryAfter(cfg.QuotaWindow)))
					return sendError(c, fiber.StatusTooManyRequests, "quota exceeded for this API key", "rate_limit_error")
				}
			} else {
				// Exempt identity: skip the commit as well.
				quotaID = ""
			}
		}

		start := time.Now()
		// Method/Path/ClientKey are backed by the pooled request/ctx buffers,
		// which are recycled once the handler returns. The streaming usage hook
		// runs after that, so copy every ctx-derived string into owned memory
		// here — one place, so a newly added field can't be missed on the async
		// path. (Model is already an owned string from the JSON decode.)
		ev := &UsageEvent{
			Model:        model,
			Method:       utils.CopyString(c.Method()),
			Path:         utils.CopyString(strippedPath),
			ClientKey:    utils.CopyString(key),
			RequestBytes: int64(len(c.BodyRaw())),
		}
		if policy != nil {
			// Tenant comes from the resolver, not the pooled ctx buffers, so
			// it needs no copy for the async streaming path.
			ev.Tenant = policy.Tenant
		}
		// The quota commit runs on the async streaming path, so the identity
		// must be owned memory: the tenant is resolver-owned, but the raw key
		// aliases pooled ctx buffers — use the owned ClientKey copy for it.
		if quotaID != "" {
			if quotaID == key {
				ev.quotaID = ev.ClientKey
			} else {
				ev.quotaID = quotaID
			}
		}

		resp, served, sendErr := sendWithRetry(c, &cfg, strippedPath, key, clientDialect, ev, jsonBody)
		if resp == nil {
			ev.Err = sendErr
			fireUsage(&cfg, ev, start)
			if errors.Is(sendErr, errUntranslatable) {
				// Nothing upstream-side failed: the request itself cannot be
				// expressed in any reachable upstream's dialect.
				return sendError(c, fiber.StatusBadRequest, sendErr.Error(), "invalid_request_error")
			}
			return fiber.ErrBadGateway
		}
		fiber.StoreInContext(c, providerKey, ev.Provider)

		// The serving upstream's dialect decides response translation; on
		// exhaustion this is whichever upstream produced the relayed error.
		xlateFrom := DialectUnspecified
		if served != nil && needsTranslation(clientDialect, served.Dialect) {
			xlateFrom = served.Dialect
		}

		if isStreamingResponse(resp) {
			ev.Streamed = true
			var tc streamTranscoder
			if xlateFrom != DialectUnspecified {
				tc = newTranscoder(c, resp, xlateFrom, ev.Model, jsonBody)
				if tc == nil {
					// Encoded or non-SSE streaming responses cannot be
					// transcoded; drop the connection rather than relay a
					// stream the client's SDK cannot parse.
					abortUpstreamResponse(resp)
					ev.Err = errUntranslatableResponse
					fireUsage(&cfg, ev, start)
					return sendError(c, fiber.StatusBadGateway, "the upstream stream cannot be translated", "api_error")
				}
			}
			return relayStream(c, &cfg, resp, ev, start, tc)
		}
		return relayBuffered(c, &cfg, resp, ev, start, xlateFrom)
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
	logger.RegisterContextTag(fiberlog.TagAIKey, func(ctx any) string {
		return redact.Prefix(KeyFromContext(ctx))
	})
	logger.RegisterContextTag(fiberlog.TagAIProvider, ProviderFromContext)
	logger.RegisterContextTag(fiberlog.TagAIModel, ModelFromContext)
}

// sendError responds with a JSON error object in the client's dialect so
// native SDK clients can parse gateway-generated failures: the Anthropic
// error envelope for /v1/messages callers, the OpenAI shape otherwise. The
// error type strings the gateway uses (authentication_error,
// invalid_request_error, rate_limit_error, api_error) are valid in both
// dialects verbatim.
func sendError(c fiber.Ctx, status int, message, errorType string) error {
	c.Status(status)
	if d, ok := fiber.ValueFromContext[Dialect](c, dialectKey); ok && d == DialectAnthropic {
		return c.JSON(antErrorEnvelope{
			Type:  evtError,
			Error: &antErrorBody{Type: errorType, Message: message},
		})
	}
	return c.JSON(oaiErrorEnvelope{
		Error: &oaiErrorBody{Message: message, Type: errorType},
	})
}

// stripPrefix removes the mount prefix from the request path, keeping the
// result rooted at "/". The comparison is case-insensitive because Fiber's
// default routing (CaseSensitive: false) matches the mount case-insensitively,
// so /OpenAI must strip a "/openai" prefix just as /openai does.
func stripPrefix(path, prefix string) string {
	if prefix == "" || len(path) < len(prefix) || !utils.EqualFold(path[:len(prefix)], prefix) {
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

// decodePath fully percent-decodes a request path for policy checks. It
// decodes repeatedly so multiply-encoded traversal (e.g. %252e%252e, which a
// single decode leaves as %2e%2e) is resolved before the dot-dot and
// allow-list checks. On a malformed escape it returns the last good form so
// the still-encoded remainder is inspected.
func decodePath(path string) string {
	// A handful of passes is far more than any legitimate path needs; the cap
	// bounds work on adversarial deeply-nested encodings.
	for range 5 {
		if !strings.ContainsRune(path, '%') {
			break
		}
		decoded, err := url.PathUnescape(path)
		if err != nil || decoded == path {
			break
		}
		path = decoded
	}
	return path
}

func containsDotDot(path string) bool {
	// Treat a backslash as a separator too: some upstreams normalize "\" to
	// "/", so "..\..\x" must be caught as traversal even though URL path
	// segments split on "/". ReplaceAll returns path unchanged (no alloc) when
	// it has no backslash, the common case.
	for part := range strings.SplitSeq(strings.ReplaceAll(path, `\`, "/"), "/") {
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

// utf8BOM is the UTF-8 byte-order mark some clients prepend to JSON bodies.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// sniffModel best-effort decodes the "model" field from a JSON request body and
// reports whether the body could actually be inspected for one. It keys off the
// body shape (a leading '{') rather than the Content-Type, so a JSON body sent
// with a non-JSON Content-Type cannot hide its model from the AllowedModels
// check.
//
// The second result is false only when the body arrived content-encoded and the
// gateway could not decode it to a JSON object — an unknown, stacked, or
// oversized (bomb) encoding. A caller enforcing AllowedModels must reject such a
// request rather than forward a model it could not check (the upstream would
// decode and run it). An uncompressed non-JSON body (multipart audio, binary) is
// verifiable with an empty model: a genuine non-model request that is left
// unrestricted.
//
// The third result is the (content-decoded, BOM/whitespace-trimmed) JSON object
// bytes when the body decoded successfully, for Upstream.ModelMap rewriting; it
// is nil otherwise. It may alias the pooled request buffer, so it must only be
// used before the handler returns.
//
//nolint:gocritic // the results are documented above; naming them would violate nonamedreturns
func sniffModel(c fiber.Ctx) (string, bool, []byte) {
	raw := c.BodyRaw()
	body := raw
	encoded := false
	if enc := c.Get(fiber.HeaderContentEncoding); enc != "" && !strings.EqualFold(strings.TrimSpace(enc), "identity") {
		encoded = true
		// Decode within min(BodyLimit, sniffDecodeMax): never more than the body
		// the server already accepts uncompressed, and never more than the fixed
		// ceiling (so a large BodyLimit can't turn a tiny bomb into a huge
		// decompression). On failure, fall back to inspecting the raw body: a
		// stale Content-Encoding header on a plain JSON body (a common
		// intermediary footgun) then still sniffs cleanly, while a genuinely
		// encoded body's raw bytes are not JSON and stay unverifiable below.
		limit := int64(c.App().Config().BodyLimit)
		if limit <= 0 || limit > sniffDecodeMax {
			limit = sniffDecodeMax
		}
		if decoded, ok := boundedDecompress(enc, raw, limit); ok {
			body = decoded
		}
	}

	// Strip any mix of leading whitespace and UTF-8 BOMs, in any order.
	for {
		trimmed := bytes.TrimPrefix(bytes.TrimLeft(body, " \t\r\n"), utf8BOM)
		if len(trimmed) == len(body) {
			break
		}
		body = trimmed
	}

	if len(body) > 0 && body[0] == '{' {
		// The body declares itself a JSON object. To allow it under a model
		// policy we must be able to read its model; a body we cannot decode
		// (trailing data, excessive depth, a still-encoded layer) is
		// unverifiable regardless of Content-Encoding, since a more lenient
		// upstream parser could still extract a disallowed model from it.
		var m struct {
			Model string `json:"model"`
		}
		if err := c.App().Config().JSONDecoder(body, &m); err != nil {
			return "", false, nil
		}
		return m.Model, true, body
	}

	// Not a JSON object. A content-encoded body we could not turn into JSON
	// cannot be checked; an uncompressed non-JSON body (multipart audio, binary)
	// is a genuine non-model request and is left unrestricted.
	return "", !encoded, nil
}
