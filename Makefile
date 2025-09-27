.PHONY: all test check clean catalog payment cart shipping images recommendations fmt lint install-tools

# Default target
all: catalog payment cart shipping images recommendations

# Output directory for binaries
BIN_DIR := ./bin

# Ensure bin directory exists
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Build individual services
catalog: $(BIN_DIR)
	@echo "Building Product Catalog Service..."
	@cd services/catalog && go build -o ../../$(BIN_DIR)/catalog-service .

payment: $(BIN_DIR)
	@echo "Building Payment Service..."
	@cd services/payment && go build -o ../../$(BIN_DIR)/payment-service .

cart: $(BIN_DIR)
	@echo "Building Cart Service..."
	@cd services/cart && go build -o ../../$(BIN_DIR)/cart-service .

shipping: $(BIN_DIR)
	@echo "Building Shipping Service..."
	@cd services/shipping && go build -o ../../$(BIN_DIR)/shipping-service .

images: $(BIN_DIR)
	@echo "Building Image Service..."
	@cd services/images && go build -o ../../$(BIN_DIR)/images-service .

recommendations: $(BIN_DIR)
	@echo "Building Recommendations Service..."
	@cd services/recommendations && go build -o ../../$(BIN_DIR)/recommendations-service .

# Run tests
test:
	@echo "Running tests for Common package..."
	@cd common && go test ./...
	@echo "Running tests for Product Catalog Service..."
	@cd services/catalog && go test ./...
	@echo "Running tests for Payment Service..."
	@cd services/payment && go test ./...
	@echo "Running tests for Cart Service..."
	@cd services/cart && go test ./...
	@echo "Running tests for Shipping Service..."
	@cd services/shipping && go test ./...
	@echo "Running tests for Image Service..."
	@cd services/images && go test ./...
	@echo "Running tests for Recommendations Service..."
	@cd services/recommendations && go test ./...

# Run integration tests
integration-test:
	@echo "Running integration tests..."
	@cd integration && go test -v ./...

# Format Go code
fmt:
	@echo "Formatting Go code..."
	@cd common && go fmt ./...
	@cd services/catalog && go fmt ./...
	@cd services/payment && go fmt ./...
	@cd services/cart && go fmt ./...
	@cd services/shipping && go fmt ./...
	@cd services/images && go fmt ./...
	@cd services/recommendations && go fmt ./...
	@cd integration && go fmt ./...

# Run linter
lint: $(BIN_DIR)/golangci-lint
	@echo "Running golangci-lint..."
	@cd common && ../$(BIN_DIR)/golangci-lint run ./... || true
	@cd services/catalog && ../../$(BIN_DIR)/golangci-lint run ./... || true
	@cd services/payment && ../../$(BIN_DIR)/golangci-lint run ./... || true
	@cd services/cart && ../../$(BIN_DIR)/golangci-lint run ./... || true
	@cd services/shipping && ../../$(BIN_DIR)/golangci-lint run ./... || true
	@cd services/images && ../../$(BIN_DIR)/golangci-lint run ./... || true
	@cd services/recommendations && ../../$(BIN_DIR)/golangci-lint run ./... || true
	@cd integration && ../$(BIN_DIR)/golangci-lint run ./... || true

# Install development tools
install-tools: $(BIN_DIR)/golangci-lint

$(BIN_DIR)/golangci-lint: $(BIN_DIR)
	@echo "Installing golangci-lint..."
	@GOBIN=$(shell pwd)/$(BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0

# Run all checks (formatting, tests, and linting)
check: fmt test lint

# Clean up binaries
clean:
	@echo "Cleaning up binaries..."
	@rm -rf $(BIN_DIR)

# Help target
help:
	@echo "Griffin Commerce Demo - Makefile targets:"
	@echo "  all                - Build all services (default)"
	@echo "  catalog            - Build Product Catalog Service"
	@echo "  payment            - Build Payment Service"
	@echo "  cart               - Build Cart Service"
	@echo "  shipping           - Build Shipping Service"
	@echo "  images             - Build Image Service"
	@echo "  recommendations    - Build Recommendations Service"
	@echo "  test               - Run all unit tests"
	@echo "  integration-test   - Run integration tests"
	@echo "  fmt                - Format all Go code with go fmt"
	@echo "  lint               - Run golangci-lint on all code"
	@echo "  check              - Run fmt, test, and lint"
	@echo "  install-tools      - Install development tools (golangci-lint)"
	@echo "  clean              - Remove all compiled binaries"
	@echo "  help               - Show this help message"