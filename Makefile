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

## docs-install: Install docs dependencies
docs-install:
	@echo "Installing dependencies..."
	@command -v python3 >/dev/null 2>&1 || { echo "Python 3 is required but not installed."; exit 1; }
	@echo "Dependencies checked ✓"

## docs-serve: Run local development server with hot reload
docs-serve: docs-clean
	@echo "Starting local development server with hot reload..."
	@echo "Checking if port 8000 is available..."
	@if lsof -Pi :8000 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "Port 8000 is already in use. Attempting to stop existing server..."; \
		pkill -f "python.*http.server.*8000" 2>/dev/null || true; \
		sleep 2; \
		if lsof -Pi :8000 -sTCP:LISTEN -t >/dev/null 2>&1; then \
			echo "Failed to free port 8000. Please manually stop the process using port 8000 and try again."; \
			echo "You can find the process with: lsof -Pi :8000"; \
			exit 1; \
		fi; \
	fi
	@echo "Setting up build directory structure..."
	@mkdir -p build
	@cp -r site/* build/
	@mkdir -p build/docs
	@cp -r docs/* build/docs/
	@echo "Starting server..."
	@(cd build && python3 -m http.server 8000 2>/dev/null) & \
	SERVER_PID=$$!; \
	sleep 1; \
	if ! kill -0 $$SERVER_PID 2>/dev/null; then \
		echo "Failed to start server. Port may still be in use."; \
		exit 1; \
	fi; \
	echo "Server running at http://localhost:8000"; \
	echo "Landing page: http://localhost:8000"; \
	echo "Documentation: http://localhost:8000/docs"; \
	echo "Watching for changes in site/ and docs/ directories..."; \
	echo "Press Ctrl+C to stop"; \
	trap 'echo "Stopping server..."; kill $$SERVER_PID 2>/dev/null; exit' INT TERM; \
	if command -v fswatch >/dev/null 2>&1; then \
		fswatch -o site docs | while read f; do \
			echo "Files changed, rebuilding..."; \
			cp -r site/* build/ 2>/dev/null || true; \
			cp -r docs/* build/docs/ 2>/dev/null || true; \
			echo "Rebuild complete"; \
		done; \
	elif command -v inotifywait >/dev/null 2>&1; then \
		while inotifywait -r -e modify,create,delete site docs >/dev/null 2>&1; do \
			echo "Files changed, rebuilding..."; \
			cp -r site/* build/ 2>/dev/null || true; \
			cp -r docs/* build/docs/ 2>/dev/null || true; \
			echo "Rebuild complete"; \
		done; \
	else \
		echo "No file watcher found. Install fswatch (brew install fswatch) or inotify-tools for hot reload."; \
		echo "Running without hot reload - server will continue until Ctrl+C"; \
		wait $$SERVER_PID; \
	fi

## docs-build: Build docs for production
docs-build: docs-clean
	@echo "Building site for production..."
	@mkdir -p build
	@echo "Copying landing page files..."
	@cp -r site/* build/
	@cp scripts/install.sh build/
	@echo "Copying documentation files..."
	@mkdir -p build/docs
	@cp -r docs/* build/docs/
	@echo "Creating .nojekyll file..."
	@touch build/.nojekyll
	@if [ -f "CNAME" ]; then cp CNAME build/; fi
	@echo "Build complete! Output in build/"

## docs-clean: Clean docs build directory
docs-clean:
	@echo "Cleaning build directory..."
	@rm -rf build
	@echo "Clean complete!"

## docs-stop: Stop any running docs servers
docs-stop:
	@echo "Stopping docs servers..."
	@pkill -f "python.*http.server.*8000" 2>/dev/null || echo "No docs servers found running on port 8000"

# Test the production build locally
deploy-test: build
	@echo "Testing production build..."
	@cd build && python3 -m http.server 8001

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