# Extractor Performance Plan

This document is the implementation plan for making the `extractors` package
(`github.com/gofiber/fiber/v3/extractors`) and the internal helpers it runs on
cheaper per request, without changing any public API or documented behavior.
It was written from measurements taken on this branch at commit `6cb9479`
(Go 1.26.0, linux/amd64, `-cpu 1`), and every step below was prototyped
against the existing test suite before being written down, so the expected
numbers are measured, not estimated.

Read the whole document before writing code. Sections 1 to 3 are the rules;
section 4 is the work, in the order it should be done; section 5 is the
acceptance bar; section 6 is the delivery checklist; section 7 lists what was
considered and rejected, so it is not tried again.

## 1. Scope and ground rules

### 1.1 What is in scope

- `extractors/extractors.go`: `Chain`, `ExtractWithSource`, the cycle guard,
  the winner-capture bookkeeping, `FromAuthHeader`'s token68 scan,
  `FromParam`'s config read.
- `internal/headerlookup/headerlookup.go` and `internal/fieldname/fieldname.go`:
  the header reads behind `FromHeader` and `FromAuthHeader`.
- `internal/ctxlocal`: unchanged, but used.
- One small hook in the root `fiber` package (a new internal package plus an
  `init` in the root) so hot paths can read three `Config` booleans without
  copying the whole 624-byte `Config` struct.
- `middleware/session/store.go`: the per-request `Locals` write that boxes a
  72-byte `Extractor` by value.

### 1.2 What must not change

- **Public API.** No new exported identifiers in `extractors`, no signature
  changes, no new fields on `Extractor`. The positional literal
  `Extractor{fn, "k", "", nil, SourceCustom}` must keep compiling
  (`Test_ExtractWithSource/unkeyed_literal_still_compiles_without_extra_fields`).
- **The `Ctx` interface.** Never add an exported method to `DefaultCtx`,
  `DefaultReq` or `DefaultRes`: `make generate` (ifacemaker) would add it to the
  generated interfaces and break every custom `Ctx`. Any root-package help for
  the extractors goes through an internal package, the way `internal/ctxlocal`
  already does.
- **Every documented behavior** in `extractors/README.md`,
  `docs/guide/extractors.md` and the godoc, and every existing test. The
  invariants that matter most are spelled out in section 3.
- **Allocation counts.** Every path that is 0 allocs/op today stays 0. The
  session, keyauth and csrf benchmarks must not gain an allocation.

### 1.3 Process rules

- Work step by step, one commit per step, in the order of section 4. Each
  commit must build, pass `go test ./extractors/... ./internal/... -race` and
  lint clean on its own.
- Measure before and after every step with the benchmarks from Step 0 and
  compare with benchstat:

  ```bash
  go test ./extractors/ -run '^$' -bench 'Benchmark_Extractor' -benchmem -count 10 -cpu 1 > /tmp/old.txt
  # ... make the change ...
  go test ./extractors/ -run '^$' -bench 'Benchmark_Extractor' -benchmem -count 10 -cpu 1 > /tmp/new.txt
  go run golang.org/x/perf/cmd/benchstat@latest /tmp/old.txt /tmp/new.txt
  ```

  Keep a step only if benchstat reports an improvement (p < 0.05) on the
  benchmarks the step targets and no regression elsewhere. If a step does not
  reproduce the numbers in section 5, stop and find out why before moving on;
  do not stack a second change on top of an unexplained result.
- The container's CPU frequency is not pinned, so use `-count 10` (or more)
  and read the benchstat confidence intervals, not single runs.
- Do not widen the work: no refactors of code the steps do not touch, no
  changes to `docs/` beyond what section 6 lists.

### 1.4 Tooling notes for this environment

- Go 1.26.0 is the module's Go version. The `golangci-lint` binary installed at
  `/usr/local/bin/golangci-lint` (v2.5.0, built with Go 1.25) refuses this
  module. Only the Makefile's pinned invocation works, and it does work here
  (verified, 0 issues on the baseline):

  ```bash
  make lint
  # or, for a few packages while iterating:
  GOTOOLCHAIN=go1.26.0 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./extractors/ ./internal/... .
  ```

- `make test` runs the whole suite with `-race -count=1 -shuffle=on` through
  gotestsum and takes several minutes; run package-level tests while iterating
  and the full target before pushing.
- `make betteralign` rewrites struct field order (`-apply`) across the repo;
  run it before `make format` and `make lint` so the order it wants is what
  gets linted.
- `make markdown` needs `markdownlint-cli2`; `npx --yes markdownlint-cli2
  "extractors/*.md"` works from this container. The config (`.markdownlint.yml`)
  forbids hard tabs even inside code blocks (MD010), which is why the Go in
  this document is indented with spaces. Run `gofmt` on anything copied out of
  it.
- Linters that bite in this code (all enabled in `.golangci.yml`):
  `errcheck` with `check-type-assertions` and `check-blank` (never `v, _ :=
  x.(T)`, never `_ = f()` for an error), `forcetypeassert` (always comma-ok),
  `nonamedreturns`, `exhaustive` (every `switch` on `Source` needs a `default`
  or all cases), `err113` (no dynamic errors), `wrapcheck`, `gocritic`,
  `revive` (`flag-parameter` fires on boolean parameters; the existing
  `//nolint:revive // flag-parameter: ...` comments in `fieldname` show the
  accepted form), `modernize`, `testifylint`, `tparallel` and the AGENTS.md rule
  that every test and subtest calls `t.Parallel()` first.

## 2. Baseline and where the time goes

All numbers are ns/op on one CPU with the benchmarks of Appendix A (the
`Benchmark_Extractor_*` names below are the committed ones; the baseline was
taken with the same bodies). "Hit" means the value is present, "Miss" that it
is absent. `Chain3` is `Chain(FromHeader, FromCookie, FromQuery)`.

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| FromHeader_Hit | 65.7 | 0 | 0 |
| FromHeader_Miss | 59.0 | 0 | 0 |
| FromHeader_Hit_NoNormalize (`DisableHeaderNormalizing`) | 161.6 | 80 | 3 |
| FromHeader_Hit_Immutable | 131.7 | 80 | 1 |
| FromAuthHeader_Bearer_Hit (JWT-sized token) | 175.5 | 0 | 0 |
| FromAuthHeader_Basic_Hit | 99.2 | 0 | 0 |
| FromAuthHeader_Miss | 63.3 | 0 | 0 |
| FromCookie_Hit | 12.6 | 0 | 0 |
| FromQuery_Hit | 17.5 | 0 | 0 |
| FromForm_Hit | 62.6 | 0 | 0 |
| FromParam_Hit | 16.2 | 0 | 0 |
| FromParam_Hit_Escaped | 101.7 | 8 | 1 |
| FromCustom_Hit | 2.5 | 0 | 0 |
| Chain1_Hit (one child, header present) | 116.5 | 0 | 0 |
| Chain3_HitFirst | 102.3 | 0 | 0 |
| Chain3_HitLast | 150.1 | 0 | 0 |
| Chain3_Miss | 143.4 | 0 | 0 |
| ChainNested_HitInner | 187.6 | 0 | 0 |
| ExtractWithSource_Leaf | 116.0 | 0 | 0 |
| ExtractWithSource_Chain3_HitFirst | 370.3 | 32 | 2 |
| ExtractWithSource_Chain3_HitLast | 467.3 | 32 | 2 |
| ExtractWithSource_Chain3_Miss | 272.5 | 0 | 0 |
| ExtractWithSource_Nested_HitInner | 708.2 | 64 | 4 |
| ExtractWithSource_BareChain_HitLast (nil `Extract`, `Chain` set) | 396.2 | 0 | 0 |
| FromHeader_Hit_FreshRequest (user values reset per iteration) | 66.4 | 0 | 0 |
| Chain1_Hit_FreshRequest | 108.5 | 0 | 0 |
| Chain3_HitLast_FreshRequest | 144.9 | 0 | 0 |
| ExtractWithSource_Chain3_HitFirst_FreshRequest | 364.0 | 32 | 2 |
| Token68 (JWT + short base64 + early reject) | 131.2 | 0 | 0 |
| Token68_JWT | 92.0 | 0 | 0 |

