package aigateway

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/fiber/v3/extractors"
)

// AuthScheme describes how a credential is carried in a request header.
type AuthScheme struct {
	// Header is the header name, e.g. "Authorization", "x-api-key" or "api-key".
	Header string

	// Scheme is an optional prefix inside the header value, e.g. "Bearer".
	// Empty means the header carries the bare key.
	Scheme string
}

// AuthBearer returns the AuthScheme used by OpenAI-compatible APIs:
// "Authorization: Bearer <key>".
func AuthBearer() AuthScheme {
	return AuthScheme{Header: fiber.HeaderAuthorization, Scheme: "Bearer"}
}

// AuthHeader returns an AuthScheme that carries the bare key in the given
// header, e.g. AuthHeader("x-api-key") for Anthropic.
func AuthHeader(name string) AuthScheme {
	return AuthScheme{Header: name}
}

// Upstream is one relay target. The first Upstream in Config.Upstreams is the
// primary; the rest are ordered fallbacks and must speak the same wire API.
type Upstream struct {
	// Headers are extra headers set on every request to this upstream,
	// e.g. {"anthropic-version": "2023-06-01"}.
	//
	// Optional. Default: nil
	Headers map[string]string

	// ModelMap rewrites the "model" field of JSON request bodies for this
	// upstream: a request for a key model is relayed with the mapped value,
	// e.g. {"gpt-4o": "my-azure-deployment"} — so fallback upstreams that
	// name the same model differently (Azure deployments, another provider's
	// equivalent model) can share a chain. Keys are exact model names.
	// Models without an entry are relayed unchanged. A mapped body is
	// re-encoded (top-level key order may change; other values are preserved
	// byte-for-byte), and a content-encoded body is relayed decoded.
	//
	// Optional. Default: nil
	ModelMap map[string]string

	// Name identifies this upstream in UsageEvent and logger tags.
	//
	// Required.
	Name string

	// URL is the base URL prepended to the (prefix-stripped) request path,
	// e.g. "https://api.openai.com". It must be an absolute http(s) URL.
	//
	// Required.
	URL string

	// Key is the server-side key injected in unified-key mode.
	//
	// Required unless Config.ForwardClientKey is true.
	Key string

	// Auth is how the key is injected upstream.
	//
	// Optional. Default: AuthBearer()
	Auth AuthScheme

	// Weight is this upstream's share of traffic under StrategyWeighted.
	// Ignored by other strategies.
	//
	// Optional. Default: 1 (values <= 0 are normalized to 1)
	Weight int
}

// KeyPolicy is a per-key policy returned by Config.PolicyResolver. Its lists
// tighten (never widen) the gateway-wide AllowedModels/AllowedPaths: a request
// must pass both the global lists and the key's lists.
type KeyPolicy struct {
	// Tenant labels the key's owner; it is carried into UsageEvent.Tenant
	// for per-tenant usage accounting.
	//
	// Optional. Default: ""
	Tenant string

	// AllowedModels restricts this key to the listed models (exact or
	// trailing-* wildcard). Empty adds no per-key model restriction.
	//
	// Optional. Default: nil
	AllowedModels []string

	// AllowedPaths restricts this key to the listed endpoint paths (after
	// PathPrefix strip; exact or trailing-* wildcard). Empty adds no per-key
	// path restriction.
	//
	// Optional. Default: nil
	AllowedPaths []string

	// TokensPerWindow overrides Config.TokensPerWindow for this key:
	// > 0 sets the key's own token limit, 0 inherits the global limit,
	// < 0 exempts the key from token limits entirely.
	//
	// Optional. Default: 0 (inherit)
	TokensPerWindow int64

	// BudgetPerWindow overrides Config.BudgetPerWindow (USD) for this key:
	// > 0 sets the key's own budget, 0 inherits, < 0 exempts the key.
	//
	// Optional. Default: 0 (inherit)
	BudgetPerWindow float64
}

// ModelPrice is the price of one model in USD per million tokens, used to
// compute UsageEvent.Cost.
type ModelPrice struct {
	// InputPerMTok is the USD price per million input (prompt) tokens.
	InputPerMTok float64

	// OutputPerMTok is the USD price per million output (completion) tokens.
	OutputPerMTok float64
}

