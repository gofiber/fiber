# Proxy Middleware Performance Plan

Goal: cut per-request allocations and ns/op on the proxy hot paths
(`Do`, `Forward`, `Balancer`, `DomainForward`, `BalancerForward`,
`DoRedirects`) without changing behavior, security guarantees, or the
public API.

## Baselines

Captured locally on Go 1.25, Linux, `-benchtime=200ms`. Numbers are best
of 3 runs; full table:

```
BenchmarkCurrentSecurityPolicy                57.2 ns/op    32 B/op   1 allocs/op
BenchmarkResolvePolicy_Nil                    58.1 ns/op    32 B/op   1 allocs/op
BenchmarkResolvePolicy_Override               42.3 ns/op    32 B/op   1 allocs/op
BenchmarkSchemeAllowed_HTTP                    9.0 ns/op     0 B/op   0 allocs/op
BenchmarkSchemeAllowed_EmptyAllowlist          9.4 ns/op     0 B/op   0 allocs/op
BenchmarkValidateUpstream_IPLiteral          549.6 ns/op   160 B/op   2 allocs/op
BenchmarkValidateUpstreamForBalancer_IPLit   444.1 ns/op   160 B/op   2 allocs/op
BenchmarkResolveRedirect_HTTPSDowngradeBlk   962.5 ns/op   192 B/op   4 allocs/op
BenchmarkResolveRedirect_AllowedAcrossOrig  1003   ns/op   192 B/op   4 allocs/op
BenchmarkStripHopByHop_NoConnection          294.4 ns/op     0 B/op   0 allocs/op
BenchmarkStripHopByHop_WithConnection        280.8 ns/op     0 B/op   0 allocs/op
BenchmarkJoinUpstreamPath_RootBase           433.6 ns/op   224 B/op   3 allocs/op
BenchmarkJoinUpstreamPath_PrefixBase         441.2 ns/op   232 B/op   3 allocs/op
BenchmarkIsBlockedIP_PublicV4                 67.2 ns/op     0 B/op   0 allocs/op
```

These benchmarks live nowhere yet; **item P0** of this plan adds them
checked-in so every change has a before/after.

## Findings (file:line, why it costs, what to do)

### P1 — Per-request `AllowedSchemes` deep-copy (`security.go:101-146`)

`currentSecurityPolicy()` and `resolvePolicy()` call
`normalizePolicy()`, which does
`append([]string(nil), policy.AllowedSchemes...)` on every read. That's
the 32 B / 1 alloc baseline above, and it fires once per
`Do`/`Forward`/`Balancer`/`DomainForward`/`BalancerForward` request.

**Why the copy exists.** Defensive: if a caller passes a slice to
`WithSecurityPolicy` and later mutates it, the global allowlist would
shift outside `policyLock`. CodeRabbit flagged this earlier.

**Fix.** Move the defensive copy to *install* time only. Switch the
global store to `atomic.Pointer[SecurityPolicy]`, populated via
`WithSecurityPolicy(policy)` which clones once and stores. Readers do
`p := activePolicy.Load(); return *p`, returning the struct by value
with a slice header that aliases the immutable backing array. The
`sync.RWMutex` goes away too.

**Expected delta.** `currentSecurityPolicy()` → ~5 ns / 0 alloc.
Compounds across every Do/Forward request.

**Risk.** Low — same defensive guarantee, moved earlier in the
lifecycle. Existing `Test_Security_WithSecurityPolicy_RestoresDefaults`
covers the install/restore contract.

### P2 — `DomainForward` / `BalancerForward` re-validate the configured upstream on every request (`proxy.go:380-396`, `420-435`)

Both helpers call `validateUpstream(addr, policy)` inside the returned
handler. The `addr` is the *handler-construction-time* upstream — it
never changes per request. So we're paying `url.Parse` (160 B / 2
allocs, ~450 ns) on every match.

**Fix.** Lift the validation into the constructor. Parse + validate
once; keep the `*url.URL` in the closure. The handler then only does
the host-match / X-Real-IP / `joinUpstreamPath` work and calls
`doActionWithPolicy` with the validated URL string. For
`BalancerForward`, validate every server once at construction; the
round-robin then indexes into a `[]*url.URL` slice.

**Expected delta.** ~450 ns and 160 B per matching request removed.
For workloads that lean on `DomainForward`/`BalancerForward`, that's
a measurable hit. Construction cost moves from zero to one
`validateUpstream` per server; trivially amortized.

**Risk.** Low. Validation is moved earlier, not removed. Failures now
panic at handler construction instead of returning a 500 — but
that's already the contract for `Balancer` (which uses
`validateUpstreamForBalancer` at construction). Document in
the migration notes.