Isolated costs measured with throwaway micro-benchmarks (not committed):

| What | ns/op | Note |
| --- | ---: | --- |
| `c.App()` | 2.0 | interface call |
| `c.App().Config()` then two field reads | 25.0 | the 624-byte copy is not elided by the compiler |
| `c.Locals(key)` (get, one entry stored) | 7.0 | linear scan with interface equality |
| `ctxlocal.Set(c, key, v)` | ~10 | scan plus append |
| Current `Chain.Extract` guard bookkeeping alone | 31.2 | 3 `Locals` gets/sets plus the depth read |
| Current `ExtractWithSource` capture bookkeeping alone | 195.9 | 2 allocs: the `[]Source` copy and its boxing |
| `fasthttp` `PeekAll("X-API-Key")` | 37.1 | 15 of it is `getHeaderKeyBytes` normalizing the key on every call |
| Storing an `Extractor` by value in `Locals` | 76.4 | 80 B, 1 alloc (session middleware does this per request) |
| Storing `*Extractor` in `Locals` | 10.5 | 0 allocs |
| Pooled per-request state, first use in a request | ~35 | `Locals` miss, `sync.Pool` Get, `Locals` set, recycle |
| Pooled per-request state, later uses | 9.5 | one `Locals` get |

CPU profile of `Chain3_HitFirst` plus `ExtractWithSource_Chain3_HitFirst`
(flat, top entries): `headerlookup.Combined` 13%, `userData.Set` 9%,
`DefaultReq.Locals` 8%, `userData.Get` 8%, the `Chain` closure 8%,
`runtime.efaceeq` 5%, `ctxlocal.Set` 5%, `normalizeHeaderKeyValidated` 4%,
`mallocgc` 3%. About 40% of the two benchmarks is `Locals` bookkeeping.

Root causes, in order of cost:

1. **`Locals` bookkeeping around every chain call.** `Chain.Extract` does four
   `Locals` operations per call (guard get, guard set, depth get, deferred guard
   clear), each a linear scan of fasthttp's user-value slice with `any`
   equality. `ExtractWithSource` adds five more for depth and the winner
   stack, and every winner push allocates a fresh `[]Source` and boxes it
   (`pushChainWinningSource`). This is the 195 ns and 2 allocs on top of a
   chain, 4 allocs when nested. Steps 1 fixes this.
2. **`App.Config()` returns 624 bytes by value.** `headerlookup.Value`,
   `headerlookup.Combined` and `headerlookup.Canonical` copy the whole struct
   to read two booleans, on every `FromHeader` and `FromAuthHeader` call.
   `FromParam` does it whenever the value contains `%`. Measured at 23 ns, a
   third of a header extraction. Step 3 fixes this.
3. **token68 scan.** `isValidToken68` runs a range-compare `switch` per byte.
   A 256-entry table is half the cost on a JWT-sized token. Step 2.
4. **`DisableHeaderNormalizing` path allocates three times per read.**
   `fieldname.Lines` passes a closure to `VisitAll`, which fasthttp implements
   on top of `All()`, so the closure and the `values` slice escape. Step 5.
5. **Session stores the winning `Extractor` by value in `Locals`.** One 80-byte
   allocation per request for any chain-configured session store. Step 4.

## 3. Invariants to preserve

These are pinned by tests in `extractors/extractors_test.go`; run the named
tests after every change to Step 1. If a redesign cannot satisfy one, the
redesign is wrong, not the test.

Chain semantics (`Test_Extractor_Chain*`, `Test_ExtractWithSource*`):

- First child returning a non-empty value with a nil error wins; an empty
  value with a nil error is a miss, not a win.
- Children with a nil `Extract` are skipped and do not touch the error.
- When every child fails, the last error is returned; `ErrNotFound` when no
  child produced an error; the empty `Chain()` always returns `ErrNotFound`
  with `Source == SourceCustom`.
- `Chain` runs a private copy of its children; mutating, clearing or making the
  public `Chain` field recursive after construction changes nothing about what
  `Extract` runs or what `ExtractWithSource` reports
  (`public_Chain_slice_mutation_does_not_affect_Extract`,
  `recursive_public_Chain_metadata_uses_captured_source`,
  `clearing_public_Chain_still_returns_captured_source`).
- Re-entering a chain that is already executing on the same request returns
  `ErrChainCycle`, whether the re-entry comes through `Extract` or
  `ExtractWithSource`, directly or through another chain
  (`Test_Extractor_Chain_Cycle_Prevention`, `source_path_detects_cycle`,
  `cross_api_reentry_via_Extract_detects_cycle`,
  `Test_ExtractWithSource_BareChain/self-referencing chain is refused`).
- The guard is released on return, including after a refused walk, so the
  same chain can run again later in the request, from any entry point
  (`shared_guard_allows_sequential_Extract_then_ExtractWithSource`,
  `Test_Extractor_Chain_ReusedAcrossMiddleware`).
- Chain identity is the public `Chain` backing array (address of its first
  element). A bare `Extractor{Chain: ...}` with nil `Extract` is walked by
  `ExtractWithSource` under the same identity scheme.

Source capture (`Test_Extractor_Chain_ExtractSource`, `Test_ExtractWithSource`):

- `ExtractWithSource` always calls `Extract` when it is set, so decorated or
  replaced `Extract` functions are honored; it never re-walks children to
  guess provenance and never runs a child twice
  (`no_double_extract_of_stateful_custom_child`,
  `replaced_Extract_without_base_uses_static_Source`).
- On success with a capture, the winning child's `Source` is reported; a
  nested chain's winner propagates outward
  (`nested_chain_reports_inner_winning_source`); a chain-level decorator that
  calls the base and succeeds still reports the winner
  (`chain_override_success_still_reports_winning_source`).
- A child that recorded a winner and was then rejected must not leak that
  winner to the next child that succeeds
  (`rejected_nested_chain_does_not_poison_fallback_source`,
  `shared_guard_blocks_cross_entry_during_source_path`).
- On any error, or an empty value, the declared static `Source` is returned
  and the value is empty.
- A plain `Extract` call must leave nothing behind that a later
  `ExtractWithSource` on an unrelated leaf could pick up
  (`bare_Extract_does_not_pollute_later_ExtractWithSource`).
- Leaves report their declared `Source`; a hand-rolled `Extractor` without a
  `Source` reports `SourceHeader` (the zero value).

Header semantics (`Test_Extractor_FromHeader_*`, `Test_Extractor_FromAuthHeader_*`,
`Test_FromHeader_IgnoresHeaderNameCase`, `Test_FromAuthHeader_IgnoresHeaderNameCase`):

- Header names match case-insensitively under both normalizing modes.
- `FromAuthHeader` refuses a request carrying `Authorization` on more than one
  line, whatever the lines hold; `FromHeader` joins repeated lines with `", "`
  in order received, except `Cookie`, which is re-assembled by fasthttp with
  `"; "`.
- A single present-but-empty line is `ErrNotFound`.
- `Immutable` still gets a copied string; without it the bytes are aliased.
- Token68 validation: `A-Z a-z 0-9 - . _ ~ + /`, then optional `=` padding,
  nothing after the padding, no leading `=`, no whitespace, exactly one space
  after the scheme, scheme matched case-insensitively
  (`Test_isValidToken68`, `Test_Extractor_FromAuthHeader_Token68_Validation`,
  `Test_Extractor_FromAuthHeader_RFC_Compliance`).

