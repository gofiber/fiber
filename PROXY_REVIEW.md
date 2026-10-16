# Proxy middleware review: findings and fixes

Scope: `middleware/proxy/` and `docs/middleware/proxy.md`, reviewed on two axes
(Standards: does the code follow the repo's documented standards; Spec: does it
do what the docs and commit messages say). Line numbers in the findings refer to
the code as reviewed (`fcc95e7`), before any fix.

Legend: `- [ ]` open, `- [x]` done. Each finished item says how it was fixed.

Ground rule from the request: the tests that lack `t.Parallel()` stay
sequential. Each got a comment saying the omission is intended so the test
checks what it is meant to check.

## 1. Standards: hard violations

- [x] **H1** `AGENTS.md`: "`t.Parallel()` at the start of each test and subtest".
  23 of 176 tests and 2 of 21 subtests in `middleware/proxy` had none:
  - `proxy_test.go`: 114, 698, 1039, 1086, 1133
  - `security_rebinding_test.go`: 153 (+ subtest 184), 230, 245, 268, 296,
    339 (+ subtest 348), 372, 390, 408, 424, 435, 455, 474
  - `security_test.go`: 147, 335, 592, 696
  - `target_test.go`: 686

  **Fixed as requested:** `t.Parallel()` was not added. Each of the 25 now opens
  with a comment starting `Intentionally not parallel:` that names the
  process-wide state it owns (the security policy, the DNS resolver or the shared
  client) or the timing it measures, and says it must run alone to test that
  properly. The four notes that already existed use the same wording now. An AST
  scan of the package finds exactly these 25 and none without the note.
- [x] **H2** `CONTRIBUTING.md`: features need unit tests. No test touched
  `Config.MaxResponseBodySize`, `SecurityPolicy.KeepHopByHopHeaders` or
  `Config.WriteBufferSize`, and three branches were never run
  (`security.go:464-466`, `:633`, `:952-955`).

  **Fixed:** statement coverage of the package went from 99.2% to 100.0%, with no
  uncovered block. New tests:
  - `Test_Proxy_NewHostClient`: the config reaches the host client
    (`WriteBufferSize`, `ReadBufferSize`, `MaxResponseBodySize`, `MaxConns`,
    `DialDualStack`, TLS and its TLS 1.2 floor, the dial guard).
  - `Test_Proxy_Balancer_MaxResponseBodySize`: an upstream body over the limit
    fails with `fasthttp.ErrBodyTooLarge`; one within it, or no limit, passes.
  - `Test_Proxy_Balancer_WriteBufferSize`: request headers larger than the buffer
    arrive whole.
  - `Test_Security_KeepHopByHopHeaders_Balancer` and `_Do`: the opt-out lets the
    hop-by-hop fields through in both directions; the default strips them.
  - `Test_Security_NewGuardedClientDialer_DialsValidatedIPWithGuardDialer`
    (`:633`): the strict-policy dial of a validated IP goes through the guard's
    own dialer.
  - `Test_Coverage_CanFastJoinPath_RepeatedSeparator` (`:952-955`).
  - `:464-466` was the zero-address check after a DNS lookup. A `net.Resolver`
    never returns an empty list with a nil error, so no test could reach it; it
    went away when the two host validators were merged (K1c).

## 2. Standards: judgement calls

- [x] **J1** Prefer `gofiber/utils/v2` helpers: `strings.Clone` (`proxy.go:484`)
  next to `utils.CopyString`; `net.SplitHostPort` (`security.go:487`, `:590`)
  next to `utils.SplitHostPort`.
  **Fixed:** `doActionWithPolicy` uses `utils.CopyString`; `dialGuard.dialChecked`
  uses `utils.SplitHostPort`, which follows `net.SplitHostPort`'s rules and
  reports failure as `ok == false`.
- [x] **J2** `bench_test.go:102`: `_, _ = resolveRedirect(...) //nolint:errcheck // bench`
  was not a justification.
  **Fixed:** the loop keeps the error, and the benchmark asserts after the loop
  that the last result is `ErrRedirectDowngrade`, which also replaces the
  pre-loop anchor.
- [x] **J3** Secure by default: `MaxResponseBodySize` is unbounded by default and
  the default client has no read/write timeout, so `Do`, `Forward`,
  `DomainForward` and `BalancerForward` have neither a timeout nor a body cap.
  **Fixed by documenting, not by changing defaults** (a default timeout or body
  cap would cut long streams and large transfers for existing users). New docs
  section "Response body size, timeouts and connection caps" with a client that
  sets `ReadTimeout`, `WriteTimeout` and `MaxResponseBodySize`; matching godoc on
  `defaultClient`, `Forward`, `Do` and `Config.MaxResponseBodySize`.