// RetryConfig controls same-upstream retries and backoff.
type RetryConfig struct {
	// Attempts is the maximum number of tries per upstream. 1 means no
	// same-upstream retry; a failure moves to the next upstream immediately.
	//
	// Optional. Default: 1
	Attempts int

	// Backoff is the initial delay between attempts, doubled per attempt.
	//
	// Optional. Default: 250 * time.Millisecond
	Backoff time.Duration

	// MaxBackoff caps the computed backoff and honored Retry-After values.
	// A Retry-After above the cap skips the wait and moves straight to the
	// next attempt or upstream.
	//
	// Optional. Default: 2 * time.Second
	MaxBackoff time.Duration
}

// Config defines the config for middleware.
//
// Fields are ordered for struct alignment (pointers before scalars); the
// documented, logical grouping is reflected in the docs table.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// KeyValidator validates the client (virtual) key before relaying.
	// Return false or an error to reject the request with 401.
	//
	// SECURITY: in unified-key mode, leaving this nil makes the gateway an
	// open relay for your Upstream.Key.
	//
	// Optional. Default: nil
	KeyValidator func(c fiber.Ctx, key string) (bool, error)

	// PolicyResolver resolves the per-key policy for the client (virtual)
	// key. Returning an error or a nil policy rejects the request with 401,
	// so it can replace KeyValidator; return &KeyPolicy{} to accept a key
	// without extra restrictions. When both are set, KeyValidator runs
	// first. It is not called for keyless requests admitted by
	// AllowClientKeyMissing — only the global allow-lists apply to those.
	//
	// Optional. Default: nil
	PolicyResolver func(c fiber.Ctx, key string) (*KeyPolicy, error)

	// OnUsage is called once per relayed request with request metadata and,
	// when parseable, token usage. For streaming responses it runs on the
	// response writer goroutine after the stream ends and must not touch
	// fiber.Ctx.
	//
	// Optional. Default: nil
	OnUsage func(e *UsageEvent)

	// OnRequest is a guardrail/transform hook that runs after authentication
	// and before any policy check, so the path and body it produces are what
	// the allow-lists inspect and what is relayed. It may mutate
	// RelayRequest.Path and RelayRequest.Body (nil Body = leave the body
	// untouched). Returning an error rejects the request: a *fiber.Error's
	// code is used, any other error maps to 403.
	//
	// Optional. Default: nil
	OnRequest func(c fiber.Ctx, r *RelayRequest) error

	// OnResponse runs for buffered (non-streaming) upstream responses after
	// token usage is parsed and before the response is sent; it may mutate
	// RelayResponse.Body and RelayResponse.Status. Returning an error turns
	// the response into a 502. Streaming responses are relayed pass-through
	// and never invoke it.
	//
	// Optional. Default: nil
	OnResponse func(c fiber.Ctx, r *RelayResponse) error

	// QuotaStore tracks per-identity token/cost totals for quota admission.
	// The identity is KeyPolicy.Tenant when set, else the client key.
	// Setting it (or a global limit below) activates quota enforcement.
	//
	// Optional. Default: an in-process store when quotas are active
	QuotaStore QuotaStore

	// Client is the Fiber client used for upstream requests. Streaming of
	// response bodies is enabled on it during initialization. HeaderTimeout
	// bounds each attempt up to the response headers, so the client should
	// not set its own total timeout.
	//
	// Optional. Default: an internal client
	Client *client.Client

	// stripHeaders is the set (lower-cased header names) of credential headers
	// removed from every upstream request before the upstream key is injected.
	// It is derived in configDefault from the well-known auth headers, every
	// Upstream.Auth.Header, and the header(s) the KeyExtractor reads, so a
	// custom extractor header or auth style cannot leak a client credential
	// upstream or let a client smuggle a second credential through.
	stripHeaders map[string]struct{}

	// Prices maps model names (exact or trailing-* wildcard; the longest
	// wildcard wins) to their price, enabling UsageEvent.Cost. Lookup uses
	// the model the client requested, even when ModelMap relays a different
	// name upstream. Models without an entry yield Cost 0.
	//
	// Optional. Default: nil
	Prices map[string]ModelPrice

	// ParamDefaults injects top-level fields into JSON request bodies when
	// the client did not set them, e.g. {"temperature": 0.2, "user": "gw"}.
	// Values must be JSON-encodable; a "model" key panics at construction
	// (it would bypass the model policy — use Upstream.ModelMap). Defaults
	// are advisory: a client can always send its own value (bounded only by
	// MaxTokensCap for the max-token fields).
	//
	// Optional. Default: nil
	ParamDefaults map[string]any

	// rawParamDefaults is ParamDefaults pre-encoded to raw JSON fragments at
	// construction, so the per-request injection does no re-encoding.
	rawParamDefaults map[string]json.RawMessage

	// rr is the round-robin rotation counter shared by all requests of this
	// mount. A pointer so copying Config does not copy the atomic.
	rr *atomic.Uint64

	// stripQuery and stripCookies name the query params / cookies the
	// KeyExtractor reads the client credential from; they are removed from the
	// relayed request so a query- or cookie-based credential is not forwarded
	// upstream. Derived in configDefault.
	stripQuery   []string
	stripCookies []string

	// breakers holds the per-upstream circuit-breaker state, index-aligned
	// with Upstreams. Nil when BreakerThreshold is 0 (breaker disabled).
	// Created in configDefault.
	breakers []*upstreamBreaker

	// PathPrefix is stripped from the request path before it is joined with
	// Upstream.URL, e.g. "/openai" when mounted as app.Use("/openai", ...).
	//
	// Optional. Default: ""
	PathPrefix string

	// Upstreams is the ordered relay chain: primary first, fallbacks after.
	//
	// Required.
	Upstreams []Upstream

	// AllowedModels restricts the "model" field of JSON request bodies.
	// Entries match exactly or by trailing-* wildcard, e.g. "gpt-4o*". The
	// list only restricts requests that declare a model, so endpoints without
	// one (GET /v1/models, multipart audio) are not blocked; pair with
	// AllowedPaths to bound endpoints. Empty means all models are allowed.
	//
	// Optional. Default: nil
	AllowedModels []string

	// AllowedPaths restricts relayed endpoint paths (after PathPrefix strip)
	// by exact or trailing-* wildcard match, e.g. "/v1/chat/completions".
	// Empty means all paths are allowed.
	//
	// Optional. Default: nil
	AllowedPaths []string

	// KeyExtractor locates the client credential on the incoming request.
	//
	// Optional. Default: Chain(FromAuthHeader("Bearer"),
	// FromHeader("x-api-key"), FromHeader("api-key"))
	KeyExtractor extractors.Extractor

	// Retry controls same-upstream retries and backoff.
	//
	// Optional. Default: see RetryConfig
	Retry RetryConfig

	// HeaderTimeout bounds one attempt from dialing through receiving the
	// upstream response headers (it also covers sending the request body). It
	// does not cap the duration of a streaming response body.
	//
	// Optional. Default: 30 * time.Second
	HeaderTimeout time.Duration

	// StreamIdleTimeout aborts a streaming response when no bytes arrive
	// from upstream for this long. It is an idle timeout, not a total cap,
	// so long-running streams are unaffected while data flows.
	//
	// Optional. Default: 90 * time.Second
	StreamIdleTimeout time.Duration

	// TokensPerWindow is the default per-identity token limit per
	// QuotaWindow. A request is rejected with 429 when its identity's
	// window total already reached the limit (post-paid: actual usage is
	// committed after the response, so one in-flight request may overshoot).
	// 0 means no token limit. Override per key via KeyPolicy.
	//
	// Optional. Default: 0
	TokensPerWindow int64

	// BudgetPerWindow is the default per-identity USD budget per
	// QuotaWindow, enforced like TokensPerWindow using UsageEvent.Cost
	// (so it requires Prices to have an effect). 0 means no budget.
	//
	// Optional. Default: 0
	BudgetPerWindow float64

	// QuotaWindow is the fixed quota window. Totals reset at wall-aligned
	// window boundaries.
	//
	// Optional. Default: time.Hour
	QuotaWindow time.Duration

	// MaxTokensCap clamps the max_tokens, max_completion_tokens, and
	// max_output_tokens fields of JSON request bodies when present and
	// above the cap. Like AllowedModels, an encoded body that cannot be
	// inspected is rejected while a cap is set. 0 disables the cap.
	//
	// Optional. Default: 0
	MaxTokensCap int

	// Strategy selects how the upstream to try first is chosen:
	// StrategyOrdered (the chain order), StrategyRoundRobin, or
	// StrategyWeighted (by Upstream.Weight). Failover after the first
	// choice always proceeds through the remaining candidates.
	//
	// Optional. Default: StrategyOrdered
	Strategy Strategy

	// BreakerCooldown is how long an upstream stays skipped after its
	// breaker opens. When it elapses the upstream is probed again: a
	// success closes the breaker, another failure reopens it.
	//
	// Optional. Default: 30 * time.Second (when BreakerThreshold > 0)
	BreakerCooldown time.Duration

	// BreakerThreshold opens an upstream's circuit breaker after this many
	// consecutive failed attempts (network errors or retryable statuses):
	// the upstream is skipped for BreakerCooldown instead of being retried
	// on every request. When every upstream's breaker is open, all are
	// tried anyway rather than failing outright. 0 disables the breaker.
	//
	// Optional. Default: 0
	BreakerThreshold int

	// MaxResponseSize caps the bytes read from an upstream response,
	// buffered or streamed. Responses exceeding the cap fail with 502
	// (buffered) or are truncated at the cap (streamed). 0 disables the cap.
	//
	// Optional. Default: 0
	MaxResponseSize int64

	// ForwardClientKey relays the client's own credential upstream
	// (pass-through mode). When false, the client credential is stripped
	// and Upstream.Key is injected (unified-key mode).
	//
	// Optional. Default: false
	ForwardClientKey bool

	// AllowClientKeyMissing permits requests without a client credential.
	// Only valid in unified-key mode.
	//
	// Optional. Default: false
	AllowClientKeyMissing bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Retry: RetryConfig{
		Attempts:   1,
		Backoff:    250 * time.Millisecond,
		MaxBackoff: 2 * time.Second,
	},
	HeaderTimeout:     30 * time.Second,
	StreamIdleTimeout: 90 * time.Second,
}

