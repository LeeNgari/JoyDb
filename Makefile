.PHONY: test test-verbose test-integration test-integration-verbose install-tools build run clean help

# Default target
.DEFAULT_GOAL := help

# Install testing tools
install-tools:
	@echo "Installing gotestsum..."
	go install gotest.tools/gotestsum@latest
	@echo "✓ gotestsum installed"

# Build the application
build:
	@echo "Building..."
	go build -o rdbms ./cmd/rdbms
	@echo "✓ Build complete: ./rdbms"

# Run the application
repl: build
	./rdbms

# Make server
server: build
	./rdbms --server
	
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
	rm -f rdbms
	@echo "✓ Clean complete"

# Show help
help:
	@echo "RDBMS Makefile Commands:"
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
