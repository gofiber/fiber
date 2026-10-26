---
id: proxy
---

# Proxy

The Proxy middleware forwards requests to one or more upstream servers.

## Signatures

```go
// Balancer creates a load balancer among multiple upstream servers. Servers (or Client) is required: without it Balancer panics at startup.
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
// WithClient sets the global client the runtime helpers dispatch through. It panics for a nil client.
func WithClient(cli *fasthttp.Client)
// WithSecurityPolicy installs the global SecurityPolicy and returns the previous one.
func WithSecurityPolicy(policy SecurityPolicy) SecurityPolicy
// DefaultSecurityPolicy returns the secure-by-default SecurityPolicy.
func DefaultSecurityPolicy() SecurityPolicy
```

## Security

The proxy middleware applies several defenses by default. They can be relaxed via `Config.SecurityPolicy` (for `Balancer`) or `proxy.WithSecurityPolicy` (for the runtime helpers `Do`, `Forward`, `DoRedirects`, `DoTimeout`, `DoDeadline`, `DomainForward` and `BalancerForward`).

### SSRF protection

Upstream addresses that resolve to loopback, RFC 1918 private, link-local (including the `169.254.169.254` cloud-metadata address), multicast, unspecified, RFC 6598 CGNAT, or another special-use range (`0.0.0.0/8`, `192.0.0.0/24`, `198.18.0.0/15`, `240.0.0.0/4` with the broadcast address, and the deprecated IPv6 site-local `fec0::/10`) are rejected with `ErrUpstreamHostBlocked`. If any resolved IP falls in a blocked range the upstream is rejected, mitigating DNS-rebinding attempts that return a mix of public and private answers. A name that cannot be resolved, or that resolves to no address, is rejected too, because the check fails closed. That error also matches `ErrUpstreamHostUnresolvable`, and `errors.As` finds the `*net.DNSError` of a failed lookup, so a handler can answer `502` or retry for an outage and keep `403` for an address that was judged.

