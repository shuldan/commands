.PHONY: lint fmt fmt-check test test-coverage vet all ci install-tools

GOBIN := $(shell go env GOPATH)/bin

GOLANGCI_LINT := $(GOBIN)/golangci-lint
GOIMPORTS := $(GOBIN)/goimports
GOSEC := $(GOBIN)/gosec

GOLANGCI_LINT_VERSION := v2.4.0
GOSEC_VERSION := v2.22.4

install-tools:
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)
	@echo "Installing goimports..."
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "Installing gosec $(GOSEC_VERSION)..."
	@go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)

$(GOIMPORTS):
	@echo "Installing goimports..."
	@go install golang.org/x/tools/cmd/goimports@latest

$(GOSEC):
	@echo "Installing gosec $(GOSEC_VERSION)..."
	@go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)

lint: $(GOLANGCI_LINT)
	@echo "Running golangci-lint..."
	$(GOLANGCI_LINT) run --config .golangci-lint.yaml

security: $(GOSEC)
	@echo "Running gosec security scanner..."
	$(GOSEC) -exclude-dir="test" ./...

fmt: $(GOIMPORTS)
	@echo "Running go fmt and goimports..."
	@find . -name "*.go" -not -path "./vendor/*" -exec gofmt -s -w {} \;
	@$(GOIMPORTS) -local github.com/shuldan/commands -w $$(find . -name "*.go" -not -path "./vendor/*")

fmt-check: $(GOIMPORTS)
	@echo "Checking code formatting..."
	@unformatted=$$(gofmt -s -l . | grep -v vendor | grep .go || true); \
	if [ -n "$$unformatted" ]; then \
		echo "❌ Unformatted files found:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@unformatted_imports=$$($(GOIMPORTS) -local github.com/shuldan/commands -l . | grep -v vendor || true); \
	if [ -n "$$unformatted_imports" ]; then \
		echo "❌ Unformatted imports:"; \
		echo "$$unformatted_imports"; \
		exit 1; \
	fi
	@echo "✅ All files are properly formatted"

vet:
	@echo "Running go vet..."
	@go vet ./...

test:
	@echo "Running tests..."
	@go test -race -count=1 -timeout 60s ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@go tool cover -func=coverage.out

bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./... -run=^$$

all: fmt-check vet lint security test

ci: fmt-check vet lint test-coverage
	@echo "✅ All CI checks passed."

clean:
	@echo "Cleaning..."
	@go clean -testcache
	@rm -f coverage.out coverage.html

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@go mod tidy

help:
	@echo "Available targets:"
	@echo "  install-tools  - Install required tools"
	@echo "  fmt           - Format code"
	@echo "  fmt-check     - Check code formatting"
	@echo "  lint          - Run linter"
	@echo "  security      - Run security scanner"
	@echo "  vet           - Run go vet"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  bench         - Run benchmarks"
	@echo "  all           - Run all checks (local)"
	@echo "  ci            - Run CI checks"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download and verify dependencies"
	@echo "  help          - Show this help"