One test inspects the implementation rather than behavior:
`Test_ExtractWithSource_BareChain` asserts `ctx.Locals(guard) == false` after
a walk. Step 1 replaces that representation; rewrite the assertion to the new
internal query (`chainStateFor(ctx).isActive(guard)`) and keep the behavioral
half of the test (a second walk on the same ctx succeeds) as is.

## 4. The work

### Step 0: benchmarks first

Add `extractors/bench_test.go` with the contents of Appendix A, commit it on
its own, and record the baseline (`-count 10`) in a file you keep for the PR
description. The names start with `Benchmark_Extractor_` so `make benchmark`
and the CI benchmark workflow (which shards `-bench=.` and skips `_Parallel`
names) pick them up. The file has no `_Parallel` variants on purpose.

Two things in that file are deliberate and must stay:

- `FromParam` runs its loop inside a route handler invoked through
  `app.Handler()(fctx)` on the benchmark goroutine, because route parameters
  are only reachable from a matched route and `app.Test` would run the handler
  on another goroutine.
- The `_FreshRequest` benchmarks call `c.RequestCtx().ResetUserValues()` after
  every extraction. That is what fasthttp does between requests
  (`Request.Reset` calls `userValues.Reset`, which calls `Close` on any value
  implementing `io.Closer`), so those benchmarks measure the per-request
  first-use cost that the reused-ctx benchmarks hide. Step 1 is judged on both.

Also add a keyauth end-to-end benchmark, because the middleware that most
users run on every request has none today: in `middleware/keyauth/keyauth_test.go`
add `Benchmark_KeyAuth_Bearer` (a `FromAuthHeader("Bearer")` config with a
constant-time validator, one warmed `app.Handler()` call per iteration through
a `fasthttp.RequestCtx` that carries `Authorization: Bearer <jwt>`) and
`Benchmark_KeyAuth_Chain` (same with `Chain(FromAuthHeader("Bearer"),
FromHeader("X-API-Key"), FromQuery("api_key"))` and only the query present).
Follow the shape of `Benchmark_Middleware_CSRF_Check` in
`middleware/csrf/csrf_test.go` for the request setup.

### Step 1: one pooled per-request state for every chain

This is the largest win and the most delicate change. It replaces the guard
`Locals` entries, the depth counter and the winner stack with one request-local
value holding a small mutable struct.

#### Design

```go
// chainState is the bookkeeping every chain on one request shares: the chains
// currently executing (the cycle guard), how many ExtractWithSource frames are
// open (whether winners are recorded at all) and the Source of the last child
// that won while one was.
type chainState struct {
    active []*byte // guards of the chains executing right now, innermost last
    win    Source  // Source of the innermost winning child while hasWin
    depth  int     // open ExtractWithSource frames; winners are recorded while > 0
    hasWin bool
}

// chainStateKey is the Locals key of the request's chainState.
type chainStateKey struct{}

var chainStatePool = sync.Pool{
    New: func() any { return &chainState{active: make([]*byte, 0, 4)} },
}
```

- One `Locals` entry per request, keyed by the zero-size `chainStateKey{}`
  (boxing it costs nothing) and holding a `*chainState` (pointer-shaped, no
  allocation to box).
- The state comes from a `sync.Pool`. It goes back through `Close`: fiber's
  own `Locals` documentation promises that "Close method is called on each
  value implementing io.Closer before removing the value from ctx", and
  fasthttp's `userData.Reset` does exactly that on every `Request.Reset`
  (verified on the real request path: a test that counted `Close` calls
  across `app.Test` saw one per request). So a request that runs any number
  of chains allocates nothing for them.
- The cycle guard becomes a stack of chain identities (`active`), still the
  address of the first element of the public `Chain` array, so the identity
  scheme and the shared-guard semantics are unchanged. Checking membership is
  a pointer comparison over a slice that holds one or two entries.
- "Capture active" is `depth > 0`, a field read instead of a `Locals` get.
- The winner stack becomes a single `(win, hasWin)` pair. A stack was only
  needed because the previous design could not reset state cheaply; with a
  field, the parent chain clears `hasWin` before each child and reads it after,
  which gives exactly the truncate/pop semantics the tests pin (see the
  walkthrough below).

#### Code

Everything from the `ExtractWithSource` doc comment down to (not including)
`Contains` is replaced by the following, and the `chainGuardKey` type is
deleted. Add `"slices"` and `"sync"` to the imports (the `modernize` linter
rejects a hand-written contains loop over `active`). Indentation is spaces here
because of the markdown linter; gofmt it.

```go
func ExtractWithSource(e Extractor, c fiber.Ctx) (string, Source, error) {
    if e.Extract != nil {
        st := chainStateFor(c)
        st.enterCapture()
        defer st.leaveCapture()

        v, err := e.Extract(c)
        // Read the capture before the deferred clear runs.
        src, captured := st.win, st.hasWin
        if err != nil {
            return "", e.Source, err
        }
        if v == "" {
            return "", e.Source, ErrNotFound
        }
        if captured {
            return v, src, nil
        }
        return v, e.Source, nil
    }
    if len(e.Chain) > 0 {
        return extractChainWithSource(&e, c)
    }
    return "", e.Source, ErrNotFound
}

// chainGuardFor returns the identity Chain.Extract and extractChainWithSource
// share for the public Chain backing array: the address of its first element,
// which is stable for the life of the array.
func chainGuardFor(chain []Extractor) (*byte, bool) {
    if len(chain) == 0 {
        return nil, false
    }
    return (*byte)(unsafe.Pointer(&chain[0])), true //nolint:gosec // G103: identity for the cycle guard only, never dereferenced
}

// chainStateFor returns the request's chainState, creating it on first use.
//
// The state is a Locals value, so fasthttp hands it back through Close when it
// resets the request's user values, which returns it to the pool: a request
// that runs any number of chains costs no allocation for them.
func chainStateFor(c fiber.Ctx) *chainState {
    if st, ok := c.Locals(chainStateKey{}).(*chainState); ok && st != nil {
        return st
    }
    st, ok := chainStatePool.Get().(*chainState)
    if !ok || st == nil {
        st = &chainState{active: make([]*byte, 0, 4)}
    }
    ctxlocal.Set(c, chainStateKey{}, st)
    return st
}

// Close returns the state to the pool. fasthttp calls it, as it does for every
// request-local value implementing io.Closer, when the request is reset; it is
// not for callers.
func (s *chainState) Close() error {
    s.active = s.active[:0]
    s.win = 0
    s.depth = 0
    s.hasWin = false
    chainStatePool.Put(s)
    return nil
}

// enter marks the chain identified by guard as executing and reports false,
// leaving the state untouched, if it already is: a cycle.
func (s *chainState) enter(guard *byte) bool {
    if slices.Contains(s.active, guard) {
        return false
    }
    s.active = append(s.active, guard)
    return true
}

// leave unmarks the innermost executing chain.
func (s *chainState) leave() {
    if n := len(s.active); n > 0 {
        s.active = s.active[:n-1]
    }
}

// isActive reports whether the chain identified by guard is executing.
func (s *chainState) isActive(guard *byte) bool {
    return slices.Contains(s.active, guard)
}

// enterCapture opens an ExtractWithSource frame. Any winner a previous frame
// left is forgotten so a leaf extracted next is not attributed to it.
func (s *chainState) enterCapture() {
    s.depth++
    s.hasWin = false
}

// leaveCapture closes an ExtractWithSource frame and forgets its winner.
func (s *chainState) leaveCapture() {
    s.depth--
    s.hasWin = false
}

func extractChainWithSource(e *Extractor, c fiber.Ctx) (string, Source, error) {
    guard, ok := chainGuardFor(e.Chain)
    if !ok {
        return "", SourceCustom, ErrNotFound
    }
    st := chainStateFor(c)
    if !st.enter(guard) {
        return "", e.Source, ErrChainCycle
    }
    defer st.leave()

    var lastErr error
    lastSource := e.Source
    for i := range e.Chain {
        child := &e.Chain[i]
        if child.Extract == nil && len(child.Chain) == 0 {
            continue
        }
        // Nested chains and leaves both go through ExtractWithSource.
        v, src, err := ExtractWithSource(*child, c)
        if err == nil && v != "" {
            return v, src, nil
        }
        if err != nil {
            lastErr = err
            lastSource = src
        }
    }
    if lastErr != nil {
        return "", lastSource, lastErr
    }
    return "", e.Source, ErrNotFound
}
```