An IPv6 literal cannot carry a blocked IPv4 address past the list. The IPv4 address embedded in an IPv4-mapped (`::ffff:a.b.c.d`), IPv4-compatible, IPv4-translated (`::ffff:0:a.b.c.d`), NAT64 well-known (`64:ff9b::/96`) or intra-site tunnel (RFC 5214) address is checked against the same ranges. The 6to4 (`2002::/16`), Teredo (`2001::/32`) and NAT64 local-use (`64:ff9b:1::/48`) ranges are rejected as a whole, because unwrapping one position would leave a bypass: Teredo can hide an IPv4 address in either of two fields, the NAT64 local-use prefix lets the operator choose where it sits, and 6to4 is deprecated ([RFC 7526](https://www.rfc-editor.org/rfc/rfc7526)).

For `Balancer`, the resolved IP is re-validated at **dial time** (via a guarded `Dial` and `DialTimeout` on each upstream `fasthttp.HostClient`), which both defeats DNS-rebinding and avoids resolving hostnames at startup — a transient DNS failure won't panic your application. DNS lookups are bounded by a 5-second timeout, or by the time the request has left when that is shorter: a dial that a `DoTimeout`, a `DoDeadline` or `Config.Timeout` bounds gives its lookup and its connects that time between them, and a connect that nothing bounds has five seconds.

A host that passes the blocklist and has only IPv6 addresses is dialed when `DialDualStack` is set. Otherwise the request fails with an error that says so; it matches neither `ErrUpstreamHostBlocked` nor `ErrUpstreamHostUnresolvable`, because the lookup succeeded and no address was refused.

The check that `Do`, `DoRedirects`, `DoTimeout`, `DoDeadline`, `Forward`, `DomainForward` and `BalancerForward` make before each request remembers what a hostname resolved to for one second, so a request does not pay a resolver round trip. The blocklist is applied to the remembered answer on every request, so a policy change takes effect at once, and an answer it refuses is refused each time. The dial-time check never uses that memory: it resolves again on every new connection. Only the shared client (the default one, or the one registered with `proxy.WithClient`) is served from it, because only its dials are checked again where they are made. A request through a client passed per call, or through a client with a `Transport`, is judged by a lookup of its own, and so is a name whose answer holds more than 32 addresses, which is not remembered: the memory the check can take stays fixed whatever a resolver answers.

:::caution DNS-rebinding scope
Dial-time re-validation applies to the connections the proxy makes itself. `Balancer` builds its `HostClient`s with a guarded `Dial` and `DialTimeout`. The runtime helpers (`Do`, `DoRedirects`, `DoTimeout`, `DoDeadline`, `Forward`, `DomainForward` and `BalancerForward`) install the same guard on the `*fasthttp.Client` they dispatch through, using its `ConfigureClient` hook after any hook the client already had. The default client and a client registered with `proxy.WithClient` are guarded before their first request. A client passed to a helper per call is guarded on first use, so hosts it had already dialed keep their existing host clients: register such a client with `proxy.WithClient` before it serves traffic, or hand the proxy a client that nothing else uses. Connections made by your own code are yours to guard: a `*fasthttp.Client` with a `Transport` dials by itself, and the dialers of a custom `Config.Client` (`*fasthttp.LBClient`) are yours, so validate the resolved IP there.
:::

`Do`, `DoRedirects`, `DoTimeout`, `DoDeadline` and `Forward` return a rejected upstream as an error. `Balancer`, `DomainForward` and `BalancerForward` check their upstreams when the handler is built and panic with that error instead, so a bad entry stops the application at startup. `DomainForward` and `BalancerForward` also resolve each hostname for that check unless `AllowPrivateIPs` is set, so an unresolvable name panics too, and so does a loopback or private upstream: call `proxy.WithSecurityPolicy` before building them when the upstream is internal. `Balancer` leaves hostname resolution to dial time and rejects only an IP literal in a blocked range at startup.

Set `SecurityPolicy.AllowPrivateIPs = true` to opt out — required when proxying to internal services on the same network.

### When the policy is read

`Balancer` settles on its policy once, when it is built: `Config.SecurityPolicy` if set, otherwise the one installed with `proxy.WithSecurityPolicy` at that moment. `Do`, `DoRedirects`, `DoTimeout`, `DoDeadline`, `Forward`, `DomainForward` and `BalancerForward` read the installed policy on every request, and the dial-time guard on the clients they dispatch through reads it on every dial. `DomainForward` and `BalancerForward` also check their upstreams against it when they are built, but that check is no snapshot: relaxing the policy to build them and then restoring the default leaves them refusing every request with `ErrUpstreamHostBlocked`, while a `Balancer` built under the same relaxed policy keeps working. Install the policy before building these handlers and keep it installed for as long as they serve. A `defer proxy.WithSecurityPolicy(prev)` in `main` is fine, since it runs when the program ends; restoring the policy when a setup function returns is not.

### Scheme allowlist

Only `http` and `https` upstream schemes are accepted by default; `file://`, `gopher://`, `ftp://`, and other schemes are rejected with `ErrUpstreamSchemeNotAllowed`. An empty address, one without a host, or one that cannot be parsed is rejected with `ErrUpstreamHostInvalid`; for an address that cannot be parsed, the `*url.Error` from `net/url` is wrapped beside it, so `errors.As` still finds it. Override the schemes via `SecurityPolicy.AllowedSchemes`.

### HTTPS-to-HTTP redirect downgrades

`DoRedirects` rejects redirects from HTTPS origins to plaintext HTTP targets with `ErrRedirectDowngrade`. Following such a redirect would leak any cookies or `Authorization` headers established under TLS. Set `SecurityPolicy.AllowHTTPSDowngrade = true` to override.

### Headers on a cross-origin redirect

When a redirect crosses to a **different origin**, that is another scheme, host or port ([RFC 6454](https://www.rfc-editor.org/rfc/rfc6454), where the default port stands in for a missing one), `DoRedirects` drops `Authorization`, `Proxy-Authorization`, `Proxy-Authenticate`, `WWW-Authenticate`, `Cookie` and the obsolete `Cookie2` from that hop and from every hop after it, so credentials bound to the original origin are not forwarded to a third-party upstream. This is `net/http`'s own set. A redirect to the same origin retains these headers. Spelling the default port out in the `Location` (`http://example.com/` to `http://example.com:80/next`) is still the same origin, while `http` to `https` is not.

:::caution Application-defined credentials
Every other header the request still carries after the [hop-by-hop stripping](#rfc-9110-hop-by-hop-header-stripping) is forwarded to the redirect target unchanged, as `net/http` does. That includes credentials the proxy cannot recognize, such as `X-Api-Key` or `X-Auth-Token`, whether your handler set them or the client sent them. The upstream chooses where a redirect goes, so if it can be made to redirect to a host you do not control (an open redirect, for example), list the names of such headers in `SecurityPolicy.RedirectSensitiveHeaders`, or call `Do` and decide on each redirect yourself.
:::

`SecurityPolicy.RedirectSensitiveHeaders` names extra request headers that `DoRedirects` drops on a cross-origin redirect, in addition to the set above. Names match case-insensitively, and surrounding whitespace and empty entries are ignored. The list is empty by default, so nothing changes until you set it. Only `DoRedirects` reads it: `Do`, `Forward`, `Balancer` and the other helpers never follow a redirect.

```go
policy := proxy.DefaultSecurityPolicy()
policy.RedirectSensitiveHeaders = []string{"X-Api-Key", "X-Auth-Token"}
proxy.WithSecurityPolicy(policy)
```

The Fiber client draws the line elsewhere, as `net/http` does: it keeps these headers for the same hostname on any port and for its subdomains, and drops them only for another hostname. A redirect from `http` to `https`, or to another port, of the same host therefore drops them here and keeps them there, see [SetRedirectSensitiveHeaders](../client/rest.md#setredirectsensitiveheaders) on the client.

### Redirect method and body

`DoRedirects` rewrites the request for each hop according to the status that sent it there ([RFC 9110 §15.4](https://www.rfc-editor.org/rfc/rfc9110#section-15.4)). A `303 See Other` turns every method except `GET` and `HEAD` into `GET`. A `301 Moved Permanently` or `302 Found` turns `POST` into `GET`, as browsers and fasthttp do, and leaves any other method alone. A `307 Temporary Redirect` or `308 Permanent Redirect` repeats the request as it was. When the method changes, the body is dropped, including a form your handler already read, together with the fields that frame or describe it or expect it: `Content-Type`, `Content-Length`, `Content-Encoding`, `Content-Language`, `Content-Location`, `Transfer-Encoding`, `Trailer`, `Expect`, `Digest` and `Content-MD5`. A hop that differs from the request your handler holds goes out as a copy, so that request keeps its method, body, form, uploaded files and credentials. The copy keeps what it is sent with: the request timeout, the user values and the body of a multipart form the server parsed before your handler ran.

A redirect that keeps the body of a request whose body is streamed (`StreamRequestBody`) is followed when the first hop left all of that body in memory: a body of known length that fits what the server reads ahead of the handler, 8 KiB, an empty one included. A chunked body or a longer one fails the redirect with `fasthttp.ErrRedirectBodyStream`, as in fasthttp's own redirect loop: the first hop consumed the stream, and what it left cannot be sent again. A redirect that drops the body goes on.

### RFC 9110 hop-by-hop header stripping

`Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, and `Upgrade` are stripped from both the outbound request and the inbound response, along with every header listed in the `Connection` field per RFC 9110 §7.6.1. This prevents request smuggling (`TE`/`Transfer-Encoding`), proxy-credential forwarding, and protocol-upgrade leaks. The legacy `KeepConnectionHeader` option preserves only the literal `Connection` header for backwards compatibility; the other hop-by-hop headers are still stripped. To preserve every hop-by-hop header (not recommended), set `SecurityPolicy.KeepHopByHopHeaders = true`.

The names a client lists in `Connection` are removed from the request as it was received, with one intentional exception to RFC 9110 §7.6.1: the forwarding headers `X-Real-IP`, `Forwarded`, and every `X-Forwarded-*` header stay. They tell the upstream about the client as this hop saw it, and an upstream that trusts the gateway acts on them, so they are the gateway's to write and not the client's to have removed. `Balancer`, `Forward`, `DomainForward`, and `BalancerForward` write `X-Real-IP` only after the listing has been applied, and the exception also covers a forwarding header your application sets before the proxy runs, including `X-Real-IP` set by hand before `Do`. A client loses nothing by it: a forwarding header it sends is forwarded whenever `Connection` does not name it.

:::caution Headers set before the proxy runs
Any other header your application adds to the request before the proxy handler runs — in an earlier middleware, or before calling `Do` — is part of the received request as far as the `Connection` listing is concerned, and a client can have it removed by naming it there. Set headers meant for the upstream in `Balancer`'s `ModifyRequest`, which runs after the listing has been applied.
:::

### Via header

A `Via` field a message already carries is forwarded unchanged. To add the proxy to the chain ([RFC 9110 §7.6.3](https://www.rfc-editor.org/rfc/rfc9110#section-7.6.3)), set the field in `ModifyRequest` and `ModifyResponse`:

```go
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{"http://localhost:3001"},
    ModifyRequest: func(c fiber.Ctx) error {
        c.Request().Header.Add(fiber.HeaderVia, "1.1 gateway")
        return nil
    },
    ModifyResponse: func(c fiber.Ctx) error {
        c.Response().Header.Add(fiber.HeaderVia, "1.1 gateway")
        return nil
    },
}))
```

### TLS minimum version

`Config.TLSConfig` is cloned with `MinVersion: tls.VersionTLS12` if no minimum is configured, so deprecated TLS versions cannot be negotiated by accident.

### Upstream protocol

The request reaches the upstream as HTTP/1.1, whatever protocol the inbound request used. A protocol token inherited from it, such as `HTTP/2` behind the `net/http` adaptor, is reset so that the request line stays valid.

### Response body size, timeouts and connection caps

`Config.MaxResponseBodySize` bounds upstream response bodies to protect against memory exhaustion. `Config.MaxConnsPerHost` (default `1024`) caps concurrent connections per upstream to limit fan-out from a single hot host. `Balancer` also bounds each request with `Config.Timeout` (default one second).

The runtime helpers (`Do`, `DoRedirects`, `Forward`, `DomainForward` and `BalancerForward`) dispatch through a client with fasthttp's defaults: no read or write timeout and no response size cap. `DoTimeout` and `DoDeadline` bound a single request, and the time they allow, like `Config.Timeout`, covers the connect as well. To bound every request, hand the proxy a client with `ReadTimeout`, `WriteTimeout` and `MaxResponseBodySize` set, registered with `proxy.WithClient` or passed per call:

```go
proxy.WithClient(&fasthttp.Client{
    NoDefaultUserAgentHeader: true,
    DisablePathNormalizing:   true,
    ReadTimeout:              30 * time.Second,
    WriteTimeout:             10 * time.Second,
    MaxResponseBodySize:      10 << 20, // 10 MiB
})
```

### X-Real-IP spoof prevention

`Balancer`, `Forward`, `DomainForward`, and `BalancerForward` automatically overwrite the `X-Real-IP` header with `c.IP()` before forwarding, so clients cannot spoof their address. The address is read from the request as received and written after the `Connection` listing has been applied, and `X-Real-IP` is exempt from that listing, so a client cannot have the header removed by naming it there either (see [hop-by-hop header stripping](#rfc-9110-hop-by-hop-header-stripping)). `Balancer` writes it before `ModifyRequest` runs, so `ModifyRequest` can still set its own value. `DomainForward` only applies the overwrite when the request host matches the configured hostname (matched case-insensitively per RFC 9110 §4.2.3, with or without a port in the `Host` header); non-matching requests are passed on to the next handler unchanged. A configured hostname without a port matches any port. One with a port, such as `example.com:8080` or `[::1]:8080` for an IPv6 literal, matches only that port. A hostname written with non-ASCII letters is converted to Punycode (`xn--` labels) once when the handler is built, because clients send that form in `Host`. After any of the forwarding helpers returns, the request carries its original URI and `Host` again, and a redirect that `DoRedirects` follows leaves its method, body, form and credentials as the client sent them, so middleware running after `Next` still sees that request. The hop-by-hop headers removed before dispatch stay removed, and `X-Real-IP` keeps the value written for the upstream.

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

`Balancer`, `DomainForward` and `BalancerForward` forward the path the router matched, `c.Path()`, followed by the query. They do not forward the request line as it arrived. The two differ, and the difference is what an attacker uses: fasthttp's normalization of the raw request line decodes `%2F` into a separator and merges repeated slashes, so `/public/..%2Fadmin/secret` and `//admin/secret` used to reach the upstream as `/admin/secret`, a path no middleware mounted on `/admin` had seen. Routed, they stay `/public/..%2Fadmin/secret` and `//admin/secret`, and by default the proxy refuses to forward either, since an upstream that decodes `%2F` or merges `//` would read them as `/admin/secret` as well. Dot segments are resolved before matching, so `/public/../admin/secret` runs the `/admin` middleware and is forwarded as `/admin/secret`. An escape of an unreserved character is decoded (`%41` becomes `A`), every other escape is kept as sent and a stray `%` is forwarded as `%25`. A byte a path may not carry raw ([RFC 3986 Section 3.3](https://www.rfc-editor.org/rfc/rfc3986#section-3.3)), such as `|`, `{`, `"` or a byte of UTF-8, is forwarded as its escape, as fasthttp wrote it before, so `/über` reaches the upstream as `/%C3%BCber`; the sub-delims (`!$&'()*+,;=`), `:` and `@` stay raw, a `;` only under `SecurityPolicy.AllowPathParameters` (see below). With `UnescapePath` enabled the decoded path is escaped again segment by segment, so a separator that came from `%2F` is forwarded as the separator the router matched it as, and a `;`, which the router then takes for part of a name, as `%3B`.

The query is forwarded byte for byte as the client sent it, so a signed URL keeps its signature. Only when a handler changed the arguments through `QueryArgs()` is it the changed arguments, as fasthttp serializes them; reading them, as `c.Query()` does, changes nothing. `Balancer` puts the original request line back once the upstream has answered, so `c.OriginalURL()` in a middleware that runs afterwards is unchanged.

- `Balancer` reaches each server with the port and the scheme it was configured with, whichever the request arrived on: an entry without a port is dialed on 80 (`http`) or 443 (`https`), and an `https` server works behind a plain-HTTP listener, as an `http` server does behind a TLS one. With a custom `Config.Client` the `HostClient`s are yours, and so is the scheme they expect: set it on the request in `ModifyRequest`.
- `Balancer` dials the configured host and forwards the target as is, so every entry in `Servers` must be a scheme and host only. An entry with a path, userinfo, query or fragment panics at startup with `ErrUpstreamNotOrigin` instead of dropping that part silently, which would have left `http://backend/api` reaching all of the upstream. An entry without a scheme, such as `backend:8080`, is read as `http://backend:8080`, and a lone trailing slash (`http://backend/`) is ignored. `DomainForward` and `BalancerForward` prepend the path of their upstream URL, and the joined path keeps the upstream host pinned in configuration whatever the request contains: `//attacker.example/path`, `@attacker` and `/foo://hijack.example` all stay paths.
- `Balancer` answers a request whose `Host` header carries userinfo, such as `svc:pw@backend`, with `400 Bad Request`. fasthttp would read the userinfo as credentials and send them upstream as a `Basic` `Authorization` header, replacing one the application had set.
- `Balancer` also answers `400 Bad Request` for a routed target fasthttp would read as an authority rather than a path: one that begins with `//` and holds `://`, or any target beginning with `//` on a request without a `Host` header. The router never saw such a target as an authority, and it need not have arrived as one: `..` resolution builds `//u:pw@evil.example/a://b` out of `//u:pw@evil.example/a:/x/..//b`, and under `UnescapePath` a decoded `%3A%2F%2F` spells the `://`. Set as the request line, it would hand the upstream a `Host`, and a `Basic` `Authorization` header, taken from the path rather than from the application.
- `Balancer`, `DomainForward` and `BalancerForward` answer `400 Bad Request` for a routed path an upstream could read as another path, one under a prefix whose middleware never ran for it. That is a path holding:
  - an escaped slash (`%2F`) or an empty segment (`//`). An upstream that decodes `%2F` into a separator or merges repeated slashes before it matches routes, as a `net/http` file server or a fasthttp server does, would map `/public/..%2Fadmin/secret`, `/admin%2Fsecret` and `//admin/secret` to `/admin/secret`. An upstream that keeps both as sent can be reached with `SecurityPolicy.AllowAmbiguousSlashes`, described below.
  - a backslash, raw or escaped as `%5C`. WHATWG URL parsers, as in Node.js, Bun, Deno and Cloudflare Workers, and IIS read it as a separator, so `/admin\secret` and `/public/..%5Cadmin/secret` would reach `/admin`.
  - a path parameter, a `;` in any segment, such as `/admin;x/secret`, `/admin;/secret`, `/;/admin/secret` or `/public/..;/admin/secret`. Servlet containers such as Tomcat and Jetty strip the parameters from every segment before they match routes and resolve dot segments, so each of these would reach `/admin/secret`. A `;` in the query is no path parameter, and a client that means a literal `;` in a name can send it as `%3B`, which is forwarded as sent, except behind a custom `BalancingClient` (see below). An upstream that relies on `;jsessionid` can be reached with `SecurityPolicy.AllowPathParameters`, described below.
  - an escape of an unreserved character, such as `%2e` or `%70`. The router decodes every such escape when it normalizes a path, so one still in `c.Path()` can only have been forged by a stray `%`, and an upstream decoding it again would serve a name no middleware matched.

  To forward such a target regardless, build it from `c.Path()` and call `Do`.
- `SecurityPolicy.AllowAmbiguousSlashes` lets `Balancer`, `DomainForward` and `BalancerForward` forward a path holding `%2F` or `//`, for an upstream that addresses names such as GitLab-style `group%2Fproject` ids or object keys with a doubled slash. Set it on `Config.SecurityPolicy` for `Balancer`, or install it with `proxy.WithSecurityPolicy` for `DomainForward` and `BalancerForward`. Enable it only when every upstream behind the handler keeps `%2F` and `//` as sent: one that decodes `%2F` reads `/a%2F../b` as `/b`. A path parameter still needs `AllowPathParameters`; a backslash, a dot segment with parameters and a forged escape are refused regardless.
- `SecurityPolicy.AllowPathParameters` lets `Balancer`, `DomainForward` and `BalancerForward` forward a path holding path parameters, such as `/cart;jsessionid=x`, for an upstream that relies on `;jsessionid` URL rewriting. Set it where you would set `AllowAmbiguousSlashes`. Enable it only when every upstream behind the handler keeps the `;` as part of the name or checks access itself: one that strips the parameters reads `/admin;x/secret` as `/admin/secret`, a path the middleware on `/admin` in Fiber never matched. A dot segment with parameters (`..;`), behind a raw slash or an escaped one, a backslash and a forged escape are refused regardless. So is a segment made of parameters alone before another segment, such as `/;x/admin`, since it is an empty segment once the parameters are stripped, unless `AllowAmbiguousSlashes` is set as well; at the end of the path, as in `/app/;jsessionid=x`, it only spells the trailing slash and is forwarded. Under `UnescapePath` the setting has no effect: the router reads a `;` as part of the name, and it is forwarded as `%3B`.
- A path override without a leading slash, as a rewrite of `/go/*` to `$1` produces for `/go/http://u:pw@evil.example/x`, is forwarded rooted, as the path `/http://u:pw@evil.example/x`, rather than as the absolute URL a request line would read it as.
- With a custom `Client`, every `*fasthttp.HostClient` among its `Clients` gets `DisablePathNormalizing` set, since a client left normalizing would decode `%2F` into a separator and merge `//` on the way out. Any other `BalancingClient` cannot be told, so with one present `Balancer` answers `400 Bad Request` for a routed target that normalization would change: one holding an empty segment or an escape it would decode into a byte it then writes raw, such as `%3B` or `%40`. An escape of a byte it writes escaped again, such as `%20` or the `%C3%BC` of `/über`, is forwarded.
- `Do`, `Forward`, `DomainForward`, `BalancerForward` and their variants switch path normalization off on the default client, on a client registered with `WithClient` and on every host client a per-call `*fasthttp.Client` creates once the proxy has it. fasthttp applies that setting only to the host client it creates for an upstream, so a per-call client that already reached that upstream before it was handed to the proxy keeps the host client it made then, and that one still normalizes: `/public/..%2Fadmin/secret` would leave it as `/admin/secret`. Register such a client with `WithClient` before it serves requests, hand the proxy a client used for nothing else, or create the client with `DisablePathNormalizing: true`.

:::caution Upstream normalization
The proxy cannot control what the upstream does with the target beyond what it refuses to forward. A target holding `%2F`, `//`, a backslash, a path parameter (`;`) or a forged escape is not forwarded by default, so an upstream that decodes, merges or strips parameters cannot be handed a path the router did not match through them. An escape of another reserved character, such as `%3B`, `%3F` or `%23`, is forwarded as sent, and so are a segment's case and a trailing dot or space, any of which an upstream on Windows or a case-insensitive filesystem may treat as another name. Authorize on the upstream as well.
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
// Forward, DomainForward and BalancerForward read this policy on every
// request, so keep it installed while the app serves: the defer below runs
// when the enclosing function returns, so do this in main, not in a helper
// that returns before the app starts.
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

// Proxy requests while following redirects, and keep an application API key
// from leaving the origin it was sent to
policy := proxy.DefaultSecurityPolicy()
policy.RedirectSensitiveHeaders = []string{"X-Api-Key"}
proxy.WithSecurityPolicy(policy)

app.Get("/proxy", func(c fiber.Ctx) error {
    // Delete first, as for X-Real-IP above: Set replaces only one line of a
    // header the client sent twice (see there for DisableHeaderNormalizing)
    c.Request().Header.Del("X-Api-Key")
    c.Request().Header.Set("X-Api-Key", os.Getenv("UPSTREAM_API_KEY"))
    return proxy.DoRedirects(c, "https://api.example.com/items", 3)
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
        // Balancer has already written X-Real-IP; add what else the upstream needs.
        c.Request().Header.Set("X-Gateway", "edge")
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
| Servers         | `[]string`                                     | Servers defines a list of `<scheme>://<host>` HTTP servers, which are used in a round-robin manner. Each entry is a scheme and host only; one with a path, userinfo, query or fragment panics at startup with `ErrUpstreamNotOrigin`. An entry without a scheme (`host:port`) is read as `http://host:port`, and a lone trailing slash is ignored. i.e.: "[https://foobar.com](https://foobar.com), [http://www.foobar.com](http://www.foobar.com)"                                                        | (Required unless `Client` is set) |
| ModifyRequest   | `fiber.Handler`                                | ModifyRequest allows you to alter the request.                                                                                                                                                                                     | `nil`           |
| ModifyResponse  | `fiber.Handler`                                | ModifyResponse allows you to alter the response.                                                                                                                                                                                   | `nil`           |
| Timeout         | `time.Duration`                                | Timeout is the request timeout used when calling the proxy client.                                                                                                                                                                 | 1 second        |
| MaxConnsPerHost | `int`                                          | Maximum number of connections per upstream host. The default proxy client and balancer host clients use this limit unless you override it with `WithClient`, a per-handler client, or `proxy.Config`.                         | `1024`          |
| ReadBufferSize  | `int`                                          | Per-connection buffer size for reading upstream responses. This also limits the maximum response header size: raise it if an upstream sends multi-KB headers. The headers of client requests are limited by [`ReadBufferSize`](../api/fiber.md#readbuffersize) in the app's config instead. | (Not specified) |
| WriteBufferSize | `int`                                          | Per-connection buffer size for writing requests to the upstream.                                                                                                                                                                   | (Not specified) |
| KeepConnectionHeader | `bool` | Keeps the `Connection` header when set to `true`. By default the header is removed to comply with RFC 9110 §7.6.1 and avoid proxy loops. Other hop-by-hop headers are still stripped regardless of this setting. | `false` |
| TLSConfig       | `*tls.Config` | TLS config for the HTTP client. Cloned with `MinVersion: tls.VersionTLS12` when no minimum is set. | `nil`           |
| DialDualStack   | `bool`                                         | Client will attempt to connect to both IPv4 and IPv6 host addresses if set to true.                                                                                                                                                | `false`         |
| Client          | `*fasthttp.LBClient`                           | Client is a custom client when client config is complex. Each `*fasthttp.HostClient` among its `Clients` gets `DisablePathNormalizing` set when the balancer is built, unless it has it set already, in which case it is left alone; see [Request target](#request-target). When it is set, `Servers`, `Timeout`, `MaxConnsPerHost`, `ReadBufferSize`, `WriteBufferSize`, `MaxResponseBodySize`, `TLSConfig` and `DialDualStack` are not used, and its dialers are yours to guard (see [SSRF protection](#ssrf-protection)).                                                                                                                                                                           | `nil`           |
| SecurityPolicy  | `*SecurityPolicy`                              | Overrides the default SSRF, redirect, and hop-by-hop header rules for this balancer. When `nil`, the package-level policy set via `WithSecurityPolicy` is used. Either way it is settled once, when the balancer is built. See [Security](#security).                                          | `nil`           |
| MaxResponseBodySize | `int`                                       | Maximum upstream response body size in bytes. `0` keeps fasthttp's unlimited default, so set it when an upstream is not trusted to answer with a small body.                                                                                                                                              | `0`             |

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

`Servers` (or `Client`) is required: `proxy.Balancer()` without a `Config` panics at startup, as `proxy.Balancer(proxy.Config{})` does, and so does a `Client` without `Clients`.

## Default SecurityPolicy

When `Config.SecurityPolicy` is `nil` (and `proxy.WithSecurityPolicy` has not been called), the package falls back to the value returned by `proxy.DefaultSecurityPolicy()`:

```go
// DefaultSecurityPolicy returns the secure-by-default policy.
func DefaultSecurityPolicy() proxy.SecurityPolicy {
    return proxy.SecurityPolicy{
        AllowedSchemes:           []string{"http", "https"},
        RedirectSensitiveHeaders: nil,
        AllowPrivateIPs:          false,
        AllowHTTPSDowngrade:      false,
        KeepHopByHopHeaders:      false,
        AllowAmbiguousSlashes:    false,
        AllowPathParameters:      false,
    }
}
```
