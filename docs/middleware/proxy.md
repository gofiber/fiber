---
id: proxy
---

# Proxy

The Proxy middleware forwards requests to one or more upstream servers.

## Signatures

```go
// Balancer creates a load balancer among multiple upstream servers.
func Balancer(config ...Config) fiber.Handler
// Forward performs the given http request and fills the given http response.
func Forward(addr string, clients ...*fasthttp.Client) fiber.Handler
// Do performs the given http request and fills the given http response.
func Do(c fiber.Ctx, addr string, clients ...*fasthttp.Client) error
// DoRedirects performs the given http request and fills the given http response while following up to maxRedirectsCount redirects.
func DoRedirects(c fiber.Ctx, addr string, maxRedirectsCount int, clients ...*fasthttp.Client) error
// DoDeadline performs the given request and waits for response until the given deadline.
func DoDeadline(c fiber.Ctx, addr string, deadline time.Time, clients ...*fasthttp.Client) error
// DoTimeout performs the given request and waits for response during the given timeout duration.
func DoTimeout(c fiber.Ctx, addr string, timeout time.Duration, clients ...*fasthttp.Client) error
// DomainForward performs the given http request based on the provided domain and fills the given http response.
func DomainForward(hostname string, addr string, clients ...*fasthttp.Client) fiber.Handler
// BalancerForward performs the given http request based round robin balancer and fills the given http response.
func BalancerForward(servers []string, clients ...*fasthttp.Client) fiber.Handler
```

## Security

The proxy middleware applies several defenses by default. They can be relaxed via `Config.SecurityPolicy` (for `Balancer`) or `proxy.WithSecurityPolicy` (for the runtime helpers `Do`, `Forward`, `DoRedirects`, `DoTimeout`, `DoDeadline`).

### SSRF protection

Upstream addresses that resolve to loopback, RFC 1918 private, link-local (including the `169.254.169.254` cloud-metadata address), multicast, unspecified, or RFC 6598 CGNAT ranges are rejected with `ErrUpstreamHostBlocked`. If any resolved IP falls in a blocked range the upstream is rejected, mitigating DNS-rebinding attempts that return a mix of public and private answers.

For `Balancer`, the resolved IP is re-validated at **dial time** (via a guarded `Dial` on each upstream `fasthttp.HostClient`), which both defeats DNS-rebinding and avoids resolving hostnames at startup — a transient DNS failure won't panic your application. DNS lookups are bounded by a 5-second timeout.

:::caution DNS-rebinding scope
The dial-time re-validation only applies to `Balancer`, because those `HostClient`s are constructed by the middleware. The runtime helpers — `Do`, `DoRedirects`, `DoTimeout`, `DoDeadline`, `Forward`, `DomainForward`, and `BalancerForward` — validate the upstream host up front, then dispatch through the shared or user-supplied `*fasthttp.Client`, which re-resolves the name without the guard. Against a **rebinding-capable resolver** these paths have a check/use window and are not fully mitigated. If that is part of your threat model, use `Balancer` (with `AllowPrivateIPs = false`), or supply a client whose `Dial` performs its own resolved-IP validation.
:::

Set `SecurityPolicy.AllowPrivateIPs = true` to opt out — required when proxying to internal services on the same network.

### Scheme allowlist

Only `http` and `https` upstream schemes are accepted by default; `file://`, `gopher://`, `ftp://`, and other schemes are rejected. Override via `SecurityPolicy.AllowedSchemes`.

### HTTPS-to-HTTP redirect downgrades

`DoRedirects` rejects redirects from HTTPS origins to plaintext HTTP targets with `ErrRedirectDowngrade`. Following such a redirect would leak any cookies or `Authorization` headers established under TLS. Set `SecurityPolicy.AllowHTTPSDowngrade = true` to override.

When a redirect crosses to a **different host**, `DoRedirects` strips the `Authorization`, `Proxy-Authorization`, and `Cookie` headers so credentials bound to the original origin are not forwarded to a third-party upstream. Same-host redirects retain these headers.

### RFC 7230 hop-by-hop header stripping

`Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, and `Upgrade` are stripped from both the outbound request and the inbound response, along with every header listed in the `Connection` field per RFC 7230 §6.1. This prevents request smuggling (`TE`/`Transfer-Encoding`), proxy-credential forwarding, and protocol-upgrade leaks. The legacy `KeepConnectionHeader` option preserves only the literal `Connection` header for backwards compatibility; the other hop-by-hop headers are still stripped. To preserve every hop-by-hop header (not recommended), set `SecurityPolicy.KeepHopByHopHeaders = true`.

### TLS minimum version

`Config.TLSConfig` is cloned with `MinVersion: tls.VersionTLS12` if no minimum is configured, so deprecated TLS versions cannot be negotiated by accident.

### Response body size and connection caps

`Config.MaxResponseBodySize` bounds upstream response bodies to protect against memory exhaustion. `Config.MaxConnsPerHost` (default `1024`) caps concurrent connections per upstream to limit fan-out from a single hot host.

### X-Real-IP spoof prevention

`Forward`, `DomainForward`, and `BalancerForward` automatically overwrite the `X-Real-IP` header with `c.IP()` before forwarding, so clients cannot spoof their address. `DomainForward` only applies the overwrite when the request host matches the configured hostname (matched case-insensitively per RFC 9110 §4.2.3); non-matching requests are passed through unchanged.

If you're using `Balancer` with the `Config` struct, you can replicate the protection in `ModifyRequest`. When using `Do`, `DoRedirects`, `DoDeadline`, or `DoTimeout` directly, the `X-Real-IP` header is not set automatically — set it manually if needed:

```go
c.Request().Header.Set("X-Real-IP", c.IP())
```

### Path concatenation safety

`DomainForward` and `BalancerForward` previously concatenated the configured upstream with `c.OriginalURL()`. Crafted request paths beginning with `//` could exploit URL parsing to redirect the proxy at a different host (network-path reference injection). The proxy now sanitises the joined path so the upstream host pinned in configuration is preserved regardless of the inbound request.

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/proxy"
)
```

Once your Fiber app is initialized, you can use the middleware as shown:

```go
// Use proxy.WithClient to set a global custom client.
proxy.WithClient(&fasthttp.Client{
    NoDefaultUserAgentHeader: true,
    DisablePathNormalizing:   true,
    MaxConnsPerHost:          2048,
    // Allow self-signed certificates when proxying to HTTPS targets.
    // SECURITY: disables certificate verification — use only when the
    // upstream is on a trusted network.
    TLSConfig: &tls.Config{
        InsecureSkipVerify: true,
        MinVersion:         tls.VersionTLS12,
    },
})

// Relax SSRF protection for local development against loopback servers.
// SECURITY: in production, leave AllowPrivateIPs false (the default) and
// list explicit upstream hosts so the proxy cannot be coerced into
// reaching internal services or cloud-metadata endpoints.
prev := proxy.WithSecurityPolicy(proxy.SecurityPolicy{
    AllowedSchemes:  []string{"http", "https"},
    AllowPrivateIPs: true,
})
defer proxy.WithSecurityPolicy(prev)

// Forward requests for a specific domain with proxy.DomainForward.
app.Get("/payments", proxy.DomainForward("docs.gofiber.io", "http://localhost:8000"))

// Forward to a URL using a custom client
app.Get("/gif", proxy.Forward("https://i.imgur.com/IWaBepg.gif", &fasthttp.Client{
    NoDefaultUserAgentHeader: true,
    DisablePathNormalizing:   true,
}))

// Make a proxied request within a handler
app.Get("/:id", func(c fiber.Ctx) error {
    url := "https://i.imgur.com/" + c.Params("id") + ".gif"
    if err := proxy.Do(c, url); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Proxy requests while following redirects
app.Get("/proxy", func(c fiber.Ctx) error {
    if err := proxy.DoRedirects(c, "http://google.com", 3); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Proxy requests and wait up to five seconds before timing out
app.Get("/proxy", func(c fiber.Ctx) error {
    if err := proxy.DoTimeout(c, "http://localhost:3000", time.Second * 5); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Proxy requests with a deadline one minute from now
app.Get("/proxy", func(c fiber.Ctx) error {
    if err := proxy.DoDeadline(c, "http://localhost", time.Now().Add(time.Minute)); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Minimal round-robin balancer
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://localhost:3001",
        "http://localhost:3002",
        "http://localhost:3003",
    },
}))

// Keep the Connection header when proxying
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://localhost:3001",
    },
    KeepConnectionHeader: true,
}))

