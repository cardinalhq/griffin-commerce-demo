# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 CardinalHQ, Inc.

.PHONY: all build test check clean fmt lint install-tools images docker-build docker-push

# Default target
all: build

# Output directory for binaries
BIN_DIR := ./bin

# Ensure bin directory exists
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Build the Griffin CLI
build: $(BIN_DIR)
	@echo "Building Griffin CLI..."
	@go build -o $(BIN_DIR)/griffin .

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
	@echo "Running tests for DBaaS Service..."
	@cd services/dbaas && go test ./...

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
	@cd services/dbaas && go fmt ./...
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
	@cd services/dbaas && ../../$(BIN_DIR)/golangci-lint run ./... || true
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
	@echo "  all                - Build Griffin CLI (default)"
	@echo "  build              - Build Griffin CLI binary"
	@echo "  test               - Run all unit tests"
	@echo "  integration-test   - Run integration tests"
	@echo "  fmt                - Format all Go code with go fmt"
	@echo "  lint               - Run golangci-lint on all code"
	@echo "  check              - Run fmt, test, and lint"
	@echo "  install-tools      - Install development tools (golangci-lint)"
	@echo "  images             - Build and push Docker images to registry"
	@echo "  docker-build       - Build Docker image locally"
	@echo "  docker-push        - Push Docker image to registry"
	@echo "  clean              - Remove all compiled binaries"
	@echo "  help               - Show this help message"

# Docker configuration
REGISTRY := public.ecr.aws/cardinalhq.io
IMAGE_NAME := griffin-commerce-demo
TAG ?= latest
PLATFORMS := linux/arm64,linux/amd64

# Build and push multi-arch Docker images using GoReleaser
images:
	@echo "Building and pushing multi-arch Docker images using GoReleaser..."
	@if ! command -v goreleaser > /dev/null; then \
		echo "Installing GoReleaser..."; \
		go install github.com/goreleaser/goreleaser/v2@latest; \
	fi
	@goreleaser release --clean

# Build Docker images locally without pushing (test build)
docker-build:
	@echo "Building Docker images locally with GoReleaser (no push)..."
	@if ! command -v goreleaser > /dev/null; then \
		echo "Installing GoReleaser..."; \
		go install github.com/goreleaser/goreleaser/v2@latest; \
	fi
	@goreleaser release --snapshot --clean --skip=publish

# Push Docker image to registry
docker-push: docker-build
	@echo "Tagging and pushing image to registry..."
	@docker tag $(IMAGE_NAME):$(TAG) $(REGISTRY)/$(IMAGE_NAME):$(TAG)
	@docker tag $(IMAGE_NAME):$(TAG) $(REGISTRY)/$(IMAGE_NAME):latest
	@docker push $(REGISTRY)/$(IMAGE_NAME):$(TAG)
	@docker push $(REGISTRY)/$(IMAGE_NAME):latest