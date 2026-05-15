---
id: hostauthorization
---

# Host Authorization

Host authorization middleware for [Fiber](https://github.com/gofiber/fiber) that validates the incoming `Host` header against a configurable allowlist. Protects against [DNS rebinding attacks](https://en.wikipedia.org/wiki/DNS_rebinding) where an attacker-controlled domain resolves to the application's internal IP, causing browsers to send requests with a malicious Host header.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/hostauthorization"
)
```

Once your Fiber app is initialized, choose one of the following approaches:

### Basic Usage

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"api.myapp.com"},
}))

app.Get("/users", func(c fiber.Ctx) error {
    return c.JSON(getUsers())
})

// Host: api.myapp.com → 200 OK
// Host: evil.com      → 403 Forbidden
```

### Subdomain Wildcards

A `*.` prefix matches any subdomain but **not** the bare domain itself:

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"*.myapp.com"},
}))

// Host: api.myapp.com  → 200 OK
// Host: www.myapp.com  → 200 OK
// Host: myapp.com      → 403 Forbidden
```

To allow both the bare domain and all subdomains, include both:

```go
AllowedHosts: []string{"myapp.com", "*.myapp.com"},
```

### Internationalized Domain Names (IDN)

Browsers always transmit the `Host` header in ASCII (Punycode) form, so IDN entries in `AllowedHosts` are converted to Punycode at startup. You can configure entries in either form — they are equivalent:

```go
AllowedHosts: []string{"münchen.example.com"}                 // Unicode
AllowedHosts: []string{"xn--mnchen-3ya.example.com"}          // Punycode (what the browser sends)
```

Both match an incoming request whose Host header is `xn--mnchen-3ya.example.com`.

### Skipping Health Checks

Use `Next` to bypass host validation for specific paths:

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com", "*.myapp.com"},
    Next: func(c fiber.Ctx) bool {
        return c.Path() == "/healthz"
    },
}))

// Host: evil.com GET /healthz → 200 OK (skipped)
// Host: evil.com GET /users   → 403 Forbidden
```

### Dynamic Validation

Use `AllowedHostsFunc` for hosts that can't be known at startup:

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHostsFunc: func(host string) bool {
        // Look up tenant domains from database, cache, etc.
        return isRegisteredTenant(host)
    },
}))
```

`AllowedHostsFunc` is only called when static `AllowedHosts` don't match, so you can combine both:

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com", "*.myapp.com"},
    AllowedHostsFunc: func(host string) bool {
        return isRegisteredCustomDomain(host)
    },
}))
```

### Custom Error Response