func defaultKeyExtractor() extractors.Extractor {
	return extractors.Chain(
		extractors.FromAuthHeader("Bearer"),
		extractors.FromHeader("x-api-key"),
		extractors.FromHeader("api-key"),
	)
}

// collectExtractorCredentials records, for every extractor in the chain, where
// the client credential is carried so it can be stripped before relaying:
// header names into cfg.stripHeaders, query params into cfg.stripQuery, cookie
// names into cfg.stripCookies. It uses Extractor.Contains, whose traversal is
// cycle-safe (a self-referential chain would otherwise recurse forever).
func (cfg *Config) collectExtractorCredentials() {
	cfg.KeyExtractor.Contains(func(e extractors.Extractor) bool {
		switch e.Source {
		case extractors.SourceForm, extractors.SourceParam, extractors.SourceCustom:
			// Form (request body), route param (path), and custom extractors
			// cannot be stripped without rewriting the body/path (and a custom
			// extractor's location is opaque). In unified-key mode that would
			// silently relay the client credential upstream, so fail fast and
			// require a header/query/cookie extractor instead. In pass-through
			// mode the client credential is meant to go upstream, so it is fine.
			// This runs before the Key check so an empty-key custom extractor
			// cannot slip through unstripped.
			if !cfg.ForwardClientKey {
				panic("fiber: aigateway unified-key mode requires a header, query, or cookie KeyExtractor; a form, param, or custom extractor cannot be stripped and would leak the client credential upstream")
			}
		case extractors.SourceHeader, extractors.SourceAuthHeader:
			if e.Key != "" {
				cfg.stripHeaders[strings.ToLower(e.Key)] = struct{}{}
			}
		case extractors.SourceQuery:
			if e.Key != "" {
				cfg.stripQuery = append(cfg.stripQuery, e.Key)
			}
		case extractors.SourceCookie:
			if e.Key != "" {
				cfg.stripCookies = append(cfg.stripCookies, e.Key)
			}
		}
		return false // visit every extractor; never short-circuit
	})
}

