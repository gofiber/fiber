GOVERSION ?= $(shell go env GOVERSION)

## help: 💡 Display available commands
.PHONY: help
help:
	@echo '⚡️ GoFiber/Fiber Development:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## audit: 🚀 Conduct quality checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	GOTOOLCHAIN=$(GOVERSION) go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## benchmark: 📈 Benchmark code performance
.PHONY: benchmark
benchmark:
	go test ./... -benchmem -bench=. -run=^Benchmark_$

## coverage: ☂️  Generate coverage report
.PHONY: coverage
coverage:
	GOTOOLCHAIN=$(GOVERSION) go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -coverprofile=/tmp/coverage.out -covermode=atomic
	go tool cover -html=/tmp/coverage.out

## format: 🎨 Fix code format issues
.PHONY: format
format:
	GOTOOLCHAIN=$(GOVERSION) go run mvdan.cc/gofumpt@latest -w -l .

## markdown: 🎨 Find markdown format issues (Requires markdownlint-cli2)
.PHONY: markdown
markdown:
	@which markdownlint-cli2 > /dev/null || npm install -g markdownlint-cli2
	markdownlint-cli2 "**/*.md" "#vendor"

## lint: 🚨 Run lint checks
.PHONY: lint
lint:
	GOTOOLCHAIN=$(GOVERSION) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

## test: 🚦 Execute all tests
.PHONY: test
test:
	GOTOOLCHAIN=$(GOVERSION) go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -shuffle=on

## longtest: 🚦 Execute all tests 10x
.PHONY: longtest
longtest:
	GOTOOLCHAIN=$(GOVERSION) go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=15 -shuffle=on

## tidy: 📌 Clean and tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy -v

## betteralign: 📐 Optimize alignment of fields in structs
.PHONY: betteralign
betteralign:
	GOTOOLCHAIN=$(GOVERSION) go run github.com/dkorunic/betteralign/cmd/betteralign@v0.8.0 -test_files -generated_files -apply ./...

## generate: ⚡️ Generate msgp && interface implementations
.PHONY: generate
generate:
	go install github.com/tinylib/msgp@latest
	go install github.com/vburenin/ifacemaker@f30b6f9bdbed4b5c4804ec9ba4a04a999525c202
	go generate ./...

# actionspin: 🤖 Bulk replace GitHub actions references from version tags to commit hashes
.PHONY: actionspin
actionspin:
	GOTOOLCHAIN=$(GOVERSION) go run github.com/mashiike/actionspin/cmd/actionspin@latest
