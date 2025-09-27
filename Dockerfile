# Dockerfile for GoReleaser
# GoReleaser will provide the pre-built binaries
FROM alpine:3.19
ARG TARGETPLATFORM

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    nodejs \
    npm \
    curl \
    bash

COPY $TARGETPLATFORM/catalog-service $TARGETPLATFORM/payment-service $TARGETPLATFORM/cart-service $TARGETPLATFORM/shipping-service $TARGETPLATFORM/images-service $TARGETPLATFORM/recommendations-service /app/bin/

# Copy pre-built frontend (built by GoReleaser hook)
COPY frontend/dist /app/frontend/dist
COPY frontend/package*.json /app/frontend/
COPY frontend/vite.config.ts /app/frontend/
COPY frontend/vite.config.prod.ts /app/frontend/

# Copy static assets for image service
COPY services/images/static /app/services/images/static

# Install Vite for preview mode
WORKDIR /app/frontend
RUN npm install vite

# Copy entrypoint
WORKDIR /app
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Make all binaries executable
RUN chmod +x /app/bin/*

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:${PORT:-5173}/health || exit 1

EXPOSE 5173

ENTRYPOINT ["/app/entrypoint.sh"]
