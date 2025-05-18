# AGENTS.md

## 1️⃣ Module Initialization & Dependencies

- echo "🔧 Initialize Go module (if not already done)"
- go mod init github.com/gofiber/fiber           # for v3: requires Go ≥ 1.23
- echo "⬇️ Download all project dependencies"
- go get ./...                                  # recursively fetches all modules
- echo "🔌 Download Makefile tools"
- go install gotest.tools/gotestsum@latest      # for `make test`, `make coverage`
- go install golang.org/x/vuln/cmd/govulncheck@latest   # for `make audit`
- go install mvdan.cc/gofumpt@latest            # for `make format`
- go install github.com/tinylib/msgp@latest     # for `make generate`
- go install github.com/vburenin/ifacemaker@975a95966976eeb2d4365a7fb236e274c54da64c  # for `make generate`
- go install github.com/dkorunic/betteralign/cmd/betteralign@latest  # for `make betteralign`
- echo "🧹 Tidy up modules"
- go mod tidy                                   # removes unused dependencies