The default response is **403 Forbidden**. **421 Misdirected Request** ([RFC 9110 §15.5.20](https://www.rfc-editor.org/rfc/rfc9110#section-15.5.20)) is a semantically closer choice for "wrong host for this server" — CDNs like Cloudflare and Fastly use it for this case. Either is reasonable; pick one via `ErrorHandler`:

```go
// 403 with a JSON body
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com"},
    ErrorHandler: func(c fiber.Ctx, err error) error {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "unauthorized host",
        })
    },
}))

// 421 Misdirected Request — closer to the RFC-defined semantics
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com"},
    ErrorHandler: func(c fiber.Ctx, _ error) error {
        return c.SendStatus(fiber.StatusMisdirectedRequest) // 421
    },
}))
```

### Combined with Domain() Router

`hostauthorization` acts as a security gate; [`Domain()`](https://docs.gofiber.io/api/app#domain) handles routing:

```go
// Security layer — reject anything not from our hosts
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com", "*.myapp.com"},
    Next: func(c fiber.Ctx) bool {
        return c.Path() == "/healthz"
    },
}))

// Routing layer — direct allowed hosts to the right handlers
app.Domain("api.myapp.com").Get("/users", listUsers)
app.Domain(":tenant.myapp.com").Get("/dashboard", tenantDashboard)
app.Get("/healthz", healthCheck)
```

## Config

| Property         | Type                          | Description                                                                                       | Default |
|:-----------------|:------------------------------|:--------------------------------------------------------------------------------------------------|:--------|
| Next             | `func(fiber.Ctx) bool`        | Defines a function to skip this middleware when returned true.                                     | `nil`   |
| AllowedHosts     | `[]string`                    | List of permitted hosts. Supports exact match and subdomain wildcard (`*.example.com`).            | `nil`   |
| AllowedHostsFunc | `func(string) bool`           | Dynamic validator called only when no static AllowedHosts rule matches. Receives the normalized hostname: port stripped, trailing dot removed, IPv6 brackets removed, lowercased, IDN converted to Punycode.  | `nil`   |
| ErrorHandler     | `fiber.ErrorHandler`          | Called when a request is rejected. Receives `ErrForbiddenHost` as the error.                      | 403     |

Either `AllowedHosts` or `AllowedHostsFunc` (or both) must be provided. The middleware panics at startup if neither is set.

## Default Config

```go
var ConfigDefault = Config{}
```

There is no useful default — you must provide at least `AllowedHosts` or `AllowedHostsFunc`.

## Host Matching

The middleware matches hosts in this order:

1. **Exact match** — case-insensitive, port and trailing dot stripped, IDN labels in Punycode form
2. **Subdomain wildcard** — `"*.myapp.com"` matches `api.myapp.com` but not `myapp.com`
3. **AllowedHostsFunc** — called only if no static rule matched

The first match wins. If nothing matches, `ErrorHandler` is called.

## Host Normalization

Before matching, both incoming hosts and `AllowedHosts` entries are normalized at startup:

- Port is stripped (`example.com:8080` → `example.com`)
- Trailing dot removed (`example.com.` → `example.com`)
- IPv6 brackets removed (`[::1]` → `::1`)
- Lowercased
- IDN labels converted to ASCII/Punycode (`münchen.example.com` → `xn--mnchen-3ya.example.com`)
- RFC 1035 length limits enforced at startup: ≤253 chars total, ≤63 chars per label (panic on violation)

## Filtering by Client IP

This middleware filters by the `Host` *header*, not by the client's source IP. To restrict access by client IP, use Fiber's [`TrustProxy` / `TrustProxyConfig`](https://docs.gofiber.io/whats_new#trusted-proxies) configuration — those are the correct knobs for IP allowlisting and CIDR ranges of trusted proxies.

## Proxy Support

The middleware uses Fiber's `c.Hostname()`, which respects `X-Forwarded-Host` when [`TrustProxy`](https://docs.gofiber.io/api/fiber#config) is enabled. When `TrustProxy` is disabled (the default), `X-Forwarded-Host` is ignored and the raw `Host` header is used.

fasthttp itself is HTTP/1.x only. HTTP/2 support requires an external library (e.g. `fasthttp2`) plugged in via `Server.NextProto`. Those libraries are responsible for mapping the HTTP/2 `:authority` pseudo-header to a Host value before the request reaches Fiber handlers, so the middleware should work transparently once H2 is wired up — but this is the H2 library's responsibility, not fasthttp's or this middleware's.

## RFC Compliance

- **RFC 9110 Section 7.2** — Host and port are separate components; port is stripped before matching
- **RFC 9110 Section 17.1** — Origin servers should reject misdirected requests
- **RFC 9112 Section 3.2** — Requests with missing Host headers should be rejected
- **RFC 1035** — `AllowedHosts` entries are validated against the 253-char total / 63-char per-label limits
- Returns **403 Forbidden** (not 400) because the request is syntactically valid but semantically unauthorized

:::note
**RFC 9110 §15.5.20** defines **421 Misdirected Request** as a semantically closer response for host mismatches ("the request was directed at a server unable or unwilling to produce an authoritative response for the target URI"). CDNs like Cloudflare and Fastly use 421 for this case. To use 421 instead of 403, set a custom `ErrorHandler`:

```go
ErrorHandler: func(c fiber.Ctx, err error) error {
    return c.SendStatus(fiber.StatusMisdirectedRequest) // 421
},
```

:::