### P3 — `doActionWithPolicy` allocates `validateUpstream`'s output twice (`proxy.go:226-244`)

```go
u, err := validateUpstream(addr, policy)
...
scheme := strings.Clone(u.Scheme)
targetURL := strings.Clone(u.String())
req.SetRequestURI(targetURL)
req.URI().SetSchemeBytes([]byte(scheme))
```

`u.String()` already builds a fresh string. `strings.Clone(u.String())`
clones it a second time. Same for `u.Scheme`: `url.Parse` typically
returns scheme as a `strings.ToLower(...)` allocation, already
independent of the input.

**Why the clones exist.** Workaround for the aliasing regression we
hit during the original security pass — `u.Scheme` could share bytes
with the caller's `addr`, which itself may be a slice of the request
buffer about to be overwritten by `SetRequestURI`.

**Fix.** Audit `url.Parse`'s string ownership for the inputs we
actually pass. Today's `addr` paths come from (a) caller string
literal, (b) `c.OriginalURL()` (Fiber-owned, not buffer-aliasing
post-Clone), or (c) `joinUpstreamPath` (returns a fresh `u.String()`).
None of these alias the request buffer once we reach
`doActionWithPolicy`. The aliasing bug occurred specifically when
`addr` was a slice of `req.Header.requestURI`. As long as `addr` is a
Go string with a different backing array, `u.String()` produces a
fresh allocation we own.

**Conservative fix.** Drop the second `strings.Clone` (on
`u.String()`) — it's redundant. Keep the `u.Scheme` clone only if a
focused test confirms the aliasing path is still reachable.

**Expected delta.** ~30-60 B and 1 alloc per request.

### P4 — `joinUpstreamPath` parses paths it could short-circuit (`security.go:478-525`)

The common path through `DomainForward`/`BalancerForward` is a
request path like `/api/v1/widgets?q=1` against a base with no path
prefix (`http://upstream.example`). That hits `url.Parse` (one
alloc), URL field assignment, `out.String()` (second alloc), plus
the loop-allocation in `strings.TrimSuffix`/`strings.TrimPrefix`.
The bench shows 224 B / 3 allocs / 433 ns.

**Fix.** Add a fast path: when the request path starts with `/` and
contains none of the security-relevant patterns (`//`, `@` before
the first `?`, control bytes, scheme literal `://`), and the base
has no path prefix to merge, we can build the result as
`base.Scheme + "://" + base.Host + requestPath` directly with a
single `strings.Builder.Grow + Write`. The fuzz tests
(`FuzzJoinUpstreamPath`) already cover the safety of this shortcut
— any input where the shortcut fires would also have to pass the
full path through unchanged today.

**Expected delta.** Target ~120 B / 1 alloc / ~150 ns on the common
case. Falls back to the existing logic for crafted paths.

**Risk.** Medium — every condition in the fast-path test is a
potential bypass. Mitigation: keep the fuzz test, add specific
seeds for every short-circuit decision boundary, run the existing
`Test_Security_JoinUpstreamPath_*` suite against the new code.

### P5 — `resolveRedirect` allocates `previousScheme` and `[]byte("https")` every hop (`proxy.go:338-360`)

```go
previousScheme := append([]byte(nil), uri.Scheme()...)
...
if ... bytes.EqualFold(previousScheme, []byte(schemeHTTPS)) ...
```

`append([]byte(nil), ...)` is one heap allocation; `[]byte(schemeHTTPS)`
is another. Both happen per redirect hop. Bench: 192 B / 4 allocs.

**Fix.**
- `previousScheme` is at most 5 bytes ("https"). Hold it in an
  array on the stack: `var schemeBuf [8]byte; previousScheme :=
  append(schemeBuf[:0], uri.Scheme()...)`. Same semantics, no heap.
- Replace `[]byte(schemeHTTPS)` with a package-level
  `var httpsBytes = []byte(schemeHTTPS)`.

**Expected delta.** -2 allocs / ~32 B per `resolveRedirect` call.

**Risk.** Trivial.

### P6 — `followRedirects` re-runs `validateUpstream` on the initial URL (`proxy.go:268-281`)

`doActionWithPolicy` already validated `addr` and produced a `*url.URL`.
Then `followRedirects` reads `req.URI().FullURI()` (already a string
allocation), passes it back into `validateUpstream`, which does
`url.Parse` again and may do a DNS lookup for the SSRF check.

**Why it's there.** Carrying the validated `*url.URL` through made
the CodeQL alert clean. The "validate again at the loop entry"
comment notes this is defensive plumbing.

**Fix.** Plumb the validated `*url.URL` from `doActionWithPolicy`
into `followRedirects`. The CodeQL guarantee is preserved — the
sink (`req.SetRequestURI(currentURL.String())`) is still fed only
from a `validateUpstream` return value, just one we computed
already.

