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

## markdown: 🎨 Find markdown format issues (Requires markdownlint-cli2)
.PHONY: markdown
markdown:
	@which markdownlint-cli2 > /dev/null || npm install -g markdownlint-cli2
	markdownlint-cli2 "**/*.md" "#vendor"

## lint: 🚨 Run lint checks
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || $(MAKE) install-lint
	golangci-lint run

## install-lint: 🛠 Install golangci-lint
.PHONY: install-lint
install-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b /usr/local/bin v1.64.7

## modernize: 🛠 Run gopls modernize
.PHONY: modernize
modernize:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix -test=false ./...

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

## generate: ⚡️ Generate msgp && interface implementations
.PHONY: generate
generate:
	go install github.com/tinylib/msgp@latest
	go install github.com/vburenin/ifacemaker@f30b6f9bdbed4b5c4804ec9ba4a04a999525c202
	go generate ./...
