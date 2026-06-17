# Proxy Middleware Security Audit

Scope: `middleware/proxy/*`, `docs/middleware/proxy.md`, `client/` (redirect
handling).

This document records the findings of the security audit and the
remediations applied in the same change. Severities follow CVSS-style
qualitative bands.

## Summary

| # | Severity | Category | Status |
|---|----------|----------|--------|
| 1 | Critical | SSRF — no private/internal IP blocking | Fixed |
| 2 | High     | URL path concatenation can mutate upstream host | Fixed |
| 3 | High     | Scheme allowlist not enforced (file://, gopher://) | Fixed |
| 4 | High     | HTTPS→HTTP redirect downgrade in `DoRedirects` | Fixed |
| 5 | High     | Hop-by-hop headers leaked (RFC 7230 §6.1) | Fixed |
| 6 | High     | HTTPS→HTTP redirect downgrade in `client.composeRedirectURL` | Fixed |
| 7 | Medium   | TLS minimum version not enforced | Fixed |
| 8 | Medium   | No upstream response body size cap | Fixed (opt-in) |
| 9 | Medium   | No per-host connection cap | Fixed (opt-in) |
| 10| Low      | Aliasing of caller-supplied address into URI buffer | Fixed |
| 11| High     | DNS-rebinding (check/use gap) between validation and dial | Fixed |
| 12| High     | Credentials forwarded across host on redirect | Fixed |
| 13| Medium   | Unbounded DNS lookup / startup panic on DNS failure | Fixed |

## Post-review hardening

The following were added in response to PR review (#4405):

### 11. DNS-rebinding (check/use gap) — Balancer dial-time revalidation (High)

Validating the hostname once up front does not stop a malicious resolver
from returning a public IP during validation and a private one at connect
time. The Balancer now installs an SSRF-guarded `fasthttp.HostClient.Dial`
(`newSSRFDialer`) that re-resolves and re-validates every resolved address
at dial time (rejecting if *any* answer is blocked) before connecting,
when `AllowPrivateIPs` is false. Runtime helpers (`Do`/`Forward`/…) keep
the best-effort up-front check; callers that need rebinding protection
there should supply a client with their own guarded dialer.

### 12. Cross-host credential stripping on redirect (High)

`followRedirects` now drops `Authorization`, `Proxy-Authorization`, and
`Cookie` when a redirect crosses to a different host, so origin-bound
secrets are not leaked to a third-party upstream. Same-host redirects keep
their headers.

### 13. Bounded DNS resolution / no startup panic (Medium)

DNS lookups during validation and dialing use `net.DefaultResolver` with a
5s `context` timeout so a slow resolver cannot hang a request. The
Balancer no longer performs DNS at construction time for hostname
upstreams (only IP literals are checked eagerly); resolution/validation is
deferred to the dial guard, avoiding crash loops when DNS is briefly
unavailable at startup.

## Findings

### 1. SSRF — no private/internal IP blocking (Critical)

**Location.** `middleware/proxy/proxy.go` (Balancer, doAction,
DomainForward, BalancerForward).

**Issue.** Any caller-supplied upstream address was forwarded without
validation. Targets such as `127.0.0.1`, `10.x.x.x`, `192.168.x.x`,
`169.254.169.254` (cloud metadata), `::1`, and CGNAT addresses were all
reachable. Combined with applications that build upstream URLs from
request data, this is a classic SSRF surface.

**Fix.** New `SecurityPolicy` struct with `AllowPrivateIPs bool`
(default `false`). `validateUpstream` resolves the hostname and rejects
loopback, RFC 1918, link-local (covers `169.254.169.254`), multicast,
unspecified, and RFC 6598 CGNAT addresses unless explicitly allowed.
When any DNS answer resolves to a blocked range the upstream is
rejected, mitigating DNS-rebinding attempts that mix public and private
answers. Exposed via `Config.SecurityPolicy` and package-level
`proxy.WithSecurityPolicy`.

### 2. URL path concatenation can mutate upstream host (High)

**Location.** `middleware/proxy/proxy.go:245` (DomainForward),
`proxy.go:285` (BalancerForward).

**Issue.** Both helpers used `addr + c.OriginalURL()`. A request whose
path begins with `//` (or contains `@`) was a network-path reference
that, when re-parsed downstream, could redirect the proxy to a different
host.

**Fix.** New `joinUpstreamPath` helper rebuilds the URL with the
validated upstream base and a sanitized path; leading `//` is collapsed
and any authority/scheme component on the request side is discarded.
Covered by `FuzzJoinUpstreamPath`.

### 3. Scheme allowlist not enforced (High)

**Location.** `middleware/proxy/proxy.go:230` (`getScheme`).

**Issue.** `getScheme` extracted whatever scheme the caller supplied;
fasthttp's HostClient happily attempted `gopher://`, `ftp://`, etc.
`file://` URLs (without a host) were also possible via direct
`SetRequestURI` paths.

**Fix.** Replaced `getScheme` with `validateUpstream`, which checks the
scheme against `SecurityPolicy.AllowedSchemes` (default `["http",
"https"]`).

### 4. HTTPS→HTTP redirect downgrade in DoRedirects (High)

**Location.** `middleware/proxy/proxy.go:153-160` (DoRedirects).

**Issue.** `DoRedirects` delegated the entire redirect loop to
`fasthttp.Client.DoRedirects`, which follows any 3xx Location, including
plaintext HTTP after an HTTPS handshake. Cookies, `Authorization`
headers, and other secrets sent on the HTTPS leg would leak in the
clear.

**Fix.** `DoRedirects` now drives its own redirect loop
(`followRedirects` / `resolveRedirect`). Each hop is revalidated against
the active `SecurityPolicy`; HTTPS→HTTP transitions return
`ErrRedirectDowngrade` unless `AllowHTTPSDowngrade` is set.

### 5. Hop-by-hop headers leaked (High)

**Location.** `middleware/proxy/proxy.go:72-74,206,210`.

**Issue.** Only the `Connection` header was being stripped. RFC 7230 §6.1
requires intermediaries to strip every header listed in `Connection`
plus `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`,
`Trailer`, `Transfer-Encoding`, and `Upgrade`. Leaving these in place
enables:

- Request smuggling via mismatched `Transfer-Encoding` / `TE`.
- Forwarding of `Proxy-Authorization` to internal services.
- Protocol-upgrade smuggling via `Upgrade`.

**Fix.** New `stripHopByHopRequestHeaders` and
`stripHopByHopResponseHeaders` helpers honor RFC 7230 §6.1. The legacy
`KeepConnectionHeader` option preserves only the literal `Connection`
header for back-compat; the others are still removed. Override via
`SecurityPolicy.KeepHopByHopHeaders` for full back-compat with the
previous behavior (not recommended).

### 6. HTTPS→HTTP redirect downgrade in client.composeRedirectURL (High)

**Location.** `client/transport.go:354-377`.

**Issue.** The Fiber HTTP client's redirect helper restricted Location
schemes to http/https but happily downgraded an HTTPS request to
plaintext HTTP, mirroring the proxy bug.

**Fix.** `composeRedirectURL` now refuses the downgrade with
`client.ErrRedirectDowngrade`. Same-origin and upgrade transitions are
unaffected. Tested in `client/transport_test.go`.

### 7. TLS minimum version not enforced (Medium)

**Location.** `middleware/proxy/proxy.go:49`.

**Issue.** The HostClient inherited the caller's `TLSConfig` as-is. If
the caller passed `nil` or omitted `MinVersion`, fasthttp could
negotiate TLS 1.0/1.1.

**Fix.** `secureTLSConfig` clones the caller's config and forces
`MinVersion: tls.VersionTLS12` when no minimum is set; the caller's
struct is never mutated.

### 8. No upstream response body size cap (Medium)

**Location.** `config.go` (missing field).

**Issue.** A malicious or compromised upstream could send unbounded
response bodies and exhaust proxy memory.

**Fix.** New `Config.MaxResponseBodySize` plumbed through to
`HostClient.MaxResponseBodySize`. `0` keeps fasthttp's unlimited default
for back-compat; production deployments should set a finite limit.

### 9. No per-host connection cap (Medium)

**Location.** `config.go` (missing field).

**Issue.** A single upstream could saturate the proxy's outbound
sockets (file descriptors) if no per-host cap was set, easing
resource-exhaustion DoS.

**Fix.** New `Config.MaxConnsPerHost` plumbed through to
`HostClient.MaxConns`. `0` falls back to fasthttp's
`DefaultMaxConnsPerHost` (512).

### 10. Aliasing of caller-supplied address into URI buffer (Low)

**Location.** `middleware/proxy/proxy.go` (`doActionWithPolicy`).

**Issue.** Discovered during fix implementation. When the caller passes
an `addr` string that itself is a slice of the request buffer (e.g.
`strings.TrimPrefix(c.OriginalURL(), "/")`), Go's `url.Parse` returns a
`url.URL` whose `Scheme`/`Host` fields alias the same memory. The
proxy's `SetRequestURI(targetURL)` then overwrote that buffer mid-flow,
corrupting the parsed scheme/host bytes before they were applied.
Triggered the regression `unsupported protocol "ttp:"`.

**Fix.** `strings.Clone` snapshots the scheme and target URL into fresh
allocations before `SetRequestURI` is called.

## Verification

```sh
# Run the proxy and client test suites
go test ./middleware/proxy/ ./client/ -count=1 -skip IPv6

# Run the fuzz seeds and a short fuzz session
go test ./middleware/proxy/ -run 'Fuzz' -count=1
go test ./middleware/proxy/ -run '^$' -fuzz FuzzValidateUpstream -fuzztime=10s
go test ./middleware/proxy/ -run '^$' -fuzz FuzzJoinUpstreamPath -fuzztime=10s

# Linting
golangci-lint run ./middleware/proxy/... ./client/...

# Lint Markdown docs (this file and docs/middleware/proxy.md)
make markdown
```

The `Test_Proxy_Balancer_IPv6_Upstream*` tests fail on hosts without
IPv6 (pre-existing, also fails on `main`).

## Behavioral changes (breaking)

These align with the user-approved "secure-by-default (breaking)"
direction. Migration is a small Config addition for each:

1. Proxy targets resolving to loopback / RFC 1918 / link-local /
   multicast / CGNAT addresses are rejected by default. To proxy to
   internal services on the same network, set
   `Config.SecurityPolicy.AllowPrivateIPs = true` (or call
   `proxy.WithSecurityPolicy` for the runtime helpers).
2. Only `http` and `https` upstream schemes are accepted by default.
3. `DoRedirects` rejects HTTPS→HTTP redirects.
4. RFC 7230 §6.1 hop-by-hop headers are stripped both ways.
5. `Config.TLSConfig` is cloned with `MinVersion: tls.VersionTLS12`
   when no minimum is set.
6. The Fiber HTTP client (`client.Client.DoRedirects`) rejects
   HTTPS→HTTP downgrades.

## Out of scope / follow-ups

- Per-IP or per-host rate limits (would belong in a separate middleware,
  e.g. `middleware/limiter`).
- Cookie/Authorization stripping on cross-origin redirects in the client
  package (separate hardening task).
- Sandboxing of `ModifyRequest`/`ModifyResponse` user callbacks (out of
  scope; user code is trusted).