In `Chain`, compute the guard once at construction and replace the closure:

```go
    kids := append([]Extractor(nil), extractors...)
    pub := append([]Extractor(nil), kids...)
    primarySource := kids[0].Source
    primaryKey := kids[0].Key
    // Guard on the public Chain array so ExtractWithSource shares the same
    // cycle identity (private kids stay execution-only).
    guard, _ := chainGuardFor(pub)

    return Extractor{
        Extract: func(c fiber.Ctx) (string, error) {
            st := chainStateFor(c)
            if !st.enter(guard) {
                return "", ErrChainCycle
            }
            defer st.leave()

            var lastErr error // last error encountered (including ErrNotFound)

            // Winners are recorded only inside an ExtractWithSource frame, so
            // a bare Extract does nothing but the cycle guard.
            capture := st.depth > 0
            for i := range kids {
                kid := &kids[i]
                if kid.Extract == nil {
                    continue
                }
                if capture {
                    // Forget whatever the previous child left behind.
                    st.hasWin = false
                }
                v, err := kid.Extract(c)
                if err == nil && v != "" {
                    // Prefer a Source a nested Chain.Extract recorded;
                    // otherwise the child's declared Source (leaves).
                    if capture && !st.hasWin {
                        st.win = kid.Source
                        st.hasWin = true
                    }
                    return v, nil
                }
                if err != nil {
                    lastErr = err
                }
            }
            if capture {
                // A chain that failed leaves no winner for its parent.
                st.hasWin = false
            }
            if lastErr != nil {
                return "", lastErr
            }
            return "", ErrNotFound
        },
        Source: primarySource,
        Key:    primaryKey,
        Chain:  pub,
    }
```

`guard, _ := chainGuardFor(pub)` is fine for `errcheck` (it is a bool, not an
error) but `dogsled`/`gocritic` may not like the blank; if they do, write
`guard, ok := chainGuardFor(pub)` and return the `notFound` extractor when
`!ok` (it cannot happen, `pub` is non-empty there). Loop by index with
`kid := &kids[i]` rather than `for _, extractor := range kids`: the range form
copies 72 bytes per child.

Update the `ExtractWithSource` doc comment: "pushed on a request-local stack"
becomes "recorded in request-local state". Nothing in `README.md` or
`docs/guide/extractors.md` describes the mechanism, so they stay.

#### Why the semantics are unchanged (walk these through against the tests)

- Winner attribution: the parent clears `hasWin` before each child. A leaf
  child leaves it false, so the parent records the leaf's declared `Source`.
  A nested built-in chain that wins sets `(win, hasWin)` before returning, so
  the parent keeps it. A custom leaf that internally ran a chain that won and
  then returned its own value is attributed to the inner winner, as today
  ("prefer a Source pushed by a nested Chain.Extract").
- Rejection: a decorated nested chain that wins and is then rejected returns
  an error; the parent goes to the next child and clears `hasWin` first, so
  the query fallback reports `SourceQuery`
  (`rejected_nested_chain_does_not_poison_fallback_source`).
- Legacy `Extract` never records anything because `depth == 0`; an
  `ExtractWithSource` frame clears `hasWin` on entry and on exit, so nothing
  leaks between frames (`bare_Extract_does_not_pollute_later_ExtractWithSource`).
- Cycles: `enter` refuses an identity already on the stack, and the deferred
  `leave` pops it on every return path including the refused one, so
  sequential use works and cross-API re-entry is still refused.
- Errors: `ExtractWithSource` returns the declared `e.Source` on any error and
  on an empty value, exactly as before.

#### Things to get right

- **Panics.** Keep the `defer`s. They are open-coded (no defer in a loop) and
  cost about a nanosecond. Without them a recovered panic inside a child would
  leave a guard on the stack for the rest of the request.
- **Custom `Ctx`.** `chainStateFor` reads through `c.Locals` and writes through
  `ctxlocal.Set`, so a custom context that overrides `Locals` keeps working;
  its store may never call `Close`, in which case the state is simply garbage
  collected. Add a test with `fiber.NewWithCustomCtx` (see
  `internal/ctxlocal/ctxlocal_test.go` for the recording-context pattern)
  covering a chain hit, a cycle and a sequential reuse.
- **A ctx without a fasthttp request** (`DefaultReq.Locals` returns the value
  without storing it when `r.c.fasthttp == nil`): every call would take a
  state from the pool and never return it. That is a released or fake context,
  which is misuse, but do not let it panic: the code above handles it (it just
  allocates).
- **`Close` is exported on an unexported type** so that `io.Closer` is
  satisfied; `unused` does not flag exported methods. Do not call it yourself
  outside tests; in tests use `require.NoError(t, st.Close())`, never
  `_ = st.Close()` (`errcheck` `check-blank`).
- **betteralign** had nothing to say about `chainState` in the field order
  shown above (checked with the pinned v0.8.0); if `make betteralign` still
  reorders it, accept what it produces.
- **Concurrency.** The state is per request and the pool is safe; run the
  package tests with `-race -count 5` and add a test that runs 64 goroutines
  each doing `AcquireCtx`, a chain hit, a cycle refusal and
  `ResetUserValues` in a loop, to exercise the pool under the race detector.

#### Tests to add or change (`extractors/extractors_test.go`)

- Rewrite the two `require.Equal(t, false, ctx.Locals(guard))` assertions in
  `Test_ExtractWithSource_BareChain` to
  `require.False(t, chainStateFor(ctx).isActive(guard))`.
- `Test_Chain_StateIsRecycledOnRequestReset`: register a route whose handler
  runs a chain, grabs `chainStateFor(c)`, sets `st.win = SourceQuery;
  st.hasWin = true` as a marker and saves the pointer; call
  `app.Handler()(fctx)` with your own `fasthttp.RequestCtx`, then
  `fctx.Request.Reset()` (what the server does between requests) and assert
  the marker is gone (`hasWin == false`, `len(active) == 0`, `depth == 0`).
  This pins that fasthttp closes the value.
- `Test_Chain_NoAllocationsWithSource`: `testing.AllocsPerRun(1000, ...)` of
  `ExtractWithSource(chain3, ctx)` after one warm-up call must be 0; same for
  the nested chain. This is the regression guard for the whole step.
- The custom-`Ctx` test and the concurrency test described above.
- Keep every existing test untouched otherwise.

#### Expected effect (measured on the prototype, reused ctx)

`Chain1_Hit` 116 → 81 ns, `Chain3_HitLast` 150 → 110, `ChainNested` 188 →
123, `ExtractWithSource_Leaf` 116 → 80, `ExtractWithSource_Chain3_HitFirst`
370 → 105 ns and 2 → 0 allocs, `ExtractWithSource_Nested` 708 → 143 ns and 4 →
0 allocs, `BareChain` 396 → 169. On a fresh request (state built and
recycled every time) the single-chain call is 108 → 95 ns,
so the common keyauth shape (one chain per request) does not regress, and any
second chain call in the same request costs 10 ns of state access instead of
31.