// configDefault is a helper function to set default values
func configDefault(config ...Config) Config {
	if len(config) < 1 || len(config[0].Upstreams) == 0 {
		panic("fiber: aigateway middleware requires at least one upstream")
	}
	cfg := config[0]

	for i := range cfg.Upstreams {
		up := &cfg.Upstreams[i]
		if up.Name == "" {
			panic(fmt.Sprintf("fiber: aigateway upstream #%d requires a name", i))
		}
		if up.URL == "" {
			panic(fmt.Sprintf("fiber: aigateway upstream %q requires a URL", up.Name))
		}
		u, err := url.Parse(up.URL)
		if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			panic(fmt.Sprintf("fiber: aigateway upstream %q requires an absolute http(s) URL", up.Name))
		}
		up.URL = strings.TrimRight(up.URL, "/")
		if up.Auth.Header == "" {
			up.Auth = AuthBearer()
		}
		if up.Key == "" && !cfg.ForwardClientKey {
			panic(fmt.Sprintf("fiber: aigateway upstream %q requires a key (or set ForwardClientKey)", up.Name))
		}
	}

	if cfg.ForwardClientKey && cfg.AllowClientKeyMissing {
		panic("fiber: aigateway cannot combine ForwardClientKey with AllowClientKeyMissing")
	}

	if cfg.PathPrefix != "" {
		if !strings.HasPrefix(cfg.PathPrefix, "/") {
			cfg.PathPrefix = "/" + cfg.PathPrefix
		}
		cfg.PathPrefix = strings.TrimRight(cfg.PathPrefix, "/")
	}

	if cfg.KeyExtractor.Extract == nil {
		cfg.KeyExtractor = defaultKeyExtractor()
	}

	// Build the credential-header strip set: the well-known auth headers, plus
	// every upstream's auth header, plus whatever header(s) the extractor reads.
	cfg.stripHeaders = map[string]struct{}{
		strings.ToLower(fiber.HeaderAuthorization): {},
		"x-api-key": {},
		"api-key":   {},
	}
	for i := range cfg.Upstreams {
		if h := cfg.Upstreams[i].Auth.Header; h != "" {
			cfg.stripHeaders[strings.ToLower(h)] = struct{}{}
		}
	}
	cfg.collectExtractorCredentials()
	if cfg.Retry.Attempts <= 0 {
		cfg.Retry.Attempts = ConfigDefault.Retry.Attempts
	}
	if cfg.Retry.Backoff <= 0 {
		cfg.Retry.Backoff = ConfigDefault.Retry.Backoff
	}
	if cfg.Retry.MaxBackoff <= 0 {
		cfg.Retry.MaxBackoff = ConfigDefault.Retry.MaxBackoff
	}
	if cfg.HeaderTimeout <= 0 {
		cfg.HeaderTimeout = ConfigDefault.HeaderTimeout
	}
	if cfg.StreamIdleTimeout <= 0 {
		cfg.StreamIdleTimeout = ConfigDefault.StreamIdleTimeout
	}
	if cfg.MaxResponseSize < 0 {
		cfg.MaxResponseSize = 0
	}
	if cfg.BreakerThreshold < 0 {
		cfg.BreakerThreshold = 0
	}
	if cfg.BreakerThreshold > 0 {
		if cfg.BreakerCooldown <= 0 {
			cfg.BreakerCooldown = defaultBreakerCooldown
		}
		cfg.breakers = make([]*upstreamBreaker, len(cfg.Upstreams))
		for i := range cfg.breakers {
			cfg.breakers[i] = &upstreamBreaker{}
		}
	}
	for model, price := range cfg.Prices {
		if price.InputPerMTok < 0 || price.OutputPerMTok < 0 {
			panic(fmt.Sprintf("fiber: aigateway price for model %q must not be negative", model))
		}
	}

	switch cfg.Strategy {
	case StrategyOrdered, StrategyRoundRobin, StrategyWeighted:
	default:
		panic(fmt.Sprintf("fiber: aigateway unknown Strategy %d", cfg.Strategy))
	}
	if cfg.Strategy == StrategyRoundRobin {
		cfg.rr = &atomic.Uint64{}
	}
	for i := range cfg.Upstreams {
		if cfg.Upstreams[i].Weight <= 0 {
			cfg.Upstreams[i].Weight = 1
		}
	}

	if cfg.TokensPerWindow < 0 || cfg.BudgetPerWindow < 0 {
		panic("fiber: aigateway global TokensPerWindow/BudgetPerWindow must not be negative (per-key exemptions go in KeyPolicy)")
	}
	if cfg.QuotaWindow <= 0 {
		cfg.QuotaWindow = defaultQuotaWindow
	}
	// Quotas are active when a store is supplied or a global limit is set.
	// Per-key-only limits need one of those to activate the machinery.
	if cfg.QuotaStore == nil && (cfg.TokensPerWindow > 0 || cfg.BudgetPerWindow > 0) {
		cfg.QuotaStore = newMemoryQuotaStore()
	}

	if cfg.MaxTokensCap < 0 {
		cfg.MaxTokensCap = 0
	}

	if len(cfg.ParamDefaults) > 0 {
		cfg.rawParamDefaults = make(map[string]json.RawMessage, len(cfg.ParamDefaults))
		for k, v := range cfg.ParamDefaults {
			if k == "model" {
				panic("fiber: aigateway ParamDefaults must not set \"model\" (it would bypass the model policy; use Upstream.ModelMap)")
			}
			raw, err := json.Marshal(v)
			if err != nil {
				panic(fmt.Sprintf("fiber: aigateway ParamDefaults[%q] is not JSON-encodable: %v", k, err))
			}
			cfg.rawParamDefaults[k] = raw
		}
	}

	if cfg.Client == nil {
		cfg.Client = client.New()
	}
	cfg.Client.SetStreamResponseBody(true)
	cfg.Client.SetDisablePathNormalizing(true)

	return cfg
}