**Expected delta.** -160 B / -2 allocs / ~-450 ns *and* one fewer
DNS lookup on the redirect path.

**Risk.** Low — change the `followRedirects` signature to take a
`*url.URL` initial target. Only one caller (`DoRedirects`).

### P7 — Hop-by-hop strip is fine; one micro-opt available (`security.go:163-211`)

`stripHopByHopRequestHeaders` and `connectionListedHeaders` already
hit 0 allocs in the benches (294 ns no-Connection, 281 ns
with-Connection). The only remaining cost is `string(v)` inside
`connectionListedHeaders` which would only fire when the Connection
header *is* present. Replacing `strings.SplitSeq(string(v), ",")`
with `bytes.SplitSeq(v, []byte{','})` and using
`utils.TrimSpace` on the byte slice keeps it 0-alloc but shaves a
few ns. Low priority; do only if P1-P6 don't move overall numbers
enough.

### P8 — `DefaultSecurityPolicy()` allocates an `AllowedSchemes` slice every call (`security.go:87-94`)

Called from `schemeAllowed` (fallback when `allowed` is nil) and any
test/caller that asks for a fresh default. Currently 0 allocs in the
schemeAllowed bench because the fallback rarely fires — but if
`policy.AllowedSchemes` is nil somewhere (which can happen for a
default-zeroed `SecurityPolicy`), each request allocates a fresh
`[]string{schemeHTTP, schemeHTTPS}`.

**Fix.** Introduce a package-level immutable
`var defaultAllowedSchemes = []string{schemeHTTP, schemeHTTPS}` and
reference it from both `DefaultSecurityPolicy()` and `schemeAllowed`'s
nil-fallback. Callers retain the documented behavior of being able
to mutate `policy.AllowedSchemes` they obtained from
`DefaultSecurityPolicy()` because *they* asked for a snapshot — but
mutating that slice no longer affects the shared default backing
array (since the snapshot copies on first install via P1's
`WithSecurityPolicy`).

**Expected delta.** Nominal on the bench, but removes a sharp edge
where a forgotten `AllowedSchemes: nil` would alloc per request.

**Risk.** Low. Document that `DefaultSecurityPolicy().AllowedSchemes`
must not be mutated in-place; install policies through
`WithSecurityPolicy` or `Config.SecurityPolicy`.

## Sequencing

1. **P0 — bench scaffolding.** Land `bench_test.go` first so every
   subsequent PR can quote `benchstat` deltas. Tiny change, no risk.
2. **P1 — atomic policy pointer.** Biggest single per-request win,
   touches one file, well tested. Land alone.
3. **P2 — lift `DomainForward`/`BalancerForward` validation to
   construction.** Behaviorally biggest, document the
   "panic-at-construction-on-bad-config" change.
4. **P5 + P8 — micro-allocs (`previousScheme`, `httpsBytes`,
   `defaultAllowedSchemes`).** Trivial, group into one PR.
5. **P6 — plumb validated `*url.URL` into `followRedirects`.**
6. **P3 — drop the redundant `strings.Clone(u.String())` after
   adding a regression test for the aliasing path.**
7. **P4 — `joinUpstreamPath` fast path.** Last because it has the
   highest correctness risk; needs the fuzz suite and dedicated
   boundary tests.

Each PR runs `go test ./middleware/proxy/... -race -count=1` and
`benchstat` against the captured baseline. The plan target is **at
least 50% fewer allocations and ~30% lower ns/op** on the
`BenchmarkValidateUpstream_*` and `BenchmarkResolveRedirect_*` paths,
and **near-zero per-request alloc** on the
`currentSecurityPolicy`/`resolvePolicy` path.

## Out of scope (documented to avoid scope creep)

- Replacing `fasthttp.AcquireURI`/`ReleaseURI` with a custom pool —
  fasthttp's pool is already pooled, no clear win.
- Changing TLS handling. `secureTLSConfig` runs at construction, not
  per request.
- Connection-pool tuning (`MaxConnsPerHost`, `MaxConnDuration`) —
  configurable, not a code change.
- DNS caching. The dial-time SSRF guard already short-circuits on
  IP literals; adding a TTL cache risks defeating DNS rebinding
  protection and is a separate design discussion.

## Verification gate per PR

- `go test ./middleware/proxy/... -race -count=1 -skip IPv6`
- `go test ./middleware/proxy/... -fuzz Fuzz... -fuzztime=10s` for
  P3, P4, P6 (anything touching URL handling)
- `benchstat baseline.txt new.txt` showing the targeted bench
  improved without regressing the others
- `golangci-lint run ./middleware/proxy/...`