#### If the pooled state is rejected in review

Fallback design B, same semantics, no pool: keep three `Locals` entries but
store only values that box without allocating: the per-chain guard stays a
`bool` under the `chainGuardKey{id}` key as today; `depth` becomes an `int`
(values below 256 box for free); the winner becomes a single `int`-typed
`Source` under a `chainWinKey{}` plus a `bool` under `chainHasWinKey{}`
(clear before each child, read after). That removes every allocation from
`ExtractWithSource` and keeps the legacy path exactly as it is, but every
operation is still a `Locals` scan, so expect `ExtractWithSource_Chain3` around
170 ns instead of 105. Only do this if a maintainer asks.

### Step 2: token68 lookup table

Replace the `switch` in `isValidToken68` with a table. Semantics are identical:
non-empty, no leading `=`, a run of allowed bytes, then only `=`.

```go
// token68Chars marks the bytes token68 allows before padding: ALPHA, DIGIT and
// "-._~+/" (RFC 7235 section 2.1). "=" is handled separately because it is
// only valid as trailing padding.
var token68Chars = [256]bool{
    '-': true, '.': true, '_': true, '~': true, '+': true, '/': true,
    '0': true, '1': true, '2': true, '3': true, '4': true, '5': true, '6': true, '7': true, '8': true, '9': true,
    'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'F': true, 'G': true, 'H': true, 'I': true, 'J': true,
    'K': true, 'L': true, 'M': true, 'N': true, 'O': true, 'P': true, 'Q': true, 'R': true, 'S': true, 'T': true,
    'U': true, 'V': true, 'W': true, 'X': true, 'Y': true, 'Z': true,
    'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true, 'g': true, 'h': true, 'i': true, 'j': true,
    'k': true, 'l': true, 'm': true, 'n': true, 'o': true, 'p': true, 'q': true, 'r': true, 's': true, 't': true,
    'u': true, 'v': true, 'w': true, 'x': true, 'y': true, 'z': true,
}

func isValidToken68(token string) bool {
    if token == "" || token[0] == '=' {
        return false
    }
    i := 0
    for i < len(token) && token68Chars[token[i]] {
        i++
    }
    for ; i < len(token); i++ {
        if token[i] != '=' {
            return false
        }
    }
    return true
}
```

Keep the existing comment about the SWAR attempt (it was 16% slower than the
switch) and add that the table halved the switch. Do not build the table in
an `init` or with a named-return helper (`nonamedreturns`); the literal above
is fine.

Tests: keep the old `switch` implementation in the test file as
`isValidToken68Reference` and add `Test_isValidToken68_MatchesReference` that
checks every single byte 0..255 alone, at the start, middle and end of a valid
token, plus the padding shapes (`=`, `==`, `a=`, `a==`, `a==b`, `a=b`, `=a`,
`a===`), plus `Fuzz_isValidToken68` comparing the two on random input with a
small seed corpus. The existing `Test_isValidToken68` and the RFC compliance
tests stay.

Expected: `Token68_JWT` 92 → 45 ns, `FromAuthHeader_Bearer_Hit` 175 → 126 ns.

### Step 3: read hot Config bits without copying Config

Two booleans are read per header extraction and one per escaped path
parameter, and each read copies 624 bytes. The extractors cannot reach
`app.config`, and `App.Config()` must keep returning a copy, so the root
package hands the bits over through an internal hook. It is the same idea as
`internal/ctxlocal` (a cheaper path that the concrete type makes possible,
with the generic path as fallback), and it adds no exported API.

New package `internal/appconfig/appconfig.go`:

```go
// Package appconfig hands request hot paths the few Config bits they read on
// every call without the copy App.Config returns by value.
//
// Config is over 600 bytes, and Config() returns it by value, so a caller that
// wants one boolean out of it pays a copy of the whole struct, measured at
// 23ns: a third of a header extraction. Package fiber sets Lookup at init to
// read the bits straight off the app it owns; the default reports !ok so every
// caller has the by-value fallback.
package appconfig

// Hot is the subset of fiber.Config read on per-request hot paths.
type Hot struct {
    Immutable                bool
    DisableHeaderNormalizing bool
    UnescapePath             bool
}

// Lookup returns the Hot bits of app, which must be a *fiber.App. Package
// fiber replaces it at init; until then, or for any other value, ok is false.
var Lookup = func(any) (Hot, bool) { return Hot{}, false }
```

New file in the root package, `appconfig_hook.go`:

```go
package fiber

import (
    "github.com/gofiber/fiber/v3/internal/appconfig"
)

// The internal packages behind the extractors read a handful of Config bits on
// every request. Config() hands back a copy of the whole struct; this hook
// reads the bits directly, so those paths do not pay for the copy.
func init() {
    appconfig.Lookup = func(a any) (appconfig.Hot, bool) {
        app, ok := a.(*App)
        if !ok || app == nil {
            return appconfig.Hot{}, false
        }
        return appconfig.Hot{
            Immutable:                app.config.Immutable,
            DisableHeaderNormalizing: app.config.DisableHeaderNormalizing,
            UnescapePath:             app.config.UnescapePath,
        }, true
    }
}
```

In `internal/headerlookup/headerlookup.go` add:

```go
// hot returns the Config bits this package reads per request without copying
// the whole Config where the app is fiber's own.
func hot(c fiber.Ctx) appconfig.Hot {
    if h, ok := appconfig.Lookup(c.App()); ok {
        return h
    }
    return hotFromConfig(c.App().Config())
}

// hotFromConfig is the by-value fallback, kept separate so it can be tested
// without replacing Lookup.
func hotFromConfig(cfg fiber.Config) appconfig.Hot {
    return appconfig.Hot{
        Immutable:                cfg.Immutable,
        DisableHeaderNormalizing: cfg.DisableHeaderNormalizing,
        UnescapePath:             cfg.UnescapePath,
    }
}

// UnescapePath reports whether the router already percent-decoded the path,
// read the same cheap way as Canonical.
func UnescapePath(c fiber.Ctx) bool {
    return hot(c).UnescapePath
}
```

and replace the three `c.App().Config()` reads (`Canonical`, `Value`,
`Combined`) with `hot(c)`; the two `cfg := ...` locals keep their names since
only `.Immutable` and `.DisableHeaderNormalizing` are read. In `FromParam`,
replace `c.App().Config().UnescapePath` with `headerlookup.UnescapePath(c)`.

`gochecknoinits` and `gochecknoglobals` are not enabled in `.golangci.yml`, so
the `init` and the package variable lint clean (verified on the prototype; the
only finding was `grouper` insisting on the grouped `import (...)` form shown
above). `depguard` allows internal imports. The hook must be in a file of
package `fiber` (not `fiber_test`), and `go vet` must stay clean on the root.

Tests:

- Root package, new `appconfig_hook_test.go` (package `fiber`, parallel):
  `New(Config{Immutable: true, DisableHeaderNormalizing: true, UnescapePath:
  true})` and `New()` both report the matching `Hot` with `ok == true`;
  `Lookup(nil)`, `Lookup("x")` and `Lookup((*App)(nil))` report `ok == false`.
- `internal/headerlookup` has no tests today. Add `headerlookup_test.go`
  covering `Value` (single line, repeated line refused, empty line, both
  normalizing modes, `Immutable` copies), `Combined` (join, `Cookie` special
  case, empty line, both modes), `Canonical`, `UnescapePath`, and
  `hotFromConfig`. The existing extractor tests exercise most of this
  indirectly; the point is a direct home for the package's behavior now that it
  has a second code path.

