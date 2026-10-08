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

The proxy middleware applies several defenses by default. They can be relaxed via `Config.SecurityPolicy` (for `Balancer`) or `proxy.WithSecurityPolicy` (for the runtime helpers `Do`, `Forward`, `DoRedirects`, `DoTimeout`, `DoDeadline`, `DomainForward` and `BalancerForward`).

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

When a redirect crosses to a **different host**, `DoRedirects` strips `Authorization`, `Proxy-Authorization`, `Proxy-Authenticate`, `WWW-Authenticate`, `Cookie` and the obsolete `Cookie2`, so credentials bound to the original origin are not forwarded to a third-party upstream. This is `net/http`'s own set. Same-host redirects retain these headers.

### RFC 7230 hop-by-hop header stripping

`Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, and `Upgrade` are stripped from both the outbound request and the inbound response, along with every header listed in the `Connection` field per RFC 7230 §6.1. This prevents request smuggling (`TE`/`Transfer-Encoding`), proxy-credential forwarding, and protocol-upgrade leaks. The legacy `KeepConnectionHeader` option preserves only the literal `Connection` header for backwards compatibility; the other hop-by-hop headers are still stripped. To preserve every hop-by-hop header (not recommended), set `SecurityPolicy.KeepHopByHopHeaders = true`.

The names a client lists in `Connection` are removed from the request as it was received, with one intentional exception to RFC 9110 §7.6.1: the forwarding headers `X-Real-IP`, `Forwarded`, and every `X-Forwarded-*` header stay. They tell the upstream about the client as this hop saw it, and an upstream that trusts the gateway acts on them, so they are the gateway's to write and not the client's to have removed. `Balancer`, `Forward`, `DomainForward`, and `BalancerForward` write `X-Real-IP` only after the listing has been applied, and the exception also covers a forwarding header your application sets before the proxy runs, including `X-Real-IP` set by hand before `Do`. A client loses nothing by it: a forwarding header it sends is forwarded whenever `Connection` does not name it.

:::caution Headers set before the proxy runs
Any other header your application adds to the request before the proxy handler runs — in an earlier middleware, or before calling `Do` — is part of the received request as far as the `Connection` listing is concerned, and a client can have it removed by naming it there. Set headers meant for the upstream in `Balancer`'s `ModifyRequest`, which runs after the listing has been applied.
:::

### TLS minimum version

`Config.TLSConfig` is cloned with `MinVersion: tls.VersionTLS12` if no minimum is configured, so deprecated TLS versions cannot be negotiated by accident.

### Response body size and connection caps

`Config.MaxResponseBodySize` bounds upstream response bodies to protect against memory exhaustion. `Config.MaxConnsPerHost` (default `1024`) caps concurrent connections per upstream to limit fan-out from a single hot host.

### X-Real-IP spoof prevention

`Balancer`, `Forward`, `DomainForward`, and `BalancerForward` automatically overwrite the `X-Real-IP` header with `c.IP()` before forwarding, so clients cannot spoof their address. The address is read from the request as received and written after the `Connection` listing has been applied, and `X-Real-IP` is exempt from that listing, so a client cannot have the header removed by naming it there either (see [hop-by-hop header stripping](#rfc-7230-hop-by-hop-header-stripping)). `Balancer` writes it before `ModifyRequest` runs, so `ModifyRequest` can still set its own value. `DomainForward` only applies the overwrite when the request host matches the configured hostname (matched case-insensitively per RFC 9110 §4.2.3, with or without a port in the `Host` header); non-matching requests are passed on to the next handler unchanged. After any of the forwarding helpers returns, the request carries its original URI and `Host` again, so middleware running after `Next` still sees the request the client sent.

When using `Do`, `DoRedirects`, `DoDeadline`, or `DoTimeout` directly, the `X-Real-IP` header is not set automatically — set it manually if needed:

```go
ip := c.IP()
c.Request().Header.Del("X-Real-IP")
c.Request().Header.Add("X-Real-IP", ip)
```

`Set` alone is not enough here. It replaces the first field line with that name
and leaves any others in place, so a client that sends `X-Real-IP` twice keeps
one of its own values on the wire, and the upstream — which may read the last
line, or join the pair per RFC 9110 §5.2 — attributes the request to an address
the client chose. Delete first so exactly one line survives. Resolve `c.IP()`
before the delete as well: with `Config.ProxyHeader` set to `X-Real-IP`, `c.IP()`
reads the very header being replaced. The line you write survives a client
listing `X-Real-IP` in `Connection`, as the forwarding headers are exempt from
that listing.

:::caution With `DisableHeaderNormalizing`, delete every spelling

`Del` matches the stored key byte for byte. That finds every line while Fiber
canonicalizes header names, which is the default — but under
[`DisableHeaderNormalizing`](../api/fiber.md#config) the store keeps the
spelling the client sent, and lower case is what HTTP/2 and HTTP/3 put on the
wire. `Del("X-Real-IP")` then leaves a client-sent `x-real-ip` untouched and the
`Add` lands beside it, which is the pair the delete exists to prevent.

The middleware's own `Balancer`, `Forward`, `DomainForward` and `BalancerForward` handle
this. Doing it by hand takes one pass to collect the spellings and another to
remove them, since deleting while iterating the store is not safe:

```go
ip := c.IP()
h := &c.Request().Header

