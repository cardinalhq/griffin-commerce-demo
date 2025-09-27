# Multi-stage Dockerfile for Griffin Commerce Demo
# Single image that can run any service based on SERVICE_NAME env var

# Stage 1: Build Go services
FROM golang:1.25-alpine AS go-builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy all source code first
COPY . .

# Download dependencies
RUN go mod download || true

# Build all services with CGO disabled
ENV CGO_ENABLED=0
RUN cd services/catalog && go build -o ../../bin/catalog-service . && \
    cd ../payment && go build -o ../../bin/payment-service . && \
    cd ../cart && go build -o ../../bin/cart-service . && \
    cd ../shipping && go build -o ../../bin/shipping-service . && \
    cd ../images && go build -o ../../bin/images-service . && \
    cd ../recommendations && go build -o ../../bin/recommendations-service .

# Stage 2: Build frontend
FROM node:24-alpine AS frontend-builder
WORKDIR /build

# Copy package files
COPY frontend/package*.json ./
RUN npm ci

# Copy frontend source
COPY frontend/ ./

# Build frontend for production
RUN npm run build

# Stage 3: Runtime image
FROM alpine:3.19
WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    nodejs \
    npm \
    curl \
    bash

# Copy Go binaries from builder
COPY --from=go-builder /build/bin/ /app/bin/

# Copy frontend build and source for serving
COPY --from=frontend-builder /build/dist /app/frontend/dist
COPY --from=frontend-builder /build/package*.json /app/frontend/
COPY --from=frontend-builder /build/vite.config.ts /app/frontend/
COPY --from=frontend-builder /build/vite.config.prod.ts /app/frontend/

# Copy static assets for image service
COPY services/images/static /app/services/images/static

# Install Vite for preview mode
WORKDIR /app/frontend
RUN npm install vite

# Create entrypoint script
WORKDIR /app
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:${PORT:-5173}/health || exit 1

# Default port (will be overridden per service)
EXPOSE 5173

# Run the entrypoint script
ENTRYPOINT ["/app/entrypoint.sh"]