Expected: `FromHeader_Hit` 66 → 52 ns, `FromAuthHeader_Bearer_Hit` 126 → 112
(after Step 2), `FromAuthHeader_Miss` 63 → 51, `FromParam_Hit_Escaped` 102 →
84, and every chain benchmark drops by about 14 ns per header child it runs.

Alternative if maintainers refuse an `init` hook: a one-entry cache in
`headerlookup` keyed by `*fiber.App` (`atomic.Pointer` to a struct holding the
app pointer and its `Hot`), refreshed by copying `Config()` on a miss. It is
self-contained, costs about the same on a hit, and thrashes harmlessly when
two apps share a process (every parallel test does). It is the second choice
because it is a cache with a stale-on-mutation caveat rather than a direct
read.

### Step 4: session stores the winning extractor by pointer

`middleware/session/store.go` `getSessionID` records which chain child
supplied the ID with `ctxlocal.Set(c, sessionExtractorContextKey,
chainExtractor)`. Boxing the 72-byte `Extractor` value costs 76 ns and an
80-byte allocation on every request of a chain-configured store; a pointer
into the config's own `Chain` slice costs 10 ns and nothing.

- In `getSessionID`, loop by index and store `&extractor.Chain[i]`; for the
  single-extractor case store `&s.Extractor`. Both point into memory owned by
  the `Store` for its lifetime and never written after construction.
- In `getSession`, read `c.Locals(sessionExtractorContextKey).(*extractors.Extractor)`
  with comma-ok and dereference into `sess.extractor` (a 72-byte copy, no
  allocation); keep `extractors.Extractor{}` as the fallback.
- No test references the key or `Session.extractor` directly, so the change is
  internal. Add `Benchmark_Session_ChainExtractor` next to `Benchmark_Session`
  (a store configured with `extractors.Chain(extractors.FromHeader("X-Session"),
  extractors.FromCookie("session_id"))` and only the cookie present) and show
  the allocation drop in the PR.
- `Benchmark_Session/default` will not move: it uses the default single cookie
  extractor and a reused ctx that caches the session ID local.

### Step 5: no allocations on the `DisableHeaderNormalizing` header path