- [x] **J4** Docs out of step with the code.
  **Fixed:** the `ModifyRequest` example no longer sets `X-Real-IP` (it adds an
  `X-Gateway` header and says `Balancer` already wrote `X-Real-IP`); the `Config`
  godoc (`Client`, `Servers`, `SecurityPolicy`, `MaxResponseBodySize`) now says
  what the docs table says.
- [x] **J5** RFC: no `Via` header and no documented deviation; the obsolete
  RFC 7230 was still cited.
  **Fixed:** new docs section "Via header" states that the middleware does not
  add `Via` (RFC 9110 §7.6.3), that an existing one is forwarded unchanged, and
  shows how to add one with `ModifyRequest`/`ModifyResponse`. Every RFC 7230 §6.1
  citation (code, test comments, docs heading and anchor, the config table) now
  cites RFC 9110 §7.6.1.
- [x] **J6** No benchmark for `Balancer`, `Forward` or `BalancerForward`.
  **Fixed:** `BenchmarkBalancer_ProxyPath`, `BenchmarkForward_ProxyPath` and
  `BenchmarkBalancerForward_ProxyPath` (a bare fasthttp context and an upstream
  that never dials). The setup `BenchmarkDomainForward_HostMatchPath` used is now
  the shared `benchNoopUpstream`.

## 3. Code smells (all judgement calls)

- [x] **K1** Duplicated Code.
  - [x] (a) tail of `DomainForward` and `BalancerForward`: now one `forwardRouted`.
  - [x] (b) `Balancer`'s routed-target check: it carries two conditions the
    forwarders do not have (`ParsesAsPath`, the opaque-client check), so it stays
    its own; what is common is already `segmentsAsRouted`. Nothing further to share.
  - [x] (c) `validateHostForSSRF` and `resolveAndValidateHost`: the first now
    calls the second.
  - [x] (d) the hop-by-hop `if cfg.KeepConnectionHeader {...} else {...}` blocks:
    the exempt-header list is built once, when `Balancer` is built, and both
    strips take it.
  - [x] (e) `newSSRFDialer` and `guardedDial` repeated split, resolve and dial:
    both go through `dialGuard.dialChecked`.
- [x] **K2** Data Clumps. `dialDualStack` and the dialer for a validated address
  travelled together through five dial-guard functions.
  **Fixed:** they are the two fields of `dialGuard`; the five functions became its
  methods (`strictDialer`, `clientDialer`, `clientDialerWithTimeout`,
  `guardedDial`, `dialValidatedIPs`), and the five `//nolint:revive` went with
  them. The `normalized` flag is now only on `stripHopByHopRequestHeaders`, where
  it is hot-path and mirrors `fieldname.Del`'s own documented flag: it left
  `delConnectionListedHeaders` (production always passed `false`),
  `stripCrossHostHeaders` and `followRedirects` (a rare path that can search every
  spelling without a flag). `//nolint:revive` directives: 9 before, 1 after.
- [x] **K3** Divergent Change: `security.go` (985 lines) mixed policy, header
  stripping, upstream validation, dial guards, IP ranges, TLS and URL joining, and
  the client guard sat in `proxy.go`.
  **Fixed:** split by concern, as a pure move first (the top-level declarations
  were diffed before and after): `security.go` (policy, 226), `headers.go`
  (hop-by-hop, cross-host and `X-Real-IP` handling, 164), `upstream.go` (parsing,
  validation, path joining, 280), `ssrf.go` (host validation, dial guards, client
  guard, 305), `blockedip.go` (the blocklist, 142); `secureTLSConfig` moved to
  `config.go`; `proxy.go` keeps the handlers (713).
- [x] **K4** Mysterious Name.
  - [x] (a) the package-level `client` is `sharedClient`; nothing shadows it now.
  - [x] (b) the comments citing "P2/P3/P6" and "the PR description" say what they
    mean; no such reference is left.
  - [x] (c) `parseUpstreamScheme` is `parseAllowedUpstream`, and its comment says
    it checks the scheme and the host.
- [x] **K5** Middle Man: `containsCTLOrDEL` is inlined into `resolveRedirect`, with
  its explanation.
- [x] **K6** Speculative Generality: the unreachable `prev == nil` branch of
  `WithSecurityPolicy` is gone (`init` always installs a policy).

## 4. Spec issues

- [x] **P1** `Balancer()` with no argument built, then panicked on its first
  request.
  **Fixed:** `configDefault` runs the `Servers`/`Client` check on the no-argument
  path too, so `Balancer()` panics at startup like `Balancer(Config{})`. The
  existing test that codified the old behaviour was replaced; new
  `Test_Proxy_Balancer_NoArgsPanicsAtStartup`. The wording in `whats_new.md`
  (both places) and `proxy.md` no longer promises "defaults".