var spellings []string
for k := range h.All() {
    if utils.EqualFold(utils.UnsafeString(k), "X-Real-IP") {
        spellings = append(spellings, string(k))
    }
}
for _, name := range spellings {
    h.Del(name)
}

h.Add("X-Real-IP", ip)
```

`utils` here is `github.com/gofiber/utils/v2`.

:::

### Request target

`Balancer`, `DomainForward` and `BalancerForward` forward the path the router matched, `c.Path()`, followed by the query. They do not forward the request line as it arrived. The two differ, and the difference is what an attacker uses: fasthttp's normalization of the raw request line decodes `%2F` into a separator and merges repeated slashes, so `/public/..%2Fadmin/secret` and `//admin/secret` used to reach the upstream as `/admin/secret`, a path no middleware mounted on `/admin` had seen. Routed, they stay `/public/..%2Fadmin/secret` and `//admin/secret`, and by default the proxy refuses to forward either, since an upstream that decodes `%2F` or merges `//` would read them as `/admin/secret` as well. Dot segments are resolved before matching, so `/public/../admin/secret` runs the `/admin` middleware and is forwarded as `/admin/secret`. An escape of an unreserved character is decoded (`%41` becomes `A`), every other escape is kept as sent and a stray `%` is forwarded as `%25`. A byte a path may not carry raw ([RFC 3986 Section 3.3](https://www.rfc-editor.org/rfc/rfc3986#section-3.3)), such as `|`, `{`, `"` or a byte of UTF-8, is forwarded as its escape, as fasthttp wrote it before, so `/über` reaches the upstream as `/%C3%BCber`; the sub-delims (`!$&'()*+,;=`), `:` and `@` stay raw. With `UnescapePath` enabled the decoded path is escaped again segment by segment, so a separator that came from `%2F` is forwarded as the separator the router matched it as.

The query is forwarded byte for byte as the client sent it, so a signed URL keeps its signature. Only when a handler changed the arguments through `QueryArgs()` is it the changed arguments, as fasthttp serializes them; reading them, as `c.Query()` does, changes nothing. `Balancer` puts the original request line back once the upstream has answered, so `c.OriginalURL()` in a middleware that runs afterwards is unchanged.

