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
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## benchmark: 📈 Benchmark code performance
.PHONY: benchmark
benchmark:
	go test ./... -benchmem -bench=. -run=^Benchmark_$

## coverage: ☂️  Generate coverage report
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
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1 run ./...

## test: 🚦 Execute all tests
.PHONY: test
test:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -shuffle=on

## tidy: 📌 Clean and tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy -v
