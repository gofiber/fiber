# Fiber v2/v3 feature comparison — 2026-09-08

## Result and release-note recommendation

This run does **not** support a general “v3 is faster” headline. It shows a mixed
set of changes in an as-shipped stack comparison with small, specified workloads.
The most useful release-note addition is a link to the reproducible matrix and
results, with narrowly scoped examples rather than an aggregate speedup.

- Query binding took a median **3.366 µs in v2 versus 1.600 µs in v3.5.0** in this
  harness. Its reported allocations were **496 B / 22 allocations versus
  64 B / 3 allocations** in every sample. The separate development snapshot
  reported the same allocation counts.
- The custom, output-matched 404 case also took less time in v3, with
  **56 B / 3 allocations versus 8 B / 1 allocation** in every sample. The default
  404 is a different workload because its body changed, and is kept separate.
- There are costs as well: the CORS simple case and the middleware-locals case
  went from **0 B / 0 allocations to 16 B / 1 allocation**, while CORS preflight
  went from **144 B / 2 allocations to 208 B / 4 allocations**. Header lookup and
  several other cases took longer in this run. These are observations, not a
  diagnosis of the code responsible.
- The static/parameter/wildcard cases and the larger JSON workloads did not
  show a statistically distinguishable release-to-release timing difference
  under benchstat's default test in this sample. This is **not** proof of equal
  performance. Small timing differences and isolated p-values need replication,
  especially when considering many cases at once.

A possible release-note entry, without claiming application throughput:

> A reproducible v2/v3 microbenchmark comparison is available with pinned
> dependencies and raw samples. In the tested query-binding workload, v3 used
> 3 allocations per operation versus 22 in v2; other workloads showed both
> improvements and additional costs. See the full matrix and its limitations
> before applying these results to an application.

## Versions and environment

| Label | Fiber version | Source commit | fasthttp |
| --- | --- | --- | --- |
| v2 | `v2.52.15` | `5c7a80b327c130e93041e569e685825f804d5f15` | `v1.51.0` |
| v3 | `v3.5.0` | `741d8511a75f408ddf93eb41b175df0165714f11` | `v1.73.0` |
| snapshot | `v3.5.1-0.20260907141705-f73d37fb732f` | `f73d37fb732fd431339b8a250d84314f02d215a5` | `v1.74.0` |

- Toolchain: **go1.26.8**, linux/amd64, GOAMD64=v1, CGO enabled.
- Host: Linux virtual machine exposing 8 logical CPUs from an AMD EPYC 9645;
  benchmark children pinned to logical CPU 7, `GOMAXPROCS=1`.
- This is a **shared VM**, not an exclusive/reserved CPU or production server.
- Measurement: 2026-09-08T07:59:45.382685+00:00 to 2026-09-08T08:10:58.803885+00:00 (UTC).
- Twelve balanced rounds, all six variant orders represented twice; 36 fresh
  subprocesses; 500 ms target duration per case; **792 observations retained**.
- Each version was resolved in a separate module. v3.5.0 uses utils/v2 v2.4.1;
  the development snapshot uses v2.5.1. Dependencies are not equalized, so this
  does not attribute a difference to Fiber alone.

The raw fasthttp floor itself changed from 72.525 ns/op in the v2 stack to
102.665 ns/op in the v3 release stack. That is one reason to interpret these as
stack-and-harness observations, not isolated Fiber overhead. The floor is not
subtracted from the measurements.

## Output-matched scenarios

Each cell is **median ns/op (MAD%)**, from 12 samples. MAD is median absolute
deviation divided by the median, **not a confidence interval**. The exact
benchstat 95% intervals and pairwise comparisons are in the
[complete timing/allocation table](results/2026-09-08/benchstat-matched.txt).
All comparisons there use v2 as the baseline, including the snapshot column;
they are not a snapshot-versus-v3 optimization comparison.

| Scenario | v2.52.15 | v3.5.0 | Development snapshot |
| --- | ---: | ---: | ---: |
| `cors_preflight` | 1,990.50 (2.5%) | 2,197.00 (5.3%) | 2,558.00 (6.2%) |
| `cors_simple` | 760.15 (5.4%) | 1,009.50 (7.8%) | 1,041.00 (5.8%) |
| `header_lookup` | 398.70 (4.9%) | 490.60 (6.3%) | 486.60 (3.8%) |
| `json_bind` | 2,516.00 (4.7%) | 2,742.50 (2.3%) | 2,826.00 (3.6%) |
| `json_bind_1024` | 12,880.00 (5.0%) | 12,474.50 (4.3%) | 11,944.00 (6.2%) |
| `json_bind_16384` | 149,425.50 (4.7%) | 140,945.00 (5.2%) | 143,790.00 (4.2%) |
| `json_response` | 627.40 (3.6%) | 653.10 (2.8%) | 683.05 (3.0%) |
| `json_response_1024` | 2,895.50 (6.3%) | 3,002.00 (4.2%) | 2,898.00 (4.3%) |
| `json_response_16384` | 29,437.00 (8.7%) | 29,151.50 (4.1%) | 28,610.50 (4.0%) |
| `locals` | 372.70 (7.6%) | 432.05 (7.2%) | 424.25 (4.4%) |
| `middleware_chain` | 331.50 (9.4%) | 345.25 (6.0%) | 331.35 (5.9%) |
| `not_found_matched_response` | 1,018.50 (4.4%) | 503.00 (3.5%) | 484.65 (7.7%) |
| `parameter` | 350.20 (6.7%) | 333.00 (5.1%) | 350.95 (7.2%) |
| `query_bind` | 3,365.50 (4.5%) | 1,599.50 (2.6%) | 1,576.50 (5.8%) |
| `query_lookup` | 392.30 (7.6%) | 401.90 (6.2%) | 410.30 (7.0%) |
| `recover_no_panic` | 345.75 (6.1%) | 320.95 (4.7%) | 335.50 (10.5%) |
| `static` | 304.80 (3.6%) | 320.40 (5.3%) | 299.60 (4.3%) |
| `static_routes_100` | 1,236.00 (4.2%) | 1,373.00 (9.3%) | 1,380.50 (4.3%) |
| `static_routes_1000` | 9,881.00 (4.0%) | 11,251.50 (5.2%) | 11,193.00 (5.9%) |
| `wildcard` | 399.30 (8.0%) | 404.30 (4.3%) | 389.50 (6.0%) |

