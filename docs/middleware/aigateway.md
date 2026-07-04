---
id: aigateway
---

# AI Gateway

The AI Gateway middleware turns a Fiber app into a gateway for LLM provider APIs (OpenAI, Anthropic, OpenRouter, Azure OpenAI, and any compatible endpoint). Clients point their native SDKs at the gateway's base URL; the middleware relays each request to the real provider and streams the response back — including Server-Sent Event token streams, which are forwarded chunk by chunk without buffering.

It is a pass-through relay: no request or response translation happens, so clients speak each provider's native wire API. On top of the relay, the middleware handles:

- **Key management** — forward the client's own credential upstream, or strip it and inject a server-side unified key, optionally validating client (virtual) keys first.
- **Policy** — restrict which models and endpoint paths may be used.
- **Resilience** — retry retryable failures (429/5xx, network errors) with backoff and fail over across an ordered chain of upstreams.
- **Usage accounting** — a per-request hook with latency, status, attempts, and token usage parsed from JSON responses and SSE streams (best-effort).

## Signatures

```go
func New(config ...Config) fiber.Handler
func KeyFromContext(ctx any) string
func ProviderFromContext(ctx any) string
func ModelFromContext(ctx any) string

// Upstream presets
func OpenAI(key string) Upstream
func Anthropic(key string) Upstream
func OpenRouter(key string) Upstream
func AzureOpenAI(endpoint, key string) Upstream

// Auth header styles
func AuthBearer() AuthScheme
func AuthHeader(name string) AuthScheme
```

The `...FromContext` helpers accept a `fiber.Ctx`, a `*fasthttp.RequestCtx`, or a `context.Context`.

## Examples

Import the middleware package that is part of the Fiber web framework:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/aigateway"
)
```

### Unified key with virtual client keys

The gateway holds the real provider key. Clients authenticate with their own virtual keys, which are validated before the request is relayed:

```go
app.Use("/openai", aigateway.New(aigateway.Config{
    PathPrefix: "/openai",
    Upstreams:  []aigateway.Upstream{aigateway.OpenAI(os.Getenv("OPENAI_API_KEY"))},
    KeyValidator: func(c fiber.Ctx, key string) (bool, error) {
        return lookupVirtualKey(key), nil
    },
}))
```

Clients keep using their native SDK and only change the base URL:

```go
// OpenAI SDK: baseURL = "https://gateway.example.com/openai/v1"
// The SDK's Authorization header carries the virtual key.
```

### Multiple providers

Mount one gateway instance per provider:

```go
app.Use("/openai", aigateway.New(aigateway.Config{
    PathPrefix: "/openai",
    Upstreams:  []aigateway.Upstream{aigateway.OpenAI(openaiKey)},
    KeyValidator: validate,
}))

