# AGENTS.md

## Agent Instructions

> **Note for the AI agent:** Before running the startup script, verify that all dependencies and CLI tools are already available. If a `vendor/` directory exists **or** `go list -m -mod=readonly all` succeeds (indicating modules are cached), you may skip the entire startup script—including all `go mod` commands and `go install` steps. Otherwise, run the script as listed below.

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
  go install golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest
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
- **generate**: run `go generate` after installing msgp and ifacemaker.
- **modernize**: run golps modernize

These targets can be invoked via `make <target>` as needed during development and testing.

## Programmatic checks

Before presenting final changes or submitting a pull request, run each of the
following commands and ensure they succeed. Include the command outputs in your
final response to confirm they were executed:

```bash
make audit
make generate
make betteralign
make modernize
make format
make test
```

All checks must pass before the generated code can be merged.
