# AGENTS.md

## Agent Instructions

> **Note for the AI agent:** Before running the startup script, verify that all dependencies and CLI tools are already available. If a `vendor/` directory exists **or** `go list -m -mod=readonly all` succeeds (indicating modules are cached), you may skip the entire startup script—including all `go mod` commands and `go install` steps. Otherwise run the script as listed below.

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
  go install github.com/vburenin/ifacemaker@975a95966976eeb2d4365a7fb236e274c54da64c  # interface impls
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
- **generate**: run `go generate` after installing msgp and ifacemaker.

These targets can be invoked via `make <target>` as needed during development and testing.
