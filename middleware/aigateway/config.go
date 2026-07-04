package aigateway

import (
	"fmt"
	"net/url"
	"strings"
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

	// OnUsage is called once per relayed request with request metadata and,
	// when parseable, token usage. For streaming responses it runs on the
	// response writer goroutine after the stream ends and must not touch
	// fiber.Ctx.
	//
	// Optional. Default: nil
	OnUsage func(e *UsageEvent)

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

	// stripQuery and stripCookies name the query params / cookies the
	// KeyExtractor reads the client credential from; they are removed from the
	// relayed request so a query- or cookie-based credential is not forwarded
	// upstream. Derived in configDefault.
	stripQuery   []string
	stripCookies []string

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
		if e.Key == "" {
			return false
		}
		switch e.Source {
		case extractors.SourceHeader, extractors.SourceAuthHeader:
			cfg.stripHeaders[strings.ToLower(e.Key)] = struct{}{}
		case extractors.SourceQuery:
			cfg.stripQuery = append(cfg.stripQuery, e.Key)
		case extractors.SourceCookie:
			cfg.stripCookies = append(cfg.stripCookies, e.Key)
		case extractors.SourceForm, extractors.SourceParam, extractors.SourceCustom:
			// Form (request body), route param (path), and custom extractors
			// cannot be stripped without rewriting the body/path (and a custom
			// extractor's location is opaque). In unified-key mode that would
			// silently relay the client credential upstream, so fail fast and
			// require a header/query/cookie extractor instead. In pass-through
			// mode the client credential is meant to go upstream, so it is fine.
			if !cfg.ForwardClientKey {
				panic("fiber: aigateway unified-key mode requires a header, query, or cookie KeyExtractor; a form, param, or custom extractor cannot be stripped and would leak the client credential upstream")
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

	if cfg.Client == nil {
		cfg.Client = client.New()
	}
	cfg.Client.SetStreamResponseBody(true)
	cfg.Client.SetDisablePathNormalizing(true)

	return cfg
}
