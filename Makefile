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

## format: 🎨 Find markdown format issues (Requires markdownlint-cli)
.PHONY: markdown
markdown:
	markdownlint-cli2 "**/*.md" "#vendor"

## lint: 🚨 Run lint checks
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60.1 run ./...

## test: 🚦 Execute all tests
.PHONY: test
test:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=1 -shuffle=on

## longtest: 🚦 Execute all tests 10x
.PHONY: longtest
longtest:
	go run gotest.tools/gotestsum@latest -f testname -- ./... -race -count=15 -shuffle=on

## tidy: 📌 Clean and tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy -v

## betteralign: 📐 Optimize alignment of fields in structs
.PHONY: betteralign
betteralign:
	go run github.com/dkorunic/betteralign/cmd/betteralign@latest -test_files -generated_files -apply ./...

## tidy: ⚡️ Generate msgp
.PHONY: msgp
msgp:
	go run github.com/tinylib/msgp@latest -file="middleware/cache/manager.go" -o="middleware/cache/manager_msgp.go" -tests=true -unexported
	go run github.com/tinylib/msgp@latest -file="middleware/session/data.go" -o="middleware/session/data_msgp.go" -tests=true -unexported
	go run github.com/tinylib/msgp@latest -file="middleware/csrf/storage_manager.go" -o="middleware/csrf/storage_manager_msgp.go" -tests=true -unexported
	go run github.com/tinylib/msgp@latest -file="middleware/limiter/manager.go" -o="middleware/limiter/manager_msgp.go" -tests=true -unexported
	go run github.com/tinylib/msgp@latest -file="middleware/idempotency/response.go" -o="middleware/idempotency/response_msgp.go" -tests=true -unexported
	go run github.com/tinylib/msgp@latest -file="redirect.go" -o="redirect_msgp.go" -tests=true -unexported