- `Balancer` dials the configured host and forwards the target as is, so every entry in `Servers` must be a scheme and host only. An entry with a path, userinfo, query or fragment panics at startup with `ErrUpstreamNotOrigin` instead of dropping that part silently, which would have left `http://backend/api` reaching all of the upstream. `DomainForward` and `BalancerForward` prepend the path of their upstream URL, and the joined path keeps the upstream host pinned in configuration whatever the request contains: `//attacker.example/path`, `@attacker` and `/foo://hijack.example` all stay paths.
- `Balancer` answers a request whose `Host` header carries userinfo, such as `svc:pw@backend`, with `400 Bad Request`. fasthttp would read the userinfo as credentials and send them upstream as a `Basic` `Authorization` header, replacing one the application had set.
- `Balancer` also answers `400 Bad Request` for a routed target fasthttp would read as an authority rather than a path: one that begins with `//` and holds `://`, or any target beginning with `//` on a request without a `Host` header. The router never saw such a target as an authority, and it need not have arrived as one: `..` resolution builds `//u:pw@evil.example/a://b` out of `//u:pw@evil.example/a:/x/..//b`, and under `UnescapePath` a decoded `%3A%2F%2F` spells the `://`. Set as the request line, it would hand the upstream a `Host`, and a `Basic` `Authorization` header, taken from the path rather than from the application.
- `Balancer`, `DomainForward` and `BalancerForward` answer `400 Bad Request` for a routed path an upstream could read as another path, one under a prefix whose middleware never ran for it. That is a path holding:
  - an escaped slash (`%2F`) or an empty segment (`//`). An upstream that decodes `%2F` into a separator or merges repeated slashes before it matches routes, as a `net/http` file server or a fasthttp server does, would map `/public/..%2Fadmin/secret`, `/admin%2Fsecret` and `//admin/secret` to `/admin/secret`. An upstream that keeps both as sent can be reached with `SecurityPolicy.AllowAmbiguousSlashes`, described below.
  - a backslash, raw or escaped as `%5C`. WHATWG URL parsers, as in Node.js, Bun, Deno and Cloudflare Workers, and IIS read it as a separator, so `/admin\secret` and `/public/..%5Cadmin/secret` would reach `/admin`.
  - a `.` or `..` segment carrying parameters, such as `..;` or `..;jsessionid=x`. Servlet containers such as Tomcat and Jetty strip the parameters before they resolve dot segments, so `/public/..;/admin/secret` would reach `/admin/secret`.
  - an escape of an unreserved character, such as `%2e` or `%70`. The router decodes every such escape when it normalizes a path, so one still in `c.Path()` can only have been forged by a stray `%`, and an upstream decoding it again would serve a name no middleware matched.

  To forward such a target regardless, build it from `c.Path()` and call `Do`.
- `SecurityPolicy.AllowAmbiguousSlashes` lets `Balancer`, `DomainForward` and `BalancerForward` forward a path holding `%2F` or `//`, for an upstream that addresses names such as GitLab-style `group%2Fproject` ids or object keys with a doubled slash. Set it on `Config.SecurityPolicy` for `Balancer`, or install it with `proxy.WithSecurityPolicy` for `DomainForward` and `BalancerForward`. Enable it only when every upstream behind the handler keeps `%2F` and `//` as sent; a backslash, a dot segment with parameters and a forged escape are refused regardless.
- A path override without a leading slash, as a rewrite of `/go/*` to `$1` produces for `/go/http://u:pw@evil.example/x`, is forwarded rooted, as the path `/http://u:pw@evil.example/x`, rather than as the absolute URL a request line would read it as.
- With a custom `Client`, every `*fasthttp.HostClient` among its `Clients` gets `DisablePathNormalizing` set, since a client left normalizing would decode `%2F` into a separator and merge `//` on the way out. Any other `BalancingClient` cannot be told, so with one present `Balancer` answers `400 Bad Request` for a routed target that normalization would change: one holding an empty segment or an escape it would decode into a byte it then writes raw, such as `%3B` or `%40`. An escape of a byte it writes escaped again, such as `%20` or the `%C3%BC` of `/über`, is forwarded.
- `Do`, `Forward`, `DomainForward`, `BalancerForward` and their variants switch path normalization off on the default client, on a client registered with `WithClient` and on every host client a per-call `*fasthttp.Client` creates once the proxy has it. fasthttp applies that setting only to the host client it creates for an upstream, so a per-call client that already reached that upstream before it was handed to the proxy keeps the host client it made then, and that one still normalizes: `/public/..%2Fadmin/secret` would leave it as `/admin/secret`. Register such a client with `WithClient` before it serves requests, hand the proxy a client used for nothing else, or create the client with `DisablePathNormalizing: true`.

