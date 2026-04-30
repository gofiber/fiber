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

A leading dot matches any subdomain but **not** the bare domain itself:

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{".myapp.com"},
}))

// Host: api.myapp.com  → 200 OK
// Host: www.myapp.com  → 200 OK
// Host: myapp.com      → 403 Forbidden
```

To allow both the bare domain and all subdomains, include both:

```go
AllowedHosts: []string{"myapp.com", ".myapp.com"},
```

### CIDR Ranges

Useful for services accessed directly by IP (e.g. internal tooling) where the `Host` header will be a raw IP address. This matches the **Host header value** against a CIDR range — it does not filter by client IP address:

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{
        "internal.myapp.com",
        "10.0.0.0/8",       // Host header IPs in this range are allowed
        "127.0.0.1",        // Host header must be exactly this IP
    },
}))

// Host: internal.myapp.com → 200 OK
// Host: 10.0.50.3          → 200 OK  (Host header IP is in 10.0.0.0/8)
// Host: 169.254.169.254    → 403 Forbidden (Host header IP not in allowlist)
```

### Skipping Health Checks

Use `Next` to bypass host validation for specific paths:

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com", ".myapp.com"},
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
    AllowedHosts: []string{"myapp.com", ".myapp.com"},
    AllowedHostsFunc: func(host string) bool {
        return isRegisteredCustomDomain(host)
    },
}))
```

### Custom Error Response

```go
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com"},
    ErrorHandler: func(c fiber.Ctx, err error) error {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "unauthorized host",
        })
    },
}))
```

### Combined with Domain() Router

`hostauthorization` acts as a security gate; [`Domain()`](https://docs.gofiber.io) handles routing:

```go
// Security layer — reject anything not from our hosts
app.Use(hostauthorization.New(hostauthorization.Config{
    AllowedHosts: []string{"myapp.com", ".myapp.com"},
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
| AllowedHosts     | `[]string`                    | List of permitted hosts. Supports exact match, subdomain wildcard (`.example.com`), and CIDR.     | `nil`   |
| AllowedHostsFunc | `func(string) bool`           | Dynamic validator called only when no static AllowedHosts rule matches. Receives the normalized hostname: port stripped, trailing dot removed, IPv6 brackets removed, lowercased.  | `nil`   |
| ErrorHandler     | `fiber.ErrorHandler`          | Called when a request is rejected. Receives `ErrForbiddenHost` as the error.                      | 403     |

Either `AllowedHosts` or `AllowedHostsFunc` (or both) must be provided. The middleware panics at startup if neither is set.

## Default Config

```go
var ConfigDefault = Config{}
```

There is no useful default — you must provide at least `AllowedHosts` or `AllowedHostsFunc`.

## Host Matching

The middleware matches hosts in this order:

1. **Exact match** — case-insensitive, port and trailing dot stripped
2. **Subdomain wildcard** — `".myapp.com"` matches `api.myapp.com` but not `myapp.com`
3. **CIDR range** — host is parsed as IP and checked against the network
4. **AllowedHostsFunc** — called only if no static rule matched

The first match wins. If nothing matches, `ErrorHandler` is called.

## Host Normalization

Before matching, the incoming host is normalized:

- Port is stripped (via `c.Hostname()`)
- Trailing dot removed (`example.com.` → `example.com`)
- IPv6 brackets removed (`[::1]` → `::1`)
- Lowercased

`AllowedHosts` entries are also lowercased at initialization.

## Proxy Support

The middleware uses Fiber's `c.Hostname()`, which respects `X-Forwarded-Host` when [`TrustProxy`](https://docs.gofiber.io/api/fiber#config) is enabled. When `TrustProxy` is disabled (the default), `X-Forwarded-Host` is ignored and the raw `Host` header is used.

fasthttp itself is HTTP/1.x only. HTTP/2 support requires an external library (e.g. `fasthttp2`) plugged in via `Server.NextProto`. Those libraries are responsible for mapping the HTTP/2 `:authority` pseudo-header to a Host value before the request reaches Fiber handlers, so the middleware should work transparently once H2 is wired up — but this is the H2 library's responsibility, not fasthttp's or this middleware's.

## RFC Compliance

- **RFC 9110 Section 7.2** — Host and port are separate components; port is stripped before matching
- **RFC 9110 Section 17.1** — Origin servers should reject misdirected requests
- **RFC 9112 Section 3.2** — Requests with missing Host headers should be rejected
- Returns **403 Forbidden** (not 400) because the request is syntactically valid but semantically unauthorized

:::note
**RFC 9110 §15.5.20** defines **421 Misdirected Request** as a semantically closer response for host mismatches ("the request was directed at a server unable or unwilling to produce an authoritative response for the target URI"). CDNs like Cloudflare and Fastly use 421 for this case. To use 421 instead of 403, set a custom `ErrorHandler`:

```go
ErrorHandler: func(c fiber.Ctx, err error) error {
    return c.SendStatus(fiber.StatusMisdirectedRequest) // 421
},
```
:::