// Or extend your balancer for customization
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://localhost:3001",
        "http://localhost:3002",
        "http://localhost:3003",
    },
    MaxConnsPerHost: 2048,
    ModifyRequest: func(c fiber.Ctx) error {
        c.Request().Header.Set("X-Real-IP", c.IP())
        return nil
    },
    ModifyResponse: func(c fiber.Ctx) error {
        c.Response().Header.Del(fiber.HeaderServer)
        return nil
    },
}))

// Or this way if the balancer is using https and the destination server is only using http.
app.Use(proxy.BalancerForward([]string{
    "http://localhost:3001",
    "http://localhost:3002",
    "http://localhost:3003",
}))


// Make round robin balancer with IPv6 support.
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://[::1]:3001",
        "http://127.0.0.1:3002",
        "http://localhost:3003",
    },
    // Enable TCP4 and TCP6 network stacks.
    DialDualStack: true,
}))
```

## Config

| Property        | Type                                           | Description                                                                                                                                                                                                                        | Default         |
|:----------------|:-----------------------------------------------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:----------------|
| Next            | `func(fiber.Ctx) bool`                        | Next defines a function to skip this middleware when it returns true.                                                                                                                                                                | `nil`           |
| Servers         | `[]string`                                     | Servers defines a list of `<scheme>://<host>` HTTP servers, which are used in a round-robin manner. i.e.: "[https://foobar.com](https://foobar.com), [http://www.foobar.com](http://www.foobar.com)"                                                        | (Required)      |
| ModifyRequest   | `fiber.Handler`                                | ModifyRequest allows you to alter the request.                                                                                                                                                                                     | `nil`           |
| ModifyResponse  | `fiber.Handler`                                | ModifyResponse allows you to alter the response.                                                                                                                                                                                   | `nil`           |
| Timeout         | `time.Duration`                                | Timeout is the request timeout used when calling the proxy client.                                                                                                                                                                 | 1 second        |
| MaxConnsPerHost | `int`                                          | Maximum number of connections per upstream host. The default proxy client and balancer host clients use this limit unless you override it with `WithClient`, a per-handler client, or `proxy.Config`.                         | `1024`          |
| ReadBufferSize  | `int`                                          | Per-connection buffer size for requests' reading. This also limits the maximum header size. Increase this buffer if your clients send multi-KB RequestURIs and/or multi-KB headers (for example, BIG cookies).                     | (Not specified) |
| WriteBufferSize | `int`                                          | Per-connection buffer size for responses' writing.                                                                                                                                                                                 | (Not specified) |
| KeepConnectionHeader | `bool` | Keeps the `Connection` header when set to `true`. By default the header is removed to comply with RFC 7230 §6.1 and avoid proxy loops. Other hop-by-hop headers are still stripped regardless of this setting. | `false` |
| TLSConfig       | `*tls.Config` | TLS config for the HTTP client. Cloned with `MinVersion: tls.VersionTLS12` when no minimum is set. | `nil`           |
| DialDualStack   | `bool`                                         | Client will attempt to connect to both IPv4 and IPv6 host addresses if set to true.                                                                                                                                                | `false`         |
| Client          | `*fasthttp.LBClient`                           | Client is a custom client when client config is complex.                                                                                                                                                                           | `nil`           |
| SecurityPolicy  | `*SecurityPolicy`                              | Overrides the default SSRF, redirect, and hop-by-hop header rules for this balancer. When `nil`, the package-level policy set via `WithSecurityPolicy` is used. See [Security](#security).                                          | `nil`           |
| MaxResponseBodySize | `int`                                       | Maximum upstream response body size in bytes. `0` keeps fasthttp's unlimited default.                                                                                                                                              | `0`             |

## Default Config

```go
var ConfigDefault = Config{
    Next:                 nil,
    ModifyRequest:        nil,
    ModifyResponse:       nil,
    MaxConnsPerHost:      1024,
    Timeout:              fasthttp.DefaultLBClientTimeout,
    KeepConnectionHeader: false,
}
```

## Default SecurityPolicy

When `Config.SecurityPolicy` is `nil` (and `proxy.WithSecurityPolicy` has not been called), the package falls back to the value returned by `proxy.DefaultSecurityPolicy()`:

```go
// DefaultSecurityPolicy returns the secure-by-default policy.
func DefaultSecurityPolicy() proxy.SecurityPolicy {
    return proxy.SecurityPolicy{
        AllowedSchemes:      []string{"http", "https"},
        AllowPrivateIPs:     false,
        AllowHTTPSDowngrade: false,
        KeepHopByHopHeaders: false,
    }
}
```