:::caution Upstream normalization
The proxy cannot control what the upstream does with the target beyond what it refuses to forward. A target holding `%2F`, `//`, a backslash, a dot segment with parameters or a forged escape is not forwarded by default, so an upstream that decodes, merges or strips parameters cannot be handed a path the router did not match through them. An escape of another reserved character, such as `%3B`, `%3F` or `%23`, is forwarded as sent, and so are a segment's case and a trailing dot or space, any of which an upstream on Windows or a case-insensitive filesystem may treat as another name. Authorize on the upstream as well.
:::

When you build the target for `Do`, `Forward` or their variants yourself, derive it from `c.Path()` and `c.Request().URI().QueryString()` rather than from `c.OriginalURL()`, which is the request line as it arrived, before the router resolved dot segments.

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
| Servers         | `[]string`                                     | Servers defines a list of `<scheme>://<host>` HTTP servers, which are used in a round-robin manner. Each entry is a scheme and host only; one with a path, userinfo, query or fragment panics at startup with `ErrUpstreamNotOrigin`. i.e.: "[https://foobar.com](https://foobar.com), [http://www.foobar.com](http://www.foobar.com)"                                                        | (Required)      |
| ModifyRequest   | `fiber.Handler`                                | ModifyRequest allows you to alter the request.                                                                                                                                                                                     | `nil`           |
| ModifyResponse  | `fiber.Handler`                                | ModifyResponse allows you to alter the response.                                                                                                                                                                                   | `nil`           |
| Timeout         | `time.Duration`                                | Timeout is the request timeout used when calling the proxy client.                                                                                                                                                                 | 1 second        |
| MaxConnsPerHost | `int`                                          | Maximum number of connections per upstream host. The default proxy client and balancer host clients use this limit unless you override it with `WithClient`, a per-handler client, or `proxy.Config`.                         | `1024`          |
| ReadBufferSize  | `int`                                          | Per-connection buffer size for requests' reading. This also limits the maximum header size. Increase this buffer if your clients send multi-KB RequestURIs and/or multi-KB headers (for example, BIG cookies).                     | (Not specified) |
| WriteBufferSize | `int`                                          | Per-connection buffer size for responses' writing.                                                                                                                                                                                 | (Not specified) |
| KeepConnectionHeader | `bool` | Keeps the `Connection` header when set to `true`. By default the header is removed to comply with RFC 7230 §6.1 and avoid proxy loops. Other hop-by-hop headers are still stripped regardless of this setting. | `false` |
| TLSConfig       | `*tls.Config` | TLS config for the HTTP client. Cloned with `MinVersion: tls.VersionTLS12` when no minimum is set. | `nil`           |
| DialDualStack   | `bool`                                         | Client will attempt to connect to both IPv4 and IPv6 host addresses if set to true.                                                                                                                                                | `false`         |
| Client          | `*fasthttp.LBClient`                           | Client is a custom client when client config is complex. Each `*fasthttp.HostClient` among its `Clients` gets `DisablePathNormalizing` set; see [Request target](#request-target).                                                                                                                                                                           | `nil`           |
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
        AllowedSchemes:        []string{"http", "https"},
        AllowPrivateIPs:       false,
        AllowHTTPSDowngrade:   false,
        KeepHopByHopHeaders:   false,
        AllowAmbiguousSlashes: false,
    }
}
```
