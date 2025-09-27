.PHONY: all test check clean catalog payment cart shipping images recommendations

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

# Alias for test
check: test

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
	@echo "  test               - Run all tests"
	@echo "  check              - Alias for test"
	@echo "  clean              - Remove all compiled binaries"
	@echo "  help               - Show this help message"