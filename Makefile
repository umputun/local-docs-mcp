# Get the latest commit branch, hash, and date
TAG=$(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
BRANCH=$(if $(TAG),$(TAG),$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null))
HASH=$(shell git rev-parse --short=7 HEAD 2>/dev/null)
TIMESTAMP=$(shell git log -1 --format=%ct HEAD 2>/dev/null | xargs -I{} date -u -r {} +%Y%m%dT%H%M%S)
GIT_REV=$(shell printf "%s-%s-%s" "$(BRANCH)" "$(HASH)" "$(TIMESTAMP)")
REV=$(if $(filter --,$(GIT_REV)),latest,$(GIT_REV)) # fallback to latest if not in git repo

all: test build

.PHONY: build
build: ## build binary
	cd cmd/local-docs-mcp && go build -ldflags "-X main.revision=$(REV) -s -w" -o ../../.bin/local-docs-mcp.$(BRANCH)
	cp .bin/local-docs-mcp.$(BRANCH) .bin/local-docs-mcp

.PHONY: install
install: ## install binary to GOPATH/bin
	go install -ldflags "-X main.revision=$(REV) -s -w" ./cmd/local-docs-mcp

.PHONY: test
test: ## run tests with race detector
	go clean -testcache
	go test -race -coverprofile=coverage.out ./...
	grep -v "_mock.go" coverage.out | grep -v mocks > coverage_no_mocks.out
	go tool cover -func=coverage_no_mocks.out
	rm coverage.out coverage_no_mocks.out

.PHONY: test-verbose
test-verbose: ## run tests with verbose output
	go test -v -race -cover ./...

.PHONY: coverage
coverage: ## generate coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: lint
lint: ## run linter
	golangci-lint run

.PHONY: fmt
fmt: ## format code
	~/.dot-files/claude/format.sh

.PHONY: clean
clean: ## remove build artifacts
	rm -rf .bin/
	rm -f coverage.out coverage.html coverage_no_mocks.out
	rm -f local-docs-mcp

.PHONY: deps
deps: ## download dependencies
	go mod download
	go mod tidy

.PHONY: run
run: ## run the server
	go run ./cmd/local-docs-mcp

.PHONY: bench
bench: ## run benchmarks
	go test -bench=. -benchmem ./...

.PHONY: version
version: ## show version info
	@echo "branch: $(BRANCH), hash: $(HASH), timestamp: $(TIMESTAMP)"
	@echo "revision: $(REV)"

.PHONY: help
help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
