.PHONY: test test-verbose test-integration test-integration-verbose install-tools build run clean help

# Default target
.DEFAULT_GOAL := help

DIST_DIR := dist

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

# Install testing tools
install-tools:
	@echo "Installing gotestsum..."
	go install gotest.tools/gotestsum@latest
	@echo "✓ gotestsum installed"

# Build the application
build:
	@echo "Building..."
	go build -o joydb ./cmd/joydb
	@echo "✓ Build complete: ./joydb"

# Run the application
repl: build
	./joydb

# Make server
server: build
	./joydb --server

build-linux: $(DIST_DIR)
	@echo "Building Linux binary..."
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/joydb-linux-amd64 ./cmd/joydb
	chmod +x $(DIST_DIR)/joydb-linux-amd64
	@echo "✓ Linux build complete"

build-windows: $(DIST_DIR)
	@echo "Building Windows binary..."
	GOOS=windows GOARCH=amd64 go build -o $(DIST_DIR)/joydb-windows-amd64.exe ./cmd/joydb
	@echo "✓ Windows build complete"


build-macos: $(DIST_DIR)
	@echo "Building macOS binary (arm64)..."
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/joydb-darwin-arm64 ./cmd/joydb
	chmod +x $(DIST_DIR)/joydb-darwin-arm64
	@echo "✓ macOS build complete"

build-all: build-linux build-windows build-macos
	@echo "✓ All platform builds complete"
# Run all tests with summary
test:
	@echo "Running all tests..."
	gotestsum --format testname -- ./...

# Run tests with verbose output
test-verbose:
	@echo "Running all tests (verbose)..."
	gotestsum --format standard-verbose -- ./...

# Run integration tests only
test-integration:
	@echo "Running integration tests..."
	gotestsum --format testname -- ./internal/integration_test/...

# Run integration tests with verbose output
test-integration-verbose:
	@echo "Running integration tests (verbose)..."
	gotestsum --format standard-verbose -- ./internal/integration_test/...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f joydb
	@echo "✓ Clean complete"

# Show help
help:
	@echo "JoyDB Makefile Commands:"
	@echo ""
	@echo "  make build                    - Build the application"
	@echo "  make run                      - Build and run the application"
	@echo "  make test                     - Run all tests with summary"
	@echo "  make test-verbose             - Run all tests with verbose output"
	@echo "  make test-integration         - Run integration tests with summary"
	@echo "  make test-integration-verbose - Run integration tests with verbose output"
	@echo "  make install-tools            - Install gotestsum and other tools"
	@echo "  make clean                    - Remove build artifacts"
	@echo "  make help                     - Show this help message"