- [x] **P2** Policy lifetime: `Balancer` snapshots the policy when built, the
  forwarders and the dial guard read it on every request.
  **Fixed in the docs**, since the code matches its godoc: new docs section "When
  the policy is read" (with the restore-after-build trap), a comment on the
  example, `whats_new.md`, and the godoc of `SecurityPolicy`, `WithSecurityPolicy`,
  `Forward` and `Config.SecurityPolicy`.
- [x] **P3** `proxy.md` said same-host redirects keep credentials, but another
  port strips them.
  **Fixed in the docs** (the stricter code is the safer one): a different origin
  means another host or another port; new `Test_Security_FollowRedirects_StripsCredentialsOnPortChange`.
- [x] **P4** Only POST was turned into GET; a 303 for PUT/PATCH/DELETE was
  replayed with its method and body.
  **Fixed in the code:** `redirectRequest` follows RFC 9110 §15.4: 303 turns every
  method but GET and HEAD into GET, 301/302 turn POST into GET, 307/308 repeat the
  request; a changed method drops the body and its framing fields. Tests:
  `Test_Security_RedirectRequest` (14 cases) and
  `Test_Security_FollowRedirects_MethodPerStatus`.
- [x] **P5** `ErrUpstreamHostInvalid` was documented as covering "cannot be
  parsed", but `errors.Is` was false for a parse failure.
  **Fixed in the code:** a parse failure wraps both the sentinel and the
  `*url.Error` (`errors.Is` and `errors.As` both work). Godoc and docs extended;
  `Test_Security_ValidateUpstream_UnparseableIsHostInvalid`.
- [x] **P6** Godoc trailed the docs (`Config.Client`, `Config.Servers`,
  `SecurityPolicy`, `WithSecurityPolicy`).
  **Fixed:** updated, together with J4 and P2.

## 5. Scope creep (behaviour nobody asked for, or never documented)

- [x] **X1** The outbound request is forced to `HTTP/1.1`.
  **Fixed in the docs:** section "Upstream protocol", and a line in `Do`'s godoc.
- [x] **X2** `followRedirects` rewrote the method and dropped the body headers on
  301/302/303 without saying so.
  **Fixed in the docs:** section "Redirect method and body" and a paragraph in
  `whats_new.md`, matching `redirectRequest` (see P4).
- [x] **X3** `Servers` accepts a scheme-less `host:port` and a lone trailing
  slash, which the docs did not say.
  **Fixed in the docs, godoc and a test:** this is the backward compatibility the
  code comment asks for, so it is documented and pinned by
  `Test_Security_ValidateUpstreamForBalancer_Forms`.

## 6. Left as it was, on purpose

- No default timeout or response-size cap (J3), and no `Via` header (J5): both
  would change behaviour for existing users, so they are documented instead.
- `Balancer`'s own routed-target check (K1b) and the `normalized` flag on
  `stripHopByHopRequestHeaders` (K2), for the reasons given on those items.

## 7. Verification

Run from the repository root on the final tree (the sandbox has go1.26.0):

| Check | Result |
| :--- | :--- |
| `make format` (gofumpt) | nothing to reformat |
| `make betteralign` | nothing to change |
| `make lint` (golangci-lint v2.14.0, whole repo) | 0 issues |
| `make test` (whole repo, `-race -count=1 -shuffle=on`) | 7535 tests, 5 skipped, 0 failures |
| proxy package, `-race -shuffle=on` | passes; statement coverage 100.0% (was 99.2%) |
| `make generate` | no changes (it needs `$(go env GOPATH)/bin` on `PATH` here) |
| `make audit` | `go mod verify` and `go vet ./...` pass. `govulncheck` reports 34 standard-library vulnerabilities, but only because the Makefile pins this sandbox's go1.26.0; the same scan on go1.26.9 reports none |
| `markdownlint-cli2` on the changed docs | 0 issues |

Not run: the benchmarks beyond a short smoke run (the three new ones execute and
allocate 8, 11 and 12 times per proxied request), since the request was to fix
the findings, not to profile them.

## 8. Commits

Three commits on `ccr-244a0c81-7ahfe2` carry the work, in this order:

1. `3a3f03e` 🧹 chore: split the proxy middleware's security.go by concern (a pure move, K3)
2. `103a287` 🐛 bug: fix the proxy review findings (Balancer(), redirect methods, upstream errors)
3. `2046765` 📒 docs: document the proxy middleware's policy lifetime, redirect rules and limits

This file was added in a fourth commit. It is a working document and can be
dropped before the branch is merged.
