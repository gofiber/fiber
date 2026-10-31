# AGENTS.md

## Agent Instructions

> **Note for the AI agent:** Before running the startup script, verify that all dependencies and CLI tools are already available. If a `vendor/` directory exists **or** `go list -m -mod=readonly all` succeeds (indicating modules are cached), you may skip the entire startup script—including all `go mod` commands and `go install` steps. Otherwise, run the script as listed below.

### General coding practices

- When adding Go tests, always invoke `t.Parallel()` at the start of each test and subtest to maximize concurrency. The exceptions are a test that calls `testing.AllocsPerRun`, which panics with `AllocsPerRun called during parallel test`, and one that installs package-level state every other test reads, such as the proxy policy `withSecurityPolicyForTest` installs: leave those sequential and say why in a comment, as `Test_Chain_NoAllocations`, `Test_Combined_DoesNotAllocate` and `Test_Security_Do_BlocksDNSRebinding` do.
- Do not hand-write the plumbing that nearly every test repeats; `internal/apptest` has helpers for it. `apptest.Do`, `apptest.Expect` and `apptest.ExpectBody` send a request through an app and check the transport error, the status and the body (a string target is a GET, anything else is built with `apptest.Req`, which takes options such as `apptest.WithHeader`, `apptest.WithHeaderIf` for a header that may be empty, and `apptest.WithHost`); `apptest.Body` reads a response body; `apptest.Ctx`, `apptest.CtxAs` and `apptest.CtxFor` borrow a request context and release it when the test ends; `apptest.Recv` waits on a channel with a deadline instead of a hand-written `select` with `time.After`; `apptest.Pairs` reads the name and value pairs of a variadic `...string`. Fold tests that differ only in their data into one table-driven test with a subtest per row instead of copying the test, and give a helper that three or more tests share a name and a doc comment.
- Take fakes from the shared test packages instead of writing another one: `internal/storagetest` has `Memory`, a storage whose errors are configured with `SetErr` and which logs the calls it served (`Calls`, `CallKeys`; `Marked` puts a label on a context so the log shows which one reached the storage), and `Failing`; `internal/clocktest` has `Clock`, a clock the test moves, and `SleepPast` for the cached second clock; `internal/loggertest` and `internal/tlstest` cover the logger output and TLS configs. A helper that two or more packages need belongs in one of these packages (or in `internal/apptest` when it is about requests, contexts or waiting); a helper only one package uses stays in that package's tests.
- Keep test files compact: no blank line right after `t.Parallel()`; a function literal whose body is a single statement goes on one line (`app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })`); table rows and short literals or calls go on one line when it stays within 120 columns (tabs count 4); assert `require.NoError(t, f())` instead of going through an `err` variable that is not used again; and do not add `// go test -run ...` hint comments. Benchmarks keep their names (CI charts them by name) and their timed loops; share only their setup.
- Prefer `github.com/gofiber/utils/v2` helpers (for example, `utils.Trim`) when performing common operations such as string manipulation, whenever it is practical and appropriate for the surrounding code.
- Keep all protocol behavior RFC-compliant (e.g., HTTP/1.1 requirements) and document any intentional deviations.
- Protect hot paths from regressions: profile changes.
- Apply secure-by-default choices (validation, timeouts, sanitization) and ensure new code hardens attack surfaces.

### Linter pitfalls (run `make lint` before every push)

The linter is the most common reason CI fails for generated changes. Always run
`make lint` locally and fix every finding *before* committing — do not rely on
CI to surface them. The `golangci-lint` config (`.golangci.yml`) enables strict
`errcheck` settings (`check-type-assertions: true` and `check-blank: true`),
which trip up the patterns below:

- **Never discard a type-assertion result with `_`.** `v, _ := x.(T)` fails
  `errcheck`. Use the comma-ok form and act on it instead:

  ```go
  v, ok := x.(T)
  if !ok {
      v = T{} // explicit fallback; the zero value is fine when that's intended
  }
  ```

- **Never discard a returned `error` with `_`** (`check-blank`). Handle it, or
  if it is genuinely safe to ignore, call the function without assignment or add
  a justified `//nolint:errcheck // reason` comment.

- When in doubt, run `make lint` (or `golangci-lint run ./<pkg>/...`) and treat a
  non-zero exit as a blocker.

---

## Startup script (reference only – do not run)

- Fetch dependencies:

  ```bash
  go mod tidy && go mod download && go mod vendor
  ```

- Install CLI tools referenced in Makefile:

  ```bash
  go install gotest.tools/gotestsum@latest                 # test runner
  go install golang.org/x/vuln/cmd/govulncheck@latest      # vulnerability scanner
  go install mvdan.cc/gofumpt@latest                       # code formatter
  go install github.com/tinylib/msgp@latest                # msgp codegen
  go install github.com/vburenin/ifacemaker@f30b6f9bdbed4b5c4804ec9ba4a04a999525c202  # interface impls
  go install github.com/dkorunic/betteralign/cmd/betteralign@latest  # struct alignment
  go mod tidy                                              # clean up go.mod & go.sum
  ```

## Makefile commands

Use `make help` to list all available commands. Common targets include:

- **audit**: run `go mod verify`, `go vet`, and `govulncheck` for quality checks.
- **benchmark**: run benchmarks with `go test`.
- **coverage**: generate a coverage report.
- **format**: apply formatting using `gofumpt`.
- **lint**: execute `golangci-lint`.
- **test**: run the test suite with `gotestsum`.
- **longtest**: run the test suite 15 times with shuffling enabled.
- **tidy**: clean and tidy dependencies.
- **betteralign**: optimize struct field alignment.
- **generate**: run `go generate` after installing msgp and ifacemaker. On a pull request a maintainer
  can get the same result without a local toolchain by commenting `/generate`, which runs the target
  and pushes the regenerated files onto the branch.

These targets can be invoked via `make <target>` as needed during development and testing.

## Pull request guidelines

- PR titles must start with a category prefix describing the change: `🐛 bug:`, `🔥 feat:`, `📒 docs:`, or `🧹 chore:`.
- Generated PR titles and bodies must summarize the *entire* set of changes on the branch (for example, based on `git log --oneline <base>..HEAD` or the full diff), **not** just the latest commit. The Summary section should reflect all modifications that will be merged.

## Programmatic checks

Before presenting final changes or submitting a pull request, run each of the
following commands and ensure they succeed. Include the command outputs in your
final response to confirm they were executed:

```bash
make audit
make generate
make betteralign
make format
make lint
make test
```

All checks must pass before the generated code can be merged.

After completing the programmatic checks above, confirm that any relevant
documentation has been updated to reflect the changes made, including PR
instructions when applicable.
