# Fiber v2/v3 version comparison

This is an in-process comparison of independently resolved Fiber releases, not
an HTTP load test or an application-capacity estimate. It measures the same
small request scenarios with each version's native APIs. Production Fiber code
and the root module's dependency graph are unchanged.

The [2026-09-08 report](REPORT.md) includes a complete measured matrix and raw data.

## Reproduce

Requirements: Python 3.9 or newer, Go with toolchain downloads enabled, a C
compiler for the race detector, and access to the pinned Go modules. Linux
`taskset` is needed only when using `--cpu`.

From the repository root:

```sh
python3 -m unittest discover -s benchmarks/versioncompare -p test_run.py -v
python3 benchmarks/versioncompare/run.py prepare /tmp/fiber-version-run
python3 benchmarks/versioncompare/run.py sample /tmp/fiber-version-run \
  --rounds 12 --benchtime 500ms --cpu 7 --seed 2714
python3 benchmarks/versioncompare/run.py summarize /tmp/fiber-version-run
```

Choose an available logical CPU, or omit `--cpu`. Affinity restricts where the
benchmark executes; it does **not** reserve that CPU from other processes or
virtual machines. Prefer an otherwise idle, dedicated host for release claims.
The output directory must be new. Preparation or sampling failures are recorded
in `manifest.json` and must be investigated; a failed or partial run is not
accepted by `summarize`. Start a new output directory for a new experiment.

`versions.json` pins Go, both releases, a separate development snapshot, and the
`benchstat` tool. The snapshot is a module pseudo-version, not a moving `main`
reference or an absolute-path replacement. Updating any pin defines a new
experiment and requires fresh preparation and measurement.

Run the pinned upstream statistics tool after sampling:

```sh
BENCHSTAT=$(python3 -c \
  'import json; print(json.load(open("benchmarks/versioncompare/versions.json"))["benchstat"])')
GOWORK=off GOTOOLCHAIN=go1.26.8 go run "$BENCHSTAT" \
  /tmp/fiber-version-run/benchstat/matched/v2.txt \
  /tmp/fiber-version-run/benchstat/matched/v3.txt \
  /tmp/fiber-version-run/benchstat/matched/snapshot.txt
GOWORK=off GOTOOLCHAIN=go1.26.8 go run "$BENCHSTAT" \
  /tmp/fiber-version-run/benchstat/controls/v2.txt \
  /tmp/fiber-version-run/benchstat/controls/v3.txt \
  /tmp/fiber-version-run/benchstat/controls/snapshot.txt
```

Do not turn the tool's geometric mean into a headline speedup: these selected
microbenchmarks are not a representative weighted application workload.

## What is compared

Each variant has its **own module and binary**. Importing v2 and v3 into one Go
module would let minimal version selection raise shared dependencies, changing
what is being compared. Here each release retains the dependencies selected by
its own module graph. Resolved versions and sums are recorded in the manifest
and each variant's `modules.json`, `go.mod`, and `go.sum`.

Consequently this is an **as-shipped stack** comparison. Differences in
fasthttp, binders, utility packages, or defaults are part of the observation;
results do not isolate the effect of Fiber's code alone. The Go compiler and
runtime are held constant across all variants, rather than using each release's
historical Go version.

The common template covers 22 cases:

| Area | Cases |
| --- | --- |
| Routing | Static, parameter, wildcard, and the final static route in tables of 100 and 1,000 routes |
| Request access | Query lookup, header lookup, and a local passed from middleware |
| Binding | JSON with a small name, a 1,024-byte name, and a 16,384-byte name; query binding |
| Responses | JSON at the same three name sizes |
| Middleware | Three forwarding handlers, CORS simple/preflight, and recovery without a panic |
| Controls | Raw fasthttp floor, each version's default 404, and a matched custom 404 |

The size suffix is the **JSON string field's byte length**, not the total request
or response length. Route-table cases register routes before timing and always
request the final registered path; they are not a randomized route mix.

Native API substitutions are explicit: v2 `BodyParser`/`QueryParser` versus v3
`Bind().Body`/`Bind().Query`, pointer versus interface contexts, and string versus
slice CORS configuration. CORS explicitly allows the same origin and `GET,POST`
methods in every variant. v3 renders the method list with an optional space after
the comma; the test checks each version's exact expected header.

## Correctness before timing

Every scenario first runs under the race detector, including repeated reuse of
one request context. Tests and subtests run in parallel. The timing binary is
then built **without** the race detector and checked for the complete case set.

For each case, both the functional test and the untimed benchmark checks:

- verify the status and the full body;
- serialize the response and read it through an HTTP parser;
- verify required CORS headers from that parsed response;
- reject trailing bytes after the response body;
- validate every bound payload field inside the handler.

This matters for CORS preflight: v2 retains `No Content` in its internal response
buffer, but its HTTP serializer correctly omits a body for status 204. Comparing
only that buffer would incorrectly reject equivalent HTTP behavior.

The default 404 is intentionally **not** output-matched: v2 returns
`Cannot GET /missing`, while v3 returns `Not Found`. That case and the raw
fasthttp floor are exported to separate control files. A second 404 case installs
the same custom error response for the output-matched group. Matching here means
the checked status, body, and header semantics, not byte-for-byte identity of all
HTTP headers or identical internal work.

## Timing and interpretation

Each timed operation includes:

1. resetting pooled request, response, and user values;
2. preparing the request method, URI, headers, and body;
3. executing the in-process handler.

App construction, route registration, correctness checks, HTTP serialization,
network I/O, kernel processing, TLS, and client overhead are outside the timer.
The reported `ns/op` is therefore **not end-to-end request latency**. The raw
fasthttp case exposes part of the harness/dependency floor; it is not subtracted
from other cases.

`GOMAXPROCS=1` is fixed. Persisted Go settings, workspaces, custom build flags,
experiments, GC limits, and runtime debug overrides are excluded from the run.
Relevant Go settings, host information, source/config/binary hashes, CPU affinity,
run order, load averages, durations, and raw-output hashes are retained.

The runner interleaves fresh benchmark subprocesses in balanced order. With
three variants, a six-round block contains every ordering once. Twelve rounds
repeat this balance twice; the seed controls the order within each block. Go's
benchmark harness calibrates each case to the selected duration, so iteration
counts may differ. Pool warm-up occurs before timing. Allocation counts are
Go benchmark per-operation measurements, not peak process memory.

`summarize` validates raw hashes and the complete set of runs/cases, reparses the
raw Go output, and checks that the stored observations match it before writing
reports. `summary.json` includes medians, median absolute deviation (MAD), and
ranges; the raw files remain the source for `benchstat` confidence intervals and
comparisons. Interleaving reduces simple ordering bias, but does not remove noisy
neighbors, CPU-frequency effects, thermal drift, or other host contention.

A result is evidence for this exact matrix and environment only. Preserve noisy
or unfavorable results, investigate failures rather than dropping samples, and
repeat on suitable hardware before making general release-performance claims.