app.Use("/anthropic", aigateway.New(aigateway.Config{
    PathPrefix: "/anthropic",
    Upstreams:  []aigateway.Upstream{aigateway.Anthropic(anthropicKey)},
    KeyValidator: validate,
}))
```

### Pass-through mode

Clients bring their own provider keys; the gateway relays them unchanged:

```go
app.Use("/openai", aigateway.New(aigateway.Config{
    PathPrefix:       "/openai",
    ForwardClientKey: true,
    Upstreams:        []aigateway.Upstream{{Name: "openai", URL: "https://api.openai.com"}},
}))
```

### Fallback and retries

Upstreams form an ordered chain — the primary first, fallbacks after. Fallbacks must speak the same wire API as the primary (for example OpenAI → Azure OpenAI). Retryable failures (429, 500, 502, 503, 504, network errors) move through the chain; everything else relays verbatim:

```go
app.Use("/openai", aigateway.New(aigateway.Config{
    PathPrefix: "/openai",
    Upstreams: []aigateway.Upstream{
        aigateway.OpenAI(openaiKey),
        aigateway.AzureOpenAI("https://my-res.openai.azure.com", azureKey),
    },
    Retry: aigateway.RetryConfig{
        Attempts:   2,               // per upstream
        Backoff:    250 * time.Millisecond,
        MaxBackoff: 2 * time.Second, // also caps honored Retry-After values
    },
}))
```

### Model and path policy

```go
app.Use("/openai", aigateway.New(aigateway.Config{
    PathPrefix:    "/openai",
    Upstreams:     []aigateway.Upstream{aigateway.OpenAI(key)},
    AllowedModels: []string{"gpt-4o*", "gpt-4.1-mini"},        // exact or trailing-* wildcard
    AllowedPaths:  []string{"/v1/chat/completions", "/v1/models"},
}))
```

`AllowedModels` only restricts requests whose JSON body declares a `model`, so
endpoints that carry no model — `GET /v1/models`, multipart audio uploads — are
not blocked. Pair it with `AllowedPaths` to bound which endpoints are reachable.

### Usage accounting

`OnUsage` fires once per relayed request. Token usage is parsed from the response body (`usage` object) for buffered responses, and best-effort from the final SSE chunks for streams (OpenAI populates stream usage when the client sets `stream_options.include_usage`):

```go
app.Use("/openai", aigateway.New(aigateway.Config{
    PathPrefix: "/openai",
    Upstreams:  []aigateway.Upstream{aigateway.OpenAI(key)},
    OnUsage: func(e *aigateway.UsageEvent) {
        // For streamed responses this runs on the stream writer goroutine
        // after the stream ends — do not touch fiber.Ctx here.
        log.Printf("provider=%s model=%s status=%d latency=%s usage=%+v err=%v",
            e.Provider, e.Model, e.StatusCode, e.Latency, e.Usage, e.Err)
    },
}))
```

### Custom provider

Any OpenAI/Anthropic-compatible endpoint works with a hand-rolled `Upstream`:

```go
app.Use("/llm", aigateway.New(aigateway.Config{
    PathPrefix: "/llm",
    Upstreams: []aigateway.Upstream{{
        Name:    "my-provider",
        URL:     "https://llm.internal.example.com",
        Auth:    aigateway.AuthHeader("x-api-key"),
        Key:     internalKey,
        Headers: map[string]string{"x-tenant": "team-a"},
    }},
}))
```

## Security

:::caution
In unified-key mode (`ForwardClientKey: false`), leaving `KeyValidator` nil makes the gateway an **open relay for your provider key**: anyone who can reach it can spend your quota. Always validate client keys or protect the route by other means.
:::

- The client's credential — and every other known auth header (`Authorization`, `x-api-key`, `api-key`) — is stripped before the upstream credential is injected, so a second credential cannot be smuggled through.
- Keys are never logged. The `ai-key` logger tag is redacted; `UsageEvent.ClientKey` is raw and must be treated as sensitive by the hook.
- The credential the client presents is stripped from the relayed request wherever the `KeyExtractor` reads it — the well-known auth headers, every `Upstream.Auth.Header`, and the specific header, query param, or cookie the extractor names — so a client credential is never forwarded upstream and a client cannot smuggle a second one. In unified-key mode prefer a header, query, or cookie extractor: a credential read from a form field or route param cannot be stripped without rewriting the body/path and would be relayed upstream.
- Hop-by-hop headers are stripped in both directions.
- Request bodies are bounded by the app's `BodyLimit`; raise it for vision or long-context payloads. Upstream responses can be capped with `MaxResponseSize`.

## Usage and timeouts

`OnUsage` parses token counts from the response body, transparently decompressing `gzip` or `deflate` responses (bounded to guard against decompression bombs) for parsing only — the client still receives the original bytes. Other encodings and content-encoded streaming responses fall back to nil usage. Streaming usage is read best-effort from the final SSE/`message_delta` chunks.

`HeaderTimeout` bounds each attempt up to the response headers (including sending the request body). It does not cap a streaming body — that is guarded by `StreamIdleTimeout`. A non-streaming (buffered) body read runs to the upstream's EOF; fasthttp's streamed body cannot be interrupted from another goroutine without racing the read, so a mid-body stall on a buffered response is bounded by the upstream and OS TCP timeouts (as with the `proxy` middleware) rather than a gateway timer.

## Logger tags

The middleware registers three custom [logger](./logger.md) tags: `ai-key` (redacted client key), `ai-provider`, and `ai-model`.

## Config

| Property              | Type                                    | Description                                                                                                                             | Default                                                                    |
|:----------------------|:----------------------------------------|:----------------------------------------------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------------|
| Next                  | `func(fiber.Ctx) bool`                  | Function to skip this middleware when returned true.                                                                                    | `nil`                                                                      |
| KeyValidator          | `func(fiber.Ctx, string) (bool, error)` | Validates the client (virtual) key before relaying. Return false or an error to reject with 401.                                        | `nil`                                                                      |
| OnUsage               | `func(*UsageEvent)`                     | Called once per relayed request with metadata and parsed token usage. Runs on the writer goroutine for streams.                          | `nil`                                                                      |
| Client                | `*client.Client`                        | Fiber client used for upstream requests. Response body streaming is enabled on it during initialization.                                 | internal client                                                            |
| PathPrefix            | `string`                                | Prefix stripped from the request path before joining it with `Upstream.URL`.                                                            | `""`                                                                       |
| Upstreams             | `[]Upstream`                            | Ordered relay chain: primary first, fallbacks after. **Required.**                                                                       | `nil`                                                                      |
| AllowedModels         | `[]string`                              | Allow-list for the `model` field of JSON request bodies. Exact or trailing-`*` wildcard match. Only restricts requests that declare a model. | `nil` (all allowed)                                                    |
| AllowedPaths          | `[]string`                              | Allow-list for relayed paths (after prefix strip). Exact or trailing-`*` wildcard match.                                                | `nil` (all allowed)                                                        |
| KeyExtractor          | `extractors.Extractor`                  | How the client credential is located on the incoming request.                                                                            | `Chain(FromAuthHeader("Bearer"), FromHeader("x-api-key"), FromHeader("api-key"))` |
| Retry                 | `RetryConfig`                           | Same-upstream retry attempts, backoff, and the cap applied to backoff and `Retry-After`.                                                | `{Attempts: 1, Backoff: 250ms, MaxBackoff: 2s}`                            |
| HeaderTimeout         | `time.Duration`                         | Per-attempt bound from dialing through receiving the response headers (also covers sending the request body). Does not cap streaming bodies. | `30 * time.Second`                                                     |
| StreamIdleTimeout     | `time.Duration`                         | Aborts a streaming response when no bytes arrive for this long. Idle timeout, not a total cap.                                          | `90 * time.Second`                                                         |
| MaxResponseSize       | `int64`                                 | Cap on bytes read from an upstream response. `0` disables the cap.                                                                      | `0`                                                                        |
| ForwardClientKey      | `bool`                                  | Relay the client's own credential upstream instead of injecting `Upstream.Key`.                                                         | `false`                                                                    |
| AllowClientKeyMissing | `bool`                                  | Permit requests without a client credential (unified-key mode only).                                                                    | `false`                                                                    |

### Upstream

| Property | Type                | Description                                                                                   | Default        |
|:---------|:--------------------|:----------------------------------------------------------------------------------------------|:----------------|
| Name     | `string`            | Identifies the upstream in `UsageEvent` and logger tags. **Required.**                        | `""`           |
| URL      | `string`            | Absolute base URL prepended to the prefix-stripped request path. **Required.**                | `""`           |
| Key      | `string`            | Server-side key injected in unified-key mode. **Required** unless `ForwardClientKey` is true. | `""`           |
| Auth     | `AuthScheme`        | How the key is injected upstream.                                                             | `AuthBearer()` |
| Headers  | `map[string]string` | Extra headers set on every request to this upstream.                                          | `nil`          |

## Default Config

```go
var ConfigDefault = Config{
    Retry: RetryConfig{
        Attempts:   1,
        Backoff:    250 * time.Millisecond,
        MaxBackoff: 2 * time.Second,
    },
    HeaderTimeout:     30 * time.Second,
    StreamIdleTimeout: 90 * time.Second,
}
```
