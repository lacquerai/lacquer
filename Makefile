.PHONY: all build test lint clean install release-local docs help

# Variables
BINARY_NAME=laq
MAIN_PATH=./cmd/laq
BUILD_DIR=bin
INSTALL_PATH=/usr/local/bin
GO_VERSION=1.24.1

# Version information
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-w -s -X github.com/lacquerai/lacquer/internal/version.Version=$(VERSION) -X github.com/lacquerai/lacquer/internal/version.Commit=$(COMMIT) -X github.com/lacquerai/lacquer/internal/version.Date=$(DATE)"

# Default target
all: lint test build

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

## build-all: Build binaries for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

## test: Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

## test-coverage: Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## benchmark: Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

## lint: Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint not installed. Install it from https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	go mod tidy

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

## install: Install binary to system
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete. Run '$(BINARY_NAME) --help' to get started."

## uninstall: Remove binary from system
uninstall:
	@echo "Removing $(BINARY_NAME) from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstallation complete."

## release-local: Create a local release using goreleaser
release-local:
	@echo "Creating local release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean --skip=publish; \
	else \
		echo "goreleaser not installed. Install it from https://goreleaser.com/install/"; \
		exit 1; \
	fi

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t ghcr.io/lacquerai/lacquer:local .

## docker-run: Run Docker container
docker-run: docker-build
	@echo "Running Docker container..."
	docker run --rm -it ghcr.io/lacquerai/lacquer:local

## docs-serve: Serve documentation locally (requires Hugo)
docs-serve:
	@echo "Serving documentation..."
	@if command -v hugo >/dev/null 2>&1; then \
		hugo server -D; \
	else \
		echo "Hugo not installed. Install it from https://gohugo.io/installation/"; \
		exit 1; \
	fi

## docs-build: Build documentation (requires Hugo)
docs-build:
	@echo "Building documentation..."
	@if command -v hugo >/dev/null 2>&1; then \
		hugo --minify; \
	else \
		echo "Hugo not installed. Install it from https://gohugo.io/installation/"; \
		exit 1; \
	fi

## deps: Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

## deps-update: Update dependencies
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

## verify: Verify dependencies
verify:
	@echo "Verifying dependencies..."
	go mod verify

## dev: Run in development mode with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Install it with: go install github.com/air-verse/air@latest"; \
		exit 1; \
	fi