Lower priority: it only affects apps that disable normalizing, but the fix is
small and the path is the one HTTP/2 and HTTP/3 deployments with lower-case
field names take. Today `FromHeader_Hit_NoNormalize` costs 162 ns and 3
allocations because `fieldname.Lines` hands a closure to `VisitAll`, which
fasthttp implements on top of `All()`, so the closure and the `values` slice
escape (`go build -gcflags=-m ./internal/fieldname/` shows "moved to heap:
values", "func literal escapes to heap", "append escapes to heap").

Change `headerlookup.Combined` so the non-canonical branch does not call
`Lines` at all: one pass over `h.All()` with a plain `for k, v := range`
counting matches and remembering the first value; if the count is 1, return it
under the same `Immutable` rule as the canonical branch; if it is more than
one, take a second pass that appends every match into a buffer sized from the
first pass, joined with `", "`. A range-over-func loop directly in the function
was measured at 19 ns and 0 allocations for this shape, against the existing
comment in `fieldname` that warns `All()` allocates: that warning is about a
different shape (the iterator kept in a function whose other branch does not
use it), so measure, and if an allocation shows up, split the fold branch into
its own function the way `foldValue` in `headerlookup` already is.

Keep the `Cookie` special case (`h.Cookie("")` before the walk) and the
"present but empty" rule. Leave `fieldname.Lines` itself alone; it has other
callers (the cache middleware) and returning a slice needs the allocation. Do
not touch the canonical branch.

Expected: `FromHeader_Hit_NoNormalize` 162 → about 70 ns, 3 → 0 allocs. Add
the benchmark's `_NoNormalize` numbers and an `AllocsPerRun == 0` test in
`headerlookup_test.go`.

### Step 6: small things folded into the steps above

- `Chain.Extract` and `extractChainWithSource` iterate by index and pass
  `*Extractor` internally (Step 1). `ExtractWithSource` keeps its by-value
  signature; it is public.
- `chainGuardFor(pub)` is computed once at construction (Step 1).
- `headerlookup.Combined`'s `utils.EqualFold(name, "Cookie")` per call is a
  length check for any name that is not six bytes long; not worth hoisting.
- `Contains` allocates a stack and a map per call. It runs at configuration
  time (csrf config validation), never per request; leave it.

## 5. Acceptance bar

Measured on the prototype of Steps 1 to 3 on this machine, reused ctx unless
stated. The implementation should land within noise of these; if a number is
worse by more than 10%, the step has a defect.

| Benchmark | Baseline | Target | Allocs |
| --- | ---: | ---: | --- |
| FromHeader_Hit | 65.7 | 52 | 0 → 0 |
| FromHeader_Miss | 59.0 | 51 | 0 → 0 |
| FromAuthHeader_Bearer_Hit | 175.5 | 112 | 0 → 0 |
| FromAuthHeader_Basic_Hit | 99.2 | 78 | 0 → 0 |
| FromAuthHeader_Miss | 63.3 | 51 | 0 → 0 |
| FromParam_Hit_Escaped | 101.7 | 84 | 1 → 1 (the decoded string) |
| Chain1_Hit | 116.5 | 68 | 0 → 0 |
| Chain3_HitFirst | 102.3 | 66 | 0 → 0 |
| Chain3_HitLast | 150.1 | 94 | 0 → 0 |
| ChainNested_HitInner | 187.6 | 115 | 0 → 0 |
| ExtractWithSource_Leaf | 116.0 | 70 | 0 → 0 |
| ExtractWithSource_Chain3_HitFirst | 370.3 | 82 | 2 → 0 |
| ExtractWithSource_Chain3_HitLast | 467.3 | 110 | 2 → 0 |
| ExtractWithSource_Nested_HitInner | 708.2 | 128 | 4 → 0 |
| ExtractWithSource_BareChain_HitLast | 396.2 | 155 | 0 → 0 |
| Chain1_Hit_FreshRequest | 108.5 | 95 | 0 → 0 |
| Chain3_HitLast_FreshRequest | 144.9 | 132 | 0 → 0 |
| ExtractWithSource_Chain3_HitFirst_FreshRequest | 364.0 | 112 | 2 → 0 |
| Token68_JWT | 92.0 | 45 | 0 → 0 |
| FromHeader_Hit_NoNormalize (Step 5) | 161.6 | ~70 | 3 → 0 |

Unchanged by design (do not spend time on them): `FromCookie` (12.6 ns),
`FromQuery` (17.5), `FromParam_Hit` (16.2), `FromCustom` (2.5), `FromForm`
(62.6, the cost is `Ctx.FormValue`'s content-type handling, see section 7).

Middleware-level: `Benchmark_Middleware_CSRF_Check` (1951 ns, 8 allocs) and
`Benchmark_Session/*` (192 allocs) must not gain allocations; the new keyauth
benchmarks should show the `FromAuthHeader` and chain savings end to end.

## 6. Verification and delivery checklist

Before each push, in this order, all from the repo root:

```bash
go build ./... && go vet ./...
go test ./extractors/ ./internal/... ./middleware/keyauth/ ./middleware/csrf/ ./middleware/session/ -race -count 3 -shuffle=on
make audit
make generate      # must produce no diff: nothing here adds a Ctx method
make betteralign
make format
make lint
make test
npx --yes markdownlint-cli2 "extractors/*.md"
git status         # generate/betteralign/format must have left nothing behind
```

Benchmarks for the PR description: the `-count 10` runs of Step 0 before and
after, through benchstat, for the extractors package and the middleware
benchmarks named above. The PR template asks for this under "Benchmarks".

Documentation: no user-facing behavior changes, so `docs/` does not change
and `docs/whats_new.md` gets no entry. `extractors/README.md` stays accurate;
if you touch it, keep it lint clean. Godoc comments in the changed functions
must describe the new mechanism (the `ExtractWithSource` comment in
particular).

Commits and PR:

- One commit per step, messages in the repository's style. Recent perf work
  used the `⚡ perf:` prefix (`git log --oneline | grep perf`), with a body that
  says what was measured and what changed, like commit `3178308`.
- PR title in the same style, for example `⚡ perf: cut per-request cost of the
  extractors`; AGENTS.md lists `🧹 chore:` as the generic prefix if a maintainer
  prefers it.
- Fill the PR template: description of each step, the benchstat tables,
  "Performance improvement" as the type of change, and check the benchmark
  and test items. Summarize the whole branch, not the last commit.
- Do not mention this document in the PR; it is a working file on this branch
  and should be removed in the final commit of the branch (or left, if the
  maintainers want it, as `extractors/PERFORMANCE_PLAN.md`).

## 7. Considered and rejected

- **Removing the cycle guard, or detecting cycles statically.** Only a custom
  `Extract` can re-enter a chain, and there is no way to tell a hand-rolled
  `Extractor{Extract: fn}` from a built-in one (functions are not comparable
  and reflect tricks are fragile). A missed cycle is a stack overflow; a guard
  costs nanoseconds. Keep it.
- **A recursion-depth limit instead of identity guards.** Changes when
  `ErrChainCycle` fires (after N re-entries instead of the first) and the tests
  pin first-re-entry refusal.
- **Per-request state on `DefaultCtx` itself** (a scratch slot the root package
  owns, exposed through an internal hook, reset in `release`). It would remove
  the remaining `Locals` lookup and the pool round trip (about 30 ns on a fresh
  request), but it adds a field and a lifecycle to `DefaultCtx` for one
  consumer, and custom contexts would still need the `Locals` path. Worth
  raising with the maintainers as a follow-up only if the `_FreshRequest`
  numbers matter to them; not part of this plan.
- **Caching the state pointer in the `Chain` closure.** The closure is shared
  by every request; anything mutable there is a data race.
- **SWAR for token68.** Already measured by a previous contributor at 16%
  slower than the switch (see the comment in the source). The table beats
  both.
- **Bypassing `PeekAll` with a manual walk of `h.All()`** to skip fasthttp's
  per-call key normalization (15 ns). It is faster only for requests with very
  few headers and it re-implements fasthttp's special-cased fields (`Host`,
  `Content-Type`, `Cookie`, ...). The right fix is upstream in fasthttp (a
  lookup that accepts an already-normalized key); out of scope.
- **Changing `App.Config()` to return a pointer** or adding exported getters
  on `App`. Public API changes for an internal need; the hook in Step 3 gives
  the same speed with no surface.
- **Optimizing `FromForm`.** Its 63 ns is `Ctx.FormValue`: a content-type
  fold, a `MediaType` parse for the multipart check and fasthttp's form
  lookup. That is `req.go`, shared by everything that reads forms, and not an
  extractor concern; memoizing the content-type checks per request would be a
  separate proposal.
- **Removing the `defer`s** in `Chain.Extract` and `ExtractWithSource`. About
  1 ns each and they are what keeps the guard consistent across a recovered
  panic.

## Appendix A: `extractors/bench_test.go`

Commit this file in Step 0 as is (after `gofmt`; the indentation here is
spaces for the markdown linter). It needs no changes in later steps: it uses
only the public API plus `isValidToken68`, which keeps its name.

```go
package extractors

import (
    "testing"

    "github.com/gofiber/fiber/v3"
    "github.com/valyala/fasthttp"
)

// A JWT-sized token68 credential.
const benchToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9P"

var (
    benchValue  string
    benchSource Source
    benchErr    error
    benchBool   bool
)

func newBenchCtx(b *testing.B, cfg ...fiber.Config) fiber.Ctx {
    b.Helper()
    app := fiber.New(cfg...)
    c := app.AcquireCtx(&fasthttp.RequestCtx{})
    b.Cleanup(func() { app.ReleaseCtx(c) })
    return c
}

func benchExtract(b *testing.B, e Extractor, c fiber.Ctx) {
    b.Helper()
    b.ReportAllocs()
    for b.Loop() {
        benchValue, benchErr = e.Extract(c)
    }
}

func benchExtractWithSource(b *testing.B, e Extractor, c fiber.Ctx) {
    b.Helper()
    b.ReportAllocs()
    for b.Loop() {
        benchValue, benchSource, benchErr = ExtractWithSource(e, c)
    }
}

func benchChain3() Extractor {
    return Chain(FromHeader("X-API-Key"), FromCookie("api_key"), FromQuery("api_key"))
}

// --- leaves -----------------------------------------------------------------

func Benchmark_Extractor_FromHeader_Hit(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Miss(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-Other", "x")
    benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Hit_ManyHeaders(b *testing.B) {
    c := newBenchCtx(b)
    for _, h := range []string{
        "Accept", "Accept-Encoding", "Accept-Language", "Cache-Control", "Referer",
        "Sec-Fetch-Site", "Sec-Fetch-Mode", "Origin", "X-Request-Id", "X-Forwarded-For",
    } {
        c.Request().Header.Set(h, "value")
    }
    c.Request().Header.Set("X-API-Key", benchToken)
    benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Hit_NoNormalize(b *testing.B) {
    c := newBenchCtx(b, fiber.Config{DisableHeaderNormalizing: true})
    c.Request().Header.DisableNormalizing()
    c.Request().Header.Set("x-api-key", benchToken)
    benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Hit_Immutable(b *testing.B) {
    c := newBenchCtx(b, fiber.Config{Immutable: true})
    c.Request().Header.Set("X-API-Key", benchToken)
    benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromAuthHeader_Bearer_Hit(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer "+benchToken)
    benchExtract(b, FromAuthHeader("Bearer"), c)
}

func Benchmark_Extractor_FromAuthHeader_Basic_Hit(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set(fiber.HeaderAuthorization, "Basic dXNlcjpwYXNzd29yZA==")
    benchExtract(b, FromAuthHeader("Basic"), c)
}

func Benchmark_Extractor_FromAuthHeader_Miss(b *testing.B) {
    c := newBenchCtx(b)
    benchExtract(b, FromAuthHeader("Bearer"), c)
}

func Benchmark_Extractor_FromAuthHeader_WrongScheme(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set(fiber.HeaderAuthorization, "Basic dXNlcjpwYXNzd29yZA==")
    benchExtract(b, FromAuthHeader("Bearer"), c)
}

func Benchmark_Extractor_FromAuthHeader_NoScheme(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer "+benchToken)
    benchExtract(b, FromAuthHeader(""), c)
}

func Benchmark_Extractor_FromCookie_Hit(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.SetCookie("session_id", "abc123")
    c.Request().Header.SetCookie("other", "x")
    benchExtract(b, FromCookie("session_id"), c)
}

func Benchmark_Extractor_FromCookie_Miss(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.SetCookie("other", "x")
    benchExtract(b, FromCookie("session_id"), c)
}

func Benchmark_Extractor_FromQuery_Hit(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?format=json&token=abc123")
    benchExtract(b, FromQuery("token"), c)
}

func Benchmark_Extractor_FromQuery_Miss(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?format=json")
    benchExtract(b, FromQuery("token"), c)
}

func Benchmark_Extractor_FromForm_Hit(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.SetMethod(fiber.MethodPost)
    c.Request().Header.SetContentType(fiber.MIMEApplicationForm)
    c.Request().SetBodyString("username=john&token=abc123")
    benchExtract(b, FromForm("token"), c)
}

func Benchmark_Extractor_FromForm_Miss(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.SetMethod(fiber.MethodPost)
    c.Request().Header.SetContentType(fiber.MIMEApplicationForm)
    c.Request().SetBodyString("username=john")
    benchExtract(b, FromForm("token"), c)
}

// benchParam runs the loop inside a matched route, on the benchmark goroutine,
// because route parameters exist only there.
func benchParam(b *testing.B, uri string) {
    b.Helper()
    app := fiber.New()
    ext := FromParam("id")
    ran := false
    app.Get("/users/:id", func(c fiber.Ctx) error {
        ran = true
        b.ReportAllocs()
        for b.Loop() {
            benchValue, benchErr = ext.Extract(c)
        }
        return nil
    })
    fctx := &fasthttp.RequestCtx{}
    fctx.Request.Header.SetMethod(fiber.MethodGet)
    fctx.Request.SetRequestURI(uri)
    app.Handler()(fctx)
    if !ran {
        b.Fatal("route did not match")
    }
}

func Benchmark_Extractor_FromParam_Hit(b *testing.B) {
    benchParam(b, "/users/abc123")
}

func Benchmark_Extractor_FromParam_Hit_Escaped(b *testing.B) {
    benchParam(b, "/users/abc%20123")
}

func Benchmark_Extractor_FromCustom_Hit(b *testing.B) {
    c := newBenchCtx(b)
    benchExtract(b, FromCustom("k", func(fiber.Ctx) (string, error) { return "v", nil }), c)
}

// --- chains -----------------------------------------------------------------

func Benchmark_Extractor_Chain1_Hit(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    benchExtract(b, Chain(FromHeader("X-API-Key")), c)
}

func Benchmark_Extractor_Chain3_HitFirst(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    benchExtract(b, benchChain3(), c)
}

func Benchmark_Extractor_Chain3_HitLast(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?api_key=abc123")
    benchExtract(b, benchChain3(), c)
}

func Benchmark_Extractor_Chain3_Miss(b *testing.B) {
    c := newBenchCtx(b)
    benchExtract(b, benchChain3(), c)
}

func Benchmark_Extractor_ChainNested_HitInner(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?api_key=abc123")
    outer := Chain(FromHeader("X-API-Key"), Chain(FromCookie("api_key"), FromQuery("api_key")))
    benchExtract(b, outer, c)
}

// Locals a real request accumulates before the extractor runs; every
// request-local lookup scans past them.
func Benchmark_Extractor_Chain3_HitFirst_WithLocals(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    for i := range 5 {
        c.Locals(i, "value")
    }
    benchExtract(b, benchChain3(), c)
}

// --- source-aware -------------------------------------------------------------

func Benchmark_Extractor_ExtractWithSource_Leaf(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    benchExtractWithSource(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_ExtractWithSource_Chain3_HitFirst(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    benchExtractWithSource(b, benchChain3(), c)
}

func Benchmark_Extractor_ExtractWithSource_Chain3_HitLast(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?api_key=abc123")
    benchExtractWithSource(b, benchChain3(), c)
}

func Benchmark_Extractor_ExtractWithSource_Chain3_Miss(b *testing.B) {
    c := newBenchCtx(b)
    benchExtractWithSource(b, benchChain3(), c)
}

func Benchmark_Extractor_ExtractWithSource_Nested_HitInner(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?api_key=abc123")
    outer := Chain(FromHeader("X-API-Key"), Chain(FromCookie("api_key"), FromQuery("api_key")))
    benchExtractWithSource(b, outer, c)
}

func Benchmark_Extractor_ExtractWithSource_BareChain_HitLast(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?api_key=abc123")
    bare := Extractor{
        Chain:  []Extractor{FromHeader("X-API-Key"), FromCookie("api_key"), FromQuery("api_key")},
        Source: SourceHeader,
    }
    benchExtractWithSource(b, bare, c)
}

// --- fresh request -----------------------------------------------------------
//
// The user values are reset after every extraction, as fasthttp does between
// requests (Request.Reset calls Close on any stored io.Closer), so these
// measure the per-request cost of whatever a chain keeps in Locals.

func Benchmark_Extractor_FromHeader_Hit_FreshRequest(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    e := FromHeader("X-API-Key")
    b.ReportAllocs()
    for b.Loop() {
        benchValue, benchErr = e.Extract(c)
        c.RequestCtx().ResetUserValues()
    }
}

func Benchmark_Extractor_Chain1_Hit_FreshRequest(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    e := Chain(FromHeader("X-API-Key"))
    b.ReportAllocs()
    for b.Loop() {
        benchValue, benchErr = e.Extract(c)
        c.RequestCtx().ResetUserValues()
    }
}

func Benchmark_Extractor_Chain3_HitLast_FreshRequest(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().SetRequestURI("/api?api_key=abc123")
    e := benchChain3()
    b.ReportAllocs()
    for b.Loop() {
        benchValue, benchErr = e.Extract(c)
        c.RequestCtx().ResetUserValues()
    }
}

func Benchmark_Extractor_ExtractWithSource_Chain3_HitFirst_FreshRequest(b *testing.B) {
    c := newBenchCtx(b)
    c.Request().Header.Set("X-API-Key", benchToken)
    e := benchChain3()
    b.ReportAllocs()
    for b.Loop() {
        benchValue, benchSource, benchErr = ExtractWithSource(e, c)
        c.RequestCtx().ResetUserValues()
    }
}

// --- token68 ------------------------------------------------------------------

func Benchmark_Extractor_Token68_JWT(b *testing.B) {
    b.ReportAllocs()
    for b.Loop() {
        benchBool = isValidToken68(benchToken)
    }
}
```

The existing `Benchmark_isValidToken68` in `extractors_test.go` (three mixed
inputs) stays; it is the number quoted in the source comment.

## Appendix B: throwaway diagnostics used for section 2

Not to be committed. They isolate costs that the committed benchmarks only
show in aggregate, and they reference internals that Step 1 deletes.

- `Config` copy: loop `cfg := c.App().Config(); sink = cfg.Immutable ||
  cfg.DisableHeaderNormalizing` (25 ns) against `sink = c.App()` (2 ns).
- `Locals` get: `c.Locals(k{})` with one entry stored (7 ns), with five other
  entries (8 ns), through `*fiber.DefaultCtx` directly (7.3 ns: the interface
  call is not the cost, the scan is).
- Guard bookkeeping: `chainGuardFor(pub)`, `c.Locals(guard).(bool)`,
  `ctxlocal.Set(c, guard, true)`, `chainWinCaptureActive(c)`,
  `ctxlocal.Set(c, guard, false)` (31 ns).
- Capture bookkeeping: `enterChainWinCapture`, `chainWinStackLen`,
  `truncateChainWinStack`, `pushChainWinningSource`, `popChainWinningSource`,
  `leaveChainWinCapture` (196 ns, 32 B, 2 allocs).
- Pooled state: `Locals` get miss, `sync.Pool` Get, `ctxlocal.Set`, then
  `Close` (45 ns including a simulated reset), and the reuse path (9.5 ns).
- fasthttp floor: `len(h.PeekAll("X-API-Key"))` 37 ns, `len(h.Peek(...))`
  34 ns, a `for k, v := range h.All()` walk with `utils.EqualFold` 19 ns and
  0 allocs.
- Boxing an `Extractor` into `Locals` by value (76 ns, 80 B) against by
  pointer (10.5 ns, 0 B).
- Escape analysis for Step 5: `go build -gcflags=-m ./internal/fieldname/ 2>&1 | grep -v inline`.
