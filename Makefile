# Pommel Makefile

# Variables
BINARY_CLI = pm
BINARY_DAEMON = pommeld
BUILD_DIR = bin
GO = go
GOFLAGS = -trimpath
TAGS = fts5
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

# Default target
.PHONY: all
all: build

# Build both binaries
.PHONY: build
build: build-cli build-daemon

.PHONY: build-cli
build-cli:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_CLI) ./cmd/pm

.PHONY: build-daemon
build-daemon:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_DAEMON) ./cmd/pommeld

# Run tests
.PHONY: test
test:
	$(GO) test -tags "$(TAGS)" -v -race -cover ./...

# Run tests with coverage report
.PHONY: coverage
coverage:
	$(GO) test -tags "$(TAGS)" -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Format code
.PHONY: fmt
fmt:
	$(GO) fmt ./...
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(GO) clean -cache -testcache

# Install binaries to GOPATH/bin
.PHONY: install
install:
	$(GO) install -tags "$(TAGS)" ./cmd/pm
	$(GO) install -tags "$(TAGS)" ./cmd/pommeld

# Run the CLI (for development)
.PHONY: run-cli
run-cli:
	$(GO) run -tags "$(TAGS)" ./cmd/pm $(ARGS)

# Run the daemon (for development)
.PHONY: run-daemon
run-daemon:
	$(GO) run -tags "$(TAGS)" ./cmd/pommeld $(ARGS)

# Tidy dependencies
.PHONY: tidy
tidy:
	$(GO) mod tidy

# Download dependencies
.PHONY: deps
deps:
	$(GO) mod download

# Show help
.PHONY: help
help:
	@echo "Pommel Build Targets:"
	@echo "  make build        - Build both binaries"
	@echo "  make build-cli    - Build CLI only"
	@echo "  make build-daemon - Build daemon only"
	@echo "  make test         - Run tests with race detection"
	@echo "  make coverage     - Generate coverage report"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make fmt          - Format code"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make install      - Install binaries to GOPATH/bin"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make deps         - Download dependencies"
