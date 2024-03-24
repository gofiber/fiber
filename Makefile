## help: 💡 Print Help Menu
.PHONY: help
help:
	@echo '⚡️ GoFiber/Fiber Development:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## audit: 🚀 Quality Control
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## benchmark: 📈 Benchmarks
.PHONY: benchmark
benchmark:
	go test ./... -benchmem -bench=. -run=^Benchmark_$

## coverage: ☂️  Coverage Report
.PHONY: coverage
coverage:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -coverprofile=/tmp/coverage.out -covermode=atomic
	go tool cover -html=/tmp/coverage.out

## format: 🎨 Fix code format issues
.PHONY: format
format:
	go run mvdan.cc/gofumpt@latest -w -l .

## lint: 🚨 Run lint checks
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.1 run ./...

## test: 🚦 Test Suite
.PHONY: test
test:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -shuffle=on

## tidy: 📌 Tidy modfile
.PHONY: tidy
tidy:
	go mod tidy -v