### Allocation measurements

Cells show **median B/op / median allocs/op** from Go's benchmark output, not
peak resident memory. The 1,024-byte JSON-name binding case varied between 6,
7, and 8 reported allocations across runs; its 7.5 median is a summary of sample
estimates, not a claim that an individual request makes half an allocation.
Those allocation samples and all timing samples are retained.

| Scenario | v2.52.15 | v3.5.0 | Development snapshot |
| --- | ---: | ---: | ---: |
| `cors_preflight` | 144 / 2 | 208 / 4 | 208 / 4 |
| `cors_simple` | 0 / 0 | 16 / 1 | 16 / 1 |
| `header_lookup` | 0 / 0 | 0 / 0 | 0 / 0 |
| `json_bind` | 256 / 6 | 256 / 6 | 256 / 6 |
| `json_bind_1024` | 2099.5 / 7.5 | 2110 / 7.5 | 2099 / 7.5 |
| `json_bind_16384` | 37141 / 9 | 37145 / 9 | 37145 / 9 |
| `json_response` | 48 / 1 | 48 / 1 | 48 / 1 |
| `json_response_1024` | 1184 / 2 | 1184 / 2 | 1184 / 2 |
| `json_response_16384` | 18465 / 2 | 18465 / 2 | 18465 / 2 |
| `locals` | 0 / 0 | 16 / 1 | 16 / 1 |
| `middleware_chain` | 0 / 0 | 0 / 0 | 0 / 0 |
| `not_found_matched_response` | 56 / 3 | 8 / 1 | 8 / 1 |
| `parameter` | 0 / 0 | 0 / 0 | 0 / 0 |
| `query_bind` | 496 / 22 | 64 / 3 | 64 / 3 |
| `query_lookup` | 0 / 0 | 0 / 0 | 0 / 0 |
| `recover_no_panic` | 0 / 0 | 0 / 0 | 0 / 0 |
| `static` | 0 / 0 | 0 / 0 | 0 / 0 |
| `static_routes_100` | 0 / 0 | 0 / 0 | 0 / 0 |
| `static_routes_1000` | 0 / 0 | 0 / 0 | 0 / 0 |
| `wildcard` | 2 / 1 | 2 / 1 | 2 / 1 |

## Separate controls

These cases are **not** included in the output-matched table. See the
[control statistics](results/2026-09-08/benchstat-controls.txt).

| Scenario | v2.52.15 median ns/op | v3.5.0 median ns/op | Snapshot median ns/op |
| --- | ---: | ---: | ---: |
| `fasthttp_floor` | 72.525 | 102.665 | 100.200 |
| `not_found_version_default` | 1,199.000 | 496.650 | 446.750 |

The default 404 body is `Cannot GET /missing` in v2 and `Not Found` in v3.
The matched case installs the same `not found` custom error response instead.
The fasthttp floor contains no Fiber handler. A geometric mean of these two
unlike controls is not a useful release metric, even though benchstat prints it.
Likewise, the unweighted geometric mean of the selected matched scenarios does
not represent an application workload and is not used as a headline result.

## Limits and verification

- The timer covers request/response/user-value reset, request preparation, and
  in-process handler execution. It excludes app construction, route registration,
  response serialization, sockets, TLS, and client/network time. `ns/op` is
  **not end-to-end request latency or production requests per second**.
- Matching means checked status, body, and required header semantics, not
  identical work or identical bytes in every header. CORS method lists are
  explicit; the legal comma-whitespace difference is tested exactly. HTTP 204
  bodies are checked after serialization, including absence of trailing bytes.
- All 22 cases passed race-enabled functional checks for each variant before
  non-race benchmark binaries were built. The runner's 17 unit tests pass.
  CI validates correctness and builds the fixtures; it does not collect timing
  numbers on shared runners.
- Across the 66 case/variant groups, timing MAD ranged from 1.7% to 10.5% and
  min-to-max ranges remained noticeable. Interleaving is not isolation. The
  p-values are exploratory, not corrected for testing many workloads, and the
  results need replication on suitable dedicated hardware before broader claims.
- No samples were removed or replaced. SHA-256 checks validate the measured
  binaries and raw records, and every stored observation is cross-checked against
  the complete raw run/case set before reports are produced.

[Reproduction commands and scenario definitions](README.md) describe the exact
native API substitutions and workloads. The source-only
[result bundle](results/2026-09-08) includes the manifest, all 36 raw files,
module graphs, functional logs, observations, and benchstat output. Executables
are deliberately omitted; prepare a fresh directory to rebuild them. To
revalidate the bundle and regenerate statistics inputs:

```sh
python3 benchmarks/versioncompare/run.py summarize \
  benchmarks/versioncompare/results/2026-09-08
